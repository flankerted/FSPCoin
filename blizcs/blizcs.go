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

package blizcs

import (
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/contatract/go-contatract/accounts"
	"github.com/contatract/go-contatract/blizparam"

	"github.com/contatract/go-contatract/blizzard"
	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/common/datacrypto"
	"github.com/contatract/go-contatract/common/param"
	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/node"
	"github.com/contatract/go-contatract/p2p"
	"github.com/contatract/go-contatract/rpc"

	"github.com/contatract/go-contatract/blizzard/storage"
	"github.com/contatract/go-contatract/p2p/discover"

	"github.com/contatract/go-contatract/blizcore"
	"github.com/contatract/go-contatract/elephant/exporter"

	"github.com/contatract/go-contatract/blizzard/storagecore"
	"github.com/contatract/go-contatract/event"
	"github.com/contatract/go-contatract/sharding"
)

const (
	SliceDBVersion = 1
	CsDBVersion    = 1
)

type SliceSegmentHash struct {
	WopId uint64
	Hash  common.DataHash
	Err   error
}

type SliceSegmentData struct {
	WopId uint64
	Data  []byte
	Err   error
}

type FarmerSliceOp struct {
	ChunkId uint64
	SliceId uint32
	OpId    uint64
}

type UserAuthData struct {
	CsAddr   common.Address
	UserAddr common.Address
	Deadline uint64
	R, S, V  *big.Int
}

type UserAuthAllowFlowData struct {
	CsAddr common.Address
	Flow   uint64
	Sign   []byte
}

// 一个集合了各个
type BlizClntObject struct {
	blizcs *BlizCS
	addr   common.Address
	id     uint64
	name   string
	meta   *blizcore.BlizObjectMeta

	// map{"chunkId" ==> map{"nodeId" ==> *Peer} }
	chunkFarmers      map[storage.ChunkIdentity]map[discover.NodeID]*Peer
	chunkFarmerDailTs map[storage.ChunkIdentity]uint64

	bFarmerReg bool // 涉及的 farmer 是否已经登记
	farmerlock sync.RWMutex

	offset       uint64 // 当前读写指针
	shutdownChan chan struct{}

	//----------

	//map["chunkId:sliceId"][nodeId]*slicedetail
	localSliceMap       map[string]*blizcore.WopInfo
	farmerSliceMaps     map[string]map[discover.NodeID]*blizcore.WopData
	farmerSliceMapsLock sync.RWMutex

	// farmerSliceOpChan chan *FarmerSliceOp
	slicedb  *sliceDB
	signPeer string

	scope          event.SubscriptionScope
	sliceReadSub   event.Subscription
	sliceReadFeed  event.Feed
	sliceWriteSub  event.Subscription
	sliceWriteFeed event.Feed
}

func (clnt *BlizClntObject) Meta() *blizcore.BlizObjectMeta {
	return clnt.meta
}

func UniqueId(addr common.Address, id uint64) string {
	return addr.ToHex() + strconv.FormatInt(int64(id), 10)
}

func (clnt *BlizClntObject) ID() string {
	return UniqueId(clnt.addr, clnt.id)
}

func (clnt *BlizClntObject) Prepare() (bool, error) {
	// 1. 将元数据 meta 中的 chunk 注册到 clnt 中
	for _, slice := range clnt.meta.SliceInfos {
		clnt.RegisterChunk(storage.Int2ChunkIdentity(slice.ChunkId))
	}

	// 2. 查询各个 chunk 对应的 farmers 列表
	clnt.Log().Info("Prepare", "chunkFarmers len", len(clnt.chunkFarmers))
	errc := make(chan error, len(clnt.chunkFarmers))

	clnt.farmerlock.Lock()
	querycnt := len(clnt.chunkFarmers)
	cIds := make([]storage.ChunkIdentity, 0, querycnt)
	for i, _ := range clnt.chunkFarmers {
		clnt.Log().Info("", "i", i)
		cIds = append(cIds, i)
	}
	for _, chunkId := range cIds {
		cId := chunkId
		// go func(clnt *BlizClntObject, cId storage.ChunkIdentity) {
		farmers, err := clnt.blizcs.queryFarmer(cId, clnt.ID(), blizparam.QueryFarmerTimeout)
		if farmers == nil {
			if err == nil {
				err = blizcore.ErrRetriveFarmerFail
			}
			errc <- err
			// return
			break
		}
		isChunkFarmer := storage.HasChunk(storage.ChunkIdentity2Int(cId))
		otherFarmerCnt := blizparam.GetOtherMinFarmerCount(isChunkFarmer)
		if len(farmers) < otherFarmerCnt {
			err = blizcore.ErrInsufficientFarmers
			errc <- err
			// return
			break
		}
		for _, farmer := range farmers {
			clnt.RegisterFarmer(cId, farmer.ID)
		}
		errc <- nil
		// return
		// }(clnt, cId)
	}
	clnt.farmerlock.Unlock()

	for i, _ := range clnt.chunkFarmers {
		clnt.Log().Info("", "i", i)
	}

	timeout := time.NewTimer(handshakeTimeout)
	defer timeout.Stop()
_wait:
	for i := 0; i < querycnt; i++ {
		select {
		case err := <-errc:
			if err != nil {
				return false, err
			}
			// 全部正确
			if i+1 == querycnt {
				clnt.Log().Info("Prepare ok")
				break _wait
			}
		case <-timeout.C:
			clnt.Log().Warn("Prepare timeout")
			return false, blizcore.ErrQueryFarmerTimeout
		}
	}

	clnt.bFarmerReg = true

	// 3.从 sliceDB 中加载本地缓存的 Wop 信息
	clnt.LoadSliceInfos()
	return true, nil
}

func (clnt *BlizClntObject) LoadSliceInfos() {
	for _, info := range clnt.meta.SliceInfos {
		sliceRWInfo := clnt.LoadSliceInfo(&info)
		if sliceRWInfo != nil {
			clnt.localSliceMap[info.String()] = sliceRWInfo
		}
	}
}

// 将 farmer 的 nodeID 注册进去，等待它 Login
func (clnt *BlizClntObject) RegisterFarmer(chunkId storage.ChunkIdentity, nodeId discover.NodeID) bool {
	// clnt.farmerlock.Lock()
	// defer clnt.farmerlock.Unlock()

	clnt.Log().Info("RegisterFarmer", "chunkId", chunkId, "nodeId", nodeId)
	if _, ok := clnt.chunkFarmers[chunkId]; !ok {
		clnt.chunkFarmers[chunkId] = make(map[discover.NodeID]*Peer, 0)
	}

	if _, ok := clnt.chunkFarmers[chunkId][nodeId]; ok {
		return true
	}

	clnt.chunkFarmers[chunkId][nodeId] = nil
	return true
}

func (clnt *BlizClntObject) IsFarmerRegistered(chunkId storage.ChunkIdentity, nodeId discover.NodeID) bool {
	clnt.farmerlock.RLock()
	defer clnt.farmerlock.RUnlock()

	if _, ok := clnt.chunkFarmers[chunkId]; !ok {
		return false
	}
	if _, ok := clnt.chunkFarmers[chunkId][nodeId]; !ok {
		return false
	}
	return true
}

func (clnt *BlizClntObject) RegisterChunk(chunkId storage.ChunkIdentity) bool {
	clnt.farmerlock.Lock()
	defer clnt.farmerlock.Unlock()

	if _, ok := clnt.chunkFarmers[chunkId]; !ok {
		clnt.Log().Info("RegisterChunk", "chunkId", storage.ChunkIdentity2Int(chunkId))
		clnt.chunkFarmers[chunkId] = make(map[discover.NodeID]*Peer, 0)
	}
	return true
}

func (clnt *BlizClntObject) IsChunkRegistered(chunkId storage.ChunkIdentity) bool {
	clnt.farmerlock.RLock()
	defer clnt.farmerlock.RUnlock()

	if _, ok := clnt.chunkFarmers[chunkId]; !ok {
		return false
	}
	return true
}

// Login
func (clnt *BlizClntObject) LoginFarmer(peer *Peer) bool {
	clnt.farmerlock.Lock()
	defer clnt.farmerlock.Unlock()
	clnt.Log().Info("LoginFarmer", "clnt.chunkFarmers len", len(clnt.chunkFarmers))
	for _, smap := range clnt.chunkFarmers {
		if _, ok := smap[peer.ID()]; ok {
			smap[peer.ID()] = peer
		}
	}
	req := &blizcore.FarmerSliceOpReq{ObjUId: clnt.ID(), SliceInfos: clnt.meta.SliceInfos}
	peer.SendFarmerSliceOp(req)

	return true
}

func (clnt *BlizClntObject) LogoutFarmer(peer *Peer) {
	clnt.farmerlock.Lock()
	defer clnt.farmerlock.Unlock()
	for _, smap := range clnt.chunkFarmers {
		if _, ok := smap[peer.ID()]; ok {
			smap[peer.ID()] = nil
		}
	}
}

func (clnt *BlizClntObject) getLinkedFarmerCount(chunkId storage.ChunkIdentity) int {
	clnt.farmerlock.RLock()
	defer clnt.farmerlock.RUnlock()

	var cnt = 0
	if smap, ok := clnt.chunkFarmers[chunkId]; ok {
		for _, peer := range smap {
			if peer != nil {
				cnt++
			}
		}
	}
	return cnt
}

