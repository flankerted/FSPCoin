// Copyright 2014 The go-contatract Authors
// This file is part of the go-contatract library.
//
// The go-contatract library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-contatract library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-contatract library. If not, see <http://www.gnu.org/licenses/>.

// Package state provides a caching layer atop the Ethereum state trie.
package state

import (
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"sync"

	"strconv"

	"github.com/contatract/go-contatract/blizcore"
	"github.com/contatract/go-contatract/blizparam"
	"github.com/contatract/go-contatract/blizzard/storage"
	"github.com/contatract/go-contatract/blizzard/storagecore"
	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/core/types"
	"github.com/contatract/go-contatract/crypto"
	"github.com/contatract/go-contatract/elephant/exporter"
	"github.com/contatract/go-contatract/event"
	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/p2p/discover"
	"github.com/contatract/go-contatract/rlp"
	"github.com/contatract/go-contatract/trie"
)

const (
	IdleSliceCountMin = 100
	collectChunkMax   = 200
)

const (
	CWriteSliceHeader = iota
	CChunkPartners
	CElephantToCsData
	CEleHeight
	CEthHeight
	CElephantToBlizcsSliceInfo
	CFarmerAward
	CBuffFromSharerCS
	CCheckClaimFile
	CCheckClaimFileEnd
	CMax
)

type revision struct {
	id           int
	journalIndex int
}

var (
	// emptyState is the known hash of an empty state trie entry.
	emptyState = crypto.Keccak256Hash(nil)

	// emptyCode is the known hash of the empty EVM bytecode.
	emptyCode = crypto.Keccak256Hash(nil)
)

type ShareObjData struct {
	Owner          common.Address
	ObjID          uint64
	Offset         uint64
	Length         uint64
	StartTime      string
	EndTime        string
	FileName       string
	SharerCSNodeID string
	Key            string
}

type SRegData struct {
	Name   string
	PubKey []byte
}

func (s *SRegData) deepCopy() *SRegData {
	ret := &SRegData{
		Name:   s.Name,
		PubKey: make([]byte, len(s.PubKey)),
	}
	copy(ret.PubKey, s.PubKey)
	return ret
}

// StateDBs within the ethereum protocol are used to store anything
// within the merkle trie. StateDBs take care of caching and storing
// nested states. It's the general query interface to retrieve:
// * Contracts
// * Accounts
type StateDB struct {
	db   Database
	trie Trie

	// This map holds 'live' objects, which will get modified while processing a state transition.
	stateObjects      map[common.Address]*stateObject
	stateObjectsDirty map[common.Address]struct{}

	stateChunks      map[uint64]*stateChunk
	stateChunksDirty map[uint64]struct{}
	chunkMaxId       uint64

	stateSlices      map[common.Address]*stateSlice
	stateSlicesDirty map[common.Address]struct{}

	// Having already received elepant height
	shardingHeight map[uint16]uint64
	// Having already received batch nonce of sharding tx
	batchNonce map[uint16]uint64
	// Share obj data
	shareObj map[common.Address][]*ShareObjData
	// Mail data,  address is receiver of the mail
	mailData map[common.Address][]*common.SDBMailData
	// Reg data
	regData map[common.Address]*SRegData
	// Nickname
	nicknames map[string]struct{}
	// Verifying award
	verifyAward map[common.Address]uint64

	// stateClaims      map[common.Address]*stateClaim // 暂时没用
	// stateClaimsDirty map[common.Address]struct{}    // 暂时没用

	// DB error.
	// State objects are used by the consensus core and VM which are
	// unable to deal with database-level errors. Any error that occurs
	// during a database read is memoized here and will eventually be returned
	// by StateDB.Commit.
	dbErr error

	// The refund counter, also used by state transitioning.
	refund uint64

	thash, bhash common.Hash
	txIndex      int
	logs         map[common.Hash][]*types.Log
	logSize      uint

	preimages map[common.Hash][]byte

	// JiangHan: state状态记录对象，这个对象记录所有对statedb中的对象改变
	// 这样方便在快照回滚的时候对操作进行undo,这个 journal 跟 tx_journal
	// 不同，tx_journal只是一个日志文件，记录本地账户的交易操作
	// Journal of state modifications. This is the backbone of
	// Snapshot and RevertToSnapshot.
	journal journal

	validRevisions []revision
	nextRevisionId int

	lock sync.Mutex

	feeds   []*event.Feed
	elebase *common.Address
}

func stateDebug(msg string, ctx ...interface{}) {
	log.Info(msg, ctx...)
}

func logVerify(ctx ...interface{}) {
	fmt.Println(ctx...)
}

// Create a new state from a given trie
func New(root common.Hash, db Database, feeds []*event.Feed, elebase *common.Address) (*StateDB, error) {
	tr, err := db.OpenTrie(root)
	if err != nil {
		return nil, err
	}
	chunkMaxId := getChunkMaxId(tr)
	return &StateDB{
		db:                db,
		trie:              tr,
		stateObjects:      make(map[common.Address]*stateObject),
		stateObjectsDirty: make(map[common.Address]struct{}),
		logs:              make(map[common.Hash][]*types.Log),
		preimages:         make(map[common.Hash][]byte),
		stateChunks:       make(map[uint64]*stateChunk),
		stateChunksDirty:  make(map[uint64]struct{}),
		stateSlices:       make(map[common.Address]*stateSlice),
		stateSlicesDirty:  make(map[common.Address]struct{}),
		chunkMaxId:        chunkMaxId,
		// stateClaims:       make(map[common.Address]*stateClaim),
		// stateClaimsDirty:  make(map[common.Address]struct{}),
		shardingHeight: make(map[uint16]uint64),
		batchNonce:     make(map[uint16]uint64),
		feeds:          feeds,
		elebase:        elebase,
		shareObj:       make(map[common.Address][]*ShareObjData),
		mailData:       make(map[common.Address][]*common.SDBMailData),
		regData:        make(map[common.Address]*SRegData),
		nicknames:      make(map[string]struct{}),
		verifyAward:    make(map[common.Address]uint64),
	}, nil
}

// setError remembers the first non-nil error it is called with.
func (self *StateDB) setError(err error) {
	if self.dbErr == nil {
		self.dbErr = err
	}
}

func (self *StateDB) Error() error {
	return self.dbErr
}

// Reset clears out all emphemeral state objects from the state db, but keeps
// the underlying state trie to avoid reloading data for the next operations.
func (self *StateDB) Reset(root common.Hash) error {
	tr, err := self.db.OpenTrie(root)
	if err != nil {
		return err
	}
	self.trie = tr
	self.stateObjects = make(map[common.Address]*stateObject)
	self.stateObjectsDirty = make(map[common.Address]struct{})
	self.thash = common.Hash{}
	self.bhash = common.Hash{}
	self.txIndex = 0
	self.logs = make(map[common.Hash][]*types.Log)
	self.logSize = 0
	self.preimages = make(map[common.Hash][]byte)
	self.clearJournalAndRefund()
	self.stateChunks = make(map[uint64]*stateChunk)
	self.stateChunksDirty = make(map[uint64]struct{})
	self.stateSlices = make(map[common.Address]*stateSlice)
	self.stateSlicesDirty = make(map[common.Address]struct{})
	// self.stateClaims = make(map[common.Address]*stateClaim)
	// self.stateClaimsDirty = make(map[common.Address]struct{})
	self.shardingHeight = make(map[uint16]uint64)
	self.batchNonce = make(map[uint16]uint64)
	self.shareObj = make(map[common.Address][]*ShareObjData)
	self.mailData = make(map[common.Address][]*common.SDBMailData)
	self.regData = make(map[common.Address]*SRegData)
	self.nicknames = make(map[string]struct{})
	self.verifyAward = make(map[common.Address]uint64)
	return nil
}

func (self *StateDB) AddLog(log *types.Log) {
	self.journal = append(self.journal, addLogChange{txhash: self.thash})

	log.TxHash = self.thash
	log.BlockHash = self.bhash
	log.TxIndex = uint(self.txIndex)
	log.Index = self.logSize
	self.logs[self.thash] = append(self.logs[self.thash], log)
	self.logSize++
}

func (self *StateDB) GetLogs(hash common.Hash) []*types.Log {
	return self.logs[hash]
}

func (self *StateDB) Logs() []*types.Log {
	var logs []*types.Log
	for _, lgs := range self.logs {
		logs = append(logs, lgs...)
	}
	return logs
}

// AddPreimage records a SHA3 preimage seen by the VM.
func (self *StateDB) AddPreimage(hash common.Hash, preimage []byte) {
	if _, ok := self.preimages[hash]; !ok {
		self.journal = append(self.journal, addPreimageChange{hash: hash})
		pi := make([]byte, len(preimage))
		copy(pi, preimage)
		self.preimages[hash] = pi
	}
}

// Preimages returns a list of SHA3 preimages that have been submitted.
func (self *StateDB) Preimages() map[common.Hash][]byte {
	return self.preimages
}

func (self *StateDB) AddRefund(gas uint64) {
	self.journal = append(self.journal, refundChange{prev: self.refund})
	self.refund += gas
}

// Exist reports whether the given account address exists in the state.
// Notably this also returns true for suicided accounts.
func (self *StateDB) Exist(addr common.Address) bool {
	return self.getStateObject(addr) != nil
}

// Empty returns whether the state object is either non-existent
// or empty according to the EIP161 specification (balance = nonce = code = 0)
func (self *StateDB) Empty(addr common.Address) bool {
	so := self.getStateObject(addr)
	return so == nil || so.empty()
}

// Retrieve the balance from the given address or 0 if object not found
func (self *StateDB) GetBalance(addr common.Address) *big.Int {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return stateObject.Balance()
	}
	return common.Big0
}

func convertCSAuthData(cs *common.CSAuthData, csAddr common.Address) common.WalletCSAuthData {
	var data common.WalletCSAuthData
	data.CsAddr = csAddr
	data.StartTime = cs.StartTime
	data.EndTime = cs.EndTime
	data.Flow = cs.Flow
	data.CSPickupTime = cs.CSPickupTime
	data.AuthAllowFlow = cs.AuthAllowFlow
	data.CSPickupFlow = cs.CSPickupFlow
	data.PayMethod = cs.PayMethod
	return data
}

// Retrieve the signed CS from the given address
func (self *StateDB) GetSignedCs(addr, curCSAddr common.Address) []common.WalletCSAuthData {
	var css []common.WalletCSAuthData
	addresses := self.GetSignCsAddr(addr)
	for _, address := range addresses {
		if !address.Equal(curCSAddr) {
			continue
		}
		tmps := self.GetDetailedSignedCs(address, addr, curCSAddr)
		for _, tmp := range tmps {
			css = append(css, tmp)
		}
	}
	return css
}

func (self *StateDB) GetDetailedSignedCs(csAddr common.Address, clientAddr common.Address, curCSAddr common.Address) []common.WalletCSAuthData {
	var css []common.WalletCSAuthData
	ads := self.GetServerCliAddr(csAddr)
	for _, ad := range ads {
		if ad.Authorizer != clientAddr {
			continue
		}
		css = append(css, convertCSAuthData(&ad, csAddr))
	}
	return css
}

