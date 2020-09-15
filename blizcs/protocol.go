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

/*
bliz implements the blizzard partner and client protocol [bliz] (sister of eth and shh)
the protocol instance is launched on each peer by the network layer if the
bzz protocol handler is registered on the p2p server.

The bliz protocol component
*/

import (
	"errors"
	"io"
	"time"

	//"fmt"
	//"net"
	//"strconv"
	//"time"
	"sync"
	//"math/big"
	//"sync/atomic"

	"github.com/contatract/go-contatract/blizcore"
	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/p2p"
	//"github.com/contatract/go-contatract/blizzard"
	//"github.com/contatract/go-contatract/blizzard/storage"
)

// Constants to match up protocol versions and messages
const (
	blizcs1 = 1
	blizcs2 = 2
)

// Official short name of the protocol used during capability negotiation.
var ProtocolName = "blizcs"

// Supported versions of the eth protocol (first is primary).
var ProtocolVersions = []uint{blizcs1}

// Added by jianghan for version retrive
var ProtocolVersionMin = blizcs1 - 1
var ProtocolVersionMax = blizcs2 + 1

// Number of implemented message corresponding to different protocol versions.
var ProtocolLengths = []uint64{0x0a + 1}

const (
	Version = 0
	//ProtocolLength     = uint64(6)
	ProtocolMaxMsgSize = 10 * 1024 * 1024
	NetworkId          = 3
)

// bliz protocol message codes
// JiangHan：（重要）eth 核心应用层协议（非底层p2p协议），阅读交易，合约等协议到这里看
const (
	// Protocol messages belonging to eth/62
	// 1 的Lengths 定义了到了8 ，这里实现了8个 --jianghan
	StatusMsg    = 0x00
	ShakehandMsg = 0x01 // 握手，keepalive

	WriteReqMsg = 0x02
	WriteRspMsg = 0x03

	ReadReqMsg = 0x04
	ReadRspMsg = 0x05

	GetFarmerSliceOpMsg = 0x06
	FarmerSliceOpMsg    = 0x07

	GetSliceMetaFromFarmerReqMsg = 0x08

	GetBuffFromSharerCSMsg = 0x09
	BuffFromSharerCSMsg    = 0x0a
)

// statusData is the network packet for the status message.
type statusData struct {
	ProtocolVersion uint32
	NetworkId       uint64
}

type rwData struct {
	code uint64
	p    *Peer
	data interface{}
}

type infoData struct {
	h    common.Hash
	data [][]byte
}

// bzz represents the swarm wire protocol
// an instance is running on each peer
type ProtocolManager struct {
	networkId    uint64
	SubProtocols []p2p.Protocol

	maxClients int
	peers      *peerSet

	//newPeerCh   chan *peer
	quitSync      chan struct{}
	noMoreClients chan struct{}

	blizcs *BlizCS

	// wait group is used for graceful shutdowns during downloading
	// and processing
	wg sync.WaitGroup

	rwMsgLock sync.RWMutex
	rwMsg     []*rwData

	// Cache the data to avoid network transmission back and forth
	mapDataLock sync.RWMutex
	mapData     map[*Peer][]*infoData
}

var errIncompatibleConfig = errors.New("incompatible configuration")

func NewProtocolManager(networkId uint64, database *sliceDB, blizcs *BlizCS) (*ProtocolManager, error) {
	// Create the protocol manager with the base fields
	manager := &ProtocolManager{
		networkId: networkId,
		//newClientCh:   	make(chan *Peer),
		noMoreClients: make(chan struct{}),
		peers:         newPeerSet(),
		quitSync:      make(chan struct{}),
		blizcs:        blizcs,
		rwMsg:         make([]*rwData, 0),
		mapData:       make(map[*Peer][]*infoData),
	}

	manager.SubProtocols = make([]p2p.Protocol, 0, len(ProtocolVersions))
	for i, version := range ProtocolVersions {
		// Compatible; initialise the sub-protocol
		version := version // Closure for the run
		manager.SubProtocols = append(manager.SubProtocols, p2p.Protocol{
			Name:    ProtocolName,
			Version: version,
			Length:  ProtocolLengths[i],
			Run: func(p *p2p.Peer, rw p2p.MsgReadWriter) error {
				peer := manager.newPeer(int(version), p, rw)
				manager.wg.Add(1)
				defer manager.wg.Done()
				// JiangHan: (重要）这里是所有blizcs协议处理的总入口，处理来自peer的消息
				return manager.handle(peer)

				/*
					select {
					case manager.newClientCh <- client:
						manager.wg.Add(1)
						defer manager.wg.Done()
						// JiangHan: (重要）这里是所有blizcs协议处理的总入口，处理来自peer的消息
						return manager.handle(client)
					case <-manager.quitSync:
						return p2p.DiscQuitting
					}
				*/
			},
		})
	}

	if len(manager.SubProtocols) == 0 {
		return nil, errIncompatibleConfig
	}
	return manager, nil
}