func (clnt *BlizClntObject) getFarmer(chunkId *storage.ChunkIdentity, nodeId *discover.NodeID) *Peer {
	clnt.farmerlock.RLock()
	defer clnt.farmerlock.RUnlock()
	clnt.Log().Info("Get Farmer", "farmercount", len(clnt.chunkFarmers), "chunkid", storage.ChunkIdentity2Int(*chunkId), "nodeid", nodeId)
	if smap, ok := clnt.chunkFarmers[*chunkId]; ok {
		return smap[*nodeId]
	}
	return nil
}

func (clnt *BlizClntObject) dailFarmers(pDesChunkId *storage.ChunkIdentity, wantLinkCnt int, bNewFarmer bool) (bool, error) {
	wantLinkCnt = 3 // qiwy: TODO
	clnt.farmerlock.RLock()
	defer clnt.farmerlock.RUnlock()

	now := uint64(time.Now().Unix())

	clnt.Log().Info("dailFarmers", "clnt.chunkFarmers len", len(clnt.chunkFarmers))
	for chunkId, smap := range clnt.chunkFarmers {
		clnt.Log().Info("dailFarmers", "chunkId", chunkId, "smap", smap)
		farmerCnt := len(smap)

		if pDesChunkId == nil || chunkId.Equal(*pDesChunkId) == false {
			continue
		}
		cnt := clnt.getLinkedFarmerCount(chunkId)

		// 刷新每个 chunk 的 farmer dail 时间戳，防止频繁拨号
		if ts, ok := clnt.chunkFarmerDailTs[*pDesChunkId]; ok {
			if now-ts < 15 {
				// continue // qiwy: TODO
			}
		}
		clnt.chunkFarmerDailTs[*pDesChunkId] = now

		if bNewFarmer {
			wantLinkCnt = wantLinkCnt + cnt
		}

		if farmerCnt < wantLinkCnt {
			log.Info("Dail farmers", "farmercnt", farmerCnt, "wantlinkcnt", wantLinkCnt)
			// return false, blizcore.ErrInsufficientFarmers
		}

		if cnt < wantLinkCnt {
			// TODO: dail
			beginIdx := 0 // rand.Intn(farmerCnt)

			cnt := 0
			dailcnt := cnt
			clnt.Log().Info("dailFarmers", "smap len", len(smap), "beginIdx", beginIdx)
			for nodeId, peer := range smap {
				if cnt < beginIdx {
					cnt++
					continue
				}
				if peer == nil {
					clnt.blizcs.Dail(&nodeId)
					dailcnt++
					if dailcnt >= wantLinkCnt {
						break
					}
				}
			}
			// if dailcnt < wantLinkCnt {
			// 	for nodeId, peer := range smap {
			// 		if peer == nil {
			// 			clnt.blizcs.Dail(&nodeId)
			// 			dailcnt++
			// 			if dailcnt >= wantLinkCnt {
			// 				break
			// 			}
			// 		}
			// 	}
			// }
		}
	}
	return true, nil
}

func (clnt *BlizClntObject) Close() {
	clnt.scope.Close()
	close(clnt.shutdownChan)
}

func (clnt *BlizClntObject) DoSeek(offset uint64) int64 {
	clnt.offset = offset
	return 0
}

/*
 * 同步思路
1. tenant 同时连接 2-3 名 farmer 进行数据写入操作（我们这里称这几个farmer为一级farmer)
2. 每次写入操作都要求对方将写入结果（slice的各个segment的merkle树根hash）反馈回来，要求它们一致写入操作方可以认为成功
3. 写入成功后，tenant需要为这个成功结果hash和这个Wop进行签名（注意，之前的写入操作不做签名处理，真正的签名这一步才做），然后使用这个签名后的Signed(WopId+MerkleHash)交给这2-3名验证成功的 farmer ，让它们对其他 farmer 进行传递，同时更新自己的 slice 进度，这样对于这2-3名farmer而言，它们就拥有了Wop以及被tenant认证的result merkle root hash，它们需要将Signed(WopId+MerkleHash)这个信息放入自己本地的缓冲队列中保留（甚至需要写入leveldb磁盘文件）。
4. 同步心跳每 5 秒执行一次，也就是每5秒，这2-3名 farmer 都将自己最新的Slice Wop Prog列表广播给自己连接的下线 farmer
5. 下线 farmer 接收到消息后，通过对比本地进度确定是否需要向这2-3名上级发起数据同步，如果是则发起请求
6. 一级 farmer 接收到请求后，开始传递数据（在传递数据过程中，一级farmer新收到的Wop，也需要一并往下线 farmer 传递）
7. 下线 farmer 接收并提交完请求的数据后，对各个 segment 进行计算，然后向一级 farmer 请求它应用的最近Wop的Signed(WopId+MerkleHash)信息，然后对比本地各个 segment 计算出来的 merklehash 结果，看看是否存在偏差，如果一致，则更新自身进度为当前WopId，如果不一致，则将自身状态修改为需要全局同步（需要重新同步整体）。
*/

func (clnt *BlizClntObject) Write(data []byte) (int, error) {
	return clnt.WriteSlices(data)
}

func (clnt *BlizClntObject) Read(buffer []byte) error {
	return clnt.ReadSlices(buffer)
}

func (clnt *BlizClntObject) SetSignPeer(peerId string) {
	clnt.signPeer = peerId
}

func (clnt *BlizClntObject) GetSignPeer() string {
	return clnt.signPeer
}

type BlizCS struct {
	bliz *blizzard.Bliz // BlizCS与Bliz暂时1对1，以后会n对1

	csbase         common.Address
	basePrivateKey *ecdsa.PrivateKey

	//objects map[string]*BlizClntObject

	// Name should contain the official protocol name,
	// often a three-letter word.
	Name string

	// Version should contain the version number of the protocol.
	Version string

	shutdownChan chan struct{} // Channel for shutting down the service

	// Logger is a custom logger to use with the blizzard Server.
	log log.Logger `toml:",omitempty"`

	config *Config

	nodeserver      *p2p.Server
	protocolManager *ProtocolManager
	accountManager  *accounts.Manager

	sliceDBDir string

	chunkFarmerChan chan *blizzard.ChunkNodes
	farmerQueryMap  map[string]*event.Feed

	shardingMinerChan chan *blizzard.ShardingNodes
	minerQueryMap     map[string]*event.Feed

	objectMap sync.Map // map[string]*BlizClntObject // ID ==> BlizClntObject 同时发起的所有clntObject

	openObjectLock sync.RWMutex
	mapOpenObject  map[string]*sync.RWMutex

	wg              sync.WaitGroup
	lock            sync.RWMutex
	registeredPeers map[discover.NodeID]*Peer

	userAuthMap          sync.Map // map[string]*UserAuthData
	usedCostMap          map[common.Address]uint64
	userAuthAllowFlowMap map[common.Address]*UserAuthAllowFlowData
	csdb                 *csDB
	// waitingClaimIds      []uint64

	elephantToCsDataCh  chan *exporter.ObjElephantToCsData
	elephantToCsDataSub event.Subscription

	elephantToBlizcsSliceInfoCh  chan *exporter.ObjElephantToBlizcsSliceInfo
	elephantToBlizcsSliceInfoSub event.Subscription

	buffFromSharerCSCh  chan *exporter.ObjBuffFromSharerCS
	buffFromSharerCSSub event.Subscription

	scope event.SubscriptionScope
}

// New creates a new P2P node, ready for protocol registration.
func New(ctx *node.ServiceContext, config *Config) (*BlizCS, error) {

	var bliz *blizzard.Bliz
	if err := ctx.Service(&bliz); err != nil {
		log.Error("[New BlizCS Service failed!] : Blizzard Service not running: %v", err)
		return nil, err
	}

	// sliceDBPath := ctx.ResolvePath("sliceinfos")
	// slicedb, err := CreateSliceDB(sliceDBPath, SliceDBVersion)
	// if err != nil {
	// 	return nil, err
	// }
	csDBPath := ctx.ResolvePath("csinfo")
	csdb, err := CreateCsDB(csDBPath, CsDBVersion)
	if err != nil {
		log.Error("CreateCsDB", "err", err)
		return nil, err
	}

	if config.Logger == nil {
		config.Logger = log.New()
	}

	// Note: any interaction with Config that would create/touch files
	// in the data directory or instance directory is delayed until Start.
	cs := &BlizCS{
		Name:    config.Name,
		Version: config.Version,
		log:     config.Logger,
		config:  config,

		sliceDBDir: ctx.ResolvePath("sliceinfos"),
		bliz:       bliz,

		chunkFarmerChan: make(chan *blizzard.ChunkNodes),
		farmerQueryMap:  make(map[string]*event.Feed),

		shardingMinerChan: make(chan *blizzard.ShardingNodes),
		minerQueryMap:     make(map[string]*event.Feed),

		mapOpenObject: make(map[string]*sync.RWMutex),

		shutdownChan: make(chan struct{}),

		csbase: common.Address{},

		accountManager:       ctx.AccountManager,
		registeredPeers:      make(map[discover.NodeID]*Peer),
		usedCostMap:          make(map[common.Address]uint64),
		userAuthAllowFlowMap: make(map[common.Address]*UserAuthAllowFlowData),
		csdb:                 csdb,
		// waitingClaimIds:      make([]uint64, 0),
	}

	terminalHandle := log.LvlFilterHandler(log.LvlWarn, log.GetDefaultTermHandle(2))
	path := ctx.ResolvePath("blizcs.txt")
	fileHandle := log.LvlFilterHandler(log.LvlInfo, log.GetDefaultFileHandle(path))
	cs.log.SetHandler(log.MultiHandler(terminalHandle, fileHandle))
	cs.Log().Info("Initialising Blizzard Client Service protocol", "versions", ProtocolVersions, "network", config.NetworkId)

	if cs.protocolManager, err = NewProtocolManager(config.NetworkId, nil, cs); err != nil {
		return nil, err
	}

	cs.CalculateSelfBase(blizparam.GetCsSelfPassphrase())
	return cs, nil
}

