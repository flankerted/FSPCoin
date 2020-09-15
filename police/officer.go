// Copyright 2018 The go-contatract Authors
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

package police

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/core/types"
	"github.com/contatract/go-contatract/core/types_elephant"
	"github.com/contatract/go-contatract/eleWallet/utils"
	"github.com/contatract/go-contatract/event"
	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/rpc"
)

const (
	workSimultaneouslySize = 64
)

var policePassphrase string

func SetPolicePassphrase(passphrase string) {
	policePassphrase = passphrase
}

func GetPolicePassphrase() string {
	return policePassphrase
}

type HeaderWithHeight struct {
	ShardingID  uint16
	LampBase    *common.Hash
	Header      *common.Hash
	BlockNumber uint64

	PreviousHeaders []*common.Hash // Hashes of the verified headers from last to this one
}

func (match *HeaderWithHeight) BlockNum() uint64 {
	return match.BlockNumber
}

func (match *HeaderWithHeight) BlockHash() common.Hash {
	return *match.Header
}

// Work is the officer task that shall be verified
type Work struct {
	ShardingTxBatch *types_elephant.ShardingTxBatch
}

type Result struct {
	Match *HeaderWithHeight // the sharding header's hash matching with the tx root hash fetched from the liaison
	Sign  string
}

// Backend wraps all methods required for verifying.
type Backend interface {
	GetEleHeaderByNumber(number uint64) *types_elephant.Header
	GetEleHeadersFromPeer(from uint64, amount int, shardingID uint16, lampNum uint64) ([]*types_elephant.Header, error)
	SignVerifyResult(data []byte, passphrase string) ([]byte, error)
	BroadcastPoliceVerify(result *Result)
	DeliverVerifiedResult(result *Result)
}

// Lamp wraps all methods required for verifying.
type Lamp interface {
	GetValidHeadersByBlockNum(blockNum uint64) map[uint16]*types.VerifiedValid
	CurrEleSealersBlockNum() rpc.BlockNumber
}

// officer is the main object which takes care of applying messages to the new state
type Officer struct {
	//config *params.ChainConfig
	elephant Backend
	lamp     Lamp

	mu  sync.Mutex
	mux *event.TypeMux

	// update loop
	workCh        chan *Work
	stop          chan struct{}
	quitCurrentOp chan struct{}
	//returnCh      chan<- *Result

	recv chan *Result

	//eth     Backend
	//proc    core.Validator
	//chainDb ethdb.Database

	coinbase common.Address

	currentMu sync.Mutex
	current   []*common.Hash                    // the hash of block being simultaneously verified
	completed map[common.Hash]*HeaderWithHeight // the block hash(key) with verified match(value) which stores the work done before
	highest   map[uint16]uint64                 // the highest height of the block in the completed

	//unconfirmed *unconfirmedBlocks // set of locally mined blocks pending canonicalness confirmations

	// atomic status counters
	readyToWork int32
	working     int32
}

func New(elephant Backend, eth Lamp) *Officer {
	officer := &Officer{
		elephant:  elephant,
		lamp:      eth,
		workCh:    make(chan *Work, workSimultaneouslySize),
		recv:      make(chan *Result),
		stop:      make(chan struct{}),
		completed: make(map[common.Hash]*HeaderWithHeight),
		highest:   make(map[uint16]uint64),
	}

	go officer.wait()

	return officer
}

func (self *Officer) PushWork(work *Work) {
	self.mu.Lock()
	defer self.mu.Unlock()
	self.currentMu.Lock()
	defer self.currentMu.Unlock()

	blockHash := work.ShardingTxBatch.Hash()
	for _, hash := range self.current {
		if hash.Equal(blockHash) {
			return
		}
	}
	blockHeight := work.ShardingTxBatch.BlockHeight
	blockShardingID := work.ShardingTxBatch.ShardingID
	for hash, completed := range self.completed {
		if hash.Equal(blockHash) {
			return
		} else {
			if blockShardingID == completed.ShardingID && blockHeight == completed.BlockNumber {
				// zzr TODO 惩罚机制 需要加挖块者的签名
				return
			}
		}
	}

	if ch := self.Work(); ch != nil {
		ch <- work
		self.current = append(self.current, &blockHash)
		atomic.AddInt32(&self.working, 1)
	}
}

