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

package blizzard

import (
	//"encoding/hex"
	"errors"
	"math/big"
	"time"

	//"math/big"
	"sync"

	"github.com/contatract/go-contatract/blizparam"
	storage "github.com/contatract/go-contatract/blizzard/storage"
	"github.com/contatract/go-contatract/blizzard/storagecore"
	"github.com/contatract/go-contatract/core_elephant"
	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/node"
	"github.com/contatract/go-contatract/p2p"
	discover "github.com/contatract/go-contatract/p2p/discover"

	//"github.com/contatract/go-contatract/ethdb"
	"github.com/contatract/go-contatract/rpc"
	"github.com/contatract/go-contatract/sharding"

	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/common/queue"
	"github.com/contatract/go-contatract/elephant"
	"github.com/contatract/go-contatract/elephant/exporter"

	"github.com/contatract/go-contatract/blizcore"
	"github.com/contatract/go-contatract/event"

	set "gopkg.in/fatih/set.v0"
)

const (
	// ChunkPartnerAsso Status
	AssoStatusIdle     = 0x00
	AssoStatusQuerying = 0x01
	AssoStatusNormal   = 0x02
	AssoStatusOnline   = 0x03
	AssoStatusExpired  = 0x04
	AssoStatusAbandom  = 0x05
)

const (
	ChunkDBVersion = 1
	ChunkFarmerMin = 1 // see blizparam\paramter.go
	ChunkFarmerMax = 32

	ChunkClaimMax      = 16 // see blizparam\paramter.go
	CoDailingFarmerMax = 64 // 同时拨号的 farmer 数量

	AssoQueryTimeout          = 180 // Asso 进行 partners 查询的 timeout 值
	ShardingNodesCacheTimeout = 300 // 分片节点缓存的 timeout值， DoChunkShardingNodesTimeout是在 AssoQuery Timeout时
	// 执行，所以需要大于 AssoQueryTimeout，这样才不会在 AssoQuery 超时的时候被错杀

	CBModePartner = 0
	CBModeClient  = 1

	MaxKnownHeartBeats        = 16384
	NeedShardingNodeCount     = 1 // see blizparam\paramter.go
	QueryFarmerStep2ChanCount = 1 // see blizparam\paramter.go
)

// p2p dail 拨号携带给对方的握手信息
// 用它来传递我们的意图，是以partner
// 身份向它寻求连接还是以client身份
// 向它寻求连接
type BlizNodeCB struct {
	ChunkId storage.ChunkIdentity
	CbMode  uint32
}

type ChunkNodes struct {
	// 跟 chunkId 确定的分片内的矿工节点
	ChunkId storage.ChunkIdentity
	Nodes   []*discover.Node
}

type ShardingNodes struct {
	// 跟 address 确定的分片内的矿工节点
	Address common.Address
	Nodes   []*discover.Node
}

type farmerDail struct {
	chunkId storage.ChunkIdentity
	cbMode  uint32

	node *discover.NodeID
}

//type FarmerReqRegistInfo struct {
//
//	farmerNodeChan			chan *ChunkNodes
//}

type ElephantCross interface {
	GetStorageManager() *storage.Manager
	CheckFarmerAward(eleHeight uint64, blockHash common.Hash, chunkID uint64, h uint64)
	GetLeftRentSize() uint64
	GetUnusedChunkCnt(address common.Address, cIds []uint64) uint32
	GetBalance(addr common.Address) *big.Int
	ObjMetaQueryFunc(query *exporter.ObjMetaQuery)
	ObjFarmerQueryFunc(query *exporter.ObjFarmerQuery)
	ObjClaimChunkQueryFunc(query *exporter.ObjClaimChunkQuery) (uint64, error)
	ObjGetObjectFunc(query *exporter.ObjGetObjectQuery) exporter.ObjectsInfo
	ObjGetCliAddrFunc(query *exporter.ObjGetCliAddr) []common.CSAuthData
	ObjGetElephantDataFunc(query *exporter.ObjGetElephantData)
	ObjGetElephantDataRspFunc(query *exporter.ObjGetElephantData) *exporter.ObjGetElephantDataRsp
	GetSliceFromState(address common.Address, objId uint64) []*storagecore.Slice
	GetClaim(addr common.Address) (int, error)
	BlockChain() *core_elephant.BlockChain
}

// Blizzard Core Farmer Service implementation.
type Bliz struct {
	// Name should contain the official protocol name,
	// often a three-letter word.
	Name string

	// Version should contain the version number of the protocol.
	Version string

	shutdownChan chan struct{} // Channel for shutting down the ethereum

	// Logger is a custom logger to use with the blizzard Server.
	log log.Logger `toml:",omitempty"`

	// chunks Asso
	assoLock     sync.RWMutex
	chunkAssoMap map[storage.ChunkIdentity]*ChunkPartnerAsso // Currently running services

	// 维护chunk分片矿工节点池，Ts用来记录最近一次 ObjFarmerQuery
	// 成功的时间，如果太久不成功，下次是需要重新获取该分片矿工节点池的
	cachedShardingNodesMap map[int][]*discover.Node
	cachedShardingNodesTs  map[int]int64

	config *Config

	protocolManager *ProtocolManager
	nodeserver      *p2p.Server

	chunkdb *chunkDB

	queryMiner1Chan chan *common.Address
	queryMiner2Chan chan *ShardingNodes

	queryFarmerStep1Chan chan *storage.ChunkIdentity // Farmer 查询第一步：传入要查询的Farmer所属 ChunkId查询通道
	queryFarmerStep2Chan chan *ChunkNodes            // Farmer 查询第二步：该Chunk所属的分片矿工节点返回
	queryFarmerStep3Chan chan *ChunkNodes            // Farmer 查询第三步：从分片矿工处获得 Farmer 节点列表

	dailQueueLock sync.RWMutex
	dailQueue     queue.SliceFifo
	// 正在 dailing 的 nodeID map
	dailingTs map[discover.NodeID]int64

	// for client
	clientFarmerChan *chan *ChunkNodes
	clientMinerChan  *chan *ShardingNodes

	// for farmer partner
	knownHeartBeats *set.Set // Set of transaction hashes known to be known by this peer

	delaySyncSliceLock sync.RWMutex
	delaySyncSlice     map[string]int64
	EleCross           ElephantCross

	writeSliceHeaderCh  chan *exporter.ObjWriteSliceHeader
	writeSliceHeaderSub event.Subscription

	chunkPartnersCh  chan *exporter.ObjChunkPartners
	chunkPartnersSub event.Subscription

	farmerAwardCh  chan *exporter.ObjFarmerAward
	farmerAwardSub event.Subscription

	scope event.SubscriptionScope
}