/*
func (self *StateDB) GetSignedCs(addr common.Address) string {
	ret := `{"csArray": [{}`
	addresses := self.GetSignCsAddr(addr)
	if addresses == nil {
		log.Info("GetSignedCs addresses nil")
		return ret
	}
	for i, address := range addresses {
		if i == 0 {
			ret = `{"csArray": [`
		} else if i != len(addresses)-1 {
			ret += ", "
		}
		ret += self.GetDetailedSignedCs(address, addr)
	}
	ret += "]}"
	// log.Info("GetSignedCs", "ret", ret)
	return ret
}

func (self *StateDB) GetDetailedSignedCs(csAddr common.Address, clientAddr common.Address) string {
	ret := "{}"
	ads := self.GetServerCliAddr(csAddr)
	if ads == nil || len(ads) == 0 {
		log.Info("GetDetailedSignedCs ads nil")
		return ret
	}
	// log.Info("GetDetailedSignedCs", "ads len", len(ads))
	for _, ad := range ads {
		if ad.Authorizer != clientAddr {
			continue
		}
		start := common.GetTimeString(ad.StartTime)
		end := common.GetTimeString(ad.EndTime)
		authAllowTime := common.GetTimeString(0)
		csPickupTime := common.GetTimeString(ad.CSPickupTime)
		usedFlow := uint64(0)
		ret = fmt.Sprintf("{\"csAddr\": \"%v\", \"start\": \"%v\", \"end\": \"%v\", \"flow\": \"%v\", "+
			"\"authAllowTime\": \"%v\", \"csPickupTime\": \"%v\", \"authAllowFlow\": \"%v\", \"csPickupFlow\": \"%v\", "+
			"\"usedFlow\": \"%v\", \"payMethod\": \"%v\"}",
			csAddr.Hex(), start, end, ad.Flow, authAllowTime, csPickupTime, ad.AuthAllowFlow, ad.CSPickupFlow, usedFlow, ad.PayMethod)
		break
	}
	return ret
}
*/
func (self *StateDB) GetNonce(addr common.Address) uint64 {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return stateObject.Nonce()
	}

	return 0
}

func (self *StateDB) GetCode(addr common.Address) []byte {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return stateObject.Code(self.db)
	}
	return nil
}

func (self *StateDB) GetCodeSize(addr common.Address) int {
	stateObject := self.getStateObject(addr)
	if stateObject == nil {
		return 0
	}
	if stateObject.code != nil {
		return len(stateObject.code)
	}
	size, err := self.db.ContractCodeSize(stateObject.addrHash, common.BytesToHash(stateObject.CodeHash()))
	if err != nil {
		self.setError(err)
	}
	return size
}

func (self *StateDB) GetCodeHash(addr common.Address) common.Hash {
	stateObject := self.getStateObject(addr)
	if stateObject == nil {
		return common.Hash{}
	}
	return common.BytesToHash(stateObject.CodeHash())
}

func (self *StateDB) GetState(a common.Address, b common.Hash) common.Hash {
	stateObject := self.getStateObject(a)
	if stateObject != nil {
		return stateObject.GetState(self.db, b)
	}
	return common.Hash{}
}

// Database retrieves the low level database supporting the lower level trie ops.
func (self *StateDB) Database() Database {
	return self.db
}

// StorageTrie returns the storage trie of an account.
// The return value is a copy and is nil for non-existent accounts.
func (self *StateDB) StorageTrie(a common.Address) Trie {
	stateObject := self.getStateObject(a)
	if stateObject == nil {
		return nil
	}
	cpy := stateObject.deepCopy(self, nil)
	return cpy.updateTrie(self.db)
}

func (self *StateDB) HasSuicided(addr common.Address) bool {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return stateObject.suicided
	}
	return false
}

/*
 * SETTERS
 */

// AddBalance adds amount to the account associated with addr
func (self *StateDB) AddBalance(addr common.Address, amount *big.Int) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.AddBalance(amount)
	}
}

// SubBalance subtracts amount from the account associated with addr
func (self *StateDB) SubBalance(addr common.Address, amount *big.Int) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SubBalance(amount)
	}
}

func (self *StateDB) SetBalance(addr common.Address, amount *big.Int) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetBalance(amount)
	}
}

func (self *StateDB) SetNonce(addr common.Address, nonce uint64) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetNonce(nonce)
	}
}

func (self *StateDB) SetCode(addr common.Address, code []byte) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetCode(crypto.Keccak256Hash(code), code)
	}
}

func (self *StateDB) SetState(addr common.Address, key common.Hash, value common.Hash) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetState(self.db, key, value)
	}
}

// Suicide marks the given account as suicided.
// This clears the account balance.
//
// The account's state object is still available until the state is committed,
// getStateObject will return a non-nil account after Suicide.
func (self *StateDB) Suicide(addr common.Address) bool {
	stateObject := self.getStateObject(addr)
	if stateObject == nil {
		return false
	}
	self.journal = append(self.journal, suicideChange{
		account:     &addr,
		prev:        stateObject.suicided,
		prevbalance: new(big.Int).Set(stateObject.Balance()),
	})
	stateObject.markSuicided()
	stateObject.data.Balance = new(big.Int)

	return true
}

//
// Setting, updating & deleting state object methods
//

// updateStateObject writes the given object to the trie.
func (self *StateDB) updateStateObject(stateObject *stateObject) {
	addr := stateObject.Address()
	data, err := rlp.EncodeToBytes(stateObject)
	if err != nil {
		panic(fmt.Errorf("can't encode object at %x: %v", addr[:], err))
	}
	self.setError(self.trie.TryUpdate(addr[:], data))
}

// deleteStateObject removes the given object from the state trie.
func (self *StateDB) deleteStateObject(stateObject *stateObject) {
	stateObject.deleted = true
	addr := stateObject.Address()
	self.setError(self.trie.TryDelete(addr[:]))
}

// Retrieve a state object given my the address. Returns nil if not found.
func (self *StateDB) getStateObject(addr common.Address) (stateObject *stateObject) {
	// Prefer 'live' objects.
	if obj := self.stateObjects[addr]; obj != nil {
		if obj.deleted {
			return nil
		}
		return obj
	}

	// Load the object from the database.
	enc, err := self.trie.TryGet(addr[:])
	if len(enc) == 0 {
		self.setError(err)
		return nil
	}
	var data Account
	if err := rlp.DecodeBytes(enc, &data); err != nil {
		log.Error("Failed to decode state object", "addr", addr, "err", err)
		return nil
	}
	// Insert into the live set.
	obj := newObject(self, addr, data, self.MarkStateObjectDirty)
	self.setStateObject(obj)
	return obj
}

func (self *StateDB) setStateObject(object *stateObject) {
	self.stateObjects[object.Address()] = object
}

// Retrieve a state object or create a new state object if nil
func (self *StateDB) GetOrNewStateObject(addr common.Address) *stateObject {
	stateObject := self.getStateObject(addr)
	if stateObject == nil || stateObject.deleted {
		stateObject, _ = self.createObject(addr)
	}
	return stateObject
}

// JiangHan: struct{}是0值，而struct{}{}是一个struct{}的实例，它就不是0了。。
// 于是就有了dirty一说
// struct{}{} // not the zero value, a real new struct{} instance
// MarkStateObjectDirty adds the specified object to the dirty map to avoid costly
// state object cache iteration to find a handful of modified ones.
func (self *StateDB) MarkStateObjectDirty(addr common.Address) {
	self.stateObjectsDirty[addr] = struct{}{}
}

// createObject creates a new state object. If there is an existing account with
// the given address, it is overwritten and returned as the second return value.
func (self *StateDB) createObject(addr common.Address) (newobj, prev *stateObject) {
	prev = self.getStateObject(addr)
	newobj = newObject(self, addr, Account{}, self.MarkStateObjectDirty)
	newobj.setNonce(0) // sets the object to dirty
	if prev == nil {
		self.journal = append(self.journal, createObjectChange{account: &addr})
	} else {
		self.journal = append(self.journal, resetObjectChange{prev: prev})
	}
	self.setStateObject(newobj)
	return newobj, prev
}

// CreateAccount explicitly creates a state object. If a state object with the address
// already exists the balance is carried over to the new account.
//
// CreateAccount is called during the EVM CREATE operation. The situation might arise that
// a contract does the following:
//
//   1. sends funds to sha(account ++ (nonce + 1))
//   2. tx_create(sha(account ++ nonce)) (note that this gets the address of 1)
//
// Carrying over the balance ensures that Ether doesn't disappear.
func (self *StateDB) CreateAccount(addr common.Address) {
	new, prev := self.createObject(addr)
	if prev != nil {
		new.setBalance(prev.data.Balance)
	}
}

func (db *StateDB) ForEachStorage(addr common.Address, cb func(key, value common.Hash) bool) {
	so := db.getStateObject(addr)
	if so == nil {
		return
	}

	// When iterating over the storage check the cache first
	for h, value := range so.cachedStorage {
		cb(h, value)
	}

	it := trie.NewIterator(so.getTrie(db.db).NodeIterator(nil))
	for it.Next() {
		// ignore cached values
		key := common.BytesToHash(db.trie.GetKey(it.Key))
		if _, ok := so.cachedStorage[key]; !ok {
			cb(key, common.BytesToHash(it.Value))
		}
	}
}

// Copy creates a deep, independent copy of the state.
// Snapshots of the copied state cannot be applied to the copy.
func (self *StateDB) Copy() *StateDB {
	self.lock.Lock()
	defer self.lock.Unlock()

	// Copy all the basic fields, initialize the memory ones
	state := &StateDB{
		db:                self.db,
		trie:              self.db.CopyTrie(self.trie),
		stateObjects:      make(map[common.Address]*stateObject, len(self.stateObjectsDirty)),
		stateObjectsDirty: make(map[common.Address]struct{}, len(self.stateObjectsDirty)),
		refund:            self.refund,
		logs:              make(map[common.Hash][]*types.Log, len(self.logs)),
		logSize:           self.logSize,
		preimages:         make(map[common.Hash][]byte),
		stateChunks:       make(map[uint64]*stateChunk, len(self.stateChunksDirty)),
		stateChunksDirty:  make(map[uint64]struct{}, len(self.stateChunksDirty)),
		stateSlices:       make(map[common.Address]*stateSlice, len(self.stateSlicesDirty)),
		stateSlicesDirty:  make(map[common.Address]struct{}, len(self.stateSlicesDirty)),
		// stateClaims:       make(map[common.Address]*stateClaim, len(self.stateClaimsDirty)),
		// stateClaimsDirty:  make(map[common.Address]struct{}, len(self.stateClaimsDirty)),
		shardingHeight: make(map[uint16]uint64, len(self.shardingHeight)),
		batchNonce:     make(map[uint16]uint64, len(self.batchNonce)),
		shareObj:       make(map[common.Address][]*ShareObjData, len(self.shareObj)),
		mailData:       make(map[common.Address][]*common.SDBMailData, len(self.mailData)),
		regData:        make(map[common.Address]*SRegData, len(self.regData)),
		nicknames:      make(map[string]struct{}),
		verifyAward:    make(map[common.Address]uint64),
		feeds:          self.feeds,
	}
	// Copy the dirty states, logs, and preimages
	for addr := range self.stateObjectsDirty {
		state.stateObjects[addr] = self.stateObjects[addr].deepCopy(state, state.MarkStateObjectDirty)
		state.stateObjectsDirty[addr] = struct{}{}
	}
	for hash, logs := range self.logs {
		state.logs[hash] = make([]*types.Log, len(logs))
		copy(state.logs[hash], logs)
	}
	for hash, preimage := range self.preimages {
		state.preimages[hash] = preimage
	}
	for addr := range self.stateChunksDirty {
		state.stateChunks[addr] = self.stateChunks[addr].deepCopy(state, state.MarkStateChunkDirty)
		state.stateChunksDirty[addr] = struct{}{}
	}
	for addr := range self.stateSlicesDirty {
		state.stateSlices[addr] = self.stateSlices[addr].deepCopy(state, state.MarkStateSliceDirty)
		state.stateSlicesDirty[addr] = struct{}{}
	}
	// log.Info("", "self.stateClaimsDirty len", len(self.stateClaimsDirty))
	// for addr := range self.stateClaimsDirty {
	// 	state.stateClaims[addr] = self.stateClaims[addr].deepCopy(state, state.MarkStateClaimDirty)
	// 	state.stateClaimsDirty[addr] = struct{}{}
	// }
	for i, v := range self.shardingHeight {
		state.shardingHeight[i] = v
	}
	for i, v := range self.batchNonce {
		state.batchNonce[i] = v
	}
	for i, v := range self.shareObj {
		state.shareObj[i] = deepCopyShareObjDatas(v)
	}
	for i, v := range self.mailData {
		state.mailData[i] = deepCopySDBMailDatas(v)
	}
	for i, v := range self.regData {
		state.regData[i] = v.deepCopy()
	}
	return state
}