func (self *Officer) markComplete(work *Work, result *Result) {
	self.currentMu.Lock()
	blockHash := work.ShardingTxBatch.Hash()
	for i, hash := range self.current {
		if hash.Equal(blockHash) {
			self.current = append(self.current[:i], self.current[i+1:]...)
		}
	}
	atomic.AddInt32(&self.working, -1)
	self.currentMu.Unlock()

	self.completed[blockHash] = result.Match
	if highest, ok := self.highest[work.ShardingTxBatch.ShardingID]; ok {
		if highest < work.ShardingTxBatch.GetBlockHeight() {
			self.highest[work.ShardingTxBatch.ShardingID] = work.ShardingTxBatch.GetBlockHeight()
		}
	} else {
		self.highest[work.ShardingTxBatch.ShardingID] = work.ShardingTxBatch.GetBlockHeight()
	}
}

func (self *Officer) Start(coinbase common.Address) {
	self.mu.Lock()
	defer self.mu.Unlock()

	self.coinbase = coinbase
	atomic.AddInt32(&self.readyToWork, 1)

	go self.updateLoop()
}

func (self *Officer) updateLoop() {
out:
	for {
		select {
		case work := <-self.workCh:
			self.mu.Lock()
			if self.quitCurrentOp != nil {
				close(self.quitCurrentOp)
			}
			self.quitCurrentOp = make(chan struct{})
			go self.work(work, self.quitCurrentOp)
			self.mu.Unlock()

		case <-self.stop:
			self.mu.Lock()
			if self.quitCurrentOp != nil {
				close(self.quitCurrentOp)
				self.quitCurrentOp = nil
			}
			self.mu.Unlock()
			break out
		}
	}
}

func (self *Officer) wait() {
	for {
		for result := range self.recv {

			if result == nil {
				continue
			}

			go self.elephant.DeliverVerifiedResult(result)

			// Broadcast the block and announce chain insertion event
			self.elephant.BroadcastPoliceVerify(result)
		}
	}
}

func (self *Officer) Work() chan<- *Work { return self.workCh }

//func (self *Officer) SetReturnCh(ch chan<- *Result) { self.returnCh = ch }
func (self *Officer) IsBusying() bool {
	return atomic.LoadInt32(&self.working) > 0
}

func (self *Officer) SetEtherbase(coinbase common.Address) {
	self.mu.Lock()
	defer self.mu.Unlock()

	self.coinbase = coinbase
}

func (self *Officer) Stop() {
	self.mu.Lock()
	defer self.mu.Unlock()

	if !atomic.CompareAndSwapInt32(&self.readyToWork, 1, 0) {
		return // agent already stopped
	}
	self.stop <- struct{}{}
	atomic.AddInt32(&self.readyToWork, -1)
done:
	// Empty work channel
	for {
		select {
		case <-self.workCh:
		default:
			break done
		}
	}
}

func (self *Officer) work(work *Work, stop <-chan struct{}) {
	if result, err := self.verify(work, stop); result.Match != nil {
		self.markComplete(work, result)
		log.Info("【✪】Police: Successfully verified new header", "block shard ID", result.Match.ShardingID,
			"block number", result.Match.BlockNumber, "block hash", result.Match.BlockHash().String())
		self.recv <- result
	} else {
		if err != nil {
			log.Warn("【✪】Police: block verifying failed", "block shard ID", result.Match.ShardingID,
				"block number", result.Match.BlockNumber, "err", err)
		}
		self.recv <- nil
	}
}

func (self *Officer) getStartHeight(work *Work) uint64 {
	verifiedHeight := self.lamp.GetValidHeadersByBlockNum(work.ShardingTxBatch.LampNum)[work.ShardingTxBatch.ShardingID].BlockHeight
	if highestHeight, ok := self.highest[work.ShardingTxBatch.ShardingID]; ok {
		if highestHeight > verifiedHeight {
			return highestHeight + 1
		} else {
			return verifiedHeight + 1
		}
	}

	return verifiedHeight + 1
}