func (pm *ProtocolManager) Log() log.Logger {
	return pm.blizcs.Log()
}

func (pm *ProtocolManager) removePeer(id string) {
	// Short circuit if the peer was already removed
	p := pm.peers.Peer(id)
	if p == nil {
		return
	}
	pm.Log().Debug("Removing BlizzardCS client", "client", id)

	pm.blizcs.ExitPeer(p)

	if err := pm.peers.Unregister(id); err != nil {
		pm.Log().Error("BlizzardCS Client removal failed", "client", id, "err", err)
	}
	// Hard disconnect at the networking layer
	p.Peer.Disconnect(p2p.DiscUselessPeer)
}

func (pm *ProtocolManager) handleRWMsgsLoop() {
	duration := time.Millisecond * 100
	ticket := time.NewTicker(duration)
	defer ticket.Stop()

	for {
		select {
		case <-ticket.C:
			pm.handleRWMsgs()
		}
	}
}

func (pm *ProtocolManager) Start(maxClients int) {
	pm.Log().Info("Start", "maxClients", maxClients)
	pm.maxClients = maxClients

	go pm.handleRWMsgsLoop()
}

func (pm *ProtocolManager) Stop() {
	pm.Log().Info("Stopping BlizzardCS protocol")

	// Quit fetcher, txsyncLoop.
	close(pm.quitSync)

	// Disconnect existing sessions.
	// This also closes the gate for any new registrations on the peer set.
	// sessions which are already established but not added to pm.peers yet
	// will exit when they try to register.
	pm.peers.Close()

	// Wait for all peer handler goroutines and the loops to come down.
	pm.wg.Wait()

	pm.Log().Info("BlizzardCS protocol stopped")
}

func (pm *ProtocolManager) newPeer(pv int, p *p2p.Peer, rw p2p.MsgReadWriter) *Peer {
	return newPeer(pv, p, newMeteredMsgWriter(rw))
}

func (pm *ProtocolManager) putCacheData(p *Peer, data [][]byte) {
	h := blizcore.DataHash(data...)
	pm.mapDataLock.Lock()
	defer pm.mapDataLock.Unlock()

	info := &infoData{h: h, data: data}
	d, ok := pm.mapData[p]
	if !ok {
		pm.mapData[p] = []*infoData{info}
		return
	}
	for _, v := range d {
		if v.h == h {
			return
		}
	}
	pm.mapData[p] = append(d, info)
}

func (pm *ProtocolManager) getCacheData(p *Peer, h common.Hash) [][]byte {
	pm.mapDataLock.Lock()
	defer pm.mapDataLock.Unlock()

	data, ok := pm.mapData[p]
	if !ok {
		return nil
	}
	for _, v := range data {
		if v.h == h {
			return v.data
		}
	}
	return nil
}

func (pm *ProtocolManager) deleteCacheData(p *Peer, h common.Hash) {
	pm.mapDataLock.Lock()
	defer pm.mapDataLock.Unlock()

	data, ok := pm.mapData[p]
	if !ok {
		return
	}
	var find bool
	infos := make([]*infoData, 0, len(data))
	for _, v := range data {
		if v.h == h {
			find = true
			continue
		}
		infos = append(infos, v)
	}
	if !find {
		return
	}
	if len(infos) == 0 {
		delete(pm.mapData, p)
		return
	}
	pm.mapData[p] = infos
}