// New creates a new P2P node, ready for protocol registration.
func New(ctx *node.ServiceContext, config *Config) (*Bliz, error) {
	var e *elephant.Elephant
	if err := ctx.Service(&e); err != nil {
		log.Error("New elephant service failed: elephant service not running: %v", err)
		return nil, err
	}

	chunkDBPath := ctx.ResolvePath("chunkinfos")
	chunkdb, err := CreateChunkDB(chunkDBPath, ChunkDBVersion)
	if err != nil {
		return nil, err
	}

	if config.Logger == nil {
		config.Logger = log.New()
	}

	// Note: any interaction with Config that would create/touch files
	// in the data directory or instance directory is delayed until Start.
	bliz := &Bliz{
		Name:    config.Name,
		Version: config.Version,
		log:     config.Logger,
		config:  config,
		chunkdb: chunkdb,

		shutdownChan: make(chan struct{}),

		queryMiner1Chan: make(chan *common.Address),
		queryMiner2Chan: make(chan *ShardingNodes),

		queryFarmerStep1Chan: make(chan *storage.ChunkIdentity),
		queryFarmerStep2Chan: make(chan *ChunkNodes),
		queryFarmerStep3Chan: make(chan *ChunkNodes),

		dailingTs: make(map[discover.NodeID]int64),

		chunkAssoMap: make(map[storage.ChunkIdentity]*ChunkPartnerAsso),

		// 缓存的分片矿工node： shardingIndex ==> miner nodes[]
		cachedShardingNodesMap: make(map[int][]*discover.Node),
		cachedShardingNodesTs:  make(map[int]int64),

		clientFarmerChan: nil,
		clientMinerChan:  nil,
		knownHeartBeats:  set.New(),

		delaySyncSlice: make(map[string]int64),
		EleCross:       e,
	}

	terminalHandle := log.LvlFilterHandler(log.LvlWarn, log.GetDefaultTermHandle(1))
	path := ctx.ResolvePath("blizzard.txt")
	fileHandle := log.LvlFilterHandler(log.LvlInfo, log.GetDefaultFileHandle(path))
	bliz.Log().SetHandler(log.MultiHandler(terminalHandle, fileHandle))
	bliz.Log().Info("Initialising Blizzard protocol", "versions", ProtocolVersions, "network", config.NetworkId)

	if bliz.protocolManager, err = NewProtocolManager(config.NetworkId, chunkdb, bliz); err != nil {
		return nil, err
	}

	return bliz, nil
}

func NewForTest(ele ElephantCross, config *Config) (*Bliz, error) {
	chunkdb, err := CreateChunkDB("", ChunkDBVersion)
	if err != nil {
		return nil, err
	}

	if config.Logger == nil {
		config.Logger = log.New()
	}

	// Note: any interaction with Config that would create/touch files
	// in the data directory or instance directory is delayed until Start.
	bliz := &Bliz{
		Name:    config.Name,
		Version: config.Version,
		log:     config.Logger,
		chunkdb: chunkdb,

		shutdownChan: make(chan struct{}),

		queryMiner1Chan: make(chan *common.Address),
		queryMiner2Chan: make(chan *ShardingNodes),

		queryFarmerStep1Chan: make(chan *storage.ChunkIdentity),
		queryFarmerStep2Chan: make(chan *ChunkNodes),
		queryFarmerStep3Chan: make(chan *ChunkNodes),

		dailingTs: make(map[discover.NodeID]int64),

		chunkAssoMap: make(map[storage.ChunkIdentity]*ChunkPartnerAsso),

		// 缓存的分片矿工node： shardingIndex ==> miner nodes[]
		cachedShardingNodesMap: make(map[int][]*discover.Node),
		cachedShardingNodesTs:  make(map[int]int64),

		clientFarmerChan: nil,
		clientMinerChan:  nil,
		knownHeartBeats:  set.New(),

		delaySyncSlice: make(map[string]int64),
		EleCross:       ele,
	}

	if bliz.protocolManager, err = NewProtocolManager(config.NetworkId, chunkdb, bliz); err != nil {
		return nil, err
	}

	return bliz, nil
}