func NewForTest(bliz *blizzard.Bliz, accMan *accounts.Manager, dataDir, passphrase string, config *Config) (*BlizCS, error) {
	// sliceDBPath := ctx.ResolvePath("sliceinfos")
	// slicedb, err := CreateSliceDB(sliceDBPath, SliceDBVersion)
	// if err != nil {
	// 	return nil, err
	// }
	csdb, err := CreateCsDB("", CsDBVersion)
	if err != nil {
		log.Error("CreateCsDB", "err", err)
		return nil, err
	}

	if config.Logger == nil {
		config.Logger = log.New()
	}

	// Note: any interaction with Config that would create/touch files
	// in the data directory or instance directory is delayed until Start.
	cs := &BlizCS{
		Name:    config.Name,
		Version: config.Version,
		log:     config.Logger,

		sliceDBDir: filepath.Join(dataDir, "sliceinfos"), // ctx.ResolvePath("sliceinfos"),
		bliz:       bliz,

		chunkFarmerChan: make(chan *blizzard.ChunkNodes),
		farmerQueryMap:  make(map[string]*event.Feed),

		shardingMinerChan: make(chan *blizzard.ShardingNodes),
		minerQueryMap:     make(map[string]*event.Feed),

		shutdownChan: make(chan struct{}),

		csbase: common.Address{},

		accountManager:       accMan,
		registeredPeers:      make(map[discover.NodeID]*Peer),
		usedCostMap:          make(map[common.Address]uint64),
		userAuthAllowFlowMap: make(map[common.Address]*UserAuthAllowFlowData),
		csdb:                 csdb,
		// waitingClaimIds:      make([]uint64, 0),
	}

	if cs.protocolManager, err = NewProtocolManager(config.NetworkId, nil, cs); err != nil {
		return nil, err
	}

	cs.CalculateSelfBase(passphrase)
	return cs, nil
}

// CreateDB creates the chain database.
func CreateSliceDB(sliceDBPath string, version int) (*sliceDB, error) {
	// If no node database was given, use an in-memory one
	db, err := newSliceDB(sliceDBPath, version)
	if err != nil {
		return nil, err
	}

	//db, err := ctx.OpenDatabase(name, config.DatabaseCache, config.DatabaseHandles)
	//if err != nil {
	//	return nil, err
	//}

	//if db, ok := db.(*ethdb.LDBDatabase); ok {
	//	db.Meter("blizzard/db/blizdata/")
	//}
	return db, nil
}

func CreateCsDB(csDBPath string, version int) (*csDB, error) {
	db, err := newCsDB(csDBPath, version)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func (clnt *BlizClntObject) LoadSliceInfo(sInfo *blizcore.SliceInfo) *blizcore.WopInfo {
	dbInfo := clnt.slicedb.Slice(clnt.ID(), storage.Int2ChunkIdentity(sInfo.ChunkId), sInfo.SliceId)
	if dbInfo == nil {
		return nil
	}
	return &dbInfo.WopInfo
}

func (clnt *BlizClntObject) SaveSliceInfo(info *blizcore.WopInfo) error {
	err := clnt.slicedb.UpdateNode(clnt.ID(), &SliceDbInfo{WopInfo: *info})
	return err
}

func (cs *BlizCS) AccountManager() *accounts.Manager { return cs.accountManager }

func (cs *BlizCS) GetSelfBase() common.Address { return cs.csbase }

func (cs *BlizCS) UnlockBase(passphrase string) error {
	// Look up the wallet containing the requested signer
	account := accounts.Account{Address: cs.csbase}

	wallet, err := cs.AccountManager().Find(account)
	if err != nil {
		return err
	}

	//if ksWallet,ok := wallet.(*keystore.keystoreWallet); ok{
	key, err1 := wallet.GetKeyCopy(account, passphrase)
	if err1 != nil {
		return err1
	}
	cs.basePrivateKey = key
	return nil
	//}
}

// base of client
// func (cs *BlizCS) GetClientBase() (common.Address, error) {
// 	return cs.GetClientAddress()
// }

// base of cs
func (cs *BlizCS) CalculateSelfBase(passwd string) (common.Address, error) {
	cs.lock.RLock()
	csbase := cs.csbase
	cs.lock.RUnlock()

	if csbase != (common.Address{}) {
		return csbase, nil
	}

	if wallets := cs.AccountManager().Wallets(); len(wallets) > 0 {
		if accounts := wallets[0].Accounts(); len(accounts) > 0 {
			csbase := accounts[0].Address

			cs.lock.Lock()
			cs.csbase = csbase
			cs.lock.Unlock()

			err := cs.UnlockBase(passwd)
			if err != nil {
				return common.Address{}, err
			}

			cs.Log().Info("BlizCS base automatically configured", "address", csbase)
			return csbase, nil
		}
	}
	return common.Address{}, fmt.Errorf("BlizCS base must be explicitly specified")
}

func (cs *BlizCS) AddObject(clnt *BlizClntObject) {
	cs.objectMap.Store(clnt.ID(), clnt)
	clnt.Log().Info("Add object")
}

func (cs *BlizCS) DeleteObject(clnt *BlizClntObject) {
	cs.objectMap.Delete(clnt.ID())
}

func (cs *BlizCS) Dail(nodeId *discover.NodeID) {
	cs.bliz.DailFarmer(storage.ChunkIdentity{}, blizzard.CBModeClient, nodeId)
}

// func sendChanChunkNodes(ex chan struct{}, ch chan *blizzard.ChunkNodes, va *blizzard.ChunkNodes) {
// 	select {
// 	case <-ex:
// 	case ch <- va:
// 	}
// }

// func sendChanShardingNodes(ex chan struct{}, ch chan *blizzard.ShardingNodes, va *blizzard.ShardingNodes) {
// 	select {
// 	case <-ex:
// 	case ch <- va:
// 	}
// }

func sendChanConnetPeer(ex chan struct{}, ch chan interface{}, va *p2p.Peer) {
	select {
	case <-ex:
	case ch <- va:
	}
}

func (cs *BlizCS) runLoop() {

	//ClientSetFarmerChan

	// Wait for different events to fire synchronisation operations
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-cs.shutdownChan:
			return

		case <-ticker.C:
			cs.checkClaim()

		case nodes, ok := <-cs.chunkFarmerChan:
			if !ok {
				return
			}
			cs.Log().Info("runLoop", "ChunkId", storage.ChunkIdentity2Int(nodes.ChunkId), "Nodes len", len(nodes.Nodes), "farmerQueryMap len", len(cs.farmerQueryMap))
			// queryFarmer回应了
			//var queryChan chan *blizzard.ChunkNodes
			for _, q := range cs.farmerQueryMap {
				go q.Send(nodes)
			}
		case nodes, ok := <-cs.shardingMinerChan:
			if !ok {
				return
			}
			cs.Log().Info("Run loop", "address", nodes.Address, "nodecount", len(nodes.Nodes), "mapcount", len(cs.minerQueryMap))
			// queryMiner 回应了
			//var queryChan chan *blizzard.ShardingNodes
			for _, q := range cs.minerQueryMap {
				go q.Send(nodes)
			}

		case obj := <-cs.elephantToBlizcsSliceInfoCh:
			if obj.CSAddr.Equal(cs.csbase) {
				cs.ElephantToBlizcsSliceInfo(obj)
			}
		case <-cs.elephantToBlizcsSliceInfoSub.Err():
			return
		}
	}
}

func (cs *BlizCS) runObjectLoop() {
	for {
		select {
		case <-cs.shutdownChan:
			return

		case req := <-cs.elephantToCsDataCh:
			switch req.Ty {
			case exporter.TypeElephantToCsDeleteFlow:
				csAddr := req.Params[0].(common.Address)
				if csAddr != cs.csbase {
					break
				}
				userAddr := req.Params[1].(common.Address)
				cs.deleteUsedFlow(userAddr)
				cs.deleteAuthAllowFlowData(userAddr)
			}
		case <-cs.elephantToCsDataSub.Err():
			return
		}
	}
}

