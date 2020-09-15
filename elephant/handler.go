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
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/contatract/go-contatract/accounts"
	"github.com/contatract/go-contatract/baseparam"
	"github.com/contatract/go-contatract/bft/consensus/hbft"
	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/consensus"
	"github.com/contatract/go-contatract/core_elephant/state"
	"github.com/contatract/go-contatract/crypto"
	"github.com/contatract/go-contatract/police"

	types "github.com/contatract/go-contatract/core/types_elephant"
	core "github.com/contatract/go-contatract/core_elephant"
	downloader "github.com/contatract/go-contatract/elephant/downloader"
	"github.com/contatract/go-contatract/elephant/exporter"
	fetcher "github.com/contatract/go-contatract/elephant/fetcher"
	"github.com/contatract/go-contatract/eth"
	"github.com/contatract/go-contatract/ethdb"
	"github.com/contatract/go-contatract/event"
	"github.com/contatract/go-contatract/internal/ethapi"
	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/p2p"
	"github.com/contatract/go-contatract/p2p/discover"
	"github.com/contatract/go-contatract/params"
	"github.com/contatract/go-contatract/rlp"

	//"github.com/contatract/go-contatract/elephant/exporter"
	"github.com/contatract/go-contatract/blizzard/storage"
)

const (
	softResponseLimit = 2 * 1024 * 1024 // Target maximum size of returned blocks, headers or node data.
	estHeaderRlpSize  = 500             // Approximate size of an RLP encoded block header

	// txChanSize is the size of channel listening to TxPreEvent.
	// The number is referenced from the size of tx pool.
	txChanSize = 4096
)

// errIncompatibleConfig is returned if the requested protocols and configs are
// not compatible (low protocol version restrictions and high requirements).
var errIncompatibleConfig = errors.New("incompatible configuration")

func errResp(code errCode, format string, v ...interface{}) error {
	return fmt.Errorf("%v - %v", code, fmt.Sprintf(format, v...))
}

type ProtocolManager struct {
	networkId uint64

	fastSync  uint32 // Flag whether fast sync is enabled (gets disabled if we already have blocks)
	acceptTxs uint32 // Flag whether we're considered synchronised (enables transaction processing)

	txpool      txPoolInterface
	blockchain  *core.BlockChain
	chainconfig *params.ElephantChainConfig
	maxPeers    int

	downloader *downloader.Downloader
	fetcher    *fetcher.Fetcher
	peers      *peerSet

	SubProtocols []p2p.Protocol

	eventMux      *event.TypeMux
	txCh          chan core.TxPreEvent
	txSub         event.Subscription
	minedBlockSub *event.TypeMuxSubscription

	// channels for fetcher, syncer, txsyncLoop
	newPeerCh       chan *peer
	txsyncCh        chan *txsync
	quitSync        chan struct{}
	noMorePeers     chan struct{}
	headerForPolice chan map[*peer][]*types.Header
	quitBFT         chan struct{}

	// wait group is used for graceful shutdowns during downloading
	// and processing
	num int32 // for debug
	wg  sync.WaitGroup

	elephant *Elephant
	lamp     *eth.Ethereum
	ethApi   *ethapi.PublicBlockChainAPI

	eleHeightCh  chan *exporter.ObjEleHeight
	eleHeightSub event.Subscription

	ethHeightCh  chan *exporter.ObjEthHeight
	ethHeightSub event.Subscription

	claimFileCh  chan uint64
	claimFileSub event.Subscription

	claimFileEndCh  chan struct{}
	claimFileEndSub event.Subscription
}

// eth通讯协议管理器
// NewProtocolManager returns a new ethereum sub protocol manager. The Ethereum sub protocol manages peers capable
// with the ethereum network.
func NewProtocolManager(config *params.ElephantChainConfig, mode downloader.SyncMode, networkId uint64, mux *event.TypeMux,
	txpool txPoolInterface, engine consensus.EngineCtt, blockchain *core.BlockChain, chaindb ethdb.Database,
	elephant *Elephant, ethereum *eth.Ethereum) (*ProtocolManager, error) {
	// Create the protocol manager with the base fields
	manager := &ProtocolManager{
		networkId:       networkId,
		acceptTxs:       1,
		eventMux:        mux,
		txpool:          txpool,
		blockchain:      blockchain,
		chainconfig:     config,
		peers:           newPeerSet(),
		newPeerCh:       make(chan *peer),
		noMorePeers:     make(chan struct{}),
		txsyncCh:        make(chan *txsync),
		quitSync:        make(chan struct{}),
		headerForPolice: make(chan map[*peer][]*types.Header),
		quitBFT:         make(chan struct{}),
		elephant:        elephant,
		lamp:            ethereum,
	}

	ethApiSetFlag := false
	for i := range manager.lamp.APIs() {
		if api, ok := manager.lamp.APIs()[i].Service.(*ethapi.PublicBlockChainAPI); ok {
			manager.SetEthApi(api)
			ethApiSetFlag = true
			break
		}
	}
	if !ethApiSetFlag {
		log.Error("[New Elephant Service failed!] : Can not get Ethereum BlockChainAPI")
		return nil, errors.New("get Ethereum BlockChainAPI failed")
	}

	// Figure out whether to allow fast sync or not
	if mode == downloader.FastSync && blockchain.CurrentBlock().NumberU64() > 0 {
		log.Warn("Blockchain not empty, fast sync disabled")
		mode = downloader.FullSync
	}

	if mode == downloader.FastSync {
		manager.fastSync = uint32(1)
	}
	// Initiate a sub-protocol for every implemented version we can handle

	// added by jianghan
	downloader.SetProtocolMinVer(int(ProtocolVersionMin), int(ProtocolVersionMax))

	manager.SubProtocols = make([]p2p.Protocol, 0, len(ProtocolVersions))
	for i, version := range ProtocolVersions {
		// Skip protocol version if incompatible with the mode of operation
		if mode == downloader.FastSync && version < elephantv2 {
			continue
		}
		// Compatible; initialise the sub-protocol
		version := version // Closure for the run
		manager.SubProtocols = append(manager.SubProtocols, p2p.Protocol{
			Name:    ProtocolName,
			Version: version,
			Length:  ProtocolLengths[i],
			Run: func(p *p2p.Peer, rw p2p.MsgReadWriter) error {
				peer := manager.newPeer(int(version), p, rw)
				select {
				case manager.newPeerCh <- peer:
					atomic.AddInt32(&manager.num, 1)
					manager.wg.Add(1)
					defFunc := func() {
						atomic.AddInt32(&manager.num, -1)
						manager.wg.Done()
					}
					defer defFunc()
					// JiangHan: (重要）这里是所有elephant协议处理的总入口，处理来自peer的消息
					return manager.handle(peer)
				case <-manager.quitSync:
					return p2p.DiscQuitting
				}
			},
			NodeInfo: func() interface{} {
				return manager.NodeInfo()
			},
			PeerInfo: func(id discover.NodeID) interface{} {
				if p := manager.peers.Peer(fmt.Sprintf("%x", id[:8])); p != nil {
					return p.Info()
				}
				return nil
			},
		})
	}
	if len(manager.SubProtocols) == 0 {
		return nil, errIncompatibleConfig
	}
	// Construct the different synchronisation mechanisms
	manager.downloader = downloader.New(mode, chaindb, manager.eventMux, blockchain, nil, manager.removePeer)

	validator := func(header *types.Header) error {
		return engine.VerifyHeader(blockchain, header)
	}
	heighter := func() uint64 {
		return blockchain.CurrentBlock().NumberU64()
	}
	inserter := func(blocks types.Blocks) (int, error) {
		// If fast sync is running, deny importing weird blocks
		if atomic.LoadUint32(&manager.fastSync) == 1 {
			log.Warn("Discarded bad propagated block", "number", blocks[0].Number(), "hash", blocks[0].Hash())
			return 0, nil
		}
		atomic.StoreUint32(&manager.acceptTxs, 1) // Mark initial sync done on any fetcher import
		return manager.blockchain.InsertChain(blocks)
	}
	manager.fetcher = fetcher.New(blockchain.GetBlockByHash, validator, manager.BroadcastBlock, heighter, inserter, manager.removePeer)

	return manager, nil
}