// CreateDB creates the chain database.
func CreateChunkDB(chunkDBPath string, version int) (*chunkDB, error) {
	// If no node database was given, use an in-memory one
	db, err := newChunkDB(chunkDBPath, version)
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

func (b *Bliz) NodeServer() *p2p.Server {
	return b.nodeserver
}

// 客户端服务将自己的minerChan接收通道设定过来
func (b *Bliz) ClientSetMinerChan(pChan *chan *ShardingNodes) {
	b.clientMinerChan = pChan
}

// 客户端服务将自己的farmerChan接收通道设定过来
func (b *Bliz) ClientSetFarmerChan(pChan *chan *ChunkNodes) {
	b.clientFarmerChan = pChan
}

func (b *Bliz) DailFarmer(chunkId storage.ChunkIdentity, cbMode uint32, nodeId *discover.NodeID) bool {
	b.dailQueueLock.Lock()
	defer b.dailQueueLock.Unlock()

	b.Log().Info("DailFarmer", "dailingTs len", len(b.dailingTs), "chunkId", chunkId, "cbMode", cbMode, "nodeId", nodeId)
	// 不能超过同时拨号的个数限制
	if len(b.dailingTs) < CoDailingFarmerMax {
		if _, ok := b.dailingTs[*nodeId]; !ok {
			b.dailQueue.Put(&farmerDail{chunkId: chunkId, cbMode: cbMode, node: nodeId})

			return true
		}
	}
	return false
}

func (b *Bliz) InDailing(nodeId *discover.NodeID) bool {
	_, ok := b.dailingTs[*nodeId]
	return ok
}

func sendChanFarmerStep2(ex chan struct{}, ch chan *ChunkNodes, va *ChunkNodes) {
	select {
	case <-ex:
	case ch <- va:
	}
}

func sendChanFarmerStep3(ex chan struct{}, ch chan *ChunkNodes, va *ChunkNodes) {
	select {
	case <-ex:
	case ch <- va:
	}
}

func sendChanMiner2(ex chan struct{}, ch chan *ShardingNodes, va *ShardingNodes) {
	select {
	case <-ex:
	case ch <- va:
	}
}

func sendChanClientMiner(ex chan struct{}, ch chan *ShardingNodes, va *ShardingNodes) {
	select {
	case <-ex:
	case ch <- va:
	}
}

func sendChanClientFarmer(ex chan struct{}, ch chan *ChunkNodes, va *ChunkNodes) {
	select {
	case <-ex:
	case ch <- va:
	}
}

func sendChanConnetPeer(ex chan struct{}, ch chan interface{}, va *p2p.Peer) {
	select {
	case <-ex:
	case ch <- va:
	}
}

// 从本地的节点K桶中以及备份nodes数据库中获取有关chunkId分片的矿工节点组
// 如果是从K桶中获取，则可以直接使用，因为K桶中的数据都是ping/pong过的
// 如果是从nodesDB中取，则需要bond一下，返回bond成功的节点组
func (b *Bliz) GetShardingMiner(shardingIdx int, rtype interface{}) {
	go func() {
		for {
			maxAge := 5 * 24 * time.Hour
			nodes := make([]*discover.Node, 32)
			ret := b.nodeserver.GetChunkAreaShardingNode(shardingIdx, maxAge, nodes)
			b.Log().Trace("GetChunkAreaShardingNode", "ret", ret)
			if ret >= NeedShardingNodeCount {
				// 获取到NeedShardingNodeCount个也可以发信号了
				if chunkId, ok := rtype.(storage.ChunkIdentity); ok {
					b.Log().Info("GetShardingMiner", "chunkId", storage.ChunkIdentity2Int(chunkId))
					sendChanFarmerStep2(b.shutdownChan, b.queryFarmerStep2Chan, &ChunkNodes{ChunkId: chunkId, Nodes: nodes[:ret]})
				} else if address, ok := rtype.(common.Address); ok {
					b.Log().Info("GetShardingMiner", "address", address)
					sendChanMiner2(b.shutdownChan, b.queryMiner2Chan, &ShardingNodes{Address: address, Nodes: nodes[:ret]})
				}
				return
			} else {
				time.Sleep(2 * time.Second)
			}
		}
	}()
}

// 刷新chunk分片节点缓冲的时间戳，表示它能正常工作
func (b *Bliz) RefreshCachedShardingNodes(shardingIdx int) {
	//shardingIdx := sharding.ShardingChunkMagic(sharding.ShardingCount, chunkId.String())
	b.cachedShardingNodesTs[shardingIdx] = time.Now().Unix()
}

// 准备chunk分片节点缓冲：判断它在查询过程中超时，如果是则置空它，否则留用
func (b *Bliz) DoChunkShardingNodesTimeout(chunkId storage.ChunkIdentity) {
	shardingIdx := sharding.ShardingChunkMagic(sharding.ShardingCount, chunkId.String())

	if ts, ok := b.cachedShardingNodesTs[shardingIdx]; ok {
		if time.Now().Unix()-ts > ShardingNodesCacheTimeout {
			// 超过5分钟没有刷新过，就表示清除掉，下次重新
			// 从K桶和 nodes节点数据库获取
			delete(b.cachedShardingNodesTs, shardingIdx)
		}
	}
}

func (b *Bliz) QueryShardingMiner(address common.Address) {
	select {
	case <-b.shutdownChan:
	case b.queryMiner1Chan <- &address:
	}
}

func (b *Bliz) QueryChunkFarmer(chunkId storage.ChunkIdentity) {
	select {
	case <-b.shutdownChan:
	case b.queryFarmerStep1Chan <- &chunkId:
	}
}

// 向sharding节点组询问某 chunk 的责任 farmers 组
func (b *Bliz) queryChunkFarmer(chunkId storage.ChunkIdentity, nodes []*discover.Node) {
	b.Log().Info("queryChunkFarmer", "nodes len", len(nodes))
	// 向这些节点发起Chunk Farmer查询请求
	connectPeerCh := make(chan *p2p.Peer)
	var connectPeerFeed event.Feed
	connectPeerSub := b.scope.Track(connectPeerFeed.Subscribe(connectPeerCh))
	defer connectPeerSub.Unsubscribe()

	var totalCnt = 0
	for i := 0; i < len(nodes); i++ {
		// 过滤自己
		if nodes[i].Equal(b.SelfNode()) {
			continue
		}
		// 发起连接
		peer := b.nodeserver.GetPeer(nodes[i].ID)
		if peer != nil {
			// 本地已经跟对方建立了连接了，而elephant连接是gctt基础服务，
			// 该协议通通支持，我们可以直接使用
			// 这里为了不堵塞（互锁），必须使用 go 来喂
			go connectPeerFeed.Send(peer)
		} else {
			// 向 sharding node 发起 farmer 查询连接
			b.nodeserver.AddPeer(&discover.NodeCB{Node: nodes[i], Cb: nil, PeerProtocol: "elephant", PeerFeed: &connectPeerFeed})
		}
		totalCnt++
	}

	// 等待请求结果反馈
	var cnt = 0
	var farmerCombos = make([]*[]discover.Node, 0)
	rspCh := make(chan *exporter.FarmerRsp)
	var rspFeed event.Feed
	rspSub := b.scope.Track(rspFeed.Subscribe(rspCh))
	defer rspSub.Unsubscribe()

	isChunkFarmer := storage.HasChunk(storage.ChunkIdentity2Int(chunkId))
	otherFarmerCnt := blizparam.GetOtherMinFarmerCount(isChunkFarmer)

_wait:
	for {
		select {
		case <-b.shutdownChan:
			return

		case peer := <-connectPeerCh:
			b.Log().Info("select", "peer", peer)
			// 发送farmerquery请求
			var req = &exporter.ObjFarmerQuery{Peer: peer, RspFeed: &rspFeed, ChunkId: chunkId}
			b.EleCross.ObjFarmerQueryFunc(req)

		case <-connectPeerSub.Err():
			return

		case rsp := <-rspCh:
			b.Log().Info("select", "rsp", rsp, "totalCnt", totalCnt)
			farmerCombos = append(farmerCombos, rsp.Farmers)
			cnt++
			if cnt >= otherFarmerCnt {
				break _wait
			}

		case <-rspSub.Err():
			return

		case <-time.After(3 * time.Second):
			break _wait
		}
	}

	// TODO:发起查询
	// 首先需要建立连接才能进行查询
	//if len(farmerCombos) > 1 {
	// qiwy: for test, TODO
	if len(farmerCombos) >= 1 {
		// 获得了两组以上回复，我们取相同部分
		var qFarmers = make([]*discover.Node, 0)
		if len(farmerCombos) > 1 {
			for _, nid := range *(farmerCombos[0]) {
				for _, nid1 := range *(farmerCombos[1]) {

					if nid.Equal(&nid1) {
						nidTmp := nid
						qFarmers = append(qFarmers, &nidTmp)
						break
					}
				}
			}
		} else {
			for _, nid := range *(farmerCombos[0]) {
				nidTmp := nid
				qFarmers = append(qFarmers, &nidTmp)
			}
		}

		//if len(qFarmers) < 4 {
		// qiwy: for test, TODO
		if len(qFarmers) < 1 {
			// 还不能提供服务，farmer数量太少
			// TODO: 这里可以考虑提供一个通知
			// 机制，可以决策让该 chunk 的farmers
			// 查询直接失败，也可以选择多等些时间
			// 再查，因为这个chunk可能还未认领完毕
			// 而处于 unavailible 状态

			// (暂时的处理方法是直接返回，让本次
			// 查询超时)
			return
		}

		farmers := make([]discover.Node, 0, len(qFarmers))
		for _, node := range qFarmers {
			farmers = append(farmers, *node)
		}

		// 刷新数据库记录，这样就将淘汰的partner自然过滤了
		// 而内存中的记录，我们就留给 assoLoop 中的运行Ts时间戳判断去删除
		//b.chunkdb.UpdateNode(&ChunkDbInfo{Id: chunkId, Partners: farmers})
		sendChanFarmerStep3(b.shutdownChan, b.queryFarmerStep3Chan, &ChunkNodes{ChunkId: chunkId, Nodes: qFarmers})

		// 成功获取到 farmers 列表，我们将这个分片的矿工缓冲刷新一下
		shardingIdx := sharding.ShardingChunkMagic(sharding.ShardingCount, chunkId.String())
		b.RefreshCachedShardingNodes(shardingIdx)

		//for _,farmer := range qFarmers {
		//	asso.bliz.dailQueue.Put(farmer)
		//	asso.dailing[farmer] = true
		//}
	}
}

// *----------------------------------------------------------------*
// *																*
// *   Blizzard 主逻辑，主要维持本地 Claim 的 Chunk Asso 的服务状态	   *
// *																*
// *----------------------------------------------------------------*
func (b *Bliz) mainLoop() {

	// 获取到 partnerList 列表，开始按照一定的量控制
	// 机制（算法），寻求一批 partner nodeID 请求
	// nodeserver 进行 dail连接
	// （维持一份 chunk partners connection 的通道，
	//  控制同时发起的 partners connection 的数量）
	// （成功获取足够 partner 连接的 chunk 设置其状态
	//  为 online状态，否则都是 offline 状态）

	// 进入一个无限loop过程，监督当前的 partner
	// 连接数量，断开长久不联系的 partner, 并监听
	// 新的 partner 连接需求进行 dail 连接

	// --------------------------------------------
	select {
	case <-b.shutdownChan:
		return
	}

}

func (b *Bliz) shardingLoop() {

	// Wait for different events to fire synchronisation operations
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-b.shutdownChan:
			return

		case <-ticker.C:
		}
	}

}