func (cs *BlizCS) ObjReadFtp2CSFunc(req *exporter.ObjReadFtp2CS) *exporter.ObjReadFtp2CSRsp {
	// buff may come from sharer's cs or self's cs
	clientAddr := cs.GetUserAddrByPeer(req.PeerId)
	ownerAddr := clientAddr
	var sharerCSPeer *Peer
	cs.Log().Info("Read ftp to cs", "sharercsnodeid", req.SharerCSNodeID)
	if req.SharerCSNodeID != "" && req.SharerCSNodeID != "0" {
		var err error
		sharerCSPeer = cs.getRegisterPeer(req.SharerCSNodeID)
		if sharerCSPeer == nil {
			cs.Log().Info("Read ftp to cs, not connected peer")
			if strings.EqualFold(req.SharerCSNodeID, cs.nodeserver.Self().ID.String()) {
				cs.Log().Info("Read ftp to cs, from this cs")
				sharerCSPeer = nil
				ownerAddr = common.HexToAddress(req.Sharer)
			} else {
				cs.Log().Info("Read ftp to cs, not this cs")
				err = errors.New("Can't find sharer cs")
			}
		}
		if err == nil {
			err = cs.checkShareObj(clientAddr.String(), req)
		}
		if err != nil {
			return cs.returnToFtp(err, nil, req, clientAddr)
		}
	}
	// buff comes from sharer's cs
	if sharerCSPeer != nil {
		cs.Log().Info("Read ftp to cs, read from sharer cs", "remote ip", sharerCSPeer.RemoteAddr().String())
		timeout := 10 * time.Second
		rspChan := make(chan *exporter.ObjReadFtp2CSRsp)
		go func(req *exporter.ObjReadFtp2CS, clientAddr common.Address, sharerCSPeer *Peer) {
			buff, err := cs.getBuffFromSharerCS(req.Sharer, sharerCSPeer, req, timeout)
			select {
			case <-time.After(timeout):
			case rspChan <- cs.returnToFtp(err, buff, req, clientAddr):
			}
		}(req, clientAddr, sharerCSPeer)
		select {
		case <-time.After(timeout):
			return nil
		case rsp := <-rspChan:
			return rsp
		}
	} else {
		cs.Log().Info("Read ftp to cs, read from self's cs")
		clnt, err := cs.GetObject(req.ObjId, ownerAddr)
		if err != nil {
			cs.Log().Error("clnt nil")
			return nil
		}
		clnt.SetSignPeer(req.PeerId)
		clnt.DoSeek(req.Offset)
		len := datacrypto.GetBlockAlignSize(datacrypto.EncryptDes, req.Len)
		buff := make([]byte, len)
		err = clnt.Read(buff)
		return cs.returnToFtp(err, buff, req, ownerAddr)
	}
}

func (cs *BlizCS) checkShareObj(cliAddr string, req *exporter.ObjReadFtp2CS) error {
	t := uint64(time.Now().Unix())

	objs := cs.GetObjectsInfo("", cliAddr, 2)
	for _, obj := range objs {
		v, ok := obj.(*exporter.ShareObjectInfo)
		if !ok {
			log.Info("Share object info type not match")
			continue
		}
		if strconv.Itoa(int(req.ObjId)) != v.ObjId {
			log.Info("Obj id not match", "reqobjid", req.ObjId, "objid", v.ObjId)
			continue
		}

		offset, _ := strconv.Atoi(v.Offset)
		length, _ := strconv.Atoi(v.Length)
		rwStart := uint64(offset)
		rwEnd := uint64(offset + length)
		reqRWStart := req.Offset
		reqRWEnd := req.Offset + req.Len
		if reqRWStart < rwStart || reqRWStart > rwEnd {
			// log.Info("RW start not match", "reqrwstart", reqRWStart, "offset", offset, "length", length)
			continue
		}
		if reqRWEnd < rwStart || reqRWEnd > rwEnd {
			// log.Info("RW end not match", "reqrwend", reqRWEnd, "offset", offset, "length", length)
			continue
		}

		if !strings.EqualFold(req.SharerCSNodeID, v.SharerCSNodeID) {
			log.Info("Share cs node id not match", "reqsharercsnodeid", req.SharerCSNodeID, "sharercsnodeid", v.SharerCSNodeID)
			continue
		}
		if common.HexToAddress(req.Sharer) != v.Owner {
			log.Info("Sharer not match", "reqsharer", req.Sharer, "sharer", v.Owner)
			continue
		}
		start, err := common.GetUnixTime(v.StartTime)
		if err != nil {
			log.Info("Start time format not match")
			continue
		}
		end, err := common.GetUnixTime(v.EndTime)
		if err != nil {
			log.Info("End time format not match")
			continue
		}
		if start > t || end < t {
			log.Info("Time not match", "starttime", common.GetTimeString(start), "endtime", common.GetTimeString(end), "nowtime", common.GetTimeString(t))
			continue
		}
		return nil
	}

	return errors.New("Not find valid share obj")
}

func (cs *BlizCS) returnToFtp(err error, buff []byte, req *exporter.ObjReadFtp2CS, clientAddr common.Address) *exporter.ObjReadFtp2CSRsp {
	t := time.Now()
	ret := common.GetErrBytes(err)
	var rawSize uint32
	if err != nil {
		cs.Log().Error("Read", "err", err)
	} else {
		log.Info("Read", "len", common.StorageSize(len(buff)), "elapsed", common.PrettyDuration(time.Since(t)))
		if !cs.ignoreFlow(buff) {
			cs.addUsedFlow(clientAddr, uint64(len(buff)))
		}
		rawSize = uint32(req.Len)
	}
	// if ret == nil && isReadHead(buff) {
	// 	buff = cs.reorganizeBuff(buff, clientAddr.Hex())
	// }
	return &exporter.ObjReadFtp2CSRsp{
		ObjId:   req.ObjId,
		Ret:     ret,
		Buff:    buff,
		RawSize: rawSize,
		PeerId:  req.PeerId,
	}
}

func (cs *BlizCS) reorganizeBuff(buff []byte, cliAddr string) []byte {
	infos := cs.GetObjectsInfo("", cliAddr, 2)
	for _, info := range infos {
		v, ok := info.(*exporter.ShareObjectInfo)
		if !ok {
			continue
		}
		log.Info("Reorganize buff", "object info", v)
	}
	if len(infos) == 0 {
		return buff
	}
	data, err := json.Marshal(infos)
	if err != nil {
		return buff
	}
	buffStr := strings.TrimSpace(string(buff))
	buffStr += "/"
	buffStr += string(data)
	buffStr += "/sharing"
	log.Info("Run object loop", "object infos", buffStr)
	buff = []byte(buffStr)
	return buff
}

func (cs *BlizCS) getBuffFromSharerCS(sharer string, peer *Peer, req *exporter.ObjReadFtp2CS, timeout time.Duration) ([]byte, error) {
	data := &blizcore.GetBuffFromSharerCSData{
		Sharer:    sharer,
		ObjId:     req.ObjId,
		Offset:    req.Offset,
		Len:       req.Len,
		SigPeerId: req.PeerId,
	}
	err := peer.SendGetBuffFromSharerCS(data)
	if err != nil {
		return nil, err
	}

	select {
	case <-time.After(timeout):
		return nil, errors.New("Get buff from sharer cs timeout")

	case rsp := <-cs.buffFromSharerCSCh:
		return rsp.Buff, nil
	case err := <-cs.buffFromSharerCSSub.Err():
		return nil, err
	}
}

func isReadHead(buff []byte) bool {
	if len(buff) == 4096 {
		return true
	}
	return false
}

func (cs *BlizCS) ignoreFlow(buff []byte) bool {
	cs.Log().Info("", "bufflen", len(buff))
	spec := "/"
	if !isReadHead(buff) {
		return false
	}
	str := strings.TrimSpace(string(buff))
	cs.Log().Info("", "str", str)
	list := strings.Split(str, spec)
	if len(list) < 3 {
		return false
	}
	total, err := strconv.ParseInt(list[1], 10, 64)
	if err != nil {
		cs.Log().Error("ParseInt 1", "err", err)
		return false
	}
	var sizeUsed int64 = 4096
	for i := 2; i < len(list)-1; i += 3 {
		s, e := strconv.ParseInt(list[i+2], 10, 64)
		if e != nil {
			cs.Log().Error("ParseInt 2", "err", e)
			return false
		}
		sizeUsed += s
		if i+3 < len(list)-1 {
			posStart, err1 := strconv.ParseInt(list[i], 10, 64)
			if err1 != nil {
				cs.Log().Error("ParseInt 3", "err", err1)
				return false
			}
			length, err2 := strconv.ParseInt(list[i+2], 10, 64)
			if err2 != nil {
				cs.Log().Error("ParseInt 4", "err", err2)
				return false
			}
			posStartNext, err3 := strconv.ParseInt(list[i+3], 10, 64)
			if err3 != nil {
				cs.Log().Error("ParseInt 5", "err", err3)
				return false
			}
			if posStart+common.GetAlign64(length) != posStartNext {
				cs.Log().Error("", "posStart", posStart, "length", length, "posStartNext", posStartNext)
				return false
			}
		}
	}
	spare, errSpare := strconv.ParseInt(list[len(list)-1], 10, 64)
	if errSpare != nil {
		cs.Log().Error("ParseInt 6", "err", errSpare)
		return false
	}
	if sizeUsed+spare != total {
		cs.Log().Error("", "sizeUsed", sizeUsed, "spare", spare, "total", total)
		return false
	}

	return true
}

func (cs *BlizCS) QueryShardingMiner(address common.Address, objId string, resultCh chan *blizzard.ShardingNodes) event.Subscription {
	key := address.String() + objId
	var resultFeed event.Feed
	sub := cs.scope.Track(resultFeed.Subscribe(resultCh))
	cs.minerQueryMap[key] = &resultFeed
	cs.bliz.QueryShardingMiner(address)
	return sub
}

