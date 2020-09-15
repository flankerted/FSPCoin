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
	"errors"
	"fmt"

	//"math/big"

	"sync"
	"time"

	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/rlp"

	//"github.com/contatract/go-contatract/core/types"
	"github.com/contatract/go-contatract/blizcore"
	"github.com/contatract/go-contatract/p2p"
	//"github.com/contatract/go-contatract/rlp"
)

var (
	errClosed            = errors.New("peer set is closed")
	errAlreadyRegistered = errors.New("peer is already registered")
	errNotRegistered     = errors.New("peer is not registered")
)

const (
	maxKnownTxs      = 32768 // Maximum transactions hashes to keep in the known list (prevent DOS)
	maxKnownBlocks   = 1024  // Maximum block hashes to keep in the known list (prevent DOS)
	handshakeTimeout = 5 * time.Second
)

// PeerInfo represents a short summary of the Ethereum sub-protocol metadata known
// about a connected peer.
type PeerInfo struct {
	Version int `json:"version"` // Ethereum protocol version negotiated
}

type Peer struct {
	id string

	// eth.peer内关联的p2p.Peer对象
	*p2p.Peer
	rw p2p.MsgReadWriter

	version int // Protocol version negotiated

	head common.Hash
	lock sync.RWMutex
}

func newPeer(version int, p *p2p.Peer, rw p2p.MsgReadWriter) *Peer {
	id := p.ID()

	return &Peer{
		Peer:    p,
		rw:      rw,
		version: version,
		id:      fmt.Sprintf("%x", id[:8]),
	}
}

// Info gathers and returns a collection of metadata known about a peer.
func (p *Peer) Info() *PeerInfo {
	return &PeerInfo{
		Version: p.version,
	}
}

// SendTransactions sends transactions to the peer and includes the hashes
// in its transaction hash set for future reference.
//func (p *Peer) SendTransactions(txs types.Transactions) error {
//	for _, tx := range txs {
//		p.knownTxs.Add(tx.Hash())
//	}
//	return p2p.Send(p.rw, TxMsg, txs)
//}

// Handshake executes the eth protocol handshake, negotiating version number,
// network IDs, difficulties, head and genesis blocks.
func (p *Peer) Handshake(network uint64) error {
	// Send out own handshake in a new thread
	errc := make(chan error, 2)
	var status statusData // safe to read after two values have been received from errc

	go func() {
		errc <- p2p.Send(p.rw, StatusMsg, &statusData{
			ProtocolVersion: uint32(p.version),
			NetworkId:       network,
		})
	}()
	go func() {
		errc <- p.readStatus(network, &status)
	}()
	timeout := time.NewTimer(handshakeTimeout)
	defer timeout.Stop()
	for i := 0; i < 2; i++ {
		select {
		case err := <-errc:
			if err != nil {
				log.Error("blizcs errc", "err", err)
				return err
			}
		case <-timeout.C:
			log.Error("blizcs Handshake timeout", "id", p.id)
			return p2p.DiscReadTimeout
		}
	}
	//p.td, p.head = status.TD, status.CurrentBlock
	log.Info("blizcs Handshake end")
	return nil
}

func errResp(code blizcore.ErrCode, format string, v ...interface{}) error {
	return fmt.Errorf("%v - %v", code, fmt.Sprintf(format, v...))
}

func (p *Peer) readStatus(network uint64, status *statusData) (err error) {
	msg, err := p.rw.ReadMsg()
	if err != nil {
		return err
	}
	if msg.Code != StatusMsg {
		return errResp(ErrNoStatusMsg, "first msg has code %x (!= %x)", msg.Code, StatusMsg)
	}
	if msg.Size > ProtocolMaxMsgSize {
		return errResp(ErrMsgTooLarge, "%v > %v", msg.Size, ProtocolMaxMsgSize)
	}
	// Decode the handshake and make sure everything matches
	if err := msg.Decode(&status); err != nil {
		return errResp(ErrDecode, "msg %v: %v", msg, err)
	}
	if status.NetworkId != network {
		return errResp(ErrNetworkIdMismatch, "%d (!= %d)", status.NetworkId, network)
	}
	if int(status.ProtocolVersion) != p.version {
		return errResp(ErrProtocolVersionMismatch, "%d (!= %d)", status.ProtocolVersion, p.version)
	}
	return nil
}