func (b *Bliz) assoLoop() {
	// 加载本地 blizDB，从中将本地参与 claim 的所有
	// chunk 信息读取出来建立各个chunk的Asso
	b.LoadLocalChunkAsso()
	// Wait for different events to fire synchronisation operations
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-b.shutdownChan:
			return

		case <-ticker.C:
			// 每5秒触发一下各个 asso 的刷新
			b.Log().Trace("ticker.C")
			b.assoLock.RLock()
			for _, asso := range b.chunkAssoMap {
				asso.run()
				b.Log().Trace("chunkAssoMap")
			}
			b.assoLock.RUnlock()

		case info := <-b.chunkPartnersCh:
			bExist := false
			for _, node := range info.Partners {
				if node.Equal(b.SelfNode()) {
					bExist = true
					break
				}
			}
			if !bExist {
				break
			}
			cId := storage.Int2ChunkIdentity(info.ChunkId)
			chunkInfo := &ChunkDbInfo{Id: cId, Partners: info.Partners}
			v, ok := b.chunkAssoMap[cId]
			if ok {
				for _, node := range info.Partners {
					nodeId := node.ID
					partner := b.protocolManager.GetPartner(nodeId.String())
					if partner != nil {
						v.partners[nodeId] = partner
					}
				}
			} else {
				chunkInfos := []*ChunkDbInfo{chunkInfo}
				b.initChunkAsso(chunkInfos)

			}
			b.chunkdb.UpdateNode(chunkInfo)
		case <-b.chunkPartnersSub.Err():
			return

		case info := <-b.farmerAwardCh:
			b.assoLock.RLock()
			for id, _ := range b.chunkAssoMap {
				chunkID := storage.ChunkIdentity2Int(id)
				b.EleCross.CheckFarmerAward(info.EleHeight, info.BlockHash, chunkID, info.EleHeight)
			}
			b.assoLock.RUnlock()
		case <-b.farmerAwardSub.Err():
			return

		case info := <-b.writeSliceHeaderCh:
			var found bool
			b.assoLock.RLock()
			for id, _ := range b.chunkAssoMap {
				chunkID := storage.ChunkIdentity2Int(id)
				if chunkID == info.Slice.ChunkId {
					found = true
					break
				}
			}
			b.assoLock.RUnlock()
			if found {
				b.EleCross.GetStorageManager().WriteSliceHeader(info.Addr, info.Slice, info.OpId)
			}
		case <-b.writeSliceHeaderSub.Err():
			return

		}
	}
}

