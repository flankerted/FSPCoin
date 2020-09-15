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

package elephant

import (
	"strconv"

	//"compress/gzip"
	//"context"
	//"fmt"
	//"io"
	"math/big"
	//"os"
	//"strings"

	"github.com/contatract/go-contatract/blizparam"
	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/common/hexutil"
	"github.com/contatract/go-contatract/core/types_elephant"
	"github.com/contatract/go-contatract/rlp"
	"github.com/contatract/go-contatract/rpc"

	//core"github.com/contatract/go-contatract/core_elephant"
	//state"github.com/contatract/go-contatract/core_elephant/state"
	//"github.com/contatract/go-contatract/core/types"
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/contatract/go-contatract/blizzard/storage"
	"github.com/contatract/go-contatract/internal/elephantapi"
	"github.com/contatract/go-contatract/log"
	miner "github.com/contatract/go-contatract/miner_elephant"
)

// PublicElephantAPI provides an API to access Contatract full node-related
// information.
type PublicElephantAPI struct {
	e *Elephant
}

// NewPublicElephantAPI creates a new Contatract protocol API for full nodes.
func NewPublicElephantAPI(e *Elephant) *PublicElephantAPI {
	return &PublicElephantAPI{e}
}

// Etherbase is the address that mining rewards will be send to
func (api *PublicElephantAPI) Etherbase() (common.Address, error) {
	return api.e.Etherbase()
}

// Coinbase is the address that mining rewards will be send to (alias for Etherbase)
func (api *PublicElephantAPI) Coinbase() (common.Address, error) {
	return api.Etherbase()
}

// Hashrate returns the POW hashrate
func (api *PublicElephantAPI) Hashrate() hexutil.Uint64 {
	return hexutil.Uint64(api.e.Miner().HashRate())
}

// getBalance -- jianghan
// GetBalance returns the amount of wei for the given address in the state of the
// given block number. The rpc.LatestBlockNumber and rpc.PendingBlockNumber meta
// block numbers are also allowed.
func (s *PublicElephantAPI) GetBalance(address common.Address) (*big.Int, error) {
	state, _, err := s.e.ApiBackend.StateAndHeaderByNumber(nil, rpc.PendingBlockNumber)
	if state == nil || err != nil {
		return nil, err
	}
	b := state.GetBalance(address)
	return b, state.Error()
}

func (s *PublicElephantAPI) GetSignedCs(address common.Address) ([]common.WalletCSAuthData, error) {
	state, _, err := s.e.ApiBackend.StateAndHeaderByNumber(nil, rpc.PendingBlockNumber)
	if state == nil || err != nil {
		return nil, err
	}
	c := state.GetSignedCs(address, s.e.etherbase)
	var nodeID string
	if s.e.node != nil {
		nodeID = s.e.node.ID.String()
	}
	for i := 0; i < len(c); i++ {
		c[i].NodeID = nodeID
	}
	return c, state.Error()
}

// RPCTransaction represents a transaction that will serialize to the RPC representation of a transaction
type RPCTransaction struct {
	BlockHash        common.Hash     `json:"blockHash"`
	BlockNumber      *hexutil.Big    `json:"blockNumber"`
	From             common.Address  `json:"from"`
	Gas              hexutil.Uint64  `json:"gas"`
	GasPrice         *hexutil.Big    `json:"gasPrice"`
	Hash             common.Hash     `json:"hash"`
	Input            hexutil.Bytes   `json:"input"`
	Nonce            hexutil.Uint64  `json:"nonce"`
	To               *common.Address `json:"to"`
	TransactionIndex hexutil.Uint    `json:"transactionIndex"`
	Value            *hexutil.Big    `json:"value"`
	V                *hexutil.Big    `json:"v"`
	R                *hexutil.Big    `json:"r"`
	S                *hexutil.Big    `json:"s"`
	ActType          uint8           `json:"acttype"`
	ActData          []byte          `json:"actdata"`
}

// newRPCTransaction returns a transaction that will serialize to the RPC
// representation, with the given location metadata set (if available).
func newRPCTransaction(tx *types_elephant.Transaction, blockHash common.Hash, blockNumber uint64, index uint64) *RPCTransaction {
	var signer types_elephant.Signer = types_elephant.FrontierSigner{}
	if tx.Protected() {
		signer = types_elephant.NewEIP155Signer(tx.ChainId())
	}
	from, _ := types_elephant.Sender(signer, tx)
	v, r, s := tx.RawSignatureValues()

	result := &RPCTransaction{
		From:     from,
		Gas:      hexutil.Uint64(tx.Gas()),
		GasPrice: (*hexutil.Big)(tx.GasPrice()),
		Hash:     tx.Hash(),
		Input:    hexutil.Bytes(tx.Data()),
		Nonce:    hexutil.Uint64(tx.Nonce()),
		To:       tx.To(),
		Value:    (*hexutil.Big)(tx.Value()),
		V:        (*hexutil.Big)(v),
		R:        (*hexutil.Big)(r),
		S:        (*hexutil.Big)(s),
		ActType:  tx.ActType(),
		ActData:  tx.ActData(),
	}
	if blockHash != (common.Hash{}) {
		result.BlockHash = blockHash
		result.BlockNumber = (*hexutil.Big)(new(big.Int).SetUint64(blockNumber))
		result.TransactionIndex = hexutil.Uint(index)
	}
	return result
}