func (pm *ProtocolManager) SetEthApi(api *ethapi.PublicBlockChainAPI) {
	if api != nil {
		pm.ethApi = api
	}
}

func (pm *ProtocolManager) removePeer(id string) {
	// Short circuit if the peer was already removed
	peer := pm.peers.Peer(id)
	if peer == nil {
		return
	}
	log.Debug("Removing Ethereum peer", "peer", id)

	// Unregister the peer from the downloader and Ethereum peer set
	pm.downloader.UnregisterPeer(id)
	if err := pm.peers.Unregister(id); err != nil {
		log.Error("Peer removal failed", "peer", id, "err", err)
	}
	// Hard disconnect at the networking layer
	if peer != nil {
		peer.Peer.Disconnect(p2p.DiscUselessPeer)
	}
}

func (pm *ProtocolManager) Start(maxPeers int) {
	pm.eleHeightCh = make(chan *exporter.ObjEleHeight)
	pm.eleHeightSub = pm.elephant.blockchain.SubscribeEleHeight(pm.eleHeightCh)

	pm.ethHeightCh = make(chan *exporter.ObjEthHeight)
	pm.ethHeightSub = pm.elephant.lamp.BlockChain().SubscribeEthHeight(pm.ethHeightCh)

	pm.claimFileCh = make(chan uint64)
	pm.claimFileSub = pm.elephant.blockchain.SubscribeCheckClaimFile(pm.claimFileCh)

	pm.claimFileEndCh = make(chan struct{})
	pm.claimFileEndSub = pm.elephant.blockchain.SubscribeCheckClaimFileEnd(pm.claimFileEndCh)

	pm.maxPeers = maxPeers

	// broadcast transactions
	pm.txCh = make(chan core.TxPreEvent, txChanSize)
	pm.txSub = pm.txpool.SubscribeTxPreEvent(pm.txCh)
	go pm.txBroadcastLoop()

	// broadcast mined blocks
	// JiangHan：这里订阅新 Mined 区块事件，以便广播
	pm.minedBlockSub = pm.eventMux.Subscribe(core.NewMinedBlockEvent{})
	go pm.minedBroadcastLoop()

	// start sync handlers
	go pm.syncer()
	go pm.txsyncLoop()
	go pm.exporterLoop()
}

func (pm *ProtocolManager) Stop() {
	log.Info("Stopping Elephant protocol")

	pm.txSub.Unsubscribe()         // quits txBroadcastLoop
	pm.minedBlockSub.Unsubscribe() // quits blockBroadcastLoop

	// Quit the sync loop.
	// After this send has completed, no new peers will be accepted.
	pm.noMorePeers <- struct{}{}

	// Quit fetcher, txsyncLoop.
	close(pm.quitSync)

	// Disconnect existing sessions.
	// This also closes the gate for any new registrations on the peer set.
	// sessions which are already established but not added to pm.peers yet
	// will exit when they try to register.
	pm.peers.Close()

	// Wait for all peer handler goroutines and the loops to come down.
	log.Warn("", "pm.num", atomic.LoadInt32(&pm.num))
	pm.wg.Wait()

	close(pm.headerForPolice)
	close(pm.quitBFT)

	log.Info("Elephant protocol stopped")
}

func (pm *ProtocolManager) newPeer(pv int, p *p2p.Peer, rw p2p.MsgReadWriter) *peer {
	return newPeer(pv, p, newMeteredMsgWriter(rw))
}

func (pm *ProtocolManager) GetInOutShardingPeers(shardingID uint16, lampHeight uint64) ([]*peer, []*peer) {
	return pm.peers.GetInOutShardingPeers(shardingID, lampHeight)
}

// handle is the callback invoked to manage the life cycle of an lamp peer. When
// this function terminates, the peer is disconnected.
func (pm *ProtocolManager) handle(p *peer) error {
	log.Info("elephant handle", "id", p.id)
	if pm.peers.Len() >= pm.maxPeers {
		return p2p.DiscTooManyPeers
	}
	p.Log().Debug("Elephant peer connected", "name", p.Name())

	// Execute the Ethereum handshake
	var lampTD *big.Int
	var (
		genesis        = pm.blockchain.Genesis()
		head           = pm.blockchain.CurrentHeader()
		hash           = head.Hash()
		number         = head.Number
		lampHeight     = pm.lamp.CurrBlockNum().Int64()
		lampBaseHeight = uint64(head.LampBaseNumber.Uint64())
		lampBaseBlock  = pm.lamp.BlockChain().GetBlock(head.LampBaseHash, lampBaseHeight)
		lampBaseHash   = common.Hash{}
		td             = pm.blockchain.GetTd(hash, number.Uint64())
		realShard      = common.GetRealSharding(pm.elephant.etherbase)
	)
	if lampBaseBlock != nil {
		lampBaseHash = lampBaseBlock.Hash()
		lampTD = pm.lamp.BlockChain().GetTd(lampBaseHash, lampBaseHeight)
	}
	if lampTD == nil {
		lampTD = new(big.Int)
	}
	if err := p.Handshake(pm.networkId, td, number, new(big.Int).SetInt64(lampHeight), new(big.Int).SetInt64(int64(lampBaseHeight)), lampBaseHash, lampTD, hash, genesis.Hash(), realShard); err != nil {
		p.Log().Debug("Elephant handshake failed", "err", err)
		return err
	}
	if rw, ok := p.rw.(*meteredMsgReadWriter); ok {
		rw.Init(p.version)
	}

	// Check the peer is inside our shard, since we do not need too many peers which are outside our shard
	shardingID := common.GetSharding(pm.elephant.etherbase, uint64(lampHeight))
	totalShard := common.GetTotalSharding(uint64(lampHeight))
	inPeers, outPeers := pm.peers.GetInOutShardingPeers(shardingID, uint64(lampHeight))
	if !*pm.elephant.policeIsWorking && len(inPeers) > 0 && len(outPeers) > 0 {
		if len(outPeers) > 2 && len(inPeers)/len(outPeers) < peerInOutRatio && common.GetSharding2(p.realShard, totalShard) != shardingID {
			p.Peer.Disconnect(p2p.DiscUselessPeer) // Hard disconnect at the networking layer

			p.Log().Debug("Elephant handshake failed", "err", p2p.ErrTooManyOutside)
			return p2p.ErrTooManyOutside
		}
	}

	// JiangHan：将这个peer注册到 本地的 peers 中去
	// Register the peer locally
	if err := pm.peers.Register(p); err != nil {
		p.Log().Error("Elephant peer registration failed", "err", err)
		return err
	}
	defer pm.removePeer(p.id)

	// JiangHan: 将这个peer注册到 downloader 中去
	// Register the peer in the downloader. If the downloader considers it banned, we disconnect
	if err := pm.downloader.RegisterPeer(p.id, p.version, p); err != nil {
		return err
	}

	// JiangHan: 将本地囤积的 pending tx 都同步给对方，让对方帮忙宣传
	// 如果对方是 minor 就可以进行收集挖矿，如果对方不是，则会交给对方继续广播
	// Propagate existing transactions. new transactions appearing
	// after this will be sent via broadcasts.
	pm.syncTransactions(p)

	// JiangHan：上述握手等流程作完，就开始循环处理跟这个 peer 的所有消息
	// readmsg 读取， 处理等等..
	// main loop. handle incoming messages.
	for {
		if err := pm.handleMsg(p); err != nil {
			if err == io.EOF {
				log.Info("Elephant message EOF")
			} else {
				log.Error("Elephant message handling failed", "err", err)
			}
			return err
		}
	}
}