func (b *Bliz) connLoop() {

	// 遍历 Asso 列表，向Asso从属的分片矿工发送获取
	// Chunk claimer List 的请求，这里要求为本地存储
	// 的 nodeDB 中寻找从属于这些分片的节点信息予以
	// 优先 dail. 这样才能在这里执行询问过程。
	// 注：为了防止矿工作弊，同时请求分片的5个矿工获取
	// 取返回列表中的相同部分
	// （维持一份 chunk claimers query 的通道，控制
	//  同时发起的 claimers query 的数量）
	dailTicker := time.NewTicker(5 * time.Second)
	defer dailTicker.Stop()

	for {
		select {
		case <-b.shutdownChan:
			return

		/*
			负责管理 nodeId 的分片的矿工节点组的获取流程，主要调用者为 BlizCS 模块
			BlizCS 模块传入目标 nodeId, 这里调度获取管理它的分片的矿工们
		*/
		case address := <-b.queryMiner1Chan:
			if address != nil {
				// 其二是chunk所属分片矿工组查询 farmer, 这一步比较麻烦，首先需要获取分组矿工列表
				// 然后跟矿工建立联系，然后获取 farmer 列表
				shardingIdx := sharding.ShardingAddressMagic(sharding.ShardingCount, address.String())
				if b.cachedShardingNodesMap[shardingIdx] == nil {
					// 这个分片已经还没有在本地请求过，需要去查询并新建联系
					b.GetShardingMiner(shardingIdx, *address)
				} else {
					// 对于这种已经有了联系node列表的情况，我们可以设计
					// 更加复杂的过期机制，重新获取新nodes组或者补充现有
					// nodes组
					// TODO:
					go func() {
						sendChanMiner2(b.shutdownChan, b.queryMiner2Chan, &ShardingNodes{Address: *address, Nodes: b.cachedShardingNodesMap[shardingIdx]})
					}()
				}
			}

		case shardingNodes := <-b.queryMiner2Chan:
			b.Log().Trace("connLoop", "shardingNodes", shardingNodes, "Nodes len", len(shardingNodes.Nodes), "clientMinerChan", b.clientMinerChan)
			// 通告本地 Client 管理器
			if b.clientMinerChan != nil {
				// 防对方堵塞我，这里用 go routine
				go func() { sendChanClientMiner(b.shutdownChan, *b.clientMinerChan, shardingNodes) }()
			}

		/*
			负责提供 chunkId 服务的 Farmer 节点组的获取流程：
			主要调用者：
				1. 本模块的 Partner Asso (如果自己也是一个farmer)
				2. BlizCS 模块(如果自己启动客户端服务)

			经历三个步骤：
				1. 传入目标 chunkId，获取负责该 chunkId 管理的分片的矿工组
				2. 向这个矿工组发起查询，询问该 chunkId 的具体服务 Farmer
				3. 将 Farmer 提供给调用者（ Asso 或者 BlizCS ）
		*/
		case chunkId := <-b.queryFarmerStep1Chan:
			if chunkId != nil {

				// 其二是chunk所属分片矿工组查询 farmer, 这一步比较麻烦，首先需要获取分组矿工列表
				// 然后跟矿工建立联系，然后获取 farmer 列表
				shardingChunkIdx := sharding.ShardingChunkMagic(sharding.ShardingCount, chunkId.String())
				if b.cachedShardingNodesMap[shardingChunkIdx] == nil {
					// 这个分片已经还没有在本地请求过，需要去查询并新建联系
					b.GetShardingMiner(shardingChunkIdx, *chunkId)
				} else {
					// 对于这种已经有了联系node列表的情况，我们可以设计
					// 更加复杂的过期机制，重新获取新nodes组或者补充现有
					// nodes组
					// TODO:

					//asso := b.chunkAssoMap[chunkId.String()]
					//if asso.status == AssoStatusQuerying {
					go b.queryChunkFarmer(*chunkId, b.cachedShardingNodesMap[shardingChunkIdx])
					//}
				}
			}
		case chunkNodes := <-b.queryFarmerStep2Chan:
			b.Log().Trace("connLoop", "chunkNodes", chunkNodes)
			if chunkNodes != nil && len(chunkNodes.Nodes) >= QueryFarmerStep2ChanCount {
				// 可以请求分片矿工获取chunk人员列表了
				shardingIdx := sharding.ShardingChunkMagic(sharding.ShardingCount, chunkNodes.ChunkId.String())
				b.cachedShardingNodesMap[shardingIdx] = chunkNodes.Nodes

				//asso := b.chunkAssoMap[chunkNodes.chunkId.String()]
				//if asso.status == AssoStatusQuerying {
				go b.queryChunkFarmer(chunkNodes.ChunkId, b.cachedShardingNodesMap[shardingIdx])
				//}
			}
		case farmerNodes := <-b.queryFarmerStep3Chan:
			b.Log().Trace("connLoop", "Nodes len", len(farmerNodes.Nodes))
			if farmerNodes != nil &&
				len(farmerNodes.Nodes) >= ChunkFarmerMin &&
				len(farmerNodes.Nodes) <= ChunkFarmerMax {

				// 从分片矿工获取到了 chunk 的 farmer 列表

				// 先不要拨号，只添加 partner到各个asso中, 由它们自行处置并拨号这些 partner
				for _, node := range farmerNodes.Nodes {
					b.RegisterPartner(node.ID, farmerNodes.ChunkId, false)
				}

				asso := b.chunkAssoMap[farmerNodes.ChunkId]
				if asso != nil {
					//if asso.status == AssoStatusQuerying {
					asso.status = AssoStatusNormal
					//}
				}

				// 也通告一下本地 Client 管理器
				if b.clientFarmerChan != nil {
					// 防对方堵塞我，这里用 go routine
					b.Log().Trace("connLoop", "farmerNodes", farmerNodes)
					nodes := *farmerNodes
					go func(nodes *ChunkNodes) { sendChanClientFarmer(b.shutdownChan, *b.clientFarmerChan, farmerNodes) }(&nodes)
				}
			}

		case <-dailTicker.C:
			// 定时走队列流程

			// 从fifo队列获取需要连接的对象信息
			// 然后开始建立连接
			for {
				b.Log().Trace("connLoop", "dailingTs len", len(b.dailingTs))
				if len(b.dailingTs) < CoDailingFarmerMax {

					b.dailQueueLock.RLock()
					element := b.dailQueue.Get()
					b.dailQueueLock.RUnlock()
					b.Log().Trace("connLoop", "element", element)
					if element != nil {
						if dail, ok := element.(*farmerDail); ok {
							if _, ok := b.dailingTs[*dail.node]; !ok {
								b.Log().Trace("connLoop", "node", dail.node)
								b.dailingTs[*dail.node] = time.Now().Unix()
								b.nodeserver.AddPeer(&discover.NodeCB{Node: discover.NewNode(*dail.node, nil, 0, 0),
									Cb:           BlizNodeCB{ChunkId: dail.chunkId, CbMode: dail.cbMode},
									PeerProtocol: "",
									PeerFeed:     nil})
							}
						}
					} else {
						break
					}
				} else {
					break
				}
			}

			now := time.Now().Unix()
			for node, ts := range b.dailingTs {
				if now-ts > 180 {
					// 超过180秒
					b.dailQueueLock.Lock()
					delete(b.dailingTs, node)
					b.dailQueueLock.Unlock()
				}
			}
		}

	}
}