// Snapshot returns an identifier for the current revision of the state.
func (self *StateDB) Snapshot() int {
	id := self.nextRevisionId
	self.nextRevisionId++
	self.validRevisions = append(self.validRevisions, revision{id, len(self.journal)})
	return id
}

// RevertToSnapshot reverts all state changes made since the given revision.
func (self *StateDB) RevertToSnapshot(revid int) {
	// Find the snapshot in the stack of valid snapshots.
	idx := sort.Search(len(self.validRevisions), func(i int) bool {
		return self.validRevisions[i].id >= revid
	})
	if idx == len(self.validRevisions) || self.validRevisions[idx].id != revid {
		panic(fmt.Errorf("revision id %v cannot be reverted", revid))
	}
	snapshot := self.validRevisions[idx].journalIndex

	// JiangHan: 快照回复，将snapshot点之后的变化日志都 undo 回去
	// Replay the journal to undo changes.
	for i := len(self.journal) - 1; i >= snapshot; i-- {
		self.journal[i].undo(self)
	}
	self.journal = self.journal[:snapshot]

	// Remove invalidated snapshots from the stack.
	self.validRevisions = self.validRevisions[:idx]
}

// GetRefund returns the current value of the refund counter.
func (self *StateDB) GetRefund() uint64 {
	return self.refund
}

// Finalise finalises the state by removing the self destructed objects
// and clears the journal as well as the refunds.
func (s *StateDB) Finalise(deleteEmptyObjects bool) {
	for addr := range s.stateObjectsDirty {
		stateObject := s.stateObjects[addr]
		//if stateObject.suicided || (deleteEmptyObjects && stateObject.empty()) {
		if stateObject.suicided || stateObject.empty() {
			s.deleteStateObject(stateObject)
		} else {
			stateObject.updateRoot(s.db)
			s.updateStateObject(stateObject)
		}
	}
	for addr := range s.stateChunksDirty {
		stateChunk := s.stateChunks[addr]
		s.updateStateChunkRent(stateChunk)
	}
	for addr := range s.stateSlicesDirty {
		stateSlice := s.stateSlices[addr]
		s.updateStateSlice(stateSlice)
	}
	// Invalidate journal because reverting across transactions is not allowed.
	s.clearJournalAndRefund()
}

// IntermediateRoot computes the current root hash of the state trie.
// It is called in between transactions to get the root hash that
// goes into transaction receipts.
func (s *StateDB) IntermediateRoot(deleteEmptyObjects bool) common.Hash {
	s.Finalise(deleteEmptyObjects)
	s.Print()
	return s.trie.Hash()
}

// Prepare sets the current transaction hash and index and block hash which is
// used when the EVM emits new state logs.
func (self *StateDB) Prepare(thash, bhash common.Hash, ti int) {
	self.thash = thash
	self.bhash = bhash
	self.txIndex = ti
}

// DeleteSuicides flags the suicided objects for deletion so that it
// won't be referenced again when called / queried up on.
//
// DeleteSuicides should not be used for consensus related updates
// under any circumstances.
func (s *StateDB) DeleteSuicides() {
	// Reset refund so that any used-gas calculations can use this method.
	s.clearJournalAndRefund()

	for addr := range s.stateObjectsDirty {
		stateObject := s.stateObjects[addr]

		// If the object has been removed by a suicide
		// flag the object as deleted.
		if stateObject.suicided {
			stateObject.deleted = true
		}
		delete(s.stateObjectsDirty, addr)
	}
}

func (s *StateDB) clearJournalAndRefund() {
	s.journal = nil
	s.validRevisions = s.validRevisions[:0]
	s.refund = 0
}

// Commit writes the state to the underlying in-memory trie database.
func (s *StateDB) Commit(deleteEmptyObjects bool) (root common.Hash, err error) {
	defer s.clearJournalAndRefund()

	// Commit objects to the trie.
	for addr, stateObject := range s.stateObjects {
		_, isDirty := s.stateObjectsDirty[addr]
		switch {
		case stateObject.suicided || (isDirty && deleteEmptyObjects && stateObject.empty()):
			log.Error("----------???----------")
			// If the object has been removed, don't bother syncing it
			// and just mark it for deletion in the trie.
			s.deleteStateObject(stateObject)
		case isDirty:
			// Write any contract code associated with the state object
			if stateObject.code != nil && stateObject.dirtyCode {
				s.db.TrieDB().Insert(common.BytesToHash(stateObject.CodeHash()), stateObject.code)
				stateObject.dirtyCode = false
			}
			// Write any storage changes in the state object to its storage trie.
			if err := stateObject.CommitTrie(s.db); err != nil {
				return common.Hash{}, err
			}
			// Update the object in the main account trie.
			s.updateStateObject(stateObject)
		}
		// delete掉 s.stateObjectsDirty[addr]处的 struct{}{},就表示清除了 dirty 位
		delete(s.stateObjectsDirty, addr)
	}

	// Commit slices to the trie.
	for addr, stateChunk := range s.stateChunks {
		_, isDirty := s.stateChunksDirty[addr]
		switch {
		case isDirty:
			// Write any storage changes in the state slice to its storage trie.
			if err := stateChunk.CommitTrie(s.db); err != nil {
				log.Error("stateSlice.CommitTrie", "err", err)
				return common.Hash{}, err
			}
			// Update the object in the main account trie.
			s.updateStateChunkRent(stateChunk)
		}
		// delete掉 s.stateSlicesDirty[addr]处的 struct{}{},就表示清除了 dirty 位
		delete(s.stateChunksDirty, addr)
	}

	// Commit slices to the trie.
	for addr, stateSlice := range s.stateSlices {
		_, isDirty := s.stateSlicesDirty[addr]
		switch {
		case isDirty:
			// Write any storage changes in the state slice to its storage trie.
			if err := stateSlice.CommitTrie(s.db); err != nil {
				log.Error("stateSlice.CommitTrie", "err", err)
				return common.Hash{}, err
			}
			// Update the object in the main account trie.
			s.updateStateSlice(stateSlice)
		}
		// delete掉 s.stateSlicesDirty[addr]处的 struct{}{},就表示清除了 dirty 位
		delete(s.stateSlicesDirty, addr)
	}

	// Write trie changes.
	root, err = s.trie.Commit(func(leaf []byte, parent common.Hash) error {
		var account Account
		if err := rlp.DecodeBytes(leaf, &account); err != nil {
			return nil
		}
		if account.Root != emptyState {
			s.db.TrieDB().Reference(account.Root, parent)
		}
		code := common.BytesToHash(account.CodeHash)
		if code != emptyCode {
			s.db.TrieDB().Reference(code, parent)
		}
		return nil
	})
	log.Debug("Trie cache stats after commit", "misses", trie.CacheMisses(), "unloads", trie.CacheUnloads())
	return root, err
}

func (self *StateDB) ClaimChunk(addr common.Address, claimData []byte, update bool) {
	var cd storage.ClaimData
	if err := rlp.DecodeBytes(claimData, &cd); err != nil {
		stateDebug("Claim chunk", "err", err)
		return
	}
	// Getting chunk id here
	cd.ChunkId = self.GetClaimChunk(addr, cd.Node, 0)

	chunk := self.GetOrNewStateChunk(cd.ChunkId)
	if chunk == nil {
		return
	}
	i := chunk.IndexClaimerDB(addr)
	// 已经申领过此chunk
	if i != -1 {
		stateDebug("Claim chunk, exist", "chunk", chunk.chunkId, "addr", addr, "totalclaimer", len(chunk.claimersDB))
		return
	}

	if self.elebase != nil && self.elebase.Equal(addr) && !storage.HasChunk(cd.ChunkId) {
		// Write original data
		// ret := storage.CheckClaimFile(*self.elebase, cd.ChunkId, "")
		// if !ret {
		// 	log.Error("Check claim file error", "addr", *self.elebase, "chunk", cd.ChunkId)
		// }
		req := cd.ChunkId
		self.feeds[CCheckClaimFile].Send(req)
		self.feeds[CCheckClaimFileEnd].Send(struct{}{}) // Wait until the chunk file is written
	}

	oldMax, _ := chunk.getClaimMaxCount()
	rootHash := storage.GetRootHash(cd.Content, cd.Hashs)
	data := ClaimerDB{
		Address:  addr,
		RootHash: rootHash,
		Node:     cd.Node,
	}
	if len(chunk.claimersDB) == 0 {
		chunk.claimersDB = ClaimersDB{data}
		self.stateChunks[chunk.chunkId] = chunk
		//if update {
		self.chunkMaxId++
		self.updateChunkMaxId(self.chunkMaxId)
		//}
	} else {
		chunk.claimersDB = append(chunk.claimersDB, data)
	}
	stateDebug("Claim chunk, add", "chunk", chunk.chunkId, "addr", addr.ShortString(), "totalclaimer", len(chunk.claimersDB))
	self.updateStateChunkClaim(chunk)
	chunk.updateClaimCount(oldMax, self.feeds)
	// self.updateStateClaimInfo(addr, cd.ChunkId)
}

func getChunkClaimKey(chunkId uint64) []byte {
	return []byte("claim_" + strconv.FormatUint(chunkId, 10))
}

func getChunkVerifyKey(chunkId uint64) []byte {
	return []byte("verify_" + strconv.FormatUint(chunkId, 10))
}

func getChunkRentKey(chunkId uint64) []byte {
	return []byte("rent_" + strconv.FormatUint(chunkId, 10))
}

func getSliceKey(addr common.Address) []byte {
	return []byte("slice_" + addr.Str())
}

func getChunkMaxIdKey() []byte {
	return []byte("chunkmaxid")
}

func getClaimKey(addr common.Address) []byte {
	return []byte("claiminfo_" + addr.Str())
}

func getShardingHeightKey(shardingID uint16) []byte {
	return []byte("shardingheight_" + strconv.FormatUint(uint64(shardingID), 10))
}

func getBatchNonceKey(shardingID uint16) []byte {
	return []byte("batchnonce_" + strconv.FormatUint(uint64(shardingID), 10))
}

func getShareObjKey(addr common.Address) []byte {
	return []byte("shareobj_" + addr.ToHex())
}

func getMailDataKey(addr common.Address) []byte {
	return []byte("maildata_" + addr.ToHex())
}

func getRegDataKey(addr common.Address) []byte {
	return []byte("regdata_" + addr.ToHex())
}

func getNicknameKey(name string) []byte {
	return []byte("nickname_" + name)
}

func getVerifyAwardKey(addr common.Address) []byte {
	return []byte("verifyaward_" + addr.ToHex())
}