// func (cs *BlizCS) CancelShardingMinerQuery(address common.Address, objId string) {
// 	key := address.String() + objId
// 	delete(cs.minerQueryMap, key)
// }

func (cs *BlizCS) QueryChunkFarmer(chunkId storage.ChunkIdentity, objId string, resultCh chan *blizzard.ChunkNodes) event.Subscription {
	cs.Log().Info("SubscribeChunkFarmer", "chunkId", chunkId)
	key := chunkId.String() + objId
	var resultFeed event.Feed
	sub := cs.scope.Track(resultFeed.Subscribe(resultCh))
	cs.farmerQueryMap[key] = &resultFeed
	cs.bliz.QueryChunkFarmer(chunkId)
	return sub
}

// func (cs *BlizCS) CancelChunkFarmerQuery(chunkId storage.ChunkIdentity, objId string) {
// 	key := chunkId.String() + objId
// 	delete(cs.farmerQueryMap, key)
// }

func (cs *BlizCS) JoinPeer(peer *Peer) {
	cs.Log().Info("Join peer", "peer", peer.ID())
	f := func(k, v interface{}) bool {
		clnt := v.(*BlizClntObject)
		clnt.LoginFarmer(peer)
		return true
	}
	cs.objectMap.Range(f)
	cs.registeredPeers[peer.ID()] = peer
}

func (cs *BlizCS) ExitPeer(peer *Peer) {
	cs.Log().Info("Exit peer", "peer", peer.ID())
	f := func(k, v interface{}) bool {
		clnt := v.(*BlizClntObject)
		clnt.LogoutFarmer(peer)
		return true
	}
	cs.objectMap.Range(f)
	if _, ok := cs.registeredPeers[peer.ID()]; ok {
		delete(cs.registeredPeers, peer.ID())
	}
}

func (cs *BlizCS) OpenObject(objId uint64, base common.Address) (*BlizClntObject, error) {
	strId := strconv.Itoa(int(objId))
	uId := UniqueId(base, objId)
	cs.openObjectLock.Lock()
	if _, ok := cs.mapOpenObject[uId]; !ok {
		var lock sync.RWMutex
		cs.mapOpenObject[uId] = &lock
	}
	cs.mapOpenObject[uId].Lock()
	defer cs.mapOpenObject[uId].Unlock()
	cs.openObjectLock.Unlock()

	// 获取一个对象的操作指针
	if v, ok := cs.objectMap.Load(uId); ok {
		cs.Log().Info("Exist")
		clnt := v.(*BlizClntObject)
		return clnt, nil
	}
	cs.Log().Info("OpenObject", "uId", uId)
	nodes, err := cs.queryShardingMiner(strId, blizparam.QueryMinerTimeout, base)
	if nodes == nil || len(nodes) == 0 {
		cs.Log().Error("queryShardingMiner", "err", err)
		return nil, err
	}
	cs.Log().Info("queryShardingMiner", "nodes len", len(nodes))
	// 通过世界状态获取该对象的 chunk:slice[] 信息
	//go queryShardingMiner
	meta, err := cs.queryObjectMeta(objId, nodes, base)
	if err != nil {
		cs.Log().Error("queryObjectMeta", "err", err)
		return nil, err
	}

	sliceDBPath := filepath.Join(cs.sliceDBDir, uId)
	slicedb, err := CreateSliceDB(sliceDBPath, SliceDBVersion)
	if err != nil {
		cs.Log().Error("CreateSliceDB", "err", err, "sliceDBPath", sliceDBPath)
		return nil, err
	}
	// 根据 chunk:slice[] 的内容，构造出 BlizObject 对象
	clnt := &BlizClntObject{
		blizcs:            cs,
		id:                (uint64)(objId),
		addr:              base,
		name:              "",
		meta:              meta,
		chunkFarmers:      make(map[storage.ChunkIdentity]map[discover.NodeID]*Peer, 0),
		chunkFarmerDailTs: make(map[storage.ChunkIdentity]uint64, 0),
		bFarmerReg:        false,
		offset:            0,
		shutdownChan:      make(chan struct{}),
		localSliceMap:     make(map[string]*blizcore.WopInfo),
		farmerSliceMaps:   make(map[string]map[discover.NodeID]*blizcore.WopData),
		slicedb:           slicedb,
	}

	ret, err := clnt.Prepare()
	if ret == false {
		slicedb.Close()
		cs.Log().Error("Prepare", "err", err)
		return nil, err
	}

	// 一切都准备妥当了，将它加入到cs对象管理列表中
	cs.AddObject(clnt)

	// 把OpenObject之前已经连接上的farmer添加进去（peer已连接，不会再调用LoginFarmer）
	clnt.AddRegisteredPeers()

	// 返回操作对象 BlizClntObject
	go clnt.trickConnFarmers(nil, 2)

	// go func(clnt *BlizClntObject) {
	// 	times := 0
	// 	ticker := time.NewTicker(1 * time.Second)
	// 	defer ticker.Stop()
	// 	for {
	// 		select {
	// 		case <-ticker.C:
	// 			ret := clnt.trickFarmerSliceOp()
	// 			times++
	// 			if ret || times > 10 {
	// 				return
	// 			}
	// 		}
	// 	}
	// }(clnt)

	return clnt, nil
}

func (cs *BlizCS) ElephantToBlizcsSliceInfo(obj *exporter.ObjElephantToBlizcsSliceInfo) {
	// clientAddr, err := cs.GetClientAddress()
	// if err != nil {
	// 	cs.Log().Error("", "err", err)
	// 	return
	// }
	// if !obj.Addr.Equal(clientAddr) {
	// 	cs.Log().Error("", "obj.Addr", obj.Addr, "clientAddr", clientAddr)
	// 	return
	// }
	uId := UniqueId(obj.Addr, obj.ObjId)
	v, ok := cs.objectMap.Load(uId)
	if !ok {
		cs.Log().Info("Clnt nil", "objid", obj.ObjId)
		return
	}
	clnt := v.(*BlizClntObject)
	sliceInfos := make([]blizcore.SliceInfo, 0, len(obj.Slices))
	for _, slice := range obj.Slices {
		sliceInfo := blizcore.SliceInfo{ChunkId: slice.ChunkId, SliceId: slice.SliceId}
		sliceInfos = append(sliceInfos, sliceInfo)
	}
	cs.Log().Info("", "sliceInfos len", len(sliceInfos))
	clnt.meta.SliceInfos = sliceInfos
}

func (clnt *BlizClntObject) AddRegisteredPeers() {
	clnt.Log().Info("AddRegisteredPeers", "registeredPeers", clnt.blizcs.registeredPeers)
	for _, peer := range clnt.blizcs.registeredPeers {
		clnt.LoginFarmer(peer)
	}
	// clnt.farmerlock.Lock()
	// defer clnt.farmerlock.Unlock()

	// for chunkId, peers := range clnt.chunkFarmers {
	// 	for nodeID, p := range peers {
	// 		if p != nil {
	// 			continue
	// 		}
	// 		peer, ok := clnt.blizcs.registeredPeers[nodeID]
	// 		if ok {
	// 			clnt.chunkFarmers[chunkId][nodeID] = peer
	// 		}
	// 	}
	// }
}

func (cs *BlizCS) GetObject(objId uint64, base common.Address) (*BlizClntObject, error) {
	if common.EmptyAddress(base) {
		cs.Log().Warn("GetObject no auth")
		return nil, errors.New("no auth")
	}
	clnt, err := cs.OpenObject(objId, base)
	if err != nil {
		return nil, err
	}
	return clnt, nil
}

func SetRWCheckCount(num int) {
	WriteCheckCount = num
	ReadCheckCount = WriteCheckCount
	log.Info("SetRWCheckCount", "num", num)
}

func (clnt *BlizClntObject) trickFarmerSliceOp() bool {
	peers := make(map[discover.NodeID]*Peer)
	bEnough := true
	count := 0
	// 每个chunk至少需要NeedFarmerCount个farmer
	clnt.farmerlock.Lock()
	for chunkId, farmers := range clnt.chunkFarmers {
		clnt.Log().Info("trickFarmerSliceOp", "chunkId", storage.ChunkIdentity2Int(chunkId))
		count = 0
		for nodeID, peer := range farmers {
			if peer == nil {
				continue
			}
			peers[nodeID] = peer
			count++
		}
		isChunkFarmer := storage.HasChunk(storage.ChunkIdentity2Int(chunkId))
		otherFarmerCnt := blizparam.GetOtherMinFarmerCount(isChunkFarmer)
		if count < otherFarmerCnt {
			bEnough = false
			break
		}
	}
	clnt.farmerlock.Unlock()

	if !bEnough {
		clnt.Log().Warn("trickFarmerSliceOp not enough", "count", count)
		return false
	}
	if len(peers) == 0 {
		clnt.Log().Warn("trickFarmerSliceOp no farmer")
		return false
	}
	for _, peer := range peers {
		req := &blizcore.FarmerSliceOpReq{ObjUId: clnt.ID(), SliceInfos: clnt.meta.SliceInfos}
		err := peer.SendFarmerSliceOp(req)
		clnt.Log().Info("SendFarmerSliceOp", "err", err)
	}
	return true
}