// handle is the callback invoked to manage the life cycle of an eth peer. When
// this function terminates, the peer is disconnected.
func (pm *ProtocolManager) handle(p *Peer) error {
	pm.Log().Info("blizcs handle", "id", p.id)
	if pm.maxClients == 0 {
		pm.Log().Warn("----------??????????----------")
		pm.maxClients = 25
	}
	if pm.peers.Len() >= pm.maxClients {
		pm.Log().Error("handle", "pm.peers.Len", pm.peers.Len(), "pm.maxClients", pm.maxClients)
		return p2p.DiscTooManyPeers
	}
	pm.Log().Info("BlizzardCS client connected", "name", p.Name())

	// Execute the Ethereum handshake
	if err := p.Handshake(pm.networkId); err != nil {
		pm.Log().Error("BilzzardCS handshake failed", "err", err)
		return err
	}

	if rw, ok := p.rw.(*meteredMsgReadWriter); ok {
		rw.Init(p.version)
	}

	// JiangHan：将这个peer注册到 本地的 peers 中去
	// Register the peer locally
	if err := pm.peers.Register(p); err != nil {
		pm.Log().Error("BilzzardCS client registration failed", "err", err)
		return err
	}

	// 将它加入到我们的 BlizClntObject 对象组中
	pm.blizcs.JoinPeer(p)

	defer pm.removePeer(p.id)

	// JiangHan：上述握手等流程作完，就开始循环处理跟这个 peer 的所有消息
	// readmsg 读取， 处理等等..
	// main loop. handle incoming messages.
	for {
		if err := pm.handleMsg(p); err != nil {
			if err == io.EOF {
				pm.Log().Info("BilzzardCS message EOF")
			} else {
				pm.Log().Error("BilzzardCS message handling failed", "err", err)
			}
			return err
		}
	}
}

// JiangHan：（重点）所有的核心协议处理和分发handler，了解eth的运作机制，顺着线索摸
// handleMsg is invoked whenever an inbound message is received from a remote
// peer. The remote connection is torn down upon returning any error.
func (pm *ProtocolManager) handleMsg(p *Peer) error {
	// Read the next message from the remote peer, and ensure it's fully consumed
	msg, err := p.rw.ReadMsg()
	if err != nil {
		return err
	}
	if msg.Size > ProtocolMaxMsgSize {
		return errResp(ErrMsgTooLarge, "%v > %v", msg.Size, ProtocolMaxMsgSize)
	}
	defer msg.Discard()

	log.Msg("CS handle msg start", "code", msg.Code)
	defer log.Msg("CS handle msg end", "code", msg.Code)

	// Handle the message depending on its contents
	switch {
	case msg.Code == StatusMsg:
		// Status messages should never arrive after the handshake
		// return errResp(ErrExtraStatusMsg, "uncontrolled status message")
		pm.Log().Error("handleMsg uncontrolled status message")
		return nil

	case msg.Code == WriteReqMsg:
		var compressBytes []byte
		if err := msg.Decode(&compressBytes); err != nil {
			// maybe attacker, discard
			log.Error("doWriteReq", "err", err)
			return err
		}
		pm.putRWMsg(msg.Code, p, compressBytes)
		return nil

	case msg.Code == WriteRspMsg:
		return pm.doWriteRes(p, &msg)

	case msg.Code == ReadReqMsg:
		var compressBytes []byte
		if err := msg.Decode(&compressBytes); err != nil {
			log.Error("doReadReq", "err", err)
			return err
		}
		pm.putRWMsg(msg.Code, p, compressBytes)
		return nil

	case msg.Code == ReadRspMsg:
		pm.doReadRes(p, &msg)
		return nil

	case msg.Code == GetFarmerSliceOpMsg:
		var req blizcore.FarmerSliceOpReq
		if err := msg.Decode(&req); err != nil {
			log.Error("doFarmerSliceOpReq", "err", err)
			return err
		}
		pm.putRWMsg(msg.Code, p, &req)
		return nil

	case msg.Code == FarmerSliceOpMsg:
		return pm.doFarmerSliceOpRsp(p, &msg)

	case msg.Code == GetSliceMetaFromFarmerReqMsg:
		var req blizcore.GetSliceMetaFromFarmerReqData
		if err := msg.Decode(&req); err != nil {
			log.Error("doSliceMetaFromFarmerRsp", "err", err)
			return err
		}
		pm.putRWMsg(msg.Code, p, &req)
		return nil

	case msg.Code == GetBuffFromSharerCSMsg:
		var req blizcore.GetBuffFromSharerCSData
		if err := msg.Decode(&req); err != nil {
			log.Error("Do get buff from sharer cs", "err", err)
			return err
		}
		pm.putRWMsg(msg.Code, p, &req)
		return nil

	case msg.Code == BuffFromSharerCSMsg:
		return pm.doBuffFromSharerCS(p, &msg)

	default:
		return errResp(ErrInvalidMsgCode, "%v", msg.Code)
	}
	//return nil
}