// Retrieve a state chunk given my chunk id. Returns nil if not found.
func (self *StateDB) getStateChunk(chunkId uint64) (stateChunk *stateChunk) {
	//debug.PrintStack()
	// Prefer 'live' objects.
	if obj := self.stateChunks[chunkId]; obj != nil {
		//fmt.Println("obj: ", obj)
		if obj.deleted {
			return nil
		}
		return obj
	}

	// Load the chunk from the database.
	enc, err := self.trie.TryGet(getChunkClaimKey(chunkId))
	if len(enc) == 0 {
		self.setError(err)
		return nil
	}
	var datas []byte
	if err := rlp.DecodeBytes(enc, &datas); err != nil {
		log.Error("Failed to decode state chunk", "chunkid", chunkId, "err", err)
		return nil
	}
	//fmt.Printf("rlp decode bytes datas: %+v\n", datas)
	//cs := strSlice2ClaimersDB(datas)
	cs := ToClaimersDB(datas)
	// log.Info("get state chunk", "cs len", len(cs))
	// Insert into the live set.
	obj := newStateChunk(self, chunkId, cs, self.MarkStateChunkDirty)

	enc, err = self.trie.TryGet(getChunkVerifyKey(chunkId))
	if len(enc) == 0 {
		//self.setError(err)
		//log.Info("try get", "err", err, "enc", enc)
		//return nil
	} else {
		var datas []byte
		if err := rlp.DecodeBytes(enc, &datas); err != nil {
			log.Error("failed to decode state chunk verify", "addr", chunkId, "err", err)
			//return nil
		} else {
			vs := ToVerifyersDB(datas)
			// log.Info("get state chunk", "vs len", len(vs))
			obj.verifyersDB = vs
		}
	}

	enc, err = self.trie.TryGet(getChunkRentKey(chunkId))
	if len(enc) == 0 {
		//self.setError(err)
		//log.Info("try get", "err", err, "enc", enc)
		//return nil
	} else {
		var datas []common.Address
		if err := rlp.DecodeBytes(enc, &datas); err != nil {
			log.Error("failed to decode state chunk rent", "addr", chunkId, "err", err)
			//return nil
		} else {
			// log.Info("get state chunk", "datas len", len(datas))
			len := len(datas)
			if len > storage.SliceCount {
				len = storage.SliceCount
			}
			obj.rentDB = datas[:len]
		}
	}

	self.setStateChunk(obj)
	return obj
}

func (self *StateDB) setStateChunk(chunk *stateChunk) {
	self.stateChunks[chunk.chunkId] = chunk
}

// MarkStateChunkDirty adds the specified chunk to the dirty map to avoid costly
// state chunk cache iteration to find a handful of modified ones.
func (self *StateDB) MarkStateChunkDirty(chunkId uint64) {
	self.stateChunksDirty[chunkId] = struct{}{}
}

// updateStateChunkClaim writes the given chunk to the trie.
func (self *StateDB) updateStateChunkClaim(stateChunk *stateChunk) {
	chunkId := stateChunk.chunkId
	datas := FromClaimersDB(stateChunk.claimersDB)
	// stateDebug("Update state chunk claim", "len", len(datas))
	data, err := rlp.EncodeToBytes(datas)
	if err != nil {
		panic(fmt.Errorf("can't encode object at %v: %v", chunkId, err))
	}
	self.setError(self.trie.TryUpdate(getChunkClaimKey(chunkId), data))
}

// deleteStateChunk removes the given chunk from the state trie.
func (self *StateDB) deleteStateChunk(stateChunk *stateChunk) {
	stateChunk.deleted = true
	chunkId := stateChunk.chunkId
	self.setError(self.trie.TryDelete(getChunkClaimKey(chunkId)))
}

// Retrieve a state chunk or create a new state chunk if nil
func (self *StateDB) GetOrNewStateChunk(chunkId uint64) *stateChunk {
	stateChunk := self.getStateChunk(chunkId)
	if stateChunk == nil || stateChunk.deleted {
		stateChunk, _ = self.createChunk(chunkId)
	}
	return stateChunk
}

// createChunk creates a new state chunk. If there is an existing account with
// the given address, it is overwritten and returned as the second return value.
func (self *StateDB) createChunk(chunkId uint64) (newobj, prev *stateChunk) {
	prev = self.getStateChunk(chunkId)
	var data ClaimersDB
	newobj = newStateChunk(self, chunkId, data, self.MarkStateChunkDirty)
	if prev == nil {
		self.journal = append(self.journal, createChunkChange{chunkId: &chunkId})
	} else {
		self.journal = append(self.journal, resetChunkChange{prev: prev})
	}
	self.setStateChunk(newobj)
	return newobj, prev
}

// Exist reports whether the given chunk id exists in the state.
// Notably this also returns true for suicided chunk id.
func (self *StateDB) ExistChunk(chunkId uint64) bool {
	return self.getStateChunk(chunkId) != nil
}

// Empty returns whether the state chunk is non-existent
func (self *StateDB) EmptyChunk(chunkId uint64) bool {
	so := self.getStateChunk(chunkId)
	return so == nil
}

func (self *StateDB) GetFarmerDataRes(chunkId uint64) *storage.FarmerData {
	sc := self.getStateChunk(chunkId)
	if sc == nil {
		// log.Warn("state chunk chunk id nil")
		return nil
	}
	// addrs := make([]common.Address, 0, len(sc.claimersDB))
	// for _, v := range sc.claimersDB {
	// 	addrs = append(addrs, v.Address)
	// }
	nodes := make([]discover.Node, 0, len(sc.claimersDB))
	for _, v := range sc.claimersDB {
		if v.Node != nil {
			nodes = append(nodes, *v.Node)
		}
	}
	cpd := &storage.FarmerData{
		ChunkId:  chunkId,
		CanClaim: sc.CanClaim,
		CanUse:   sc.CanUse,
		//Addresss: addrs,
		//AddrIp:   make([]string, 0, len(addrs)),
		Nodes: nodes,
	}
	return cpd
}

func getMaxVerify(verifyers VerifyersDB) (uint8, common.Hash) {
	var max uint8
	var hash common.Hash
	hashCount := make(map[common.Hash]uint8)
	for _, v := range verifyers {
		h := v.RootHash
		if _, ok := hashCount[h]; !ok {
			hashCount[h] = 1
		} else {
			hashCount[h] += 1
		}
		if max >= hashCount[h] {
			continue
		}
		max = hashCount[h]
		hash = h
	}
	return max, hash
}

func (self *StateDB) VerifyChunk(addr common.Address, verifyData []byte) {
	var vd storage.VerifyData
	if err := rlp.DecodeBytes(verifyData, &vd); err != nil {
		stateDebug("Verify chunk", "err", err)
		return
	}

	chunk := self.getStateChunk(vd.ChunkId)
	if chunk == nil {
		return
	}
	i := chunk.IndexClaimerDB(addr)
	// 没有申领过此chunk
	if i == -1 {
		stateDebug("Verify chunk, not claim", "chunk", chunk.chunkId, "addr", addr.ShortString())
		return
	}
	rootHash := storage.GetRootHash(vd.Content, vd.Hashs)
	// stateDebug("Verify chunk", "chunk", chunk.chunkId, "addr", addr.ShortString(), "roothash", rootHash.ShortString())
	i = chunk.IndexVerifyerDB(addr)
	// 找到了，更新
	if i != -1 {
		chunk.verifyersDB[i].RootHash = rootHash
	} else {
		data := VerifyerDB{
			Address:  addr,
			RootHash: rootHash,
		}
		chunk.verifyersDB = append(chunk.verifyersDB, data)
	}
	self.updateStateChunkVerify(chunk)

	max, hash := getMaxVerify(chunk.verifyersDB)
	copyCnt := blizparam.GetMinCopyCount()
	if max < uint8(copyCnt) {
		logVerify("Verify chunk", "max", max)
		return
	}
	if !rootHash.Equal(hash) {
		log.Info("Verify chunk, hash not match", "hash", rootHash.ShortString(), "maxhash", hash.ShortString())
		return
	}

	h := self.getVerifyAward(addr)
	if h >= vd.H {
		logVerify("Verify chunk", "h", h, "newh", vd.H)
		return
	}
	self.setVerifyAward(addr, vd.H)
	self.updateVerifyAward(addr, vd.H)
	// 检验空间获得2个CTT
	value := int64(2000000000000000000)
	self.AddBalance(addr, big.NewInt(value))
}

func (self *StateDB) verifyChunkAward(claimers ClaimersDB, value int64, chunkID uint64) {
	for _, claimer := range claimers {
		addr := claimer.Address
		// old := self.GetBalance(addr)
		self.AddBalance(addr, big.NewInt(value))
		// stateDebug("Verify chunk award", "chunk", chunkID, "shortaddr", addr.ShortString(), "oldbalance", old, "addBalance", value)
	}
}

// updateStateChunkVerify writes the given chunk to the trie.
func (self *StateDB) updateStateChunkVerify(stateChunk *stateChunk) {
	chunkId := stateChunk.chunkId
	datas := FromVerifyersDB(stateChunk.verifyersDB)
	data, err := rlp.EncodeToBytes(datas)
	if err != nil {
		panic(fmt.Errorf("can't encode chunk verify at %v: %v", chunkId, err))
	}
	self.setError(self.trie.TryUpdate(getChunkVerifyKey(chunkId), data))
}

func (self *StateDB) RentUnits(addr common.Address, count uint64) ([]uint64, map[uint64][]uint32, uint64) {
	// chunkIds := self.collectChunkSlice()
	stateDebug("Rent Units", "count", len(self.stateChunks))
	chunkIds := make([]uint64, 0)
	chunkIdSliceIds := make(map[uint64][]uint32)
	currentCount := uint64(0)
	bEnd := false

	for chunkId := uint64(1); chunkId <= collectChunkMax; chunkId++ {
		sc := self.getStateChunk(chunkId)
		if sc == nil {
			continue
		}
		if !sc.CanUse {
			log.Info("chunk can't use", "chunk", chunkId)
			// sc.Log()
			continue
		}
		for i := 0; i < storage.SliceCount; i++ {
			if !common.EmptyAddress(sc.rentDB[i]) {
				continue
			}
			sliceId := uint32(i + 1)
			// log.Info("rent size", "chunkId", chunkId, "sliceId", sliceId)
			chunkIds = append(chunkIds, chunkId)
			chunkIdSliceIds[chunkId] = append(chunkIdSliceIds[chunkId], sliceId)
			currentCount++
			if currentCount >= count {
				bEnd = true
				break
			}
		}
		if bEnd {
			break
		}
	}
	return chunkIds, chunkIdSliceIds, currentCount
}

func (self *StateDB) RentSize(addr common.Address, rentData []byte, update bool) {
	// if !update {
	// 	return
	// }
	var rd storage.RentData
	if err := rlp.DecodeBytes(rentData, &rd); err != nil {
		stateDebug("Rent size", "err", err)
		return
	}
	stateDebug("Rent size", "addr", addr.ShortString())
	size := rd.Size
	count := size / storage.SliceSize
	if count*storage.SliceSize < size {
		count++
	}
	if count == 0 {
		log.Error("Rent size, count 0")
		return
	}
	chunkIds, chunkIdSliceIds, rentCount := self.RentUnits(addr, count)
	if rentCount < uint64(count) {
		log.Error("Rent size", "rentcount", rentCount, "count", count)
		return
	}
	stateSlice := self.GetOrNewStateSlice(addr)
	if stateSlice == nil {
		log.Error("Rent size, state slice nil")
		return
	}
	// for chunkId, sliceIds := range chunkIdSliceIds {
	for _, chunkId := range chunkIds {
		sliceIds, ok := chunkIdSliceIds[chunkId]
		if !ok {
			continue
		}
		for _, sliceId := range sliceIds {
			self.stateChunks[chunkId].rentDB[sliceId-1] = addr
			bFound := false
			for _, sl := range stateSlice.slices {
				if sl.ChunkId == chunkId && sl.SliceId == sliceId {
					bFound = true
					break
				}
			}
			if !bFound {
				slice := &ObjSlice{
					ChunkId: chunkId,
					SliceId: sliceId,
					ObjId:   rd.ObjId,
				}
				stateSlice.slices = append(stateSlice.slices, slice)

				sslice := &storagecore.Slice{ChunkId: chunkId, SliceId: sliceId}
				// storage.WriteSliceHeader(addr, sslice, 0)
				result := &exporter.ObjWriteSliceHeader{
					Addr:  addr,
					Slice: sslice,
					OpId:  1,
				}
				self.feeds[CWriteSliceHeader].Send(result)
			}
		}
		self.updateStateChunkRent(self.stateChunks[chunkId])
	}
	// self.PrintTrieHash("aaa")
	go elephantToBlizcsSliceInfo(addr, rd.ObjId, stateSlice.slices, self.feeds, rd.CSAddr)
	self.updateStateSlice(stateSlice)
	// self.PrintTrieHash("bbb")
}

