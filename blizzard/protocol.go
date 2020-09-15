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

/*
bliz implements the blizzard partner and client protocol [bliz] (sister of eth and shh)
the protocol instance is launched on each peer by the network layer if the
bzz protocol handler is registered on the p2p server.

The bliz protocol component
*/

import (
	"errors"
	"io"
	"strings"

	//"fmt"
	//"net"
	//"strconv"
	"sync"
	"time"

	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/p2p"

	storage "github.com/contatract/go-contatract/blizzard/storage"
	"github.com/contatract/go-contatract/p2p/discover"

	//"github.com/contatract/go-contatract/common"
	//"github.com/contatract/go-contatract/ethdb"
	"github.com/contatract/go-contatract/blizcore"
)

// Constants to match up protocol versions and messages
const (
	bliz1 = 1
	bliz2 = 2
)

// Official short name of the protocol used during capability negotiation.
var ProtocolName = "bliz"

// Supported versions of the eth protocol (first is primary).
var ProtocolVersions = []uint{bliz1}

// Added by jianghan for version retrive
var ProtocolVersionMin = bliz1 - 1
var ProtocolVersionMax = bliz1 + 1

// Number of implemented message corresponding to different protocol versions.
var ProtocolLengths = []uint64{0x0e + 1}

const (
	Version = 0
	//ProtocolLength     = uint64(18)
	ProtocolMaxMsgSize = 10 * 1024 * 1024
	NetworkId          = 3
)

// bliz protocol message codes
// JiangHan：（重要）eth 核心应用层协议（非底层p2p协议），阅读交易，合约等协议到这里看
const (
	// Protocol messages belonging to eth/62
	// 1 的Lengths 定义了到了8 ，这里实现了8个 --jianghan
	StatusMsg = 0x00

	// client, farmer partner 请求协议
	GetChunkInfoMsg = 0x01 // 获取某chunk的信息（最新应用更新的WopId，有多少个启用的Slice，等等基本信息)
	ChunkInfoMsg    = 0x02

	GetSliceInfoMsg = 0x02 // 获取某chunk:Slice的信息（最新应用更新的WopId，segment hash组，等等基本信息)
	SliceInfoMsg    = 0x03

	GetSegmentDataMsg = 0x05 // segment获取操作，只有partner能够执行，用户同步segment
	SegmentDataMsg    = 0x06

	GetWriteOpMsg = 0x07 // 获取WriteOp数据
	WriteOpMsg    = 0x08 //

	HeartBeatMsg = 0x09

	ChunkPartnerIndMsg = 0x0a // 给对方一个指示：表示自己从属哪个chunk,对方可以更新PartnerAsso关系

	GetWriteDataMsg = 0x0b // 获取slice数据
	WriteDataMsg    = 0x0c //

	GetHeaderHashMsg = 0x0d // 获取slice header hash
	HeaderHashMsg    = 0x0e //
)

// statusData is the network packet for the status message.
type statusData struct {
	ProtocolVersion uint32
	NetworkId       uint64

	Cb BlizNodeCB // 初次shakehand协商的合作chunkId
}

type rwData struct {
	code uint64
	p    *partner
	data interface{}
}

// bzz represents the swarm wire protocol
// an instance is running on each peer
type ProtocolManager struct {
	bliz         *Bliz
	networkId    uint64
	SubProtocols []p2p.Protocol

	maxPartners int
	partners    *partnerSet

	newPartnerCh   chan *partner
	quitSync       chan struct{}
	noMorePartners chan struct{}

	chunkDB *chunkDB
	// wait group is used for graceful shutdowns during downloading
	// and processing
	wg sync.WaitGroup

	rwMsgLock sync.RWMutex
	rwMsg     []*rwData
}

var errIncompatibleConfig = errors.New("incompatible configuration")