// JiangHan：（重点）所有的核心协议处理和分发handler，了解eth的运作机制，顺着线索摸
// handleMsg is invoked whenever an inbound message is received from a remote
// peer. The remote connection is torn down upon returning any error.
func (pm *ProtocolManager) handleMsg(p *peer) error {
	// Read the next message from the remote peer, and ensure it's fully consumed
	msg, err := p.rw.ReadMsg()
	if err != nil {
		return err
	}
	if msg.Size > ProtocolMaxMsgSize {
		return errResp(ErrMsgTooLarge, "%v > %v", msg.Size, ProtocolMaxMsgSize)
	}
	defer msg.Discard()

	log.Msg("Ele handle msg start", "code", msg.Code)
	defer log.Msg("Ele handle msg end", "code", msg.Code)

	// Handle the message depending on its contents
	switch {
	case msg.Code == StatusMsg:
		// Status messages should never arrive after the handshake
		return errResp(ErrExtraStatusMsg, "uncontrolled status message")

	// Block header query, collect the requested headers and reply
	// 获取一组 block header, 从消息中携带的 hash 或者 number 标记开始的一组
	// 如果消息中的 origin.hash == common.hash ，也就是nil，就表示是以 number 为查找
	// key, 否则就以 hash 为查找key
	case msg.Code == GetBlockHeadersMsg:
		// Decode the complex header query
		var query getBlockHeadersData
		if err := msg.Decode(&query); err != nil {
			return errResp(ErrDecode, "%v: %v", msg, err)
		}
		hashMode := query.Origin.Hash != (common.Hash{})
		// Gather headers until the fetch or network limits is reached
		var (
			bytes   common.StorageSize
			headers []*types.Header
			unknown bool
		)
		for !unknown && len(headers) < int(query.Amount) && bytes < softResponseLimit && len(headers) < downloader.MaxHeaderFetch {
			// Retrieve the next header satisfying the query
			var origin *types.Header
			if hashMode {
				origin = pm.blockchain.GetHeaderByHash(query.Origin.Hash)
			} else {
				origin = pm.blockchain.GetHeaderByNumber(query.Origin.Number)
			}
			if origin == nil {
				break
			}
			// Check different sharding
			pShardingID := common.GetSharding3(p.GetRealShard(), origin.LampNumberU64())
			lShardingID := common.GetSharding(pm.elephant.etherbase, origin.LampNumberU64())
			if pShardingID != lShardingID {
				break
			}

			number := origin.Number.Uint64()
			headers = append(headers, origin)
			bytes += estHeaderRlpSize

			// Advance to the next header of the query
			switch {
			case query.Origin.Hash != (common.Hash{}) && query.Reverse:
				// Hash based traversal towards the genesis block
				for i := 0; i < int(query.Skip)+1; i++ {
					if header := pm.blockchain.GetHeader(query.Origin.Hash, number); header != nil {
						query.Origin.Hash = header.ParentHash
						number--
					} else {
						unknown = true
						break
					}
				}
			case query.Origin.Hash != (common.Hash{}) && !query.Reverse:
				// Hash based traversal towards the leaf block
				var (
					current = origin.Number.Uint64()
					next    = current + query.Skip + 1
				)
				if next <= current {
					infos, _ := json.MarshalIndent(p.Peer.Info(), "", "  ")
					p.Log().Warn("GetBlockHeaders skip overflow attack", "current", current, "skip", query.Skip, "next", next, "attacker", infos)
					unknown = true
				} else {
					if header := pm.blockchain.GetHeaderByNumber(next); header != nil {
						if pm.blockchain.GetBlockHashesFromHash(header.Hash(), query.Skip+1)[query.Skip] == query.Origin.Hash {
							query.Origin.Hash = header.Hash()
						} else {
							unknown = true
						}
					} else {
						unknown = true
					}
				}
			case query.Reverse:
				// Number based traversal towards the genesis block
				if query.Origin.Number >= query.Skip+1 {
					query.Origin.Number -= query.Skip + 1
				} else {
					unknown = true
				}

			case !query.Reverse:
				// Number based traversal towards the leaf block
				query.Origin.Number += query.Skip + 1
			}
		}
		return p.SendBlockHeaders(headers, query.ForPolice)

	// 一批 headers 回来，作为上面的GetBlockHeadersMsg的回复
	case msg.Code == BlockHeadersMsg:
		// A batch of headers arrived to one of our previous requests
		var headers []*types.Header
		if err := msg.Decode(&headers); err != nil {
			return errResp(ErrDecode, "msg %v: %v", msg, err)
		}

		// Filter out any explicitly requested headers, deliver the rest to the downloader
		filter := len(headers) == 1
		if filter {
			// Irrelevant of the fork checks, send the header to the fetcher just in case
			headers = pm.fetcher.FilterHeaders(p.id, headers, time.Now())
		}
		if len(headers) > 0 || !filter {
			err := pm.downloader.DeliverHeaders(p.id, headers)
			if err != nil {
				log.Debug("Failed to deliver headers", "err", err)
			}
		}

	case msg.Code == BlockHeadersForPoliceMsg:
		// A batch of headers arrived to one of our previous requests
		var headers []*types.Header
		if err := msg.Decode(&headers); err != nil {
			return errResp(ErrDecode, "msg %v: %v", msg, err)
		}

		go func() {
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()
			headerForPolice := make(map[*peer][]*types.Header)
			headerForPolice[p] = headers

			for {
				select {
				case <-ticker.C:
					return

				case pm.headerForPolice <- headerForPolice:
					return
				}
			}
		}()

	case msg.Code == GetBlockBodiesMsg:
		// Decode the retrieval message
		msgStream := rlp.NewStream(msg.Payload, uint64(msg.Size))
		if _, err := msgStream.List(); err != nil {
			return err
		}
		// Gather blocks until the fetch or network limits is reached
		var (
			hash   common.Hash
			bytes  int
			bodies []rlp.RawValue
		)
		for bytes < softResponseLimit && len(bodies) < downloader.MaxBlockFetch {
			// Retrieve the hash of the next block
			if err := msgStream.Decode(&hash); err == rlp.EOL {
				break
			} else if err != nil {
				return errResp(ErrDecode, "msg %v: %v", msg, err)
			}
			// Retrieve the requested block body, stopping if enough was found
			if data := pm.blockchain.GetBodyRLP(hash); len(data) != 0 {
				bodies = append(bodies, data)
				bytes += len(data)
			}
		}
		return p.SendBlockBodiesRLP(bodies)

	case msg.Code == BlockBodiesMsg:
		// A batch of block bodies arrived to one of our previous requests
		var request blockBodiesData
		if err := msg.Decode(&request); err != nil {
			return errResp(ErrDecode, "msg %v: %v", msg, err)
		}
		// Deliver them all to the downloader for queuing
		trasactions := make([][]*types.Transaction, len(request))
		inputTxs := make([][]*types.ShardingTxBatch, len(request))
		uncles := make([][]*types.Header, len(request))
		hbftStages := make([][]*types.HBFTStageCompleted, len(request))

		for i, body := range request {
			trasactions[i] = body.Transactions
			inputTxs[i] = body.InputTxs
			uncles[i] = body.Uncles
			hbftStages[i] = body.HbftStages
		}
		// Filter out any explicitly requested bodies, deliver the rest to the downloader
		filter := len(trasactions) > 0 || len(inputTxs) > 0 || len(uncles) > 0
		if filter {
			trasactions, inputTxs, uncles = pm.fetcher.FilterBodies(p.id, trasactions, inputTxs, uncles, time.Now(), hbftStages)
		}
		if len(trasactions) > 0 || len(inputTxs) > 0 || len(uncles) > 0 || !filter {
			err := pm.downloader.DeliverBodies(p.id, trasactions, inputTxs, uncles, hbftStages)
			if err != nil {
				log.Debug("Failed to deliver bodies", "err", err)
			}
		}

	case p.version >= elephantv2 && msg.Code == GetNodeDataMsg:
		// Decode the retrieval message
		msgStream := rlp.NewStream(msg.Payload, uint64(msg.Size))
		//log.Info("【JH-INFO】GetNodeDataMsg", "p.version", p.version, "msg.Size", uint64(msg.Size) )
		if _, err := msgStream.List(); err != nil {
			return err
		}
		// Gather state data until the fetch or network limits is reached
		var (
			hash  common.Hash
			bytes int
			data  [][]byte
		)
		for bytes < softResponseLimit && len(data) < downloader.MaxStateFetch {
			// Retrieve the hash of the next state entry
			if err := msgStream.Decode(&hash); err == rlp.EOL {
				break
			} else if err != nil {
				return errResp(ErrDecode, "msg %v: %v", msg, err)
			}
			// Retrieve the requested state entry, stopping if enough was found
			if entry, err := pm.blockchain.TrieNode(hash); err == nil {
				data = append(data, entry)
				bytes += len(entry)
				//log.Info("【JH-INFO】", "len(entry)", len(entry) )
			}
		}
		//log.Info("【JH-INFO】", "len(data)", len(data) )
		return p.SendNodeData(data)

	case p.version >= elephantv2 && msg.Code == NodeDataMsg:
		// A batch of node state data arrived to one of our previous requests
		var data [][]byte
		if err := msg.Decode(&data); err != nil {
			return errResp(ErrDecode, "msg %v: %v", msg, err)
		}
		// Deliver all to the downloader
		if err := pm.downloader.DeliverNodeData(p.id, data); err != nil {
			log.Debug("Failed to deliver node state data", "err", err)
		}

	case p.version >= elephantv2 && msg.Code == GetReceiptsMsg:
		// Decode the retrieval message
		msgStream := rlp.NewStream(msg.Payload, uint64(msg.Size))
		if _, err := msgStream.List(); err != nil {
			return err
		}
		// Gather state data until the fetch or network limits is reached
		var (
			hash     common.Hash
			bytes    int
			receipts []rlp.RawValue
		)
		for bytes < softResponseLimit && len(receipts) < downloader.MaxReceiptFetch {
			// Retrieve the hash of the next block
			if err := msgStream.Decode(&hash); err == rlp.EOL {
				break
			} else if err != nil {
				return errResp(ErrDecode, "msg %v: %v", msg, err)
			}
			// Retrieve the requested block's receipts, skipping if unknown to us
			results := pm.blockchain.GetReceiptsByHash(hash)
			bEmpty := true
			for _, result := range results {
				if result == nil {
					continue
				}
				bEmpty = false
				break
			}
			if bEmpty {
				if header := pm.blockchain.GetHeaderByHash(hash); header == nil || header.ReceiptHash != types.EmptyRootHash {
					continue
				}
			}
			// If known, encode and queue for response packet
			if encoded, err := rlp.EncodeToBytes(results); err != nil {
				log.Error("Failed to encode receipt", "err", err)
			} else {
				receipts = append(receipts, encoded)
				bytes += len(encoded)
			}
		}
		return p.SendReceiptsRLP(receipts)

	case p.version >= elephantv2 && msg.Code == ReceiptsMsg:
		// A batch of receipts arrived to one of our previous requests
		var receipts [][]*types.Receipt
		if err := msg.Decode(&receipts); err != nil {
			return errResp(ErrDecode, "msg %v: %v", msg, err)
		}
		// Deliver all to the downloader
		if err := pm.downloader.DeliverReceipts(p.id, receipts); err != nil {
			log.Debug("Failed to deliver receipts", "err", err)
		}

	case msg.Code == NewBlockHashesMsg:
		var announces newBlockHashesData
		if err := msg.Decode(&announces); err != nil {
			return errResp(ErrDecode, "%v: %v", msg, err)
		}
		// Mark the hashes as present at the remote node
		for _, block := range announces {
			p.MarkBlock(block.HashWithBft)
		}
		// Schedule all the unknown hashes for retrieval
		unknown := make(newBlockHashesData, 0, len(announces))
		for _, block := range announces {
			if !pm.blockchain.HasBlock(block.Hash, block.Number) {
				unknown = append(unknown, block)
			}
		}
		for _, block := range unknown {
			pm.fetcher.Notify(p.id, block.Hash, block.Number, time.Now(), p.RequestOneHeader, p.RequestBodies)
		}

	// 来自peer的新区块宣告
	case msg.Code == NewBlockMsg:
		// Retrieve and decode the propagated block
		var request newBlockData
		if err := msg.Decode(&request); err != nil {
			return errResp(ErrDecode, "%v: %v", msg, err)
		}
		block := request.Block

		if block.HBFTStageChain().Len() != 2 {
			log.Warn("discard new received block whose BFT completed stages are invalid",
				"hash", block.Hash(), "hashWithBft", block.HashWithBft())
			return nil
		}

		go pm.handleNewBlock(block, p, msg.ReceivedAt)

	// jh: 来自peer的交易信息
	case msg.Code == TxMsg:
		//case msg.Code == TxChunkRent:
		//case msg.Code == TxChunkVerify:
		// Transactions arrived, make sure we have a valid and fresh chain to handle them
		if atomic.LoadUint32(&pm.acceptTxs) == 0 {
			break
		}
		// Transactions can be processed, parse all of them and deliver to the pool
		var txs []*types.Transaction
		if err := msg.Decode(&txs); err != nil {
			return errResp(ErrDecode, "msg %v: %v", msg, err)
		}
		for i, tx := range txs {
			// Validate and mark the remote transaction
			if tx == nil {
				return errResp(ErrDecode, "transaction %d is nil", i)
			}
			p.MarkTransaction(tx.Hash())
		}

		// jh: 加入到本地的 remote 交易池中，调用交易池的add
		// 当插入到 pending 队列的时候，会产生 TxPreEvent
		// 接到这个事件，会继续广播（给knowTx中找不到该tx的peer)
		ethHeight := uint64(pm.lamp.CurrBlockNum())
		pm.txpool.AddRemotes(txs, ethHeight)

	case msg.Code == ShardingTxMsg:
		// Sharding transactions arrived, make sure we have a valid and fresh chain to handle them
		// if atomic.LoadUint32(&pm.acceptTxs) == 0 {
		// 	break
		// }
		// Sharding transactions can be processed, parse all of them and deliver to the pool
		var txBatches types.ShardingTxBatches
		if err := msg.Decode(&txBatches); err != nil {
			return errResp(ErrDecode, "msg %v: %v", msg, err)
		}
		for i, shardingTxBatch := range txBatches {
			// Validate and mark the remote transaction
			if shardingTxBatch == nil {
				return errResp(ErrDecode, "transaction %d is nil", i)
			}
			p.MarkShardingTxBatch(shardingTxBatch.Hash())

			if *pm.elephant.policeIsWorking {
				work := &police.Work{ShardingTxBatch: shardingTxBatch}
				pm.elephant.police.PushWork(work)
			}
		}

		// jh: 加入到本地的 remote 交易池中，调用交易池的add
		// 当插入到 pending 队列的时候，会产生 TxPreEvent
		// 接到这个事件，会继续广播（给knowTx中找不到该tx的peer)
		// lampHeight := uint64(pm.lamp.CurrBlockNum())
		etherbase := pm.elephant.etherbase
		for _, batch := range txBatches {
			pm.txpool.AddTxBatch(batch, etherbase)
		}
		// TODO, check batch nonce consecutive, if not, request the lack of batch nonce

	case msg.Code == VerifiedHeaderMsg:
		// The header verified by the police can be processed, parse it and deliver to the
		// verified header pool in the lampChain
		if pm.lamp.IsMining() {
			var verifiedHeader consensus.PoliceVerifiedHeader
			if err := msg.Decode(&verifiedHeader); err != nil {
				return errResp(ErrDecode, "msg %v: %v", msg, err)
			}

			if ch := pm.lamp.VerifiedHeaderChan(); ch != nil {
				ch <- &verifiedHeader
			}
		}

	// qiwy: 获取Chunk的认领存储矿工信息
	case msg.Code == GetFarmersMsg:
		var data *storage.GetFarmerData
		if err := msg.Decode(&data); err != nil {
			return errResp(ErrDecode, "%v: %v", msg, err)
		}
		if data == nil {
			log.Warn("get chunk storage provide data nil")
			return nil
		}
		// log.Info("get chunk storage provider", "data", data, "peer", p)

		resData := pm.elephant.GetFarmerDataRes(data.ChunkId)
		if resData == nil {
			log.Info("get chunk storage provide res data nil")
			return nil
		}
		resData.QueryId = data.QueryId
		/*
			for _, addr := range resData.Addresss {
				ip := pm.elephant.storageManager.GetIp(addr)
				if len(ip) != 0 {
					value := addr.ToHex() + storage.SplitStr + string(ip)
					resData.AddrIp = append(resData.AddrIp, value)
				}
			}
		*/
		err = p.SendFarmerMsg(resData)

		// go func() {
		req := &exporter.ObjChunkPartners{ChunkId: data.ChunkId, Partners: resData.Nodes}
		feeds := pm.elephant.blockchain.GetFeeds()
		feeds[state.CChunkPartners].Send(req)
		// }()

		return err

	case msg.Code == FarmerMsg:
		var data *storage.FarmerData
		if err := msg.Decode(&data); err != nil {
			return errResp(ErrDecode, "msg %v: %v", msg, err)
		}
		if data == nil {
			log.Error("Handle msg, farmer data nil")
			return nil
		}
		log.Info("Handle msg, farmer data", "chunk", data.ChunkId, "len", len(data.Nodes))
		go pm.doFarmerMsg(data)

	// qiwy: 检验数据
	case msg.Code == GetDataHashMsg:
		var data *storage.GetDataHashProto
		if err := msg.Decode(&data); err != nil {
			return errResp(ErrDecode, "%v: %v", msg, err)
		}
		if data == nil {
			log.Warn("data nil")
			return nil
		}
		log.Info("data hash", "data", data, "peer", p)

		resData := pm.elephant.storageManager.GetDataHash(data)
		if resData == nil {
			log.Warn("hash res data nil")
			return nil
		}
		return p.SendDataHashMsg(resData)

	case msg.Code == GetBlizObjectMetaMsg:
		var data *storage.GetObjectMetaData
		if err := msg.Decode(&data); err != nil {
			return errResp(ErrDecode, "%v: %v", msg, err)
		}
		// 从miner stateDB获取chunkId, sliceId
		slices := pm.elephant.GetSliceFromState(data.Address, data.ObjectId)
		if len(slices) == 0 {
			err := errResp(ErrDecode, "%v: %v", msg, errors.New("Not find"))
			log.Info("msg.Code GetBlizObjectMetaMsg", "err", err)
			return nil
		}
		log.Info("handleMsg", "slices len", len(slices))
		// 返回，下次再从farmer那获取metaData
		dataRsp := storage.ObjectMetaData{Data: *data, Slices: slices}
		return p.SendBlizObjectMeta(&dataRsp)

	case msg.Code == BlizObjectMetaMsg:
		var data *storage.ObjectMetaData
		if err := msg.Decode(&data); err != nil {
			return errResp(ErrDecode, "%v: %v", msg, err)
		}
		log.Info("Handle msg", "address", data.Data.Address, "object", data.Data.ObjectId, "slicecount", len(data.Slices))
		go pm.doObjectMetaMsg(data)

	// case msg.Code == BlizReHandshakeMsg:
	// 	var data *storage.BlizReHandshakeProto
	// 	if err := msg.Decode(&data); err != nil {
	// 		return errResp(ErrDecode, "%v: %v", msg, err)
	// 	}
	// 	log.Info("handleMsg", "data", data, "p", p)
	// 	select {
	// 	case p.ReStartProtocolChan <- true:
	// 	default:
	// 	}

	case msg.Code == BFTQueryMsg:
		var data hbft.BFTMsg
		if err := msg.Decode(&data); err != nil {
			return errResp(ErrDecode, "%v: %v", msg, err)
		}

		go pm.handleBFTQueryMsg(&data, p)

	case msg.Code == EthHeightMsg:
		var data uint64
		if err := msg.Decode(&data); err != nil {
			return errResp(ErrDecode, "%v: %v", msg, err)
		}
		p.SetLampHeight(int64(data))

	case msg.Code == EleHeightMsg:
		var data uint64
		if err := msg.Decode(&data); err != nil {
			return errResp(ErrDecode, "%v: %v", msg, err)
		}
		p.heightEle = int64(data)

	default:
		return errResp(ErrInvalidMsgCode, "%v", msg.Code)
	}
	return nil
}