// String implements fmt.Stringer.
func (p *Peer) String() string {
	return fmt.Sprintf("Peer %s [%s]", p.id,
		fmt.Sprintf("eth/%2d", p.version),
	)
}

// peerSet represents the collection of active peers currently participating in
// the Ethereum sub-protocol.
type peerSet struct {
	peers  map[string]*Peer
	lock   sync.RWMutex
	closed bool
}

// newPeerSet creates a new peer set to track the active participants.
func newPeerSet() *peerSet {
	return &peerSet{
		peers: make(map[string]*Peer),
	}
}

// Register injects a new peer into the working set, or returns an error if the
// peer is already known.
func (ps *peerSet) Register(p *Peer) error {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	if ps.closed {
		return errClosed
	}
	if _, ok := ps.peers[p.id]; ok {
		return errAlreadyRegistered
	}
	ps.peers[p.id] = p
	return nil
}

// Unregister removes a remote peer from the active set, disabling any further
// actions to/from that particular entity.
func (ps *peerSet) Unregister(id string) error {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	if _, ok := ps.peers[id]; !ok {
		return errNotRegistered
	}
	delete(ps.peers, id)
	return nil
}

// Peer retrieves the registered peer with the given id.
func (ps *peerSet) Peer(id string) *Peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	return ps.peers[id]
}

// Len returns if the current number of peers in the set.
func (ps *peerSet) Len() int {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	return len(ps.peers)
}

// Close disconnects all peers.
// No new peers can be registered after Close has returned.
func (ps *peerSet) Close() {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	for _, p := range ps.peers {
		p.Disconnect(p2p.DiscQuitting)
	}
	ps.closed = true
}

func (ps *peerSet) GetPeer(nodeID string) *Peer {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	for _, p := range ps.peers {
		if nodeID != p.ID().String() {
			continue
		}
		return p
	}
	return nil
}

// SendWop announces the availability of a number of blocks through
// a hash notification.
func (p *Peer) SendWop(req *blizcore.WriteOperReqData) error {
	data, err := rlp.EncodeToBytes(req)
	if err != nil {
		return fmt.Errorf("send wop err: %v", err)
	}
	compressBytes, err := common.CompressBytes(data)
	if err != nil {
		return err
	}
	return p2p.Send(p.rw, WriteReqMsg, &compressBytes)
}

func (p *Peer) SendWriteRsp(rsp *blizcore.WriteOperRspData) error {
	return p2p.Send(p.rw, WriteRspMsg, rsp)
}

func (p *Peer) SendRop(req *blizcore.ReadOperReqData) error {
	data, err := rlp.EncodeToBytes(req)
	if err != nil {
		return fmt.Errorf("send read op err: %v", err)
	}
	compressBytes, err := common.CompressBytes(data)
	if err != nil {
		return err
	}
	return p2p.Send(p.rw, ReadReqMsg, &compressBytes)
}

func (p *Peer) SendReadRsp(rsp *blizcore.ReadOperRspData) error {
	data, err := rlp.EncodeToBytes(rsp)
	if err != nil {
		return fmt.Errorf("send read rsp err: %v", err)
	}
	compressBytes, err := common.CompressBytes(data)
	if err != nil {
		return err
	}
	return p2p.Send(p.rw, ReadRspMsg, &compressBytes)
}

func (p *Peer) SendFarmerSliceOp(req *blizcore.FarmerSliceOpReq) error {
	return p2p.Send(p.rw, GetFarmerSliceOpMsg, req)
}

func (p *Peer) SendFarmerSliceOpRsp(rsp *blizcore.FarmerSliceOpRsp) error {
	return p2p.Send(p.rw, FarmerSliceOpMsg, rsp)
}

func (p *Peer) SendGetSliceMetaFromFarmerReq(rsp *blizcore.GetSliceMetaFromFarmerReqData) error {
	return p2p.Send(p.rw, GetSliceMetaFromFarmerReqMsg, rsp)
}

func (p *Peer) SendGetBuffFromSharerCS(rsp *blizcore.GetBuffFromSharerCSData) error {
	return p2p.Send(p.rw, GetBuffFromSharerCSMsg, rsp)
}

func (p *Peer) SendBuffFromSharerCS(rsp *blizcore.BuffFromSharerCSData) error {
	return p2p.Send(p.rw, BuffFromSharerCSMsg, rsp)
}