// *----------------------------------------------------------------*
// *																*
// *   Blizzard Partner数据同步逻辑									 *
// *																*
// *----------------------------------------------------------------*
func (b *Bliz) partnerSyncLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-b.shutdownChan:
			return

		case <-ticker.C:
		}
	}
}

func (b *Bliz) newEmptyChunk(chunkId storage.ChunkIdentity) (*ChunkPartnerAsso, error) {

	b.assoLock.Lock()
	defer b.assoLock.Unlock()

	if b.chunkAssoMap[chunkId] != nil {
		return b.chunkAssoMap[chunkId], nil
	}

	asso := &ChunkPartnerAsso{
		bliz:                b,
		chunkId:             chunkId,
		status:              AssoStatusIdle,
		onlineTs:            0,
		dailstep:            2,
		partners:            make(map[discover.NodeID]*partner),
		refreshTs:           make(map[discover.NodeID]int64),
		quitChan:            make(chan struct{}),
		syncCheckHeaderHash: make(map[uint32][]*checkHeaderHash),
		syncingSlice:        make(map[uint32]*partnerProg),
	}

	b.chunkAssoMap[chunkId] = asso
	return asso, nil
}

func (b *Bliz) insertChunk(asso *ChunkPartnerAsso) error {

	b.assoLock.Lock()
	defer b.assoLock.Unlock()

	if b.chunkAssoMap[asso.chunkId] != nil {
		return blizcore.ErrAssoAlreadyExist
	}

	b.chunkAssoMap[asso.chunkId] = asso
	return nil
}

/*
 * 从本地保存的chunk信息建立
 * 本地Claim的 Chunk Asso 列表
 */
func (b *Bliz) LoadLocalChunkAsso() {
	chunkIds := make([]storage.ChunkIdentity, 0, ChunkClaimMax)
	for cId := uint64(1); cId <= ChunkClaimMax; cId++ {
		chunkIds = append(chunkIds, storage.Int2ChunkIdentity(cId))
	}
	chunkInfos := b.chunkdb.AllChunks2(chunkIds, -1)
	b.initChunkAsso(chunkInfos)
}

// chunkInfos中不会有重复的blizstorage.ChunkIdentity
func (b *Bliz) initChunkAsso(chunkInfos []*ChunkDbInfo) {
	b.Log().Info("initChunkAsso", "chunkInfos len", len(chunkInfos))
	for _, info := range chunkInfos {
		b.Log().Info("", "info", info)
		asso := &ChunkPartnerAsso{
			bliz:                b,
			chunkId:             info.Id,
			status:              AssoStatusIdle,
			onlineTs:            0,
			dailstep:            2,
			partners:            make(map[discover.NodeID]*partner),
			refreshTs:           make(map[discover.NodeID]int64),
			quitChan:            make(chan struct{}),
			syncCheckHeaderHash: make(map[uint32][]*checkHeaderHash),
			syncingSlice:        make(map[uint32]*partnerProg),
		}

		if asso.PrepareLocalStorage() {
			for _, node := range info.Partners {
				asso.RegisterPartner(node.ID, info.refreshAt.Unix())
			}
			b.insertChunk(asso)
			go asso.Start()
		}
	}
}