func (pm *ProtocolManager) putRWMsg(code uint64, p *Peer, data interface{}) {
	pm.rwMsgLock.Lock()
	defer pm.rwMsgLock.Unlock()

	msg := &rwData{code: code, p: p, data: data}
	pm.rwMsg = append(pm.rwMsg, msg)
}

// handleRWMsg is special for handling msg that is related to read or write the chunk
func (pm *ProtocolManager) handleRWMsg(msg *rwData) error {
	code := msg.code
	p := msg.p

	switch {
	case code == WriteReqMsg:
		data, ok := msg.data.([]byte)
		if !ok {
			return errResp(ErrDecode, "%v", code)
		}
		ret, err := pm.doWriteReq(p, data)
		if ret == nil {
			return errResp(ErrDecode, "%v", code)
		}
		ret.ErrBytes = common.GetErrBytes(err)
		p.SendWriteRsp(ret)
		if err != nil {
			return errResp(ErrDecode, "%v: %v", msg, err)
		}
		return nil

	case code == ReadReqMsg:
		data, ok := msg.data.([]byte)
		if !ok {
			return errResp(ErrDecode, "%v", code)
		}
		ret, err := pm.doReadReq(p, data)
		if ret == nil {
			return errResp(ErrDecode, "%v", msg)
		}
		ret.ErrBytes = common.GetErrBytes(err)
		p.SendReadRsp(ret)
		if err != nil {
			e := errResp(ErrDecode, "%v: %v", msg, err)
			log.Error(e.Error())
			// return errResp(ErrDecode, "%v: %v", msg, err)
		}
		return nil

	case code == GetFarmerSliceOpMsg:
		data, ok := msg.data.(*blizcore.FarmerSliceOpReq)
		if !ok {
			return errResp(ErrDecode, "%v", code)
		}
		return pm.doFarmerSliceOpReq(p, data)

	case code == GetSliceMetaFromFarmerReqMsg:
		data, ok := msg.data.(*blizcore.GetSliceMetaFromFarmerReqData)
		if !ok {
			return errResp(ErrDecode, "%v", code)
		}
		return pm.doSliceMetaFromFarmerRsp(p, data)

	case code == GetBuffFromSharerCSMsg:
		data, ok := msg.data.(*blizcore.GetBuffFromSharerCSData)
		if !ok {
			return errResp(ErrDecode, "%v", code)
		}
		pm.doGetBuffFromSharerCS(p, data)
		return nil

	default:
		return errResp(blizcore.ErrInvalidMsgCode, "%v", code)
	}
}

func (pm *ProtocolManager) handleRWMsgs() {
	pm.rwMsgLock.Lock()
	t := len(pm.rwMsg)
	msgs := pm.rwMsg[:t]
	pm.rwMsg = pm.rwMsg[t:]
	pm.rwMsgLock.Unlock()

	peerMsgs := make(map[*Peer][]*rwData) // Group by the peer
	for _, msg := range msgs {
		peerMsgs[msg.p] = append(peerMsgs[msg.p], msg)
	}

	for _, ms := range peerMsgs {
		go func(msgs []*rwData) {
			for _, msg := range msgs {
				pm.handleRWMsg(msg)
			}
		}(ms)
	}
}