func elephantToBlizcsSliceInfo(addr common.Address, objId uint64, objSlices []*ObjSlice, feeds []*event.Feed, csAddr common.Address) {
	slices := make([]storagecore.Slice, 0, len(objSlices))
	for _, slice := range objSlices {
		if slice.ObjId != objId {
			continue
		}
		slices = append(slices, storagecore.Slice{ChunkId: slice.ChunkId, SliceId: slice.SliceId})
	}

	result := &exporter.ObjElephantToBlizcsSliceInfo{
		Addr:   addr,
		ObjId:  objId,
		Slices: slices,
		CSAddr: csAddr,
	}
	feeds[CElephantToBlizcsSliceInfo].Send(result)
}

// updateStateChunkRent writes the given chunk to the trie.
func (self *StateDB) updateStateChunkRent(stateChunk *stateChunk) {
	chunkId := stateChunk.chunkId
	datas := stateChunk.rentDB
	//log.Info("update state chunk rent", "datas len", len(datas))
	// for i, data := range datas {
	// 	if !common.EmptyAddress(data) {
	// 		log.Info("chunk rent", "id", i+1, "addr", data)
	// 	}
	// }
	data, err := rlp.EncodeToBytes(datas)
	if err != nil {
		panic(fmt.Errorf("can't encode chunk rent at %v: %v", chunkId, err))
	}
	//fmt.Printf("data: %v\n", data)
	self.setError(self.trie.TryUpdate(getChunkRentKey(chunkId), data))
}

// updateStateSlice writes the given slice to the trie.
func (self *StateDB) updateStateSlice(stateSlice *stateSlice) {
	if stateSlice == nil {
		return
	}
	// log.Info("updateStateSlice", "stateSlice", stateSlice)
	addr := stateSlice.address
	tmps := make([]ObjSlice, 0, len(stateSlice.slices))
	for _, slice := range stateSlice.slices {
		tmp := ObjSlice{ChunkId: slice.ChunkId, SliceId: slice.SliceId, ObjId: slice.ObjId}
		tmps = append(tmps, tmp)
	}
	data, err := rlp.EncodeToBytes(tmps)
	if err != nil {
		panic(fmt.Errorf("can't encode slice at %x: %v", addr[:], err))
	}
	self.setError(self.trie.TryUpdate(getSliceKey(addr), data))
}

// Retrieve a state slice given my the address. Returns nil if not found.
func (self *StateDB) getStateSlice(addr common.Address) (stateSlice *stateSlice) {
	// Prefer 'live' slices.
	if sl := self.stateSlices[addr]; sl != nil {
		return sl
	}
	// Load the slice from the database.
	enc, err := self.trie.TryGet(getSliceKey(addr))
	if len(enc) == 0 {
		self.setError(err)
		return nil
	}
	var datas []ObjSlice
	if err := rlp.DecodeBytes(enc, &datas); err != nil || len(datas) == 0 {
		log.Error("Failed to decode state slice", "addr", addr, "err", err)
		return nil
	}
	slices := make([]*ObjSlice, 0, len(datas))
	for _, data := range datas {
		slice := &ObjSlice{ChunkId: data.ChunkId, SliceId: data.SliceId, ObjId: data.ObjId}
		slices = append(slices, slice)
		// log.Info("getStateSlice", "i", i, "slice", slice)
	}
	// Insert into the live set.
	sl := newSlice(self, addr, slices, self.MarkStateSliceDirty)
	self.setStateSlice(sl)
	return sl
}

func (self *StateDB) setStateSlice(slice *stateSlice) {
	self.stateSlices[slice.address] = slice
}

func (self *StateDB) MarkStateSliceDirty(addr common.Address) {
	self.stateSlicesDirty[addr] = struct{}{}
}

// updateStateClaim writes the given claim to the trie.
func (self *StateDB) updateStateClaim(stateClaim *stateClaim) {
	if stateClaim == nil {
		return
	}
	log.Info("updateStateClaim", "ClaimId", stateClaim.claimInfo.ClaimId)
	for _, claim := range stateClaim.claimInfo.Claims {
		log.Info("", "claim", claim)
	}
	addr := stateClaim.address
	data, err := rlp.EncodeToBytes(stateClaim.claimInfo)
	if err != nil {
		panic(fmt.Errorf("can't encode claim at %x: %v", addr[:], err))
	}
	log.Info("EncodeToBytes", "data len", len(data))
	err = self.trie.TryUpdate(getClaimKey(addr), data)
	if err != nil {
		log.Error("TryUpdate", "err", err)
	}
	self.setError(err)
}

// Retrieve a state claim given my the address. Returns nil if not found.
// func (self *StateDB) getStateClaim(addr common.Address) (stateClaim *stateClaim) {
// 	// Prefer 'live' claims.
// 	if sc := self.stateClaims[addr]; sc != nil {
// 		// log.Info("getStateClaim sc", "ClaimId", sc.claimInfo.ClaimId)
// 		// for _, claim := range sc.claimInfo.Claims {
// 		// 	log.Info("", "claim", claim)
// 		// }
// 		return sc
// 	}
// 	// Load the claim from the database.
// 	enc, err := self.trie.TryGet(getClaimKey(addr))
// 	if len(enc) == 0 {
// 		log.Info("TryGet", "enc", enc, "err", err)
// 		self.setError(err)
// 		return nil
// 	}
// 	claimInfo := new(ClaimInfo)
// 	if err := rlp.DecodeBytes(enc, claimInfo); err != nil {
// 		log.Error("Failed to decode state claim", "addr", addr, "err", err)
// 		return nil
// 	}
// 	// log.Info("getStateClaim", "ClaimId", claimInfo.ClaimId)
// 	// for _, claim := range claimInfo.Claims {
// 	// 	log.Info("", "claim", claim)
// 	// }
// 	// Insert into the live set.
// 	sc := newClaim(addr, self, claimInfo, self.MarkStateClaimDirty)
// 	self.setStateClaim(sc)
// 	return sc
// }

// func (self *StateDB) setStateClaim(claim *stateClaim) {
// 	self.stateClaims[claim.address] = claim
// }

// func (self *StateDB) MarkStateClaimDirty(addr common.Address) {
// 	log.Info("MarkStateClaimDirty", "addr", addr)
// 	self.stateClaimsDirty[addr] = struct{}{}
// }

// Retrieve a state claim or create a new state claim if nil
// func (self *StateDB) GetOrNewStateClaim(addr common.Address) *stateClaim {
// 	stateClaim := self.getStateClaim(addr)
// 	if stateClaim == nil {
// 		stateClaim, _ = self.createClaim(addr)
// 	}
// 	return stateClaim
// }

// func (self *StateDB) createClaim(addr common.Address) (newSc, prev *stateClaim) {
// 	prev = self.getStateClaim(addr)
// 	var claimInfo ClaimInfo
// 	newSc = newClaim(addr, self, &claimInfo, self.MarkStateClaimDirty)
// 	// log.Info("createClaim", "ClaimId", claimInfo.ClaimId)
// 	// for _, claim := range claimInfo.Claims {
// 	// 	log.Info("", "claim", claim)
// 	// }
// 	if prev == nil {
// 		self.journal = append(self.journal, createClaimChange{account: &addr})
// 	} else {
// 		self.journal = append(self.journal, resetClaimChange{prev: prev})
// 	}
// 	self.setStateClaim(newSc)
// 	return newSc, prev
// }

// qiwy: 规则必须一致，否则会导致tx校验不通过
// same rule, such as: from small to big
func (self *StateDB) collectChunkSlice() []uint64 {
	chunkIds := make([]uint64, 0, collectChunkMax)
	// unusedCount := uint32(0)
	stateDebug("Collect chunk slice", "max", self.chunkMaxId)
	for i := uint64(1); i <= collectChunkMax; /*self.chunkMaxId*/ i++ { // qiwy: TODO
		sc := self.getStateChunk(i)
		if sc == nil {
			log.Info("chunk id nonexist", "i", i)
			continue
		}
		chunkIds = append(chunkIds, i)
		// unusedCount += getUnusedSliceCount(sc)
		// if unusedCount >= unusedCount {
		// 	return
		// }
	}
	return chunkIds
}

func getUnusedSliceCount(sc *stateChunk) uint32 {
	unusedCount := uint32(0)
	if sc == nil {
		return unusedCount
	}
	if !sc.CanUse {
		log.Info("chunk can't use", "chunkId", sc.chunkId)
		return unusedCount
	}
	for i := 0; i < storage.SliceCount; i++ {
		if !common.EmptyAddress(sc.rentDB[i]) {
			continue
		}
		unusedCount++
	}
	return unusedCount
}

func (self *StateDB) updateChunkMaxId(id uint64) {
	stateDebug("Update chunk max id", "id", id)
	data, err := rlp.EncodeToBytes(id)
	if err != nil {
		panic(fmt.Errorf("can't encode at %v: %v", id, err))
	}
	self.setError(self.trie.TryUpdate(getChunkMaxIdKey(), data))
}

func getChunkMaxId(trie Trie) uint64 {
	enc, err := trie.TryGet(getChunkMaxIdKey())
	if err != nil || len(enc) == 0 {
		return 0
	}
	var id uint64
	if err := rlp.DecodeBytes(enc, &id); err != nil {
		log.Error("Failed to decode", "err", err)
		return 0
	}
	//log.Info("get chunk id max", "id", id)
	return id
}

func (self *StateDB) updateShardingHeight(shardingID uint16, height uint64) {
	data, err := rlp.EncodeToBytes(height)
	if err != nil {
		panic(fmt.Errorf("can't encode sharding height at %d: %v", height, err))
	}
	self.setError(self.trie.TryUpdate(getShardingHeightKey(shardingID), data))
}

func (self *StateDB) getShardingHeight(shardingID uint16) uint64 {
	// Prefer 'live' height.
	if h, ok := self.shardingHeight[shardingID]; ok {
		return h
	}
	// Load the height from the database.
	enc, err := self.trie.TryGet(getShardingHeightKey(shardingID))
	if len(enc) == 0 {
		self.setError(err)
		return 0
	}
	var data uint64
	if err := rlp.DecodeBytes(enc, &data); err != nil {
		log.Error("Failed to decode sharding height", "err", err)
		return 0
	}
	// Insert into the live set.
	self.setShardingHeight(shardingID, data)
	return data
}

func (self *StateDB) setShardingHeight(shardingID uint16, height uint64) {
	self.shardingHeight[shardingID] = height
}

func (self *StateDB) GetShardingHeight(shardingID uint16) uint64 {
	return self.getShardingHeight(shardingID)
}

func (self *StateDB) UpdateStoreShardingHeight(shardingID uint16, height uint64) {
	self.setShardingHeight(shardingID, height)
	self.updateShardingHeight(shardingID, height)
}

func (self *StateDB) updateBatchNonce(shardingID uint16, nonce uint64) {
	data, err := rlp.EncodeToBytes(nonce)
	if err != nil {
		panic(fmt.Errorf("can't encode batch nonce at %d: %v", nonce, err))
	}
	self.setError(self.trie.TryUpdate(getBatchNonceKey(shardingID), data))
}

func (self *StateDB) getBatchNonce(shardingID uint16) uint64 {
	// Prefer 'live' nonce.
	if n, ok := self.batchNonce[shardingID]; ok {
		return n
	}
	// Load the nonce from the database.
	enc, err := self.trie.TryGet(getBatchNonceKey(shardingID))
	if len(enc) == 0 {
		self.setError(err)
		return 0
	}
	var data uint64
	if err := rlp.DecodeBytes(enc, &data); err != nil {
		log.Error("Failed to decode batch nonce", "err", err)
		return 0
	}
	// Insert into the live set.
	self.setBatchNonce(shardingID, data)
	return data
}

func (self *StateDB) setBatchNonce(shardingID uint16, nonce uint64) {
	self.batchNonce[shardingID] = nonce
}

func (self *StateDB) GetBatchNonce(shardingID uint16) uint64 {
	return self.getBatchNonce(shardingID)
}

