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
	"errors"
	"fmt"

	//"math/big"
	"sync"
	"time"

	//"github.com/contatract/go-contatract/common"

	"github.com/contatract/go-contatract/p2p"

	//"github.com/contatract/go-contatract/p2p/discover"

	"github.com/contatract/go-contatract/blizcore"
	storage "github.com/contatract/go-contatract/blizzard/storage"
)

var (
	errClosed            = errors.New("partner set is closed")
	errAlreadyRegistered = errors.New("partner is already registered")
	errNotRegistered     = errors.New("partner is not registered")
)

const (
	handshakeTimeout = 5 * time.Second
)

// PartnerInfo represents a short summary of the Ethereum sub-protocol metadata known
// about a connected peer.
type PartnerInfo struct {
	Version int `json:"version"` // Ethereum protocol version negotiated
}

type partner struct {
	id string

	// eth.peer内关联的p2p.Peer对象
	*p2p.Peer
	rw p2p.MsgReadWriter

	version  int         // Protocol version negotiated
	forkDrop *time.Timer // Timed connection dropper if forks aren't validated in time

	lock sync.RWMutex

	// peer associated chunkAsso
	chunkAssos    map[string]*ChunkPartnerAsso        // Currently running services
	chunkWopProgs map[string][]*blizcore.SliceWopProg // 该partner的对于某chunk的slice wop更新进度数组
}

func newPartner(version int, p *p2p.Peer, rw p2p.MsgReadWriter) *partner {
	id := p.ID()

	return &partner{
		Peer:          p,
		rw:            rw,
		version:       version,
		id:            id.String(),
		chunkAssos:    make(map[string]*ChunkPartnerAsso),
		chunkWopProgs: make(map[string][]*blizcore.SliceWopProg),
	}
}

// Info gathers and returns a collection of metadata known about a peer.
func (p *partner) Info() *PartnerInfo {
	return &PartnerInfo{
		Version: p.version,
	}
}

// Handshake executes the eth protocol handshake, negotiating version number,
// network IDs, difficulties, head and genesis blocks.
func (p *partner) Handshake(bliz *Bliz, network uint64) (*statusData, error) {
	// Send out own handshake in a new thread
	errc := make(chan error, 2)
	var status statusData // safe to read after two values have been received from errc

	go func() {
		var cb BlizNodeCB
		//var chunkId storage.ChunkIdentity
		if p.Peer.Inbound() {
			// 对于别人连过来的，我们并不知晓对方的chunk需求，握手消息带空id
		} else {
			// 对于连出去的，我们需要将自己需要对方验证的chunkId传递过去
			// 强制类型转换
			if p.Peer.CustomBlock() != nil {
				cb = p.Peer.CustomBlock().(BlizNodeCB)
			}
		}
		errc <- p2p.Send(p.rw, StatusMsg, &statusData{
			ProtocolVersion: uint32(p.version),
			NetworkId:       network,
			Cb:              cb,
		})
	}()
	go func() {
		errc <- p.readStatus(bliz, network, &status)
	}()
	timeout := time.NewTimer(handshakeTimeout)
	defer timeout.Stop()
	for i := 0; i < 2; i++ {
		// 只等待上面两个信号，就退出 for 循环
		select {
		case err := <-errc:
			if err != nil {
				bliz.Log().Error("blizzard errc", "err", err)
				return nil, err
			}
		case <-timeout.C:
			bliz.Log().Error("blizzard Handshake timeout", "id", p.id)
			return nil, p2p.DiscReadTimeout
		}
	}
	bliz.Log().Info("blizzard Handshake end")
	return &status, nil
}

func errResp(code blizcore.ErrCode, format string, v ...interface{}) error {
	return fmt.Errorf("%v - %v", code, fmt.Sprintf(format, v...))
}

func (p *partner) readStatus(bliz *Bliz, network uint64, status *statusData) (err error) {
	t := time.Now()
	msg, err := p.rw.ReadMsg()
	if err != nil {
		return err
	}
	bliz.Log().Info("readStatus", "", time.Since(t))
	if msg.Code != StatusMsg {
		return errResp(blizcore.ErrNoStatusMsg, "first msg has code %x (!= %x)", msg.Code, StatusMsg)
	}
	if msg.Size > ProtocolMaxMsgSize {
		return errResp(blizcore.ErrMsgTooLarge, "%v > %v", msg.Size, ProtocolMaxMsgSize)
	}
	// Decode the handshake and make sure everything matches
	if err := msg.Decode(&status); err != nil {
		return errResp(blizcore.ErrDecode, "msg %v: %v", msg, err)
	}
	if status.NetworkId != network {
		return errResp(blizcore.ErrNetworkIdMismatch, "%d (!= %d)", status.NetworkId, network)
	}
	if int(status.ProtocolVersion) != p.version {
		return errResp(blizcore.ErrProtocolVersionMismatch, "%d (!= %d)", status.ProtocolVersion, p.version)
	}
	if status.Cb != (BlizNodeCB{}) {
		//if storage.Compare( status.chunkId, chunkId ) == false {
		//}
		// 我们在本地找找，看该chunkId是否也在本地进行提供并维护
		//
		// if status.Cb.CbMode == CBModePartner { // qiwy: TODO, for test
		// 	if bliz.AllowPartnerLogin(status.Cb.ChunkId, p) == false {
		// 		return errResp(blizcore.ErrChunkMismatch, "chunkId:%x partnerId:%x", status.Cb.ChunkId.String(), p.ID().String())
		// 	}
		// }
	} else {
		// 对方没有在status中携带cb{mode,chunkId}消息
		// if p.Peer.Inbound() {
		// 	// 对方主动连过来的，但没有携带chunk信息，这是不允许的
		// 	return errResp(blizcore.ErrNoShakeHandsChunkInfo, "partnerId:%x", p.ID().String())
		// }
	}
	bliz.Log().Info("readStatus", "", time.Since(t))
	return nil
}