func (pm *ProtocolManager) handleNewBlock(block *types.Block, p *peer, receivedAt time.Time) {
	// First check if we have the block whose hash is same with the received one
	if pm.blockchain.HasBlock(block.Hash(), block.NumberU64()) {
		blockHashWithBft := block.HashWithBft()
		blockWeHave := pm.blockchain.GetBlockByHash(block.Hash())
		if !blockHashWithBft.Equal(blockWeHave.HashWithBft()) {
			if !(block.HBFTStageChain()[1].ViewID > blockWeHave.HBFTStageChain()[1].ViewID ||
				(block.HBFTStageChain()[1].ViewID == blockWeHave.HBFTStageChain()[1].ViewID &&
					block.HBFTStageChain()[1].Timestamp > blockWeHave.HBFTStageChain()[1].Timestamp)) {
				if err := pm.blockchain.ReplaceBlockWithoutState(block); err != nil {
					log.Warn("Write block without state failed when replacing the old block",
						"Hash", block.Hash().TerminalString(), "HashWithBft", block.HashWithBft())
				}
				return
			}
		} else {
			return
		}
	}

	// Then check if the base chain is correct
	if common.GetConsensusBft() && !pm.verifyHeaderBase(block.Header()) {
		log.Info("discard new received block whose base is not match with mine", "hash", block.Hash().String())
		return
	}
	// Check lamp height
	pLamp := p.GetLampHeight()
	trueLamp := uint64(pm.lamp.CurrBlockNum())
	if uint64(pLamp) != trueLamp {
		log.Info("discard new received block because of lamp height", "peer lamp height", pLamp, "local lamp height", trueLamp)
		return
	}
	// Check sharding
	pLampHeight := block.LampBaseNumberU64()
	pRealShard := p.GetRealShard()
	pShard := common.GetSharding3(pRealShard, pLampHeight)
	trueShard := common.GetSharding(pm.elephant.etherbase, pLampHeight)
	if pShard != trueShard {
		log.Info("discard new received block because of sharding", "peer sharding", pShard, "local sharding", trueShard)
		return
	}
	// Check lamp block which elephant is rely on
	pLampHash := block.LampHash()
	lampBlock := pm.lamp.BlockChain().GetBlock(pLampHash, pLampHeight)
	if lampBlock == nil {
		log.Info("discard new received block because of lamp block")
		return
	}
	// lampBaseHash := lampBlock.Hash()
	// if !lampBaseHash.Equal(request.LampBaseHash) {
	// 	lampTD := pm.lamp.BlockChain().GetTdByHash(lampBaseHash)
	// 	if lampTD.Cmp(request.LampTD) < 0 {
	// 		log.Info("Need sync lamp")
	// 	}
	// 	log.Info("discard new received block because of lamp td")
	// 	return nil
	// }

	block.ReceivedAt = receivedAt
	block.ReceivedFrom = p

	// Mark the peer as owning the block and schedule it for import
	p.MarkBlock(block.HashWithBft())
	pm.fetcher.Enqueue(p.id, block)

	bSync := false
	pEleHeight := block.NumberU64()
	currentBlock := pm.blockchain.CurrentBlock()
	trueEleHeight := currentBlock.NumberU64()
	if pEleHeight == trueEleHeight+1 {
		pParentHash := block.ParentHash()
		trueParentHash := currentBlock.ParentHash()
		if !pParentHash.Equal(trueParentHash) {
			bSync = true
		}
	} else if pEleHeight > trueEleHeight+1 {
		bSync = true
	} else {
		// Get local block which is same the elephant height as peer block
		heightBlock := pm.blockchain.GetBlockByNumber(pEleHeight)
		heightHash := heightBlock.Hash()
		pHash := block.Hash()
		if !pHash.Equal(heightHash) {
			heightLamp := heightBlock.LampBaseNumberU64()
			heightLampHash := heightBlock.LampHash()
			b := pm.lamp.BlockChain().GetBlock(heightLampHash, heightLamp)
			if b == nil {
				bSync = true
			}
		}
	}
	// Update the peers info if better than the previous
	parentHash := block.ParentHash()
	parentBlock := pm.blockchain.GetBlock(parentHash, pEleHeight-1)
	if parentBlock != nil {
		lampTD := pm.lamp.BlockChain().GetTd(parentBlock.LampHash(), parentBlock.LampBaseNumberU64())
		if lampTD == nil {
			lampTD = new(big.Int)
		}
		p.SetHead(parentHash, int64(parentBlock.NumberU64()), int64(parentBlock.LampBaseNumberU64()), parentBlock.LampHash(), lampTD)
	}

	if bSync {
		go pm.synchronise(p)
	}
}