func (clnt *BlizClntObject) trickConnFarmers(chunkId *storage.ChunkIdentity, dailCnt int) error {
	//连接到相关的所有 chunk farmer。
	retryCnt := 0

	prepare, err := clnt.dailFarmers(chunkId, dailCnt, true)
	if err != nil {
		return err
	}
	if prepare {
		return nil
	}

_retry:
	for {
		// 等待 farmers 成功连接个数达到超过2个
		select {
		case <-clnt.shutdownChan:
			return nil

		// 如果超时，还没有满足数量要求，就retry
		case <-time.After(5 * time.Second):
			prepare, err := clnt.dailFarmers(chunkId, dailCnt, true)
			if err != nil {
				return err
			}
			//cnt := clnt.getLinkedFarmerCount(storage.Int2ChunkIdentity(slice.ChunkId))
			if prepare {
				break _retry
			}
			retryCnt++
			if retryCnt > 3 {
				break _retry
			}
		}
	}
	return nil
}

func (cs *BlizCS) CloseObject(bco *BlizClntObject) {
	bco.Close()
}

func (cs *BlizCS) SeekObject(bco *BlizClntObject, offset uint64) int64 {
	return bco.DoSeek(offset)
}

// qiweiyou: 没有足够空间，必须先执行CreateObject
func (cs *BlizCS) WriteObject(bco *BlizClntObject, data []byte) (int, error) {
	return bco.Write(data)
}

func (cs *BlizCS) ReadObject(bco *BlizClntObject, buffer []byte) error {
	return bco.Read(buffer)
}

func (cs *BlizCS) queryFarmer(chunkId storage.ChunkIdentity, objId string, timeout int) ([]*discover.Node, error) {
	cs.Log().Info("queryFarmer", "chunkId", storage.ChunkIdentity2Int(chunkId))
	farmersCh := make(chan *blizzard.ChunkNodes)
	farmersSub := cs.QueryChunkFarmer(chunkId, objId, farmersCh)
	defer farmersSub.Unsubscribe()

	for {
		select {
		case <-cs.shutdownChan:
			return nil, nil

		case <-time.After(time.Duration(timeout) * time.Second):
			cs.Log().Warn("queryFarmer timeout")
			return nil, blizcore.ErrQueryFarmerTimeout

		case nodes := <-farmersCh:
			cs.Log().Info("queryFarmer", "ChunkId", nodes.ChunkId, "chunkId", storage.ChunkIdentity2Int(chunkId), "Nodes", nodes.Nodes)
			if nodes.ChunkId.Equal(chunkId) {
				return nodes.Nodes, nil
			}

		case err := <-farmersSub.Err():
			return nil, err
		}
	}
}

func (cs *BlizCS) queryShardingMiner(objId string, timeout int, base common.Address) ([]*discover.Node, error) {
	minersCh := make(chan *blizzard.ShardingNodes)
	// 文件属主是资源的所有者，也就是资源跟用户account账户绑定，而不是跟
	// 本机nodeID绑定
	// base, err := cs.GetClientBase()
	// if err != nil {
	// 	cs.Log().Error("GetClientBase", "err", err)
	// 	return nil, err
	// }

	minersSub := cs.QueryShardingMiner(base, objId, minersCh)
	defer minersSub.Unsubscribe()

	for {
		select {
		case <-cs.shutdownChan:
			return nil, nil

		case <-time.After(time.Duration(timeout) * time.Second):
			return nil, blizcore.ErrQueryShardingMinerTimeout

		case nodes := <-minersCh:
			cs.Log().Info("Query sharding miner", "address", nodes.Address, "nodecount", len(nodes.Nodes))
			if nodes.Address.Equal(base) {
				return nodes.Nodes, nil
			}

		case err := <-minersSub.Err():
			return nil, err
		}
	}
}

// 向sharding节点组询问某 object 的 元数据信息
func (cs *BlizCS) queryObjectMeta(objId uint64, nodes []*discover.Node, base common.Address) (*blizcore.BlizObjectMeta, error) {
	// base, err := cs.GetClientBase()
	// if err != nil {
	// 	cs.Log().Error("queryObjectMeta", "err", err)
	// 	return nil, err
	// }
	var err error

	// 向这些节点发起Chunk Farmer查询请求
	connectPeerCh := make(chan *p2p.Peer)
	var connectPeerFeed event.Feed
	connectPeerSub := cs.scope.Track(connectPeerFeed.Subscribe(connectPeerCh))
	defer connectPeerSub.Unsubscribe()

	for i := 0; i < len(nodes); i++ {
		// 发起连接
		peer := cs.getRegisterPeer(nodes[i].ID.String())
		cs.Log().Info("queryObjectMeta", "i", i, "node", nodes[i].ID, "peer", peer)
		if peer != nil {
			// 本地已经跟对方建立了连接了，而elephant连接是gctt基础服务，
			// 该协议通通支持，我们可以直接使用
			// 这里为了不堵塞（互锁），必须使用 go 来喂
			go connectPeerFeed.Send(peer.Peer)
		} else {
			// 向 sharding node 发起 miner 查询连接
			cs.nodeserver.AddPeer(&discover.NodeCB{Node: nodes[i], Cb: nil, PeerProtocol: "elephant", PeerFeed: &connectPeerFeed})
		}
	}

	// 等待请求结果反馈
	var cnt = 0
	var metaCombos = make([]*blizcore.BlizObjectMeta, 0)
	rspChan := make(chan *exporter.ObjMetaRsp)
	var rspFeed event.Feed
	rspSub := cs.scope.Track(rspFeed.Subscribe(rspChan))
	defer rspSub.Unsubscribe()

	queryCnt := blizparam.GetObjMetaQueryCount()
	if param.IsChainNode() {
		queryCnt--
	}

_wait:
	for {
		select {
		case <-cs.shutdownChan:
			return nil, errors.New("shutdown")

		case peer := <-connectPeerCh:
			cs.Log().Info("queryObjectMeta", "peer", peer)
			// 发送metaquery请求
			var req = &exporter.ObjMetaQuery{Peer: peer, RspFeed: &rspFeed, Base: base, ObjId: objId}
			cs.bliz.EleCross.ObjMetaQueryFunc(req)

		case err := <-connectPeerSub.Err():
			return nil, err

		case rsp := <-rspChan:
			cs.Log().Info("queryObjectMeta")
			metaCombos = append(metaCombos, &rsp.Meta)
			cnt++
			//if cnt >= len(nodes) {
			if cnt >= queryCnt {
				break _wait
			}

		case err := <-rspSub.Err():
			return nil, err

		case <-time.After(10 * time.Second):
			err = blizcore.ErrQueryObjectMetaTimeout
			break _wait
		}
	}

	if param.IsChainNode() {
		slices := cs.bliz.EleCross.GetSliceFromState(base, objId)
		if len(slices) != 0 {
			sliceInfos := make([]blizcore.SliceInfo, len(slices))
			for i, sc := range slices {
				sliceInfos[i].ChunkId = sc.ChunkId
				sliceInfos[i].SliceId = sc.SliceId
			}
			meta := blizcore.BlizObjectMeta{
				Owner:      base,
				SliceInfos: sliceInfos,
			}
			metaCombos = append(metaCombos, &meta)
		}
	}

	// TODO:发起查询
	// 首先需要建立连接才能进行查询
	if len(metaCombos) > 1 {
		// 获得了两组以上回复，我们取相同部分
		if meta0 := metaCombos[0]; meta0 != nil {
			if meta1 := metaCombos[1]; meta1 != nil {
				if meta0.Equal(meta1) {
					// 成功获取到正确的 meta 数据，我们将这个分片的诚实矿工缓冲刷新一下
					shardingIdx := sharding.ShardingAddressMagic(sharding.ShardingCount, base.String())
					cs.bliz.RefreshCachedShardingNodes(shardingIdx)
					return meta0, nil
				}
			}
		}
		// 刷新数据库记录，这样就将淘汰的partner自然过滤了
		// 而内存中的记录，我们就留给 assoLoop 中的运行Ts时间戳判断去删除
		//b.chunkdb.UpdateNode(&ChunkDbInfo{Id: chunkId, Partners: farmers})
		//b.queryFarmerStep3Chan <- &ChunkNodes{ChunkId: chunkId, Nodes:qFarmers}
	} else if len(metaCombos) == 1 {
		return metaCombos[0], nil
	}
	return nil, err
}

func (cs *BlizCS) GetFarmersSliceOps(sliceInfos []blizcore.SliceInfo) []blizcore.FarmerSliceOp {
	return cs.GetStorageManager().GetFarmerSliceOp(sliceInfos)
}

func (cs *BlizCS) GetFarmersSliceOpsRsp(objUId string, ops []blizcore.FarmerSliceOp, nodeId discover.NodeID) {
	v, ok := cs.objectMap.Load(objUId)
	if !ok {
		return
	}
	clnt := v.(*BlizClntObject)
	clnt.UpdateFarmersSliceOps(ops, nodeId)
}

// 获取名字为objname的对象的元数据Slice的世界状态
func (clnt *BlizClntObject) getMetaSliceStatus(objname string) {

}

// 向一组farmer获取Slice
func (clnt *BlizClntObject) getSliceData() {

}

// func (clnt *BlizClntObject) GetFarmerSliceOpChan() chan *FarmerSliceOp {
// 	return clnt.farmerSliceOpChan
// }