func (self *StateDB) UpdateStoreBatchNonce(shardingID uint16, nonce uint64) {
	self.setBatchNonce(shardingID, nonce)
	self.updateBatchNonce(shardingID, nonce)
}

// Retrieve a state slice or create a new state slice if nil
func (self *StateDB) GetOrNewStateSlice(addr common.Address) *stateSlice {
	stateSlice := self.getStateSlice(addr)
	if stateSlice == nil {
		stateSlice, _ = self.createSlice(addr)
	}
	return stateSlice
}

func (self *StateDB) createSlice(addr common.Address) (newSl, prev *stateSlice) {
	prev = self.getStateSlice(addr)
	slices := make([]*ObjSlice, 0)
	newSl = newSlice(self, addr, slices, self.MarkStateSliceDirty)
	if prev == nil {
		self.journal = append(self.journal, createSliceChange{account: &addr})
	} else {
		self.journal = append(self.journal, resetSliceChange{prev: prev})
	}
	self.setStateSlice(newSl)
	return newSl, prev
}

func (self *StateDB) GetStateSlice(addr common.Address, objId uint64) []*storagecore.Slice {
	slices := make([]*storagecore.Slice, 0)
	stateSlice := self.getStateSlice(addr)
	if stateSlice == nil {
		log.Error("Get state slice, state slice nil", "addr", addr.Hex())
		return slices
	}
	log.Info("Get state slice", "len", len(stateSlice.slices))
	for _, slice := range stateSlice.slices {
		// log.Info("range", "chunkId", slice.ChunkId, "sliceId", slice.SliceId, "objId", slice.ObjId)
		if slice.ObjId == objId {
			slice := &storagecore.Slice{ChunkId: slice.ChunkId, SliceId: slice.SliceId}
			slices = append(slices, slice)
		}
	}
	return slices
}

func (self *StateDB) GetClaimChunk(address common.Address, node *discover.Node, blockNumber uint64) uint64 {
	var chunkStartId uint64
	var clist chunkList
	// stateClaim := self.getStateClaim(address)
	// if stateClaim == nil {
	// 	chunkStartId = 1
	// 	from := chunkStartId
	// 	to := chunkStartId + claimChunkOpenMaxCount
	// 	for i := from; i < to; i++ {
	// 		cc := &chunkClaim{
	// 			chunkId: i,
	// 		}
	// 		clist.ccs = append(clist.ccs, cc)
	// 	}
	// } else {
	// 	// 有个ClaimInfo转chunkStartId、chunkClaim的过程，稳定后可以统一格式
	// 	chunkStartId = stateClaim.claimInfo.ClaimId
	// 	for _, claim := range stateClaim.claimInfo.Claims {
	// 		cc := &chunkClaim{
	// 			chunkId:    claim.ChunkId,
	// 			claimCount: claim.ClaimCount,
	// 		}
	// 		clist.ccs = append(clist.ccs, cc)
	// 	}
	// }

	chunkStartId = 1
	from := chunkStartId
	to := self.chunkMaxId
	if to < from+claimChunkOpenMaxCount {
		to = from + claimChunkOpenMaxCount
	}
	for i := from; i < to; i++ {
		cc := &chunkClaim{
			chunkId: i,
		}
		sc := self.getStateChunk(i)
		if sc != nil {
			for _, claimer := range sc.claimersDB {
				cc.nodes = append(cc.nodes, claimer.Node)
			}
		}
		clist.ccs = append(clist.ccs, cc)
	}

	chunkId := getChunkIdA(chunkStartId, clist, blockNumber, node)
	return chunkId
}

// func (self *StateDB) TestUpdateStateClaim() {
// 	addr := common.HexToAddress("0xf408058614ab9dc43342b4fa42e869fc28ee6d4c")
// 	sc := self.GetOrNewStateClaim(addr)
// 	log.Info("GetOrNewStateClaim", "sc", sc)
// 	if sc == nil {
// 		return
// 	}

// 	claims := make([]*ObjClaim, 0, 2)
// 	claim := &ObjClaim{
// 		ChunkId:    1,
// 		ClaimCount: 0,
// 	}
// 	claims = append(claims, claim)
// 	claim = &ObjClaim{
// 		ChunkId:    2,
// 		ClaimCount: 1,
// 	}
// 	claims = append(claims, claim)

// 	sc.claimInfo.ClaimId = 2
// 	sc.claimInfo.Claims = claims

// 	self.updateStateClaim(sc)
// }

// func (self *StateDB) TestGetStateClaim(addr common.Address) {
// 	sc := self.getStateClaim(addr)
// 	if sc != nil {
// 		log.Info("getStateClaim", "ClaimId", sc.claimInfo.ClaimId)
// 		for _, claim := range sc.claimInfo.Claims {
// 			log.Info("", "claim", claim)
// 		}
// 	}
// }

// func (self *StateDB) updateStateClaimInfo(addr common.Address, chunkId uint64) {
// 	sc := self.GetOrNewStateClaim(addr)
// 	if sc == nil {
// 		return
// 	}
// 	// 新开放的
// 	if sc.claimInfo.ClaimId == 0 {
// 		sc.claimInfo.ClaimId = 1
// 		from := sc.claimInfo.ClaimId
// 		to := from + claimChunkOpenMaxCount
// 		for i := from; i < to; i++ {
// 			oc := &ObjClaim{
// 				ChunkId: i,
// 			}
// 			sc.claimInfo.Claims = append(sc.claimInfo.Claims, oc)
// 		}
// 	}
// 	bFound := false
// 	for _, claim := range sc.claimInfo.Claims {
// 		if claim.ChunkId == chunkId {
// 			claim.ClaimCount++
// 			bFound = true
// 		}
// 	}
// 	if !bFound {
// 		sc.claimInfo.Claims = append(sc.claimInfo.Claims, &ObjClaim{ChunkId: chunkId, ClaimCount: 1})
// 	}
// 	self.updateStateClaim(sc)
// }

func (self *StateDB) GetBlizCSSelfObject(addr common.Address) exporter.SelfObjectsInfo {
	info := make(exporter.SelfObjectsInfo, 0)
	ss := self.getStateSlice(addr)
	if ss == nil {
		log.Info("self ss nil")
		return info
	}
	for _, os := range ss.slices {
		objId := os.ObjId
		bFound := false
		for _, v := range info {
			if v.ObjId == objId {
				bFound = true
				v.Size += storagecore.MSize
				break
			}
		}
		if !bFound {
			info = append(info, &exporter.SelfObjectInfo{ObjId: objId, Size: storagecore.MSize})
		}
	}
	return info
}

func (self *StateDB) GetBlizCSShareObject(addr common.Address) exporter.ShareObjectsInfo {
	infos := make(exporter.ShareObjectsInfo, 0)
	sos := self.getShareObj(addr)
	if sos == nil {
		log.Info("Get bliz cs share object nil")
		return infos
	}
	for _, so := range sos {
		info := &exporter.ShareObjectInfo{
			Owner:          so.Owner,
			ObjId:          strconv.Itoa(int(so.ObjID)),
			Offset:         strconv.Itoa(int(so.Offset)),
			Length:         strconv.Itoa(int(so.Length)),
			StartTime:      so.StartTime,
			EndTime:        so.EndTime,
			FileName:       so.FileName,
			SharerCSNodeID: so.SharerCSNodeID,
			Key:            so.Key,
		}
		infos = append(infos, info)
	}
	return infos
}

func (self *StateDB) SignCs(addr common.Address, signCsData []byte, update bool) {
	var data blizparam.AddrTimeFlow
	if err := rlp.DecodeBytes(signCsData, &data); err != nil {
		log.Error("Sign cs", "err", err)
		return
	}
	toAddr := common.HexToAddress(string(data.Addr))
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject == nil {
		log.Error("Sign cs, state object nil")
		return
	}

	// check
	signNeedBalance := blizparam.GetNeedBalance(data.StartTime, data.EndTime, data.Flow)
	leftBalance := self.GetBalance(addr)
	if signNeedBalance.Cmp(signNeedBalance) < 0 {
		log.Error("Sign cs", "leftbalance", leftBalance, "needbalance", signNeedBalance)
		return
	}

	olds := stateObject.data.SignCsAddrs
	for _, a := range olds {
		if a != toAddr {
			continue
		}
		log.Info("Sign cs, same", "addr", addr.ShortString(), "olds", olds)
		return
	}
	log.Info("Sign cs, not find", "addr", addr.ShortString(), "to", toAddr.ShortString())
	news := append(olds, toAddr)
	// stateObject.SetSignCsAddr(news) // handle later

	// CS module
	stateObject2 := self.GetOrNewStateObject(toAddr)
	if stateObject2 == nil {
		log.Error("Sign cs, state object 2 nil")
		return
	}
	olds2 := stateObject2.data.ServerCliAddrs
	for _, a := range olds2 {
		if a.Authorizer != addr {
			continue
		}
		log.Info("Sign cs, same", "toaddr", toAddr.ShortString(), "olds2", olds2)
		return
	}
	log.Info("Sign cs, not find", "addr", addr.ShortString(), "toaddr", toAddr.ShortString())
	d := common.CSAuthData{Authorizer: addr, StartTime: data.StartTime, EndTime: data.EndTime, Flow: data.Flow, PayMethod: data.PayMethod}
	news2 := append(olds2, d)
	stateObject2.SetServerCliAddr(news2)

	// auth sub balance
	self.SubBalance(addr, signNeedBalance)

	stateObject.SetSignCsAddr(news)
}

func (self *StateDB) CancelCs(addr common.Address, cancelCsData []byte, update bool) {
	var param blizparam.CancelCsParam
	if err := rlp.DecodeBytes(cancelCsData, &param); err != nil {
		log.Error("Cancel cs", "err", err)
		return
	}
	toAddr := common.HexToAddress(param.CsAddr)
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject == nil {
		log.Error("GetOrNewStateObject stateObject nil")
		return
	}
	olds := stateObject.data.SignCsAddrs
	pos := -1
	for i, a := range olds {
		if a != toAddr {
			continue
		}
		pos = i
		break
	}
	if pos == -1 {
		log.Error("addr has yet rent", "toAddr", toAddr)
		return
	}
	log.Info("CancelCs", "addr", addr)
	news := append(olds[:pos], olds[pos+1:]...)
	// stateObject.SetSignCsAddr(news) // handle later

	// CS module
	stateObject2 := self.GetOrNewStateObject(toAddr)
	if stateObject2 == nil {
		log.Error("GetOrNewStateObject stateObject2 nil")
		return
	}
	olds2 := stateObject2.data.ServerCliAddrs
	pos2 := -1
	for i, a := range olds2 {
		if a.Authorizer != addr {
			continue
		}
		pos2 = i
		break
	}
	if pos2 == -1 {
		log.Error("addr has yet servered", "addr", addr)
		return
	}
	news2 := append(olds2[:pos2], olds2[pos2+1:]...)
	stateObject2.SetServerCliAddr(news2)

	data := olds2[pos2]
	now := param.Now
	// cs get cost
	csBalance := blizparam.GetNeedBalance(data.CSPickupTime, now, data.AuthAllowFlow-data.CSPickupFlow)
	self.AddBalance(toAddr, csBalance)
	// second, auth return left
	retBalance := blizparam.GetNeedBalance(now, data.EndTime, data.Flow-data.AuthAllowFlow)
	self.AddBalance(addr, retBalance)

	stateObject.SetSignCsAddr(news)

	// delete blizcs data
	params := make([]interface{}, 0, 2)
	params = append(params, toAddr)
	params = append(params, addr)
	req := &exporter.ObjElephantToCsData{Ty: exporter.TypeElephantToCsDeleteFlow, Params: params}
	self.feeds[CElephantToCsData].Send(req)
}

func (self *StateDB) GetSignCsAddr(addr common.Address) []common.Address {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject == nil {
		return nil
	}
	return stateObject.data.SignCsAddrs
}