func NewProtocolManager(networkId uint64, database *chunkDB, bliz *Bliz) (*ProtocolManager, error) {
	// Create the protocol manager with the base fields
	manager := &ProtocolManager{
		bliz:           bliz,
		networkId:      networkId,
		newPartnerCh:   make(chan *partner),
		noMorePartners: make(chan struct{}),
		quitSync:       make(chan struct{}),
		chunkDB:        database,
		maxPartners:    2048,
		partners:       newPartnerSet(),
		rwMsg:          make([]*rwData, 0),
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
				partner, err := manager.newPartner(int(version), p, rw)
				if err != nil {
					return err
				}

				manager.wg.Add(1)
				defer manager.wg.Done()
				// JiangHan: (重要）这里是所有blizzard协议处理的总入口，处理来自peer的消息
				return manager.handle(partner)
				/*
					select {
					case manager.newPartnerCh <- partner:
						manager.wg.Add(1)
						defer manager.wg.Done()
						// JiangHan: (重要）这里是所有blizzard协议处理的总入口，处理来自peer的消息
						return manager.handle(partner)
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

func (pm *ProtocolManager) removePartner(id string) {
	// Short circuit if the peer was already removed
	partner := pm.partners.Partner(id)
	if partner == nil {
		return
	}
	pm.Log().Debug("Removing Blizzard partner", "partner", id)

	if err := pm.partners.Exit(id); err != nil {
		pm.Log().Error("Blizzard partner removal failed", "partner", id, "err", err)
	}
	// Hard disconnect at the networking layer
	if partner != nil {
		partner.Peer.Disconnect(p2p.DiscUselessPeer)
	}

	pm.bliz.LogoutPartner(partner)
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

func (pm *ProtocolManager) Start() {
	go pm.handleRWMsgsLoop()
}

func (pm *ProtocolManager) Stop() {
	pm.Log().Info("Stopping Blizzard protocol")

	// Quit the sync loop.
	// After this send has completed, no new peers will be accepted.

	//pm.noMorePartners <- struct{}{}

	// Quit fetcher, txsyncLoop.
	close(pm.quitSync)

	// Disconnect existing sessions.
	// This also closes the gate for any new registrations on the peer set.
	// sessions which are already established but not added to pm.peers yet
	// will exit when they try to register.
	pm.partners.Close()

	// Wait for all peer handler goroutines and the loops to come down.
	pm.wg.Wait()

	pm.Log().Info("Blizzard protocol stopped")
}

func (pm *ProtocolManager) newPartner(pv int, p *p2p.Peer, rw p2p.MsgReadWriter) (*partner, error) {
	//return newPartner(pv, p, newMeteredMsgWriter(rw)), nil

	if partner := pm.partners.Partner(p.ID().String()); partner == nil {
		return newPartner(pv, p, newMeteredMsgWriter(rw)), nil
	}
	return nil, blizcore.DiscAlreadyConnected
}

func (pm *ProtocolManager) GetPartner(id string) *partner {
	pm.Log().Info("GetPartner", "id", id, "Partner", pm.partners.Partner)
	return pm.partners.Partner(id)
}

// handle is the callback invoked to manage the life cycle of an eth peer. When
// this function terminates, the peer is disconnected.
func (pm *ProtocolManager) handle(p *partner) error {
	pm.Log().Info("blizzard handle", "id", p.id)
	if pm.partners.Len() >= pm.maxPartners {
		return p2p.DiscTooManyPeers
	}

	// Execute the Blizzard handshake
	status, err := p.Handshake(pm.bliz, pm.networkId)
	if err != nil {
		pm.Log().Error("Blizzard handshake failed", "err", err)
		return err
	}

	cb := status.Cb

	if cb == (BlizNodeCB{}) {
		// 对方那没有信息
		if p.Peer.CustomBlock() == nil {
			// 自己这里也没有信息
			//pm.Log().Error("OutBound handshake failed: can't find destinate cb{chunkId,mode}", "err", err)
			//return err
		} else {
			cb = (p.Peer.CustomBlock()).(BlizNodeCB)
		}
	}

	if rw, ok := p.rw.(*meteredMsgReadWriter); ok {
		rw.Init(p.version)
	}

	// JiangHan：将这个peer注册到 本地的 peers 中去
	// Register the peer locally
	if err := pm.partners.Join(p); err != nil {
		pm.Log().Error("Bilzzard ProtocolManager partner registration failed", "err", err)
		return err
	}

	// 注： 这里我们将 partner 概念延伸一下，它可以是 farmer同伴，也可以是普通用户请求者
	//if cb.CbMode == CBModePartner {
	//// 只有 farmer 寻求同伴才允许登录 partner ,否则该 partner 仅仅是作为普通客户端用户连接上来
	if _, err := pm.bliz.LoginPartner(p, cb.ChunkId); err != nil {
		pm.Log().Error("Bilzzard partner registration failed", "err", err)
		return err
	}
	//}

	defer pm.removePartner(p.id)

	// JiangHan：上述握手等流程作完，就开始循环处理跟这个 peer 的所有消息
	// readmsg 读取， 处理等等..
	// main loop. handle incoming messages.
	for {
		if err := pm.handleMsg(p); err != nil {
			if err == io.EOF {
				pm.Log().Info("Blizzard message EOF")
			} else {
				pm.Log().Error("Blizzard message handling failed", "err", err)
			}
			return err
		}
	}
}

// JiangHan：（重点）所有的核心协议处理和分发handler，了解eth的运作机制，顺着线索摸
// handleMsg is invoked whenever an inbound message is received from a remote
// peer. The remote connection is torn down upon returning any error.
func (pm *ProtocolManager) handleMsg(p *partner) error {
	// Read the next message from the remote peer, and ensure it's fully consumed
	msg, err := p.rw.ReadMsg()
	if err != nil {
		// JiangHan:
		// 如果外界连接中断，ReadMsg 会在 p.rw.close 处
		// 收到信号，这里会触发退出
		return err
	}
	if msg.Size > ProtocolMaxMsgSize {
		return errResp(blizcore.ErrMsgTooLarge, "%v > %v", msg.Size, ProtocolMaxMsgSize)
	}
	defer msg.Discard()

	log.Msg("Bliz handle msg start", "code", msg.Code)
	defer log.Msg("Bliz handle msg end", "code", msg.Code)

	// Handle the message depending on its contents
	switch {
	case msg.Code == StatusMsg:
		// Status messages should never arrive after the handshake
		return errResp(blizcore.ErrExtraStatusMsg, "uncontrolled status message")

	case msg.Code == GetChunkInfoMsg:
		return errResp(blizcore.ErrExtraStatusMsg, "uncontrolled status message")
	case msg.Code == GetSliceInfoMsg:
		//return errResp(blizcore.ErrExtraStatusMsg, "uncontrolled status message")
		pm.Log().Info("GetSliceInfoMsg")
		return nil

	case msg.Code == HeartBeatMsg:
		err := pm.doPartnerHeartBeat(p, msg)
		if err != nil {
			pm.Log().Error("doPartnerHeartBeat", "err", err)
		}
		return nil

	case msg.Code == GetWriteOpMsg:
		err := pm.doGetWriteOp(p, msg)
		if err != nil {
			pm.Log().Info("Do get write op", "ret", err)
		}
		return nil

	case msg.Code == WriteOpMsg:
		var msgData []*blizcore.WriteOpMsgData
		if err := msg.Decode(&msgData); err != nil {
			return errResp(blizcore.ErrDecode, "%v: %v", msg, err)
		}
		pm.putRWMsg(msg.Code, p, msgData)
		return nil

	case msg.Code == GetWriteDataMsg:
		var msgData blizcore.GetWriteDataMsgData
		if err := msg.Decode(&msgData); err != nil {
			return errResp(blizcore.ErrDecode, "%v: %v", msg, err)
		}
		pm.putRWMsg(msg.Code, p, &msgData)
		return nil

	case msg.Code == WriteDataMsg:
		var msgData blizcore.WriteDataMsgData
		if err := msg.Decode(&msgData); err != nil {
			return errResp(blizcore.ErrDecode, "%v: %v", msg, err)
		}
		pm.putRWMsg(msg.Code, p, &msgData)
		return nil

	case msg.Code == GetHeaderHashMsg:
		var msgData blizcore.GetHeaderHashMsgData
		if err := msg.Decode(&msgData); err != nil {
			return errResp(blizcore.ErrDecode, "%v: %v", msg, err)
		}
		pm.putRWMsg(msg.Code, p, &msgData)
		return nil

	case msg.Code == HeaderHashMsg:
		err := pm.doHeaderHash(p, msg)
		if err != nil {
			pm.Log().Error("doHeaderHash", "err", err)
		}
		return nil

	default:
		return errResp(blizcore.ErrInvalidMsgCode, "%v", msg.Code)
	}
	//return nil
}

func (pm *ProtocolManager) putRWMsg(code uint64, p *partner, data interface{}) {
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
	case code == WriteOpMsg:
		data, ok := msg.data.([]*blizcore.WriteOpMsgData)
		if !ok {
			return errResp(blizcore.ErrInvalidMsgCode, "%v", code)
		}
		var err error
		for _, v := range data {
			err = pm.doWriteOp(p, v)
			if err != nil {
				pm.Log().Error("doWriteOp", "err", err)
				break
			}
		}
		if len(data) > 0 {
			asso := pm.bliz.GetAsso(storage.Int2ChunkIdentity(data[0].Info.Chunk))
			if asso != nil {
				asso.deleteSyncingSlice(p, data[0].Info.Slice)
			}
		}
		return err

	case code == GetWriteDataMsg:
		data, ok := msg.data.(*blizcore.GetWriteDataMsgData)
		if !ok {
			return errResp(blizcore.ErrInvalidMsgCode, "%v", code)
		}
		err := pm.doGetWriteData(p, data)
		if err != nil {
			pm.Log().Error("doGetWriteData", "err", err)
		}
		return err

	case code == WriteDataMsg:
		data, ok := msg.data.(*blizcore.WriteDataMsgData)
		if !ok {
			return errResp(blizcore.ErrInvalidMsgCode, "%v", code)
		}
		err := pm.doWriteData(p, data)
		if err != nil {
			pm.Log().Error("doWriteData", "err", err)
		}
		asso := pm.bliz.GetAsso(storage.Int2ChunkIdentity(data.Rsp.Chunk))
		if asso != nil {
			asso.deleteSyncingSlice(p, data.Rsp.Slice)
		}
		return err

	case code == GetHeaderHashMsg:
		data, ok := msg.data.(*blizcore.GetHeaderHashMsgData)
		if !ok {
			return errResp(blizcore.ErrInvalidMsgCode, "%v", code)
		}
		err := pm.doGetHeaderHash(p, data)
		if err != nil {
			pm.Log().Error("doGetHeaderHash", "err", err)
		}
		return err

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

	peerMsgs := make(map[*partner][]*rwData) // Group by the partner
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

func indexSliceWopProgs(prog *blizcore.SliceWopProg, progs []*blizcore.SliceWopProg) int {
	for i, p := range progs {
		if p.Slice == prog.Slice {
			return i
		}
	}
	return -1
}

func mergeSliceWopProgs(olds, news []*blizcore.SliceWopProg) []*blizcore.SliceWopProg {
	ret := olds
	for _, new := range news {
		i := indexSliceWopProgs(new, ret)
		if i == -1 {
			ret = append(ret, new)
		} else if ret[i].WopId < new.WopId {
			ret[i].WopId = new.WopId
		}
	}
	return ret
}

// ------------------------------------------Protocol Implementation----------------------------------------------------

// 处理来自其他 partner 的心跳包
func (pm *ProtocolManager) doPartnerHeartBeat(p *partner, msg p2p.Msg) error {
	pm.Log().Trace("doPartnerHeartBeat", "p", p, "msg", msg)
	// 获取对方来自的 asso
	var msgdata blizcore.HeartBeatMsgData
	if err := msg.Decode(&msgdata); err != nil {
		// maybe attacker, discard
		return errResp(blizcore.ErrDecode, "%v: %v", msg, err)
	}
	pm.Log().Info("", "ChunkId", msgdata.ChunkId)
	asso := pm.bliz.GetAsso(storage.Int2ChunkIdentity(msgdata.ChunkId))
	if asso == nil {
		pm.Log().Info("asso nil")
		return nil
	}

	if pm.bliz.HasHeartBeat(&msgdata) {
		// 已经处理或者接收过，丢弃，不理会
		pm.Log().Info("HasHeartBeat")
		return nil
	}

	senderId, err := msgdata.Sender()
	if err != nil {
		pm.Log().Error("", "err", err)
		return err
	}

	// 确保不是自己曾经发出的
	ourID := discover.PubkeyID(&pm.bliz.nodeserver.PrivateKey.PublicKey)
	if strings.Compare(senderId.String(), ourID.String()) == 0 {
		pm.Log().Error("", "senderId", senderId, "ourID", ourID)
		return nil
	}

	now := uint64(time.Now().Unix())
	if now > msgdata.Ts+300 {
		pm.Log().Error("", "now", now, "msgdata.Ts", msgdata.Ts)
		// 过期消息 or 本地时间不合法
		// return nil // qiwy: TODO, for test
	}

	// 保存对方的 slice wop progs 到本地
	chunkIdStr := storage.Int2ChunkIdentity(msgdata.ChunkId).String()
	wopProgs := p.chunkWopProgs[chunkIdStr]
	p.chunkWopProgs[chunkIdStr] = mergeSliceWopProgs(wopProgs, msgdata.Progs)
	for _, prog := range msgdata.Progs {
		pm.Log().Trace("", "prog", prog)
	}

	// 做上标记，表示自己接收过
	pm.bliz.MarkHeartBeat(&msgdata)

	// 有关 slice wop progs 的处理，可以交给维护线程来做，这里发送一个通道信号

	// 消息不需要继续 pass 传递给其他 partner
	// 就算传递给它们，也无意义，因为
	/*
		for _, partner := range asso.partners {
			if partner != nil {
				partner.SendHeartBeatMsg(&msgdata)
			}
		}
	*/

	return nil
}

func (pm *ProtocolManager) doGetWriteOp(p *partner, msg p2p.Msg) error {
	var msgData blizcore.GetWriteOpMsgData
	if err := msg.Decode(&msgData); err != nil {
		return errResp(blizcore.ErrDecode, "%v: %v", msg, err)
	}
	pm.Log().Info("Do get write op", "data", msgData)

	asso := pm.bliz.GetAsso(storage.Int2ChunkIdentity(msgData.Chunk))
	if asso == nil {
		pm.Log().Error("GetAsso asso nil")
		return nil
	}
	rspData, err := asso.GetAllWriteOpDataFromQueue(&msgData)
	if err != nil {
		pm.Log().Info("Get all write op data from queue", "err", err)
		return err
	}
	err = p.SendWriteOpMsg(rspData)
	if err != nil {
		pm.Log().Error("Send write op msg", "err", err)
	}
	return err
}

func (pm *ProtocolManager) doWriteOp(p *partner, msgData *blizcore.WriteOpMsgData) error {
	return pm.bliz.WriteOpDataFromMsg(msgData)
}

func (pm *ProtocolManager) doGetWriteData(p *partner, msgData *blizcore.GetWriteDataMsgData) error {
	pm.Log().Info("doGetWriteData", "msgData", msgData)

	asso := pm.bliz.GetAsso(storage.Int2ChunkIdentity(msgData.Chunk))
	if asso == nil {
		pm.Log().Error("GetAsso asso nil")
		return nil
	}
	header, data, err := asso.GetWriteDataFromStorage(msgData)
	if err != nil {
		pm.Log().Error("GetWriteDataFromStorage", "err", err)
		return err
	}

	rspData := &blizcore.WriteDataMsgData{Rsp: msgData, Header: header, Data: data}
	err = p.SendWriteDataMsg(rspData)
	if err != nil {
		pm.Log().Error("SendWriteDataMsg", "err", err)
	}
	return err
}

func (pm *ProtocolManager) doWriteData(p *partner, msgData *blizcore.WriteDataMsgData) error {
	asso := pm.bliz.GetAsso(storage.Int2ChunkIdentity(msgData.Rsp.Chunk))
	if asso == nil {
		pm.Log().Error("GetAsso asso nil")
		return nil
	}
	err := asso.SynchronizeWrite(msgData, pm.bliz.GetStorageManager())
	if err != nil {
		pm.Log().Error("Synchronize write", "err", err)
	}
	return err
}

func (pm *ProtocolManager) doGetHeaderHash(p *partner, msgData *blizcore.GetHeaderHashMsgData) error {
	pm.Log().Info("doGetHeaderHash", "msgData", msgData)

	header := pm.bliz.GetStorageManager().ReadChunkSliceHeader(msgData.Chunk, msgData.Slice)
	if header == nil {
		return errors.New("ReadChunkSliceHeader header nil")
	}

	rspData := &blizcore.HeaderHashMsgData{Rsp: msgData, HeaderHash: header.Hash(), OpId: header.OpId}
	err := p.SendHeaderHashMsg(rspData)
	if err != nil {
		pm.Log().Error("SendHeaderHashMsg", "err", err)
	}
	return err
}

func (pm *ProtocolManager) doHeaderHash(p *partner, msg p2p.Msg) error {
	var msgData blizcore.HeaderHashMsgData
	if err := msg.Decode(&msgData); err != nil {
		return errResp(blizcore.ErrDecode, "%v: %v", msg, err)
	}
	pm.Log().Info("doHeaderHash", "msgData", msgData)

	asso := pm.bliz.GetAsso(storage.Int2ChunkIdentity(msgData.Rsp.Chunk))
	if asso == nil {
		pm.Log().Error("GetAsso asso nil")
		return nil
	}
	err := asso.UpdateCheckHeaderHash(msgData.Rsp.Slice, msgData.HeaderHash, p.ID(), msgData.OpId)
	if err != nil {
		pm.Log().Error("UpdateCheckHeaderHash", "err", err)
	}
	return err
}

func (pm *ProtocolManager) Log() log.Logger {
	return pm.bliz.Log()
}