func (self *Officer) getHeaders(shardingID uint16, txBatchHeight, from uint64, amount int, lampNumber uint64) ([]*types_elephant.Header, error) {
	myShardingID := common.GetSharding(self.coinbase, lampNumber)
	if myShardingID == shardingID {
		var (
			headers []*types_elephant.Header
			origin  *types_elephant.Header
		)
		for len(headers) < amount {
			origin = self.elephant.GetEleHeaderByNumber(txBatchHeight)
			if origin == nil {
				break
			}
			headers = append(headers, origin)
			txBatchHeight -= 1
		}
		return headers, nil
	} else {
		return self.elephant.GetEleHeadersFromPeer(txBatchHeight, int(txBatchHeight-from+1), shardingID, uint64(self.lamp.CurrEleSealersBlockNum()))
	}
}

func (self *Officer) verify(work *Work, stop <-chan struct{}) (*Result, error) {
	var thisBlockHeader *types_elephant.Header
	from := uint64(0)
	lampNumber := work.ShardingTxBatch.LampNum
	shardingID := work.ShardingTxBatch.ShardingID
	CurrValidHeaders := self.lamp.GetValidHeadersByBlockNum(uint64(self.lamp.CurrEleSealersBlockNum()))
	if CurrValidHeaders == nil {
		return &Result{}, errors.New("can not get verified valid headers from the lamp chain")
	}
	if CurrentValid, ok := CurrValidHeaders[shardingID]; !ok {
		return &Result{}, errors.New("can not get height of verified valid headers from the lamp chain")
	} else {
		from = CurrentValid.BlockHeight
	}

	previous := make(map[uint64]*types_elephant.Header, 0)
	previousHeaders := make([]*common.Hash, 0)
	blockHeight := work.ShardingTxBatch.Height()
	if headers, err := self.getHeaders(shardingID, blockHeight, from, int(blockHeight-from+1),
		uint64(self.lamp.CurrEleSealersBlockNum())); err == nil {
		blockHashInBatch := work.ShardingTxBatch.BlockHash
		OutTxsRootHash := work.ShardingTxBatch.OutTxsRoot
		lampBase := work.ShardingTxBatch.LampBase

		for _, header := range headers {
			BlockHash := header.Hash()
			if !BlockHash.Equal(*blockHashInBatch) {
				previous[header.Number.Uint64()] = header
			} else {
				if !BlockHash.Equal(*blockHashInBatch) || !header.OutputTxHash.Equal(*OutTxsRootHash) ||
					!header.LampBaseHash.Equal(*lampBase) || header.LampBaseNumber.Uint64() != lampNumber || !work.ShardingTxBatch.IsValid() {
					return &Result{}, errors.New("the sharding transactions are not valid in it's block")
				}
				if shardingID != common.GetSharding(header.Coinbase, header.LampBaseNumber.Uint64()) {
					return &Result{}, errors.New("the sharding ID of the transactions is not correct")
				}
				thisBlockHeader = header
			}
		}

		if thisBlockHeader == nil {
			return &Result{}, errors.New("can not get the working block in it's shard")
		}

		for h := blockHeight - 1; h >= from; h-- {
			if previousHead, ok := previous[h]; ok {
				previousHash := previousHead.Hash()
				if (h == blockHeight-1 && !previousHash.Equal(thisBlockHeader.ParentHash)) ||
					(h < blockHeight-1 && !previousHash.Equal(previous[h+1].ParentHash)) {
					return &Result{}, errors.New(fmt.Sprintf("can not get correct previous block number %d in the sharding", h))
				}
				previousHeaders = append(previousHeaders, &previousHash)
				if h == 0 {
					break
				}
			} else {
				return &Result{}, errors.New(fmt.Sprintf("can not get previous block number %d in the sharding", h))
			}
		}

		match := HeaderWithHeight{
			ShardingID:      shardingID,
			LampBase:        lampBase,
			Header:          blockHashInBatch,
			BlockNumber:     blockHeight,
			PreviousHeaders: previousHeaders,
		}

		// Sign result
		dataHash := utils.RlpHash([]interface{}{match.ShardingID, match.LampBase, match.Header, match.BlockNumber, match.PreviousHeaders}).Bytes()
		sigrsv, err := self.elephant.SignVerifyResult(dataHash, GetPolicePassphrase())
		if err != nil {
			return &Result{}, err
		}

		return &Result{&match, common.ToHex(sigrsv)}, nil
	} else {
		return &Result{}, err
	}
}