func (clnt *BlizClntObject) UpdateFarmersSliceOps(ops []blizcore.FarmerSliceOp, nodeId discover.NodeID) {
	clnt.farmerSliceMapsLock.Lock()
	defer clnt.farmerSliceMapsLock.Unlock()

	for _, op := range ops {
		sliceInfo := blizcore.SliceInfo{ChunkId: op.ChunkId, SliceId: op.SliceId}
		v, ok := clnt.farmerSliceMaps[sliceInfo.String()]
		if !ok {
			wopDatas := make(map[discover.NodeID]*blizcore.WopData)
			wopDatas[nodeId] = &blizcore.WopData{WopId: op.OpId}
			clnt.farmerSliceMaps[sliceInfo.String()] = wopDatas
		} else {
			if _, r := v[nodeId]; !r {
				clnt.farmerSliceMaps[sliceInfo.String()][nodeId] = &blizcore.WopData{WopId: op.OpId}
			} else {
				if clnt.farmerSliceMaps[sliceInfo.String()][nodeId].WopId < op.OpId {
					clnt.farmerSliceMaps[sliceInfo.String()][nodeId].WopId = op.OpId
				}
			}
		}
	}
}

func (clnt *BlizClntObject) GetRentStorage() []blizcore.SliceInfo {
	return clnt.meta.SliceInfos
}

func (clnt *BlizClntObject) Log() log.Logger {
	return clnt.blizcs.Log()
}

// ----------------------- Node.Service Interface Implementation --------------------------

// APIs returns the collection of RPC services the ethereum package offers.
// NOTE, some of these services probably need to be moved to somewhere else.
func (cs *BlizCS) APIs() []rpc.API {
	apis := []rpc.API{
		{
			Namespace: "blizcs",
			Version:   "1.0",
			Service:   NewPublicBlizCSAPI(cs),
			Public:    true,
		},
	}
	// Append all the local APIs and return
	return append(apis, []rpc.API{}...)
}

// Protocols implements node.Service, returning all the currently configured
// network protocols to start.
func (cs *BlizCS) Protocols() []p2p.Protocol {
	//protocols := make([]p2p.Protocol, 0)
	//return protocols
	return cs.protocolManager.SubProtocols
}

// Start is called after all services have been constructed and the networking
// layer was also initialized to spawn any goroutines required by the service.
func (cs *BlizCS) Start(server *p2p.Server) error {
	cs.elephantToCsDataCh = make(chan *exporter.ObjElephantToCsData)
	cs.elephantToCsDataSub = cs.bliz.EleCross.BlockChain().SubscribeElephantToCsData(cs.elephantToCsDataCh)

	cs.elephantToBlizcsSliceInfoCh = make(chan *exporter.ObjElephantToBlizcsSliceInfo)
	cs.elephantToBlizcsSliceInfoSub = cs.bliz.EleCross.BlockChain().SubscribeElephantToBlizcsSliceInfo(cs.elephantToBlizcsSliceInfoCh)

	cs.buffFromSharerCSCh = make(chan *exporter.ObjBuffFromSharerCS)
	cs.buffFromSharerCSSub = cs.bliz.EleCross.BlockChain().SubscribeBuffFromSharerCS(cs.buffFromSharerCSCh)

	cs.nodeserver = server
	cs.bliz.ClientSetMinerChan(&cs.shardingMinerChan)
	cs.bliz.ClientSetFarmerChan(&cs.chunkFarmerChan)

	// 启动各个主要的工作 routine
	go cs.runLoop()
	go cs.runObjectLoop()

	// 启动 sliceDB
	f := func(k, v interface{}) bool {
		clnt := v.(*BlizClntObject)
		clnt.slicedb.ensureExpirer()
		return true
	}
	cs.objectMap.Range(f)

	maxPeers := server.MaxPeers
	cs.protocolManager.Start(maxPeers)

	// go cs.prepareClient()

	return nil
}

// Stop implements node.Service, terminating all internal goroutines used by the
// Contatract protocol.
func (cs *BlizCS) Stop() error {
	f := func(k, v interface{}) bool {
		clnt := v.(*BlizClntObject)
		clnt.slicedb.close()
		clnt.Close()
		return true
	}
	cs.objectMap.Range(f)

	close(cs.shutdownChan)
	close(cs.chunkFarmerChan)
	close(cs.shardingMinerChan)

	cs.protocolManager.Stop()
	cs.scope.Close()
	return nil
}

func (cs *BlizCS) GetStorageManager() *storage.Manager {
	return cs.bliz.GetStorageManager()
}

// ty: bit 1 for selfObjectsInfo; bit 2 for shareObjectsInfo
func (cs *BlizCS) GetObjectsInfo(password string, address string, ty uint8) exporter.ObjectsInfo {
	info := make(exporter.ObjectsInfo, 0)
	base := common.HexToAddress(address)
	nodes, err := cs.queryShardingMiner("", blizparam.QueryMinerTimeout, base)
	if nodes == nil || len(nodes) == 0 {
		cs.Log().Error("queryShardingMiner", "err", err)
		return info
	}
	cs.Log().Info("queryShardingMiner", "nodes len", len(nodes))
	cs.Log().Info("HexToAddress", "base", base)
	for i := 0; i < len(nodes); i++ {
		// cs.Log().Info("GetObjectsInfo", "i", i, "nodes", nodes[i])
		// peer := cs.bliz.GetRegisterPartner(nodes[i].String())
		// if peer == nil {
		// 	continue
		// }
		var req = &exporter.ObjGetObjectQuery{Base: base, Password: password, Ty: ty}
		info = cs.bliz.EleCross.ObjGetObjectFunc(req)
		break
	}
	return info
}

func (cs *BlizCS) getUserAddrByPeer(peer string) common.Address {
	if v, ok := cs.userAuthMap.Load(peer); ok {
		return v.(*UserAuthData).UserAddr
	}
	return common.Address{}
}

func (cs *BlizCS) GetUserAddrByPeer(peer string) common.Address {
	timeoutCnt := 10
	for i := 0; i < timeoutCnt; i++ {
		addr := cs.getUserAddrByPeer(peer)
		if !common.EmptyAddress(addr) {
			return addr
		}
		time.Sleep(time.Millisecond * 200)
	}
	return cs.getUserAddrByPeer(peer)
}

func (cs *BlizCS) GetUserAuthDataByPeer(peer string) *UserAuthData {
	if v, ok := cs.userAuthMap.Load(peer); ok {
		return v.(*UserAuthData)
	}
	return nil
}

func (cs *BlizCS) UpdateUserAuthData(ftcPeer string, data *UserAuthData) {
	cs.userAuthMap.Store(ftcPeer, data)
}

func (cs *BlizCS) GetAuthorizers() common.Addresss {
	cs.Log().Info("GetAuthorizers", "csbase", cs.csbase)
	req := &exporter.ObjGetCliAddr{Address: cs.csbase}
	addresses := cs.bliz.EleCross.ObjGetCliAddrFunc(req)
	if addresses == nil {
		cs.Log().Info("Addresses nil")
		return nil
	}
	addrs := make(common.Addresss, 0, len(addresses))
	for _, addr := range addresses {
		addrs = append(addrs, addr.Authorizer)
	}
	return addrs
}

func (cs *BlizCS) PrepareAddress(addr common.Address) bool {
	objsInfo := cs.GetObjectsInfo("", addr.Hex(), 1)
	if len(objsInfo) == 0 {
		return false
	}
	for _, info := range objsInfo {
		v, ok := info.(*exporter.SelfObjectInfo)
		if !ok {
			continue
		}
		_, err := cs.GetObject(v.ObjId, addr)
		if err != nil {
			return false
		}
	}
	return true
}

func (cs *BlizCS) PrepareAddresss(addrs common.Addresss) common.Addresss {
	if len(addrs) == 0 {
		return addrs
	}
	failAddrs := make(common.Addresss, 0, len(addrs))
	for _, addr := range addrs {
		ret := cs.PrepareAddress(addr)
		if ret {
			continue
		}
		failAddrs = append(failAddrs, addr)
	}
	return failAddrs
}

func (cs *BlizCS) prepareClient() {
	addrs := cs.GetAuthorizers()
	if addrs == nil {
		return
	}
	addrs = cs.PrepareAddresss(addrs)
	if len(addrs) == 0 {
		return
	}

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			addrs = cs.PrepareAddresss(addrs)
			if len(addrs) == 0 {
				return
			}
		}
	}
}

func (cs *BlizCS) Log() log.Logger {
	return cs.log
}

func (cs *BlizCS) getUsedFlow(user common.Address) uint64 {
	used, ok := cs.usedCostMap[user]
	if ok {
		return used
	}
	key := getUsedFlowKey(user)
	value, err := cs.csdb.Get(key)
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(string(value))
	if err != nil {
		return 0
	}
	return uint64(n)
}

func (cs *BlizCS) addUsedFlow(user common.Address, size uint64) {
	used := cs.getUsedFlow(user)
	used += size
	cs.updateUsedFlow(user, used)
}

func (cs *BlizCS) updateUsedFlow(user common.Address, size uint64) {
	cs.usedCostMap[user] = size
	cs.Log().Info("Update used flow", "size", size)

	// store
	key := getUsedFlowKey(user)
	value := []byte(strconv.Itoa(int(size)))
	err := cs.csdb.Put(key, value)
	if err != nil {
		cs.Log().Error("Put", "err", err)
		return
	}
}