func (s *PublicElephantAPI) Encrypt(str string, passwd string) {
	s.e.Encrypt(str, passwd)
}

func (s *PublicElephantAPI) DesEncrypt(str string, passwd string) {
	s.e.DesEncrypt(str, passwd)
}

func (s *PublicElephantAPI) SendTransaction(password string, to string, value int64) (common.Hash, error) {
	return s.e.SendTransaction(password, to, value)
}

func (s *PublicElephantAPI) SendTransactions(password string, to string, value int64, times uint32) error {
	for i := uint32(0); i < times; i++ {
		_, err := s.SendTransaction(password, to, value)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *PublicElephantAPI) Help() {
	flagStr := "-----------------------------------------------------------------------------------------------------"
	titleStr := "Some main command lists are as following:"
	sideStr := "-"
	cmdList := []string{
		"",
		"eleminer.setEtherbase(etherbase common.Address)",
		"eleminer.start(threads *int)",
		"eleminer.stop()",
		"",
		"police.setEtherbase(etherbase common.Address)",
		"police.start()",
		"police.stop()",
		"",
		"blizcs.createObject(objId uint64, size uint64, password string)",
		"blizcs.addObjectStorage(objId uint64, size uint64, password string)",
		"blizcs.doSeek(objId uint64, offset uint64)",
		"blizcs.write(objId uint64, data string)",
		"blizcs.read(objId uint64, length uint32)",
		"blizcs.writeFile(objId uint64, fromFileName string)",
		"blizcs.readFile(objId uint64, length uint32, toFileName string)",
		"blizcs.getRentSize(objId uint64, length uint32)",
		"",
		"storage.claim(password string, chunkId uint64)",
		"storage.verify(password string, chunkId uint64)",
		"storage.rent(password string, size uint64, objId uint64)",
		"storage.writeRawData(chunkId uint64, sliceId uint32, offset uint32, data string, opId uint64)",
		"storage.readRawData(chunkId uint64, sliceId uint32, offset uint32, length uint32)",
		"storage.writeHeaderData(chunkId uint64, sliceId uint32, opId uint64)",
		"storage.readHeaderData(chunkId uint64, sliceId uint32)",
		"",
		"elephant.setLogFlag(flag uint32)",
		"elephant.getBalance(address common.Address)",
		"elephant.getEleTransaction(hash common.Hash)",
		"elephant.getMails(address common.Address)",
		"",
		"blizzard.deleteNode(cId uint64)",
	}
	spaceStr := []byte(" ")
	fmt.Println(flagStr)
	logStrs := make([]string, 0, 1+len(cmdList))
	logStrs = append(logStrs, titleStr)
	logStrs = append(logStrs, cmdList...)
	for _, v := range logStrs {
		spaceCount := len(flagStr) - len(v) - len(sideStr)*2
		lSpaceCount := spaceCount / 2
		rSpaceCount := spaceCount - lSpaceCount
		lSpace := make([]byte, 0, lSpaceCount)
		rSpace := make([]byte, 0, rSpaceCount)
		for i := 0; i < lSpaceCount; i++ {
			lSpace = append(lSpace, spaceStr...)
		}
		for i := 0; i < rSpaceCount; i++ {
			rSpace = append(rSpace, spaceStr...)
		}
		logStr := make([]byte, 0, len(flagStr))
		logStr = append(logStr, sideStr...)
		logStr = append(logStr, lSpace...)
		logStr = append(logStr, v...)
		logStr = append(logStr, rSpace...)
		logStr = append(logStr, sideStr...)
		fmt.Println(string(logStr))
	}
	fmt.Println(flagStr)
}

func (s *PublicElephantAPI) ExportKS(hexAddr string, passphrase string) string {
	log.Info("ExportKS", "hexAddr", hexAddr)
	addr := common.HexToAddress(hexAddr)
	return s.e.ExportKS(addr, passphrase)
}

//func (s *PublicElephantAPI) PbftReq(viewID int64) {
//	s.e.hbftNode.GetRequest(viewID, s.e.etherbase.Hex(), "")
//	go s.e.hbftNode.BlockSealedCompleted(viewID, s.e.etherbase.Hex())
//}

func (s *PublicElephantAPI) GetShardingIndex() {
	eth := s.e.GetEthHeight()
	log.Info("Get sharding index", "index", common.GetEleSharding(&s.e.etherbase, eth))
}

func (s *PublicElephantAPI) GetMails(address common.Address) (string, error) {
	state, _, err := s.e.ApiBackend.StateAndHeaderByNumber(nil, rpc.PendingBlockNumber)
	if state == nil || err != nil {
		return "", err
	}
	m := state.GetMails(address)
	return m, state.Error()
}

func (s *PublicElephantAPI) RegMail(addr common.Address, name string, password string) error {
	return s.e.RegMail(addr, name, password)
}

func (s *PublicElephantAPI) CheckReg(addr common.Address, name string) (bool, error) {
	state, _, err := s.e.ApiBackend.StateAndHeaderByNumber(nil, rpc.PendingBlockNumber)
	if state == nil || err != nil {
		return false, err
	}
	b := state.IsRegOrExisted(addr, name)
	return b, state.Error()
}

// PublicMinerAPI provides an API to control the miner.
// It offers only methods that operate on data that pose no security risk when it is publicly accessible.
type PublicMinerAPI struct {
	e     *Elephant
	agent *miner.RemoteAgent
}

// NewPublicMinerAPI create a new PublicMinerAPI instance.
func NewPublicMinerAPI(e *Elephant) *PublicMinerAPI {
	agent := miner.NewRemoteAgent(e.BlockChain(), e.Engine())
	e.Miner().Register(agent)

	return &PublicMinerAPI{e, agent}
}

// SetEtherbase sets the etherbase of the miner
func (api *PublicMinerAPI) SetEtherbase(etherbase common.Address) bool {
	api.e.SetEtherbase(etherbase)
	return true
}

// Start the miner with the given number of threads. If threads is nil the number
// of workers started is equal to the number of logical CPUs that are usable by
// this process. If mining is already running, this method adjust the number of
// threads allowed to use.
func (api *PublicMinerAPI) Start() bool {
	// Start the miner and return
	if !api.e.IsMining() {
		// Propagate the initial price point to the transaction pool
		api.e.lock.RLock()
		//price := api.e.gasPrice
		api.e.lock.RUnlock()

		//api.e.txPool.SetGasPrice(price)
		err := api.e.StartMining(true)
		return err == nil
	}
	return true
}

// Stop the miner
func (api *PublicMinerAPI) Stop() bool {
	api.e.StopMining()
	return true
}

// PublicPoliceAPI provides an API to control the police officer.
// It offers only methods that operate on data that pose no security risk when it is publicly accessible.
type PublicPoliceAPI struct {
	e *Elephant
	//agent *miner.RemoteAgent
}

// NewPublicPoliceAPI create a new PublicPoliceAPI instance.
func NewPublicPoliceAPI(e *Elephant) *PublicPoliceAPI {
	agent := miner.NewRemoteAgent(e.BlockChain(), e.Engine())
	e.Miner().Register(agent)

	return &PublicPoliceAPI{e}
}

// SetEtherbase sets the etherbase of the police officer
func (api *PublicPoliceAPI) SetEtherbase(etherbase common.Address) bool {
	api.e.SetEtherbase(etherbase)
	return true
}

// Start the police officer's work.
func (api *PublicPoliceAPI) Start() bool {
	// Start the police officer and return
	if !api.e.IsPoliceWorking() {

		err := api.e.StartPoliceWorking()
		return err == nil
	}
	return true
}

// Stop the police officer
func (api *PublicPoliceAPI) Stop() bool {
	api.e.StopPoliceWorking()
	return true
}

type StorageAPI struct {
	e *Elephant
}

func NewStorageAPI(e *Elephant) *StorageAPI {
	return &StorageAPI{e: e}
}

func IsPublicIP(IP net.IP) bool {
	if IP.IsLoopback() || IP.IsLinkLocalMulticast() || IP.IsLinkLocalUnicast() {
		return false
	}
	if ip4 := IP.To4(); ip4 != nil {
		switch true {
		case ip4[0] == 10:
			return false
		case ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31:
			return false
		case ip4[0] == 192 && ip4[1] == 168:
			return false
		default:
			return true
		}
	}
	return false
}

func GetPublicIP() []byte {
	//return net.IP("115.239.211.112")
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Error("get public ip", "err", err)
		return []byte{}
	}
	for _, address := range addrs {
		// 检查ip地址，判断是否回环地址
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			//log.Info("ip address", "ip", ipnet.IP.String())
			if IsPublicIP(ipnet.IP) {
				return ipnet.IP
			}
		}
	}
	log.Info("there is not public ip")
	return []byte{}
}