func (pm *ProtocolManager) handleBFTQueryMsg(msg *hbft.BFTMsg, p *peer) {
	var viewPrimary string
	coinbase := pm.elephant.etherbase
	shoudHandle := false
	sealers, _ := pm.elephant.GetCurRealSealers()
	for _, addr := range sealers {
		if coinbase.Equal(addr) {
			shoudHandle = true
			break
		}
	}
	if !shoudHandle {
		return
	}

	ticker := time.NewTicker(time.Millisecond * 100)
	defer ticker.Stop()

	if wait := pm.waitForBFTNotBusyAndHandle(&viewPrimary, ticker, msg); wait {
		pm.waitForBFTHandleAndReply(&viewPrimary, ticker, p)
	}
}

func (pm *ProtocolManager) waitForBFTNotBusyAndHandle(viewPrimary *string, ticker *time.Ticker, msg *hbft.BFTMsg) bool {
waitReceive:
	for {
		select {
		case <-pm.quitBFT:
			return false

		case <-ticker.C:
			if pm.elephant.Miner().IsStopped() && pm.elephant.hbftNode.IsBusy() {
				pm.elephant.hbftNode.SetBusy(false)
			}

			if msg.MsgType == hbft.MsgOldestFuture {
				go pm.elephant.hbftNode.HbftProcess(msg.MsgOldestFuture)
				return false
			}

			if !pm.elephant.hbftNode.IsBusy() {
				switch msg.MsgType {
				case hbft.MsgPreConfirm:
					*viewPrimary = msg.MsgPreConfirm.ViewPrimary
					go pm.elephant.hbftNode.HbftProcess(msg.MsgPreConfirm)

				case hbft.MsgConfirm:
					*viewPrimary = msg.MsgConfirm.ViewPrimary
					go pm.elephant.hbftNode.HbftProcess(msg.MsgConfirm)

				}
				pm.elephant.CreateBftMsgChan(*viewPrimary)
				break waitReceive

			} else {
				switch msg.MsgType {
				case hbft.MsgReply:
					go pm.elephant.hbftNode.HbftProcess(msg.MsgReply)
					return false
				}
			}
		}
	}

	return true
}