func (cs *BlizCS) addUsedCost(user common.Address, size uint64) { // qiwy: todo, need delete
	used := cs.getUsedFlow(user)
	sizeUpdate := blizparam.SizeForUpdateState
	oldIndex := used / sizeUpdate
	if oldIndex*sizeUpdate != used {
		oldIndex++
	}
	used += size
	newIndex := used / sizeUpdate
	if newIndex*sizeUpdate != used {
		newIndex++
	}
	cs.Log().Info("total used", "used", used)

	// store
	key := getUsedFlowKey(user)
	value := []byte(strconv.Itoa(int(used)))
	err := cs.csdb.Put(key, value)
	if err != nil {
		cs.Log().Error("Put", "err", err)
		return
	}

	// trigger to update statedb
	if newIndex != oldIndex {
		params := make([]interface{}, 0, 3)
		params = append(params, cs.csbase)
		params = append(params, user)
		params = append(params, used)
		req := &exporter.ObjGetElephantData{Ty: exporter.TypeCsToElephantAddUsedFlow, Params: params} // qiwy: todo, online, need sign data
		cs.bliz.EleCross.ObjGetElephantDataFunc(req)
	}
}

func (cs *BlizCS) deleteUsedFlow(user common.Address) {
	_, ok := cs.usedCostMap[user]
	if ok {
		delete(cs.usedCostMap, user)
	}
	key := getUsedFlowKey(user)
	err := cs.csdb.Delete(key)
	if err != nil {
		cs.Log().Error("deleteUsedFlow", "err", err)
	}
}

func (cs *BlizCS) UpdateAuthAllowFlow(userAddr common.Address, flow uint64, sign []byte) {
	data := cs.getAuthAllowFlowData(userAddr)
	// cs.Log().Info("UpdateAuthAllowFlow", "data", data)
	if data != nil && data.Flow >= flow {
		return
	}
	v := &UserAuthAllowFlowData{CsAddr: cs.csbase, Flow: flow, Sign: sign}
	cs.userAuthAllowFlowMap[userAddr] = v
	//store
	ret, err := json.Marshal(*v)
	if err != nil {
		cs.Log().Error("Marshal", "err", err)
		return
	}
	key := getAuthAllowFlowKey(userAddr)
	err = cs.csdb.Put(key, ret)
	if err != nil {
		cs.Log().Error("Put", "err", err)
		return
	}
}

func (cs *BlizCS) getAuthAllowFlowData(userAddr common.Address) *UserAuthAllowFlowData {
	v, ok := cs.userAuthAllowFlowMap[userAddr]
	if ok {
		return v
	}
	key := getAuthAllowFlowKey(userAddr)
	value, err := cs.csdb.Get(key)
	if err != nil {
		return nil
	}
	var data UserAuthAllowFlowData
	if err := json.Unmarshal(value, &data); err != nil {
		cs.Log().Error("Unmarshal", "err", err)
		return nil
	}
	cs.userAuthAllowFlowMap[userAddr] = &data
	return &data
}

func (cs *BlizCS) deleteAuthAllowFlowData(userAddr common.Address) {
	_, ok := cs.userAuthAllowFlowMap[userAddr]
	if ok {
		delete(cs.userAuthAllowFlowMap, userAddr)
	}
	key := getAuthAllowFlowKey(userAddr)
	err := cs.csdb.Delete(key)
	if err != nil {
		cs.Log().Error("deleteAuthAllowFlowData", "err", err)
	}
}

// func (db *csDB) getUsedFlow(addr common.Address) (uint64, error) {
// 	key := append(usedFlowPrefix, addr[:]...)
// 	value, err := db.Get(key)
// 	if err != nil {
// 		return 0, err
// 	}
// 	flow, err := strconv.Atoi(string(value))
// 	if err != nil {
// 		return 0, err
// 	}
// 	return uint64(flow), nil
// }

var leastDiskSize = uint64(storagecore.GSize * 5)
var leastRentSize = uint64(storagecore.MSize * 1025)
var claimMaxCount = 3
var minBalance = big.NewInt(1e+18)
var unusedChunkMaxCnt = uint32(1)

func init() {
	if runtime.GOOS != "windows" {
	}
	// Test from extending storage
	leastRentSize = uint64(storagecore.MSize * 1024)
	claimMaxCount = 200
	// Test end
}

func (cs *BlizCS) checkClaim() {
	if cs.protocolManager.peers.Len() < blizparam.GetMinCopyCount()-1 {
		log.Info("Check claim, no enough connect", "count", cs.protocolManager.peers.Len())
		return
	}

	chunkIds := storage.GetChunkIds()
	cIds := make([]uint64, 0, len(chunkIds))
	for _, id := range chunkIds {
		cId := storage.ChunkIdentity2Int(id)
		cIds = append(cIds, cId)
	}

	// _find:
	// 	for _, id := range cs.waitingClaimIds {
	// 		for _, id2 := range cIds {
	// 			if id == id2 {
	// 				continue _find
	// 			}
	// 		}
	// 		cIds = append(cIds, id)
	// 	}
	// claimCount := len(cIds)

	address := cs.GetSelfBase()
	if common.EmptyAddress(address) {
		log.Info("Check claim, no etherbase")
		return
	}
	claimCount, err := cs.bliz.EleCross.GetClaim(address)
	if err != nil {
		log.Error("Check claim, get claim error", "err", err)
		return
	}
	if claimCount >= claimMaxCount {
		log.Info("Check claim, already max", "count", claimCount)
		return
	}
	if claimCount > len(cIds) {
		log.Info("Check claim, is claiming", "count", claimCount-len(cIds))
		return
	}

	leftDisk := storage.GetDiskFreeSize(".")
	if leftDisk <= leastDiskSize {
		log.Info("Check claim, not enouth disk", "left", common.StorageSize(leftDisk))
		return
	}
	// balance := cs.bliz.EleCross.GetBalance(cs.csbase)
	// if balance.Cmp(minBalance) < 0 {
	// 	log.Info("Check claim, not enouth balance", "balance", balance)
	// 	return
	// }
	leftRent := cs.bliz.EleCross.GetLeftRentSize()
	if leftRent >= leastRentSize {
		str := fmt.Sprintf("%d mB", leftRent/storagecore.MSize)
		log.Info("Check claim, has enouth rent, left " + str)
		return
	}

	unusedChunkCnt := cs.bliz.EleCross.GetUnusedChunkCnt(address, cIds)
	if unusedChunkCnt >= unusedChunkMaxCnt {
		log.Info("Check claim, unused chunk count too many", "count", unusedChunkCnt)
		return
	}

	ret := cs.Claim()
	log.Info("Check claim", "ret", ret)
}

func (cs *BlizCS) Claim() string {
	address := cs.GetSelfBase()
	if common.EmptyAddress(address) {
		return "No etherbase"
	}
	password := blizparam.GetCsSelfPassphrase()
	var req = &exporter.ObjClaimChunkQuery{
		Password: password,
		Address:  address,
		Node:     cs.bliz.SelfNode(),
	}
	_, err := cs.bliz.EleCross.ObjClaimChunkQueryFunc(req)
	ret := "Waiting for sealing"
	if err != nil {
		ret = err.Error()
	}
	// cs.waitingClaimIds = append(cs.waitingClaimIds, cId)
	return ret
}

func (cs *BlizCS) EleObjGetElephantDataFunc(query *exporter.ObjGetElephantData) {
	cs.bliz.EleCross.ObjGetElephantDataFunc(query)
}

func (cs *BlizCS) EleObjGetElephantDataRspFunc(query *exporter.ObjGetElephantData) *exporter.ObjGetElephantDataRsp {
	return cs.bliz.EleCross.ObjGetElephantDataRspFunc(query)
}

func (cs *BlizCS) ObjWriteFtp2CSFunc(req *exporter.ObjWriteFtp2CS) *exporter.ObjWriteFtp2CSRsp {
	clientAddr := cs.GetUserAddrByPeer(req.PeerId)
	clnt, err := cs.GetObject(req.ObjId, clientAddr)
	if err != nil {
		cs.Log().Error("clnt nil")
		return nil
	}
	clnt.SetSignPeer(req.PeerId)
	clnt.DoSeek(req.Offset)
	t := time.Now()
	size, err := clnt.Write(req.Buff)
	log.Info("Write", "size", common.StorageSize(size), "elapsed", common.PrettyDuration(time.Since(t)))
	ret := common.GetErrBytes(err)
	if err == nil && !clnt.blizcs.ignoreFlow(req.Buff) {
		clnt.blizcs.addUsedFlow(clientAddr, uint64(size))
	}
	return &exporter.ObjWriteFtp2CSRsp{
		ObjId:  req.ObjId,
		Size:   uint64(size),
		RetVal: ret,
		PeerId: req.PeerId,
	}
}

func (cs *BlizCS) ObjAuthFtp2CSFunc(req *exporter.ObjAuthFtp2CS) {
	data := &UserAuthData{
		CsAddr:   req.CsAddr,
		UserAddr: req.UserAddr,
		Deadline: req.AuthDeadline,
		R:        req.R,
		S:        req.S,
		V:        req.V,
	}
	cs.UpdateUserAuthData(req.Peer, data)
}

func (cs *BlizCS) getRegisterPeer(nodeID string) *Peer {
	return cs.protocolManager.peers.GetPeer(nodeID)
}