func (api *StorageAPI) Claim(password string, chunkId uint64) bool {
	api.e.Etherbase()
	claimer := api.e.etherbase
	var err error
	// if common.EmptyAddress(claimer) {
	// 	log.Error("etherbase unset")
	// 	return false
	// }
	// if !api.e.storageManager.CheckClaimFile(claimer, chunkId) {
	// 	return false
	// }
	// api.e.storageManager.LoadClaimData(claimer, chunkId)
	// content, hashs, err := api.e.storageManager.GetVerifyData(claimer, chunkId)
	// if err != nil {
	// 	log.Error("GetVerifyData", "err", err)
	// 	return false
	// }
	addrLocker := new(elephantapi.AddrLocker)
	privateAccountAPI := elephantapi.NewPrivateAccountAPI(api.e.ApiBackend, addrLocker)
	ctx := context.Background()
	var args elephantapi.SendTxArgs
	args.From = claimer
	args.To = &args.From
	args.Value = blizparam.TxDefaultCost
	args.ActType = blizparam.TypeActClaim

	claimData := storage.ClaimData{
		ChunkId: chunkId,
		// Content: content,
		// Hashs:   hashs,
		Node: api.e.node,
	}
	if args.ActData, err = rlp.EncodeToBytes(claimData); err != nil {
		log.Error("Claim", "err", err)
		return false
	}

	if strings.Compare(password, "") == 0 {
		password = "block"
	}

	_, err = privateAccountAPI.SendTransaction(ctx, args, password)
	if err == nil {
		err = api.e.AddClaim(claimer)
	}
	return err == nil
}