// String implements fmt.Stringer.
func (p *partner) String() string {
	return fmt.Sprintf("Peer %s [%s]", p.id,
		fmt.Sprintf("bliz/%2d", p.version),
	)
}

func (p *partner) Link2Chunk(chunkId storage.ChunkIdentity, asso *ChunkPartnerAsso) bool {
	p.chunkAssos[chunkId.String()] = asso
	return true
}

func (p *partner) UnLink2Chunk(chunkId storage.ChunkIdentity) bool {
	delete(p.chunkAssos, chunkId.String())
	return true
}

// peerSet represents the collection of active peers currently participating in
// the Ethereum sub-protocol.
type partnerSet struct {
	partners map[string]*partner
	lock     sync.RWMutex
	closed   bool
}

// newPeerSet creates a new peer set to track the active participants.
func newPartnerSet() *partnerSet {
	return &partnerSet{
		partners: make(map[string]*partner),
	}
}

// Register injects a new peer into the working set, or returns an error if the
// peer is already known.
func (ps *partnerSet) Join(p *partner) error {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	if ps.closed {
		return errClosed
	}
	if _, ok := ps.partners[p.id]; ok {
		return errAlreadyRegistered
	}
	ps.partners[p.id] = p
	return nil
}

// Unregister removes a remote peer from the active set, disabling any further
// actions to/from that particular entity.
func (ps *partnerSet) Exit(id string) error {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	if _, ok := ps.partners[id]; !ok {
		return errNotRegistered
	}
	delete(ps.partners, id)
	return nil
}

// Peer retrieves the registered peer with the given id.
func (ps *partnerSet) Partner(id string) *partner {
	ps.lock.RLock()
	defer ps.lock.RUnlock()
	return ps.partners[id]
}

// Len returns if the current number of peers in the set.
func (ps *partnerSet) Len() int {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	return len(ps.partners)
}

// Close disconnects all peers.
// No new peers can be registered after Close has returned.
func (ps *partnerSet) Close() {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	for _, p := range ps.partners {
		p.Disconnect(p2p.DiscQuitting)
	}
	ps.closed = true
}

// ------------------------------------------- Message Sending -------------------------------------------------------

func (p *partner) SendHeartBeatMsg(hb *blizcore.HeartBeatMsgData) error {
	return p2p.Send(p.rw, HeartBeatMsg, hb)
}

func (p *partner) SendGetWriteOpMsg(hb *blizcore.GetWriteOpMsgData) error {
	return p2p.Send(p.rw, GetWriteOpMsg, hb)
}

func (p *partner) SendWriteOpMsg(hb []*blizcore.WriteOpMsgData) error {
	return p2p.Send(p.rw, WriteOpMsg, hb)
}

func (p *partner) SendGetWriteDataMsg(hb *blizcore.GetWriteDataMsgData) error {
	return p2p.Send(p.rw, GetWriteDataMsg, hb)
}

func (p *partner) SendWriteDataMsg(hb *blizcore.WriteDataMsgData) error {
	return p2p.Send(p.rw, WriteDataMsg, hb)
}

func (p *partner) SendGetSegmentDataMsg(hb *blizcore.GetSegmentDataMsgData) error {
	return p2p.Send(p.rw, GetSegmentDataMsg, hb)
}

func (p *partner) SendGetSliceInfoMsg(hb *blizcore.GetSliceInfoMsgData) error {
	return p2p.Send(p.rw, GetSliceInfoMsg, hb)
}

func (p *partner) SendSegmentDataMsg(data []byte) error {
	return p2p.Send(p.rw, SegmentDataMsg, data)
}

func (p *partner) SendChunkInfoMsg(data []byte) error {
	return p2p.Send(p.rw, ChunkInfoMsg, data)
}

func (p *partner) SendSliceInfoMsg(data []byte) error {
	return p2p.Send(p.rw, SliceInfoMsg, data)
}

func (p *partner) SendGetHeaderHashMsg(hb *blizcore.GetHeaderHashMsgData) error {
	return p2p.Send(p.rw, GetHeaderHashMsg, hb)
}

func (p *partner) SendHeaderHashMsg(hb *blizcore.HeaderHashMsgData) error {
	return p2p.Send(p.rw, HeaderHashMsg, hb)
}