func (b *Bliz) GetAsso(chunkId storage.ChunkIdentity) *ChunkPartnerAsso {
	asso := b.chunkAssoMap[chunkId]
	return asso
}

/*
 * 清除一个Asso
 */
func (b *Bliz) DeleteAsso(chunkId storage.ChunkIdentity) {
	b.assoLock.Lock()
	defer b.assoLock.Unlock()

	asso := b.chunkAssoMap[chunkId]
	if asso != nil {
		// 通知 asso ，要停止工作了
		close(asso.quitChan)
		// 从本地map清除
		delete(b.chunkAssoMap, chunkId)
		// 从数据库清除
		b.chunkdb.DeleteNode(chunkId)
	}
}

/*
 * 注册 partner
 * 从矿工处获得 chunk 相关的矿工组 NodeID 后，这里执行添加工作，将它们加入到chunk对应的 asso 中去
 * 这里只是注册，而不是 Login, Login是建立连接后才进行的，但是如果之前没有在这里注册，Login是不会
 * 通过的。
 */
func (b *Bliz) RegisterPartner(nodeId discover.NodeID, chunkId storage.ChunkIdentity, dail bool) (int, error) {

	asso := b.chunkAssoMap[chunkId]
	// asso.Log().Info("RegisterPartner", "chunkId", chunkId)
	if asso == nil {
		b.Log().Info("RegisterPartner asso nil")
		return 0, blizcore.ErrLocalNotChunkFarmer
	}

	partner := asso.GetPartner(nodeId)
	asso.Log().Info("RegisterPartner", "nodeId", nodeId, "partner", partner)
	if partner == nil {
		asso.RegisterPartner(nodeId, time.Now().Unix())
	} else {
		return 1, nil
	}

	// 查看是否本地 pm 已经有了对应的 partner 连接上来
	if partner := b.protocolManager.GetPartner(nodeId.String()); partner != nil {
		// 本地已经存在了，这样就不需要将它加入到拨号dail队列
		if _, err := asso.LoginPartner(partner); err != nil {
			b.Log().Error("LoginPartner", "err", err)
			return 0, err
		}
		return 1, nil
	}

	// 本地 pm 没有，我们分配拨号任务

	// 这里需要加入连接数控制，方案是实现一个队列fifo，将请求加入队列
	//if dail {
	//	b.dailQueue.Put(&discover.NodeCB{Node:discover.NewNode(nodeId, nil, 0, 0), Cb:chunkId})
	//b.nodeserver.AddPeer( &discover.NodeCB{Node:discover.NewNode(nodeId, nil, 0, 0), Cb:chunkId} )
	//}
	return 2, nil
}

func (b *Bliz) AmiLocalFarmer(chunkId storage.ChunkIdentity) bool {
	b.Log().Info("AmiLocalFarmer", "chunkAssoMap", b.chunkAssoMap)
	asso := b.chunkAssoMap[chunkId]
	if asso == nil {
		return false
	}
	b.Log().Trace("AmiLocalFarmer", "asso", asso)
	if asso.IsAbandom() {
		return false
	}
	return true
}

func (b *Bliz) AllowPartnerLogin(chunkId storage.ChunkIdentity, partner *partner) bool {
	asso := b.chunkAssoMap[chunkId]
	if asso == nil {
		return false
	}
	return asso.AllowPartnerLogin(partner)
}

func (b *Bliz) LoginPartner(partner *partner, chunkId storage.ChunkIdentity) (*partner, error) {

	b.assoLock.RLock()
	defer b.assoLock.RUnlock()

	b.Log().Info("LoginPartner", "chunkAssoMap", b.chunkAssoMap)
	for _, asso := range b.chunkAssoMap {
		asso.LoginPartner(partner)
	}

	delete(b.dailingTs, partner.ID())

	return nil, nil
	/*
		asso := b.chunkAssoMap[chunkId]
		if asso == nil {
			return nil, ErrLocalNotChunkClaimer
		}
		_, err := asso.LoginPartner(partner)
		if err == nil {
			return partner, nil
		}
		return nil, err
	*/
}

func (b *Bliz) LogoutPartner(partner *partner /*, chunkId storage.ChunkIdentity*/) {

	for _, asso := range partner.chunkAssos {

		// 我们这里不删除，因为这里的 id->partner 映射关系从
		// 本地chunk数据库加载后，将所有chunk的member.ID都
		// 建立在这里，发现其指向的partner对象为空，就可以去
		// 尝试connect
		//delete( asso, partner.ID() )

		// 置空，但member.ID->nil关系保留，可以通过遍历 member.ID
		// 重新连接建立 partner 对象
		asso.LogoutPartner(partner)
	}
}

func (bliz *Bliz) MarkHeartBeat(hbdata *blizcore.HeartBeatMsgData) {
	for bliz.knownHeartBeats.Size() >= MaxKnownHeartBeats {
		bliz.knownHeartBeats.Pop()
	}
	bliz.knownHeartBeats.Add(hbdata.Hash())
}

func (bliz *Bliz) HasHeartBeat(hbdata *blizcore.HeartBeatMsgData) bool {
	return bliz.knownHeartBeats.Has(hbdata.Hash())
}

// ----------------------- Node.Service Interface Implementation --------------------------

// APIs returns the collection of RPC services the ethereum package offers.
// NOTE, some of these services probably need to be moved to somewhere else.
func (bliz *Bliz) APIs() []rpc.API {
	apis := []rpc.API{
		{
			Namespace: "blizzard",
			Version:   "1.0",
			Service:   NewPublicBlizzardAPI(bliz),
			Public:    true,
		},
	}
	// Append all the local APIs and return
	return append(apis, []rpc.API{}...)
}

// Protocols implements node.Service, returning all the currently configured
// network protocols to start.
func (bliz *Bliz) Protocols() []p2p.Protocol {
	return bliz.protocolManager.SubProtocols
}