func (api *StorageAPI) Verify(password string, chunkId uint64, h uint64) bool {
	return api.e.Verify(password, chunkId, h)
}

func (api *StorageAPI) Rent(password string, size uint64, objId uint64) bool {
	api.e.Etherbase()
	renter := api.e.etherbase
	if common.EmptyAddress(renter) {
		log.Error("etherbase unset")
		return false
	}
	addrLocker := new(elephantapi.AddrLocker)
	privateAccountAPI := elephantapi.NewPrivateAccountAPI(api.e.ApiBackend, addrLocker)
	ctx := context.Background()
	var args elephantapi.SendTxArgs
	args.From = renter
	args.To = &args.From
	args.Value = (*hexutil.Big)(big.NewInt(5555))
	args.ActType = blizparam.TypeActRent

	rentData := storage.RentData{
		Size:  size,
		ObjId: objId,
	}
	var err error
	if args.ActData, err = rlp.EncodeToBytes(rentData); err != nil {
		log.Error("Rent", "err", err)
		return false
	}

	if strings.Compare(password, "") == 0 {
		password = "block"
	}

	privateAccountAPI.SendTransaction(ctx, args, password)
	return true
}

func (api *StorageAPI) PrintChunkData(chunkId uint64, from int, count int) {
	addr := api.e.etherbase
	api.e.storageManager.PrintChunkData(addr, chunkId, from, count)
}

func (api *StorageAPI) PrintChunkVerifyData(chunkId uint64) {
	addr := api.e.etherbase
	api.e.storageManager.PrintChunkVerifyData(addr, chunkId)
}

func (api *StorageAPI) GetChunkProvider(chunkId uint64, queryId uint64) {
	api.e.protocolManager.BroadcastChunkProvider(chunkId, queryId)
}

func (api *StorageAPI) SendBlizProtocol(msgcode uint64, params string) {
	api.e.protocolManager.BroadcastBlizProtocol(msgcode, params)
}

func (api *StorageAPI) WriteRawData(chunkId uint64, sliceId uint32, offset uint32, data string, opId uint64) {
	api.e.storageManager.WriteRawData(chunkId, sliceId, offset, []byte(data), opId, api.e.etherbase)
}

func (api *StorageAPI) ReadRawData(chunkId uint64, sliceId uint32, offset uint32, length uint32) string {
	ret, err := api.e.storageManager.ReadRawData(chunkId, sliceId, offset, length)
	if err != nil {
		return ""
	}
	return string(ret)
}

func (api *StorageAPI) WriteHeaderData(chunkId uint64, sliceId uint32, opId uint64) {
	api.e.storageManager.WriteHeaderData(chunkId, sliceId, opId)
}

func (api *StorageAPI) ReadHeaderData(chunkId uint64, sliceId uint32) {
	api.e.storageManager.ReadHeaderData(chunkId, sliceId)
}

func (api *StorageAPI) LogHeaderData(chunkId uint64, fromSlice, toSlice uint32) {
	api.e.storageManager.LogHeaderData(chunkId, fromSlice, toSlice)
}

func (api *StorageAPI) GetChunkIdList() string {
	var result string
	idList := storage.GetChunkIdList()
	log.Info("GetChunkIdList", "idList", idList)
	for i, _ := range idList {
		if i == 0 {
			result += "chunk id list as following: "
		} else {
			result += " ,"
		}
		result += strconv.Itoa(int(idList[i]))
	}
	return result
}