func (self *StateDB) GetServerCliAddr(addr common.Address) []common.CSAuthData {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject == nil {
		return nil
	}
	return stateObject.data.ServerCliAddrs
}

func (self *StateDB) SignTime(addr common.Address, signTimeData []byte, update bool) { // qiwy: todo, need delete
	var data blizparam.SignTimeParam
	if err := rlp.DecodeBytes(signTimeData, &data); err != nil {
		log.Error("Sign time", "err", err)
		return
	}
	toAddr := common.HexToAddress(string(data.Addr))
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject == nil {
		log.Error("GetOrNewStateObject stateObject nil")
		return
	}
	bFound := false
	olds := stateObject.data.SignCsAddrs
	for _, a := range olds {
		if a != toAddr {
			continue
		}
		bFound = true
		break
	}
	if !bFound {
		log.Error("SignTime SignCsAddrs not find", "addr", addr, "toAddr", toAddr)
		return
	}

	stateObject2 := self.GetOrNewStateObject(toAddr)
	if stateObject2 == nil {
		log.Error("GetOrNewStateObject stateObject2 nil")
		return
	}
	bFound = false
	olds2 := stateObject2.data.ServerCliAddrs
	news2 := make([]common.CSAuthData, 0, len(olds2))
	for _, a := range olds2 {
		if a.Authorizer != addr {
			continue
		}
		bFound = true
		news2 := append(news2, a)
		stateObject2.SetServerCliAddr(news2)
		break
	}
	if !bFound {
		log.Error("SignTime ServerCliAddrs", "addr", addr, "toAddr", toAddr)
		return
	}
}

func (self *StateDB) SignFlow(addr common.Address, signFlowData []byte, update bool) {
	var data blizparam.SignFlowParam
	if err := rlp.DecodeBytes(signFlowData, &data); err != nil {
		log.Error("Sign flow", "err", err)
		return
	}
	toAddr := common.HexToAddress(string(data.Addr))
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject == nil {
		log.Error("GetOrNewStateObject stateObject nil")
		return
	}
	bFound := false
	olds := stateObject.data.SignCsAddrs
	for _, a := range olds {
		if a != toAddr {
			continue
		}
		bFound = true
		break
	}
	if !bFound {
		log.Error("SignFlow SignCsAddrs not find", "addr", addr, "toAddr", toAddr)
		return
	}

	stateObject2 := self.GetOrNewStateObject(toAddr)
	if stateObject2 == nil {
		log.Error("GetOrNewStateObject stateObject2 nil")
		return
	}
	bFound = false
	olds2 := stateObject2.data.ServerCliAddrs
	news2 := make([]common.CSAuthData, 0, len(olds2))
	for _, a := range olds2 {
		if a.Authorizer != addr {
			continue
		}
		bFound = true
		a.AuthAllowFlow = data.AllowFlow
		news2 := append(news2, a)
		stateObject2.SetServerCliAddr(news2)
		break
	}
	if !bFound {
		log.Error("SignFlow ServerCliAddrs", "addr", addr, "toAddr", toAddr)
		return
	}
}

func (self *StateDB) PickupCost(csAddr common.Address, pickupCostData []byte, update bool) {
	var data blizparam.CsPickupCostParam
	if err := rlp.DecodeBytes(pickupCostData, &data); err != nil {
		log.Error("Pickup cost", "err", err)
		return
	}
	csObj := self.GetOrNewStateObject(csAddr)
	if csObj == nil {
		log.Error("csAddr stateObject nil")
		return
	}
	cliAddrs := csObj.data.ServerCliAddrs
	userAddr := common.HexToAddress(data.Addr)
	for i, addr := range cliAddrs {
		if addr.Authorizer != userAddr {
			continue
		}
		// flow pay
		if addr.PayMethod == 1 {
			// check sign
			hash := blizcore.CommonHash([]interface{}{csAddr, data.AuthAllowFlow})
			sender, err := blizcore.CommonSender(hash, data.Sign)
			if err != nil {
				log.Error("CommonSender", "err", err)
				break
			}
			if sender != userAddr {
				log.Error("CommonSender", "sender", sender.Hex(), "userAddr", userAddr.Hex())
				break
			}
			if data.UsedFlow <= addr.CSPickupFlow {
				log.Error("already pickup", "UsedFlow", data.UsedFlow, "CSPickupFlow", addr.CSPickupFlow)
				break
			}
			amount := blizparam.GetFlowBalance(data.UsedFlow - addr.CSPickupFlow)
			cliAddrs[i].AuthAllowFlow = data.AuthAllowFlow
			cliAddrs[i].CSPickupFlow = data.UsedFlow
			csObj.SetServerCliAddr(cliAddrs)
			self.SubBalance(userAddr, amount)
			self.AddBalance(csAddr, amount)
			break
		} else {
			now := data.Now
			if now <= addr.CSPickupTime {
				log.Warn("PickupCost no", "now", now, "CSPickupTime", addr.CSPickupTime)
				break
			}
			amount := blizparam.GetTimeBalance(now - addr.CSPickupTime)
			cliAddrs[i].CSPickupTime = now
			csObj.SetServerCliAddr(cliAddrs)
			self.SubBalance(userAddr, amount)
			self.AddBalance(csAddr, amount)
			break
		}
	}
}

func (self *StateDB) AuthAllowFlow(userAddr common.Address, authFlowData []byte, update bool) {
	var data blizparam.UserAuthAllowFlowParam
	if err := rlp.DecodeBytes(authFlowData, &data); err != nil {
		log.Error("Auth allow flow", "err", err)
		return
	}
	csAddr := common.HexToAddress(data.CsAddr)
	csObj := self.GetOrNewStateObject(csAddr)
	if csObj == nil {
		log.Error("csAddr stateObject nil")
		return
	}
	cliAddrs := csObj.data.ServerCliAddrs
	for i, addr := range cliAddrs {
		if addr.Authorizer != userAddr {
			continue
		}
		// flow pay
		if addr.PayMethod == 1 {
			if addr.AuthAllowFlow >= data.Flow {
				log.Warn("no need", "AuthAllowFlow", addr.AuthAllowFlow, "Flow", data.Flow)
				break
			}
			cliAddrs[i].AuthAllowFlow = data.Flow
			csObj.SetServerCliAddr(cliAddrs)
			break
		}
	}
}

func (self *StateDB) UsedFlow(csAddr common.Address, usedFlowData []byte, update bool) { // qiwy: todo, need delete
	var data blizparam.CsUsedFlowParam
	if err := rlp.DecodeBytes(usedFlowData, &data); err != nil {
		log.Error("Used flow", "err", err)
		return
	}
	csObj := self.GetOrNewStateObject(csAddr)
	if csObj == nil {
		log.Error("csAddr stateObject nil")
		return
	}
	cliAddrs := csObj.data.ServerCliAddrs
	userAddr := common.HexToAddress(data.Addr)
	for _, addr := range cliAddrs {
		if addr.Authorizer != userAddr {
			continue
		}
		// flow pay
		if addr.PayMethod == 1 {
			csObj.SetServerCliAddr(cliAddrs)
			break
		}
	}
}

func (self *StateDB) PickupTime(addr common.Address, pickupTimeData []byte, update bool) {
	var data blizparam.PickupTimeParam
	if err := rlp.DecodeBytes(pickupTimeData, &data); err != nil {
		log.Error("Pickup time", "err", err)
		return
	}
	toAddr := common.HexToAddress(string(data.Addr))
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject == nil {
		log.Error("GetOrNewStateObject stateObject nil")
		return
	}
	bFound := false
	olds := stateObject.data.ServerCliAddrs
	news := make([]common.CSAuthData, 0, len(olds))
	for _, a := range olds {
		if a.Authorizer != toAddr {
			continue
		}
		if a.CSPickupTime > data.PickupTime {
			log.Error("PickupTime too much", "a.CSPickupTime", common.GetTimeString(a.CSPickupTime), "data.PickupTime", common.GetTimeString(data.PickupTime))
			continue
		}
		bFound = true
		a.CSPickupTime = data.PickupTime
		news := append(news, a)
		stateObject.SetServerCliAddr(news)
		break
	}
	if !bFound {
		log.Error("PickupTime ServerCliAddrs", "addr", addr, "toAddr", toAddr)
		return
	}
}

func (self *StateDB) PickupFlow(addr common.Address, pickupFlowData []byte, update bool) {
	var data blizparam.PickupFlowParam
	if err := rlp.DecodeBytes(pickupFlowData, &data); err != nil {
		log.Error("Pickup flow", "err", err)
		return
	}
	toAddr := common.HexToAddress(string(data.Addr))
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject == nil {
		log.Error("GetOrNewStateObject stateObject nil")
		return
	}
	bFound := false
	olds := stateObject.data.ServerCliAddrs
	news := make([]common.CSAuthData, 0, len(olds))
	for _, a := range olds {
		if a.Authorizer != toAddr {
			continue
		}
		if a.AuthAllowFlow < a.CSPickupFlow+data.PickupFlow {
			log.Error("PickupFlow too much", "AuthAllowFlow", a.AuthAllowFlow, "a.CSPickupFlow", a.CSPickupFlow, "data.PickupFlow", data.PickupFlow)
			// continue // qiwy: need delete, for test
		}
		if a.PayMethod != 1 {
			log.Warn("PickupFlow", "PayMethod", a.PayMethod)
			continue
		}
		bFound = true
		a.CSPickupFlow += data.PickupFlow
		news := append(news, a)
		stateObject.SetServerCliAddr(news)

		// get serve gctt
		amount := blizparam.GetFlowBalance(data.PickupFlow)
		self.SubBalance(toAddr, amount)
		self.AddBalance(addr, amount)
		break
	}
	if !bFound {
		log.Error("PickupFlow ServerCliAddrs", "addr", addr, "toAddr", toAddr)
		return
	}
}

func (self *StateDB) ActOperate(addr common.Address, actType uint8, actData []byte, update bool) {
	switch actType {
	case blizparam.TypeActClaim:
		self.ClaimChunk(addr, actData, update)
	case blizparam.TypeActVerify:
		self.VerifyChunk(addr, actData)
	case blizparam.TypeActRent:
		self.RentSize(addr, actData, update)
	case blizparam.TypeActSignCs:
		self.SignCs(addr, actData, update)
	case blizparam.TypeActCancelCs:
		self.CancelCs(addr, actData, update)
	case blizparam.TypeActSignTime:
		self.SignTime(addr, actData, update)
	case blizparam.TypeActSignFlow:
		self.SignFlow(addr, actData, update)
	case blizparam.TypeActPickupTime:
		self.PickupTime(addr, actData, update)
	case blizparam.TypeActPickupFlow:
		self.PickupFlow(addr, actData, update)
	case blizparam.TypeActPickupCost:
		self.PickupCost(addr, actData, update)
	case blizparam.TypeActUsedFlow:
		self.UsedFlow(addr, actData, update)
	case blizparam.TypeActShareObj:
		self.ShareObj(addr, actData, update)
	case blizparam.TypeActSendMail:
		self.SendMail(addr, actData, update)
	case blizparam.TypeActRegMail:
		self.RegMail(addr, actData, update)
	default:
	}
}

func (self *StateDB) GetSignData(addr common.Address, csAddr common.Address) []interface{} {
	stateObject := self.GetOrNewStateObject(csAddr)
	if stateObject == nil {
		return nil
	}
	for _, data := range stateObject.data.ServerCliAddrs {
		if data.Authorizer != addr {
			continue
		}
		params := make([]interface{}, 0, 1)
		params = append(params, data.EndTime)
		return params
	}
	return nil
}