// Start is called after all services have been constructed and the networking
// layer was also initialized to spawn any goroutines required by the service.
func (bliz *Bliz) Start(server *p2p.Server) error {
	bliz.writeSliceHeaderCh = make(chan *exporter.ObjWriteSliceHeader)
	bliz.writeSliceHeaderSub = bliz.EleCross.BlockChain().SubscribeWriteSliceHeader(bliz.writeSliceHeaderCh)

	bliz.chunkPartnersCh = make(chan *exporter.ObjChunkPartners)
	bliz.chunkPartnersSub = bliz.EleCross.BlockChain().SubscribeChunkPartners(bliz.chunkPartnersCh)

	bliz.farmerAwardCh = make(chan *exporter.ObjFarmerAward)
	bliz.farmerAwardSub = bliz.EleCross.BlockChain().SubscribeFarmerAward(bliz.farmerAwardCh)

	bliz.nodeserver = server
	bliz.protocolManager.Start()

	// 启动各个主要的工作 routine
	go bliz.mainLoop()
	go bliz.partnerSyncLoop()

	go bliz.assoLoop()
	go bliz.connLoop()
	go bliz.shardingLoop()

	// 启动 chunkDB
	//bliz.chunkdb.ensureExpirer() // qiwy: TODO, for test

	return nil
}

// Stop implements node.Service, terminating all internal goroutines used by the
// Ethereum protocol.
func (bliz *Bliz) Stop() error {

	bliz.writeSliceHeaderSub.Unsubscribe()

	bliz.protocolManager.Stop()
	close(bliz.shutdownChan)

	// 关闭 chunkDB
	bliz.chunkdb.close()

	bliz.scope.Close()
	return nil
}

func (bliz *Bliz) GetStorageManager() *storage.Manager {
	return bliz.EleCross.GetStorageManager()
}

// qiwy: TODO, 后期需与doWriteReq统一
func (bliz *Bliz) WriteOpDataFromMsg(req *blizcore.WriteOpMsgData) error {
	bliz.Log().Info("WriteOpDataFromMsg", "Info", req.Info)
	chunkId := storage.Int2ChunkIdentity(req.Info.Chunk)
	asso := bliz.GetAsso(chunkId)
	if asso == nil {
		bliz.Log().Error("GetAsso nil")
		return errors.New("not asso")
	}

	if bliz.AmiLocalFarmer(chunkId) == false {
		bliz.Log().Error("AmiLocalFarmer", "chunkId", chunkId)
		return blizcore.ErrLocalNotChunkFarmer
	}

	header := bliz.GetStorageManager().ReadChunkSliceHeader(req.Info.Chunk, req.Info.Slice)
	if header == nil {
		bliz.Log().Error("ReadChunkSliceHeader nil")
		return blizcore.ErrSliceHeader
	}

	// Discard because of wop id
	if req.Info.WopId != header.OpId+1 {
		return nil
	}

	asso.Log().Info("", "Infos len", len(*req.Info.Infos))
	pointer := 0
	for _, info := range *req.Info.Infos {
		bliz.Log().Trace("WriteOpDataFromMsg", "Datas[pointer]", req.Datas[pointer])
		bliz.GetStorageManager().WriteChunkSlice(req.Info.Chunk, req.Info.Slice, info.Offset, req.Datas[pointer], req.Info.WopId, header.OwnerId)
		pointer++
	}
	bliz.GetStorageManager().SignChunkSlice(req.Info.Chunk, req.Info.Slice, req.Sign)
	asso.UpdateWopProg(req.Info.Slice, req.Info.WopId)

	return nil
}

func (bliz *Bliz) SelfNode() *discover.Node {
	return bliz.nodeserver.Self()
}

func (bliz *Bliz) Log() log.Logger {
	return bliz.log
}

func (b *Bliz) ShowAsso(id uint64) {
	chunkId := storage.Int2ChunkIdentity(id)
	for cId, asso := range b.chunkAssoMap {
		if id != 0 && chunkId != cId {
			continue
		}
		if asso == nil {
			log.Info("ChunkPartnerAsso nil", "chunkId", storage.ChunkIdentity2Int(cId))
			continue
		}
		asso.lock.RLock()
		for nodeId, partner := range asso.partners {
			title := "asso"
			if b.SelfNode().ID.Equal(nodeId) {
				title += " self node"
			}
			log.Info(title, "nodeId", nodeId, "partner", partner)
		}
		asso.lock.RUnlock()
	}
}

// func delaySyncKey(chunkId uint64, sliceId uint32) string {
// 	key := fmt.Sprintf("%d:%d", chunkId, sliceId)
// 	return key
// }

// func (b *Bliz) AddDelaySyncSlice(chunkId uint64, sliceId uint32) {
// 	b.delaySyncSliceLock.Lock()
// 	defer b.delaySyncSliceLock.Unlock()

// 	now := time.Now().Unix()
// 	key := delaySyncKey(chunkId, sliceId)
// 	b.delaySyncSlice[key] = now
// }

// func (b *Bliz) DelDelaySyncSlice(chunkId uint64, sliceId uint32) {
// 	b.delaySyncSliceLock.Lock()
// 	defer b.delaySyncSliceLock.Unlock()

// 	key := delaySyncKey(chunkId, sliceId)
// 	_, ok := b.delaySyncSlice[key]
// 	if !ok {
// 		return
// 	}
// 	delete(b.delaySyncSlice, key)
// }

// // 读写10秒后才同步
// func (b *Bliz) IsDelaySyncSlice(chunkId uint64, sliceId uint32) bool {
// 	b.delaySyncSliceLock.RLock()
// 	defer b.delaySyncSliceLock.RUnlock()

// 	key := delaySyncKey(chunkId, sliceId)
// 	v, ok := b.delaySyncSlice[key]
// 	if !ok {
// 		return false
// 	}
// 	now := time.Now().Unix()
// 	if now < v+10 {
// 		return true
// 	}
// 	return false
// }

func (b *Bliz) GetRegisterPartner(id string) *partner {
	return b.protocolManager.partners.Partner(id)
}