// RandomError returns abnormally to simulate random error
func (pm *ProtocolManager) RandomError(randomTimes int) bool {
	rand.Seed(time.Now().UnixNano())
	randomNum := rand.Intn(randomTimes)
	if randomNum == 0 {
		return true
	}
	return false
}

func (pm *ProtocolManager) waitForBFTHandleAndReply(viewPrimary *string, ticker *time.Ticker, p *peer) {
	num := 0
	for {
		select {
		case <-pm.quitBFT:
			return

		case <-ticker.C:
			if num > 30 {
				log.Warn("handleBFTQueryMsg timed out when waiting for handling a BFT query message")
				return
			}
			num++

		case bftReplyMsg, ok := <-pm.elephant.ReplyBftChan(*viewPrimary):
			if ok {
				if bftReplyMsg.MsgType == hbft.MsgReply && bftReplyMsg.MsgReply.MsgType == hbft.MsgPreConfirm {
					p2p.Send(p.rw, BFTQueryMsg, *bftReplyMsg)
				} else if bftReplyMsg.MsgType == hbft.MsgReply && bftReplyMsg.MsgReply.MsgType == hbft.MsgConfirm {
					p2p.Send(p.rw, BFTQueryMsg, *bftReplyMsg)
					//nodeID, err := pm.getNextSealerNodeByCfmReply(bftReplyMsg)
					//if err != nil {
					//	log.Warn(err.Error())
					//}
					//
					//err = pm.sendBFTMsgToNode(nodeID, bftReplyMsg)
					//if err != nil {
					//	log.Warn(err.Error())
					//}
				}
				return
			} else {
				return
			}
		}
	}
}