func (s *StateDB) Print() {
	return

	fmt.Println("stateObjects len", len(s.stateObjects))
	for i, v := range s.stateObjects {
		fmt.Println("i", i)
		v.Print()
	}

	fmt.Println("stateObjectsDirty len", len(s.stateObjectsDirty))

	fmt.Println("stateChunks len", len(s.stateChunks))
	for i, v := range s.stateChunks {
		count := 0
		for _, k := range v.rentDB {
			if common.EmptyAddress(k) {
				continue
			}
			count++
		}
		fmt.Println(i, ":", count)
	}
	fmt.Println("stateChunksDirty len", len(s.stateChunksDirty))

	fmt.Println("chunkMaxId", s.chunkMaxId)

	fmt.Println("stateSlices len", len(s.stateSlices))
	for i, v := range s.stateSlices {
		count := 0
		for _, k := range v.slices {
			if k.SliceId == 0 {
				continue
			}
			count++
		}
		fmt.Println(i.Hex(), ":", count)
	}
	fmt.Println("stateSlicesDirty len", len(s.stateSlicesDirty))

	fmt.Println("dbErr", s.dbErr, "refund", s.refund)

	fmt.Println("thash", s.thash.Hex(), "bhash", s.bhash.Hex())
	fmt.Println("txIndex", s.txIndex)

	fmt.Println("logs len", len(s.logs), "logSize", s.logSize)

	fmt.Println("preimages len", len(s.preimages))

	fmt.Println("validRevisions len", len(s.validRevisions))

	fmt.Println("nextRevisionId", s.nextRevisionId)
}

func (s *StateDB) PrintTrieHash(str string) {
	log.Info(str, "trie", s.trie.Hash().String())
}

func (self *StateDB) ShareObj(userAddr common.Address, authorizeData []byte, update bool) {
	var objData blizparam.TxShareObjData
	if err := rlp.DecodeBytes(authorizeData, &objData); err != nil {
		log.Error("Share obj", "err", err)
		return
	}

	var codeST common.SharingCodeST
	if err := json.Unmarshal([]byte(objData.SharingCode), &codeST); err != nil {
		log.Error("Share obj", "err", err)
		return
	}

	hash := codeST.HashNoSignature()
	recoveredAddr, err := blizcore.EcRecover(hash.Bytes(), common.FromHex(codeST.Signature))
	if err != nil {
		log.Error("Share obj", "err", err)
		return
	}

	if common.HexToAddress(codeST.Receiver) != userAddr {
		log.Error("Share obj, not match", "receiver", codeST.Receiver, "useraddr", userAddr.Hex())
		return
	}
	data := &ShareObjData{
		Owner:          recoveredAddr,
		ObjID:          codeST.FileObjID,
		Offset:         codeST.FileOffset,
		Length:         codeST.FileSize,
		StartTime:      codeST.StartTime,
		EndTime:        codeST.StopTime,
		FileName:       codeST.FileName,
		SharerCSNodeID: codeST.SharerAgentNodeID,
		Key:            codeST.Key,
	}
	shareObj := self.getShareObj(userAddr)
	for _, v := range shareObj {
		if *v == *data {
			log.Info("Share obj repeat")
			return
		}
	}
	datas := append(self.shareObj[userAddr], data)
	self.setShareObj(userAddr, datas)
	self.updateShareObj(userAddr, datas)
}

func (self *StateDB) updateShareObj(addr common.Address, datas []*ShareObjData) {
	data, err := rlp.EncodeToBytes(datas)
	if err != nil {
		panic(fmt.Errorf("can't encode share obj at %v: %v", datas, err))
	}
	self.setError(self.trie.TryUpdate(getShareObjKey(addr), data))
}

func (self *StateDB) getShareObj(addr common.Address) []*ShareObjData {
	// Prefer 'live' datas.
	if datas, ok := self.shareObj[addr]; ok {
		return datas
	}
	// Load the nonce from the database.
	enc, err := self.trie.TryGet(getShareObjKey(addr))
	if len(enc) == 0 {
		self.setError(err)
		return nil
	}
	var datas []*ShareObjData
	if err := rlp.DecodeBytes(enc, &datas); err != nil {
		log.Error("Failed to decode share obj", "err", err)
		return nil
	}
	// Insert into the live set.
	self.setShareObj(addr, datas)
	return datas
}

func (self *StateDB) setShareObj(addr common.Address, datas []*ShareObjData) {
	self.shareObj[addr] = datas
}

func (self *StateDB) GetLeftRentSize() uint64 {
	var count uint64
	for chunkId := uint64(1); chunkId <= collectChunkMax; chunkId++ {
		sc := self.getStateChunk(chunkId)
		if sc == nil {
			continue
		}
		if !sc.CanUse {
			// log.Info("Get left rent, can't use", "chunk", chunkId)
			continue
		}
		for i := 0; i < storage.SliceCount; i++ {
			if !common.EmptyAddress(sc.rentDB[i]) {
				continue
			}
			count++
		}
	}
	return count * uint64(storagecore.MSize)
}

func (self *StateDB) GetUnusedChunkCnt(address common.Address, cIds []uint64) uint32 {
	var count uint32
	for chunkId := uint64(1); chunkId <= collectChunkMax; chunkId++ {
		sc := self.getStateChunk(chunkId)
		if sc == nil {
			continue
		}
		if sc.CanUse {
			continue
		}
		for _, cId := range cIds {
			if cId == chunkId {
				count++
				break
			}
		}
	}
	return count
}

func (self *StateDB) SendMail(sender common.Address, actData []byte, update bool) {
	mailStr := string(actData)
	var txMail common.TxMailData
	err := txMail.UnmarshalJson(mailStr)
	if err != nil {
		log.Error("Send mail", "err", err)
		return
	}

	// Having attachment
	if len(txMail.FileSharing) > 0 {
		var codeST common.SharingCodeST
		if err := json.Unmarshal([]byte(txMail.FileSharing), &codeST); err != nil {
			log.Error("Send mail", "err", err)
			return
		}
		if codeST.Receiver != txMail.Receiver {
			log.Error("Send mail, not match", "file sharing receiver", codeST.Receiver, "mail receiver", txMail.Receiver)
			return
		}
	}

	sDBData := &common.SDBMailData{
		Sender:      sender.Hex(),
		Title:       txMail.Title,
		Content:     txMail.Content,
		TimeStamp:   txMail.TimeStamp,
		FileSharing: txMail.FileSharing,
		Signature:   txMail.Signature,
		EncryptType: txMail.EncryptType,
		Key:         txMail.Key,
	}
	receiver := common.HexToAddress(txMail.Receiver)
	mails := self.getMailData(receiver)
	for _, mail := range mails {
		if *mail == *sDBData {
			log.Info("Send mail repeat")
			return
		}
	}

	mails = append(mails, sDBData)
	self.setMailData(receiver, mails)
	self.updateMailData(receiver, mails)
}

func (self *StateDB) updateMailData(addr common.Address, datas []*common.SDBMailData) {
	data, err := rlp.EncodeToBytes(datas)
	if err != nil {
		panic(fmt.Errorf("can't encode upate mail at %v: %v", datas, err))
	}
	self.setError(self.trie.TryUpdate(getMailDataKey(addr), data))
}

func (self *StateDB) getMailData(addr common.Address) []*common.SDBMailData {
	// Prefer 'live' datas.
	if datas, ok := self.mailData[addr]; ok {
		return datas
	}
	// Load the mail data from the database.
	enc, err := self.trie.TryGet(getMailDataKey(addr))
	if len(enc) == 0 {
		self.setError(err)
		return nil
	}
	var datas []*common.SDBMailData
	if err := rlp.DecodeBytes(enc, &datas); err != nil {
		log.Error("Failed to decode get mail", "err", err)
		return nil
	}
	// Insert into the live set.
	self.setMailData(addr, datas)
	return datas
}

func (self *StateDB) setMailData(addr common.Address, datas []*common.SDBMailData) {
	self.mailData[addr] = datas
}

func (self *StateDB) GetMails(addr common.Address) string {
	mails := self.getMailData(addr)
	log.Info("Get mails", "count", len(mails))
	data, err := json.Marshal(mails)
	if err != nil {
		log.Error("Get mails", "err", err)
		return ""
	}
	return string(data)
}

func deepCopyShareObjDatas(datas []*ShareObjData) []*ShareObjData {
	var ret []*ShareObjData
	for _, data := range datas {
		t := new(ShareObjData)
		*t = *data
		ret = append(ret, t)
	}
	return ret
}

func deepCopySDBMailDatas(datas []*common.SDBMailData) []*common.SDBMailData {
	var ret []*common.SDBMailData
	for _, data := range datas {
		t := new(common.SDBMailData)
		*t = *data
		ret = append(ret, t)
	}
	return ret
}

func (self *StateDB) RegMail(addr common.Address, actData []byte, update bool) {
	data := new(blizparam.RegMailData)
	if err := rlp.DecodeBytes(actData, data); err != nil {
		log.Error("Failed to decode reg mail", "err", err)
		return
	}
	if self.IsRegOrExisted(addr, data.Name) {
		return
	}

	regData := &SRegData{
		Name:   data.Name,
		PubKey: data.PubKey,
	}
	self.setRegData(addr, regData)
	self.updateRegData(addr, regData)

	self.updateNickname(data.Name)
}

func (self *StateDB) updateRegData(addr common.Address, regData *SRegData) {
	data, err := rlp.EncodeToBytes(regData)
	if err != nil {
		panic(fmt.Errorf("can't encode reg at %v", err))
	}
	self.setError(self.trie.TryUpdate(getRegDataKey(addr), data))
}

func (self *StateDB) getRegData(addr common.Address) *SRegData {
	// Prefer 'live' datas.
	if data, ok := self.regData[addr]; ok {
		return data
	}
	// Load the reg data from the database.
	enc, err := self.trie.TryGet(getRegDataKey(addr))
	if len(enc) == 0 {
		self.setError(err)
		return nil
	}
	data := new(SRegData)
	if err := rlp.DecodeBytes(enc, data); err != nil {
		log.Error("Decode bytes, fail", "err", err)
		return nil
	}
	// Insert into the live set.
	self.setRegData(addr, data)
	return data
}

func (self *StateDB) setRegData(addr common.Address, data *SRegData) {
	self.regData[addr] = data
}

func (self *StateDB) IsRegOrExisted(addr common.Address, name string) bool {
	reg := self.getRegData(addr)
	if reg != nil {
		log.Error("Already registered mail")
		return true
	}
	if self.getNickname(name) {
		log.Error("Already existed name")
		return true
	}
	return false
}

func (self *StateDB) updateNickname(name string) {
	self.setError(self.trie.TryUpdate(getNicknameKey(name), []byte{0}))
}

func (self *StateDB) getNickname(name string) bool {
	// Prefer 'live' datas.
	if _, ok := self.nicknames[name]; ok {
		return true
	}
	// Load the reg nickname from the database.
	enc, err := self.trie.TryGet(getNicknameKey(name))
	if len(enc) == 0 {
		self.setError(err)
		return false
	}
	// Insert into the live set.
	self.setNickname(name)
	return true
}

func (self *StateDB) setNickname(name string) {
	self.nicknames[name] = struct{}{}
}

func (self *StateDB) updateVerifyAward(addr common.Address, h uint64) {
	k := getVerifyAwardKey(addr)
	v := []byte(strconv.FormatUint(h, 10))
	e := self.trie.TryUpdate(k, v)
	self.setError(e)
}

func (self *StateDB) getVerifyAward(addr common.Address) uint64 {
	// Prefer 'live' datas.
	if v, ok := self.verifyAward[addr]; ok {
		return v
	}
	// Load the verifying award from the database.
	k := getVerifyAwardKey(addr)
	v, err := self.trie.TryGet(k)
	if len(v) == 0 {
		self.setError(err)
		return 0
	}
	v2, err := strconv.Atoi(string(v))
	if err != nil {
		self.setError(err)
		return 0
	}
	h := uint64(v2)
	// Insert into the live set.
	self.setVerifyAward(addr, h)
	return h
}

func (self *StateDB) setVerifyAward(addr common.Address, h uint64) {
	self.verifyAward[addr] = h
}