func (pm *ProtocolManager) GetNextSealerNodesInfo() map[discover.NodeID]common.Address {
	nodeStr := make([]string, 0)
	nodesInfo := make(map[discover.NodeID]common.Address)
	sealers, newest := pm.elephant.GetCurRealSealers()
	if newest {
		nodeStr = pm.elephant.lamp.GetNewestEleNodeSealers()
	} else {
		nodeStr = pm.elephant.lamp.GetLastEleNodeSealers()
	}
	for i := 0; i < len(sealers); i++ {
		index := 0
		if i+1 >= len(sealers) {
			index = 0
		} else {
			index = i + 1
		}
		node, err := discover.ParseNode(nodeStr[index])
		if err != nil {
			nodesInfo[node.ID] = sealers[i]
		}
	}
	return nodesInfo
}

func (pm *ProtocolManager) GetNextSealerNodesByCfmReply(replyMsg *hbft.BFTMsg) (*discover.NodeID, error) {
	nodeStr := make([]string, 0)
	sealers, newest := pm.elephant.GetCurRealSealers()
	if newest {
		nodeStr = pm.elephant.lamp.GetNewestEleNodeSealers()
	} else {
		nodeStr = pm.elephant.lamp.GetLastEleNodeSealers()
	}
	for i := 0; i < len(sealers); i++ {
		if sealers[i].Equal(common.HexToAddress(replyMsg.ViewPrimary)) {
			node, err := discover.ParseNode(nodeStr[i+1])
			if err != nil {
				return nil, err
			}
			return &node.ID, nil
		}
	}

	return nil, errors.New("can not find the node ID of current sealer")
}

func (pm *ProtocolManager) sendBFTMsgToNode(nodeID *discover.NodeID, replyMsg *hbft.BFTMsg) error {
	for _, peer := range pm.peers.Peers() {
		if nodeID.Equal(peer.ID()) {
			return p2p.Send(peer.rw, BFTQueryMsg, *replyMsg)
		}
	}

	return errors.New("there is no specific node connection in connected peer set")
}

func (pm *ProtocolManager) verifyHeaderBase(header *types.Header) bool {
	if pm.lamp.LastEleSealersBlockNum() > 0 && int64(pm.lamp.CurrEleSealersBlockNum()) == header.LampBaseNumber.Int64() {
		if hash := pm.lamp.CurrEleSealersBlockHash(); hash.Equal(header.LampBaseHash) {
			return true
		} else {
			return false
		}
	} else if pm.lamp.LastEleSealersBlockNum() > 0 && int64(pm.lamp.LastEleSealersBlockNum()) == header.LampBaseNumber.Int64() {
		if hash := pm.lamp.LastEleSealersBlockHash(); hash.Equal(header.LampBaseHash) {
			return true
		} else {
			return false
		}
	} else if pm.lamp.LastEleSealersBlockNum() == 0 && int64(pm.lamp.CurrEleSealersBlockNum()) == header.LampBaseNumber.Int64() &&
		header.LampBaseNumber.Int64() == baseparam.ElectionBlockCount {
		if hash := pm.lamp.CurrEleSealersBlockHash(); hash.Equal(header.LampBaseHash) {
			return true
		} else {
			return false
		}
	} else {
		return false
	}
}

func (pm *ProtocolManager) BroadcastBFTMsg(bftMsg hbft.BFTMsg) {
	// TODO zzr classify nodes

	peers := pm.peers.Peers()
	for _, p := range peers {
		p2p.Send(p.rw, BFTQueryMsg, bftMsg)
	}
}

// BroadcastBlock will either propagate a block to a subset of it's peers, or
// will only announce it's availability (depending what's requested).
// JiangHan：广播一个新区块
func (pm *ProtocolManager) BroadcastBlock(block *types.Block, propagate, toSealersOnly bool) {
	hash := block.Hash()
	hashWithBft := block.HashWithBft()
	peers := pm.peers.PeersWithoutBlock(hashWithBft)
	if toSealersOnly {
		// may have some problems
		//sealers, _ := pm.elephant.GetCurRealSealers()
		//if sealers != nil {
		//	nodesInfo := pm.GetNextSealerNodesInfo()
		//	peers = pm.peers.GetSealerPeers(sealers, nodesInfo)
		//}

		peers = pm.peers.Peers()
	}

	// If propagation is requested, send to a subset of the peer
	if propagate {
		// Calculate the TD of the block (it's not imported yet, so block.Td is not valid)
		var td *big.Int
		if parent := pm.blockchain.GetBlock(block.ParentHash(), block.NumberU64()-1); parent != nil {
			// JiangHan: 广播新区块方案，td值累加，在发生分叉的时候方便我们决策，我们选择 td 大的方案
			//td = new(big.Int).Add(block.Difficulty(), pm.blockchain.GetTd(block.ParentHash(), block.NumberU64()-1))
			td = new(big.Int).SetInt64(0)
		} else {
			log.Error("Propagating dangling block", "number", block.Number(), "hash", hash)
			return
		}
		lampBlock := pm.lamp.BlockChain().GetBlock(block.LampHash(), block.LampBaseNumberU64())
		if lampBlock == nil {
			log.Error("Broadcast block, getting block fails", "blockid", block.LampBaseNumberU64())
			return
		}
		lampHash := lampBlock.Hash()
		lampTD := pm.lamp.BlockChain().GetTdByHash(lampHash)
		// Send the block to a subset of our peers
		transfer := peers[:int(math.Sqrt(float64(len(peers))))]
		for _, peer := range transfer {
			peer.SendNewBlock(block, td, lampHash, lampTD)
		}
		log.Trace("Propagated block", "hash", hash, "recipients", len(transfer), "duration", common.PrettyDuration(time.Since(block.ReceivedAt)))
		return
	}
	// Otherwise if the block is indeed in out own chain, announce it
	if pm.blockchain.HasBlock(hash, block.NumberU64()) {
		for _, peer := range peers {
			peer.SendNewBlockHashes([]common.Hash{hashWithBft}, []common.Hash{hash}, []uint64{block.NumberU64()})
		}
		log.Trace("Announced block", "hash", hash, "recipients", len(peers), "duration", common.PrettyDuration(time.Since(block.ReceivedAt)))
	}
}

// BroadcastTx will propagate a transaction to all peers which are not known to
// already have the given transaction.
func (pm *ProtocolManager) BroadcastTx(hash common.Hash, tx *types.Transaction) {
	// Broadcast transaction to a batch of peers not knowing about it
	peers := pm.peers.PeersWithoutTx(hash)
	//FIXME include this again: peers = peers[:int(math.Sqrt(float64(len(peers))))]
	for _, peer := range peers {
		peer.SendTransactions(types.Transactions{tx})
	}
	log.Trace("Broadcast transaction", "hash", hash, "recipients", len(peers))
}

// BroadcastShardingTxs will propagate sharding transactions to all peers which are not known to
// already have the given transactions.
func (pm *ProtocolManager) BroadcastShardingTxs(shardTxs []*types.ShardingTxBatch) {
	// Broadcast transactions sent to other sharding to a batch of peers not knowing about it
	for _, shardingTx := range shardTxs {
		hash := shardingTx.Hash()
		peers := pm.peers.PeersWithoutShardingTxBatch(hash)
		//FIXME include this again: peers = peers[:int(math.Sqrt(float64(len(peers))))]
		for _, peer := range peers {
			peer.SendShardingTxBatches(types.ShardingTxBatches{shardingTx})
		}
		log.Trace("Broadcast sharding transaction", "hash", hash, "recipients", len(peers))
	}
}

// Mined broadcast loop
func (self *ProtocolManager) minedBroadcastLoop() {
	// automatically stops if unsubscribe
	for obj := range self.minedBlockSub.Chan() {
		switch ev := obj.Data.(type) {
		case core.NewMinedBlockEvent:
			eleHeight := ev.Block.NumberU64()
			peers := self.peers.Peers()
			for _, p := range peers {
				p.SendEleHeight(&eleHeight)
			}
			self.BroadcastBlock(ev.Block, true, false)  // First propagate block to peers
			self.BroadcastBlock(ev.Block, false, false) // Only then announce to the rest
			log.Info("Broadcast mined block", "number", ev.Block.Number(), "hash", ev.Block.Hash().TerminalString(),
				"txs", len(ev.Block.Transactions()), "shardtxs", len(ev.ShardTxs))
			if ev.ShardTxs != nil && len(ev.ShardTxs) > 0 {
				self.BroadcastShardingTxs(ev.ShardTxs)
				self.DeliverShardingTxs(ev.ShardTxs)
			}
		}
	}
}

func (self *ProtocolManager) txBroadcastLoop() {
	for {
		select {
		case event := <-self.txCh:
			self.BroadcastTx(event.Tx.Hash(), event.Tx)

		// Err() channel will be closed when unsubscribing.
		case <-self.txSub.Err():
			return
		}
	}
}

// NodeInfo represents a short summary of the Ethereum sub-protocol metadata
// known about the host peer.
type NodeInfo struct {
	Network    uint64              `json:"network"`    // Ethereum network ID (1=Frontier, 2=Morden, Ropsten=3, Rinkeby=4)
	Difficulty *big.Int            `json:"difficulty"` // Total difficulty of the host's blockchain
	Genesis    common.Hash         `json:"genesis"`    // SHA3 hash of the host's genesis block
	Config     *params.ChainConfig `json:"config"`     // Chain configuration for the fork rules
	Head       common.Hash         `json:"head"`       // SHA3 hash of the host's best owned block
}

// NodeInfo retrieves some protocol metadata about the running host node.
func (self *ProtocolManager) NodeInfo() *NodeInfo {
	currentBlock := self.blockchain.CurrentBlock()
	return &NodeInfo{
		Network:    self.networkId,
		Difficulty: self.blockchain.GetTd(currentBlock.Hash(), currentBlock.NumberU64()),
		Genesis:    self.blockchain.Genesis().Hash(),
		Config:     self.blockchain.Config(),
		Head:       currentBlock.Hash(),
	}
}

func (pm *ProtocolManager) BroadcastChunkProvider(chunkId uint64, queryId uint64) {
	// Broadcast chunk provider to a batch of peers not knowing about it
	peers := pm.peers.PeersWithoutChunkProvider(chunkId)
	for _, peer := range peers {
		err := peer.GetChunkProvider(chunkId, queryId)
		if err != nil {
			log.Error("get chunk provider", "err", err)
			continue
		}
		peer.knownChunkProvider.Add(chunkId)
	}
	log.Info("Broadcast chunk provider", "chunkId", chunkId, "recipients", len(peers))
}

func (pm *ProtocolManager) BroadcastBlizProtocol(msgcode uint64, parmas string) {
	for _, peer := range pm.peers.peers {
		err := peer.SendBlizProtocol(msgcode, parmas)
		if err != nil {
			log.Error("send bliz protocol", "err", err)
			continue
		}
	}
}

func (pm *ProtocolManager) GetEleHeaderByNumber(number uint64) *types.Header {
	return pm.blockchain.GetHeaderByNumber(number)
}

func (pm *ProtocolManager) GetEleHeadersFromPeer(from uint64, amount int, shardingID uint16, lampNum uint64) ([]*types.Header, error) {
	var p *peer
	peers, _ := pm.GetInOutShardingPeers(shardingID, lampNum)
	for _, peer := range peers {
		p = peer
		go peer.RequestHeadersByNumberForPolice(from, amount, 0, true)
		break
	}
	if len(peers) == 0 {
		return nil, errNoPeerForPolice
	}

	ticker := time.NewTicker(15 * time.Second)
	headerAll := make(map[*peer][]*types.Header)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			return nil, errors.New(fmt.Sprintf("receiving %d headers from peer timed out", amount))

		case headerForPolice, ok := <-pm.headerForPolice:
			if !ok {
				return nil, errors.New("received no header from peer")
			}
			if headerForPolice != nil && headerForPolice[p] != nil && len(headerForPolice[p]) > 0 {
				headerAll[p] = append(headerAll[p], headerForPolice[p]...)
				if len(headerAll[p]) < amount && len(headerForPolice[p]) < amount {
					go p.RequestHeadersByNumberForPolice(from-uint64(len(headerForPolice[p])), amount-len(headerForPolice[p]), 0, true)
					continue
				}
				return headerAll[p], nil
			} else {
				return nil, errors.New("received no header from peer")
			}
		}
	}
}

func (pm *ProtocolManager) DeliverShardingTxs(shardTxs []*types.ShardingTxBatch) {
	if *pm.elephant.policeIsWorking {
		for _, txBatch := range shardTxs {
			work := &police.Work{ShardingTxBatch: txBatch}
			pm.elephant.police.PushWork(work)
		}
	}
}

func (pm *ProtocolManager) DeliverVerifiedResult(result *police.Result) {
	if !pm.lamp.IsMining() {
		return
	}

	verifiedHeader := consensus.PoliceVerifiedHeader{
		ShardingID: result.Match.ShardingID,
		BlockNum:   result.Match.BlockNumber,
		LampBase:   result.Match.LampBase,
		Header:     result.Match.Header,
		Previous:   result.Match.PreviousHeaders,
		Sign:       result.Sign,
	}
	if ch := pm.lamp.VerifiedHeaderChan(); ch != nil {
		ch <- &verifiedHeader
	}
}

func (pm *ProtocolManager) SignVerifyResult(data []byte, passphrase string) ([]byte, error) {
	coinbase, err := pm.elephant.Etherbase()
	if err != nil {
		return nil, err
	}

	am := pm.elephant.accountManager
	var coinbaseAcc accounts.Account
	for _, wallet := range am.Wallets() {
		for _, account := range wallet.Accounts() {
			if strings.Compare(coinbase.String(), account.Address.String()) == 0 {
				coinbaseAcc = account
			}
		}
	}

	var privKey *ecdsa.PrivateKey
	ks := fetchKeystore(am)
	if privKey, err = ks.GetKeyCopy(coinbaseAcc, passphrase); err == nil {
		defer zeroKey(privKey)

		sigrsv, err := crypto.Sign(data, privKey) // [R || S || V] format
		if err != nil {
			return nil, err
		}

		return sigrsv, nil
	} else {
		return nil, err
	}
}

// zeroKey zeroes a private key in memory.
func zeroKey(k *ecdsa.PrivateKey) {
	b := k.D.Bits()
	for i := range b {
		b[i] = 0
	}
}

// BroadcastPoliceVerify broadcasts the result of the verification of the police
func (pm *ProtocolManager) BroadcastPoliceVerify(result *police.Result) {
	peers := pm.peers.PeersWithoutVerifiedHeader(result.Sign, result.Match.Header)

	verifiedHeader := consensus.PoliceVerifiedHeader{
		ShardingID: result.Match.ShardingID,
		BlockNum:   result.Match.BlockNumber,
		LampBase:   result.Match.LampBase,
		Header:     result.Match.Header,
		Previous:   result.Match.PreviousHeaders,
		Sign:       result.Sign,
	}
	for _, peer := range peers {
		peer.SendVerifiedHeader(verifiedHeader)
	}
	log.Trace("Broadcast the header verified by the police", "hash", *result.Match.Header)
}
