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
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/contatract/go-contatract/blizzard/storage"
	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/consensus"
	types "github.com/contatract/go-contatract/core/types_elephant"
	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/p2p"
	"github.com/contatract/go-contatract/p2p/discover"
	"github.com/contatract/go-contatract/rlp"
	"gopkg.in/fatih/set.v0"
)

var (
	errClosed            = errors.New("peer set is closed")
	errAlreadyRegistered = errors.New("peer is already registered")
	errNotRegistered     = errors.New("peer is not registered")
	errNoPeerForPolice   = errors.New("peer for police is nil, since there are no specific shard peers")
)

var (
	errParam = errors.New("param error")
)

const (
	maxKnownTxs      = 32768 // Maximum transactions hashes to keep in the known list (prevent DOS)
	maxKnownBlocks   = 1024  // Maximum block hashes to keep in the known list (prevent DOS)
	handshakeTimeout = 5 * time.Second
	peerInOutRatio   = 4
)

// PeerInfo represents a short summary of the Ethereum sub-protocol metadata known
// about a connected peer.
type PeerInfo struct {
	Version        int         `json:"version"`    // Ethereum protocol version negotiated
	Difficulty     *big.Int    `json:"difficulty"` // Total difficulty of the peer's blockchain
	HeightEle      int64       `json:"height"`     // Highest height of the peer's block in it's blockchain
	LampHeight     int64       `json:"lampHeight"`
	LampBaseHeight int64       `json:"lampBaseHeight"`
	LampBaseHash   common.Hash `json:"lampBaseHash"`
	LampTD         *big.Int    `json:"lampTD"`
	Head           string      `json:"head"` // SHA3 hash of the peer's best owned block
	RealShard      uint16      `json:"realShard"`
}

type peer struct {
	id string

	// lamp.peer内关联的p2p.Peer对象
	*p2p.Peer
	rw p2p.MsgReadWriter

	version  int         // Protocol version negotiated
	forkDrop *time.Timer // Timed connection dropper if forks aren't validated in time

	head           common.Hash
	td             *big.Int
	heightEle      int64
	lampHeight     int64
	lampBaseHeight int64
	lampBaseHash   common.Hash // Hash of lamp block whose height is the height of lamp
	lampTD         *big.Int    // TD of lamp block whose height is the height of lamp
	realShard      uint16
	totalShard     uint16
	lock           sync.RWMutex

	// 这个peer贡献（知道--已经传递过）的所有交易
	knownTxs *set.Set // Set of transaction hashes known to be known by this peer
	// 这个peer贡献（知道--已经传递过）的所有交易
	knownShardingTxBatches *set.Set // Set of transaction hashes known to be known by this peer
	// 这个peer贡献的所有区块
	knownVerifiedHeaders map[string]common.Hash // Set of block header hashes that verified by a police officer known to be known by this peer
	// 这个peer贡献的所有区块
	knownBlocks *set.Set // Set of block hashes known to be known by this peer
	//
	knownChunkProvider *set.Set
	lastestEleRet      bool
}

func newPeer(version int, p *p2p.Peer, rw p2p.MsgReadWriter) *peer {
	id := p.ID()

	return &peer{
		Peer:                   p,
		rw:                     rw,
		version:                version,
		id:                     fmt.Sprintf("%x", id[:8]),
		knownTxs:               set.New(),
		knownShardingTxBatches: set.New(),
		knownBlocks:            set.New(),
		knownVerifiedHeaders:   make(map[string]common.Hash),
		knownChunkProvider:     set.New(),
	}
}

// Info gathers and returns a collection of metadata known about a peer.
func (p *peer) Info() *PeerInfo {
	hash, heightEle, lampBaseHeight, lampHash, lampTD := p.Head()
	lampHeight := p.GetLampHeight()
	realShard := p.GetRealShard()

	return &PeerInfo{
		Version: p.version,
		//Difficulty: td,
		HeightEle:      heightEle,
		LampHeight:     lampHeight,
		LampBaseHeight: lampBaseHeight,
		LampBaseHash:   lampHash,
		LampTD:         lampTD,
		Head:           hash.Hex(),
		RealShard:      realShard,
	}
}

// Head retrieves a copy of the current head hash and total difficulty of the
// peer.
func (p *peer) Head() (hash common.Hash, heightEle int64, lampHeight int64, lampHash common.Hash, lampTD *big.Int) {
	p.lock.RLock()
	defer p.lock.RUnlock()

	copy(hash[:], p.head[:])
	copy(lampHash[:], p.lampBaseHash[:])
	return hash, p.heightEle, p.lampBaseHeight, lampHash, new(big.Int).Set(p.lampTD)
}

// SetHead updates the head hash and total difficulty of the peer.
func (p *peer) SetHead(hash common.Hash, heightEle int64, lampHeight int64, lampHash common.Hash, lampTD *big.Int) {
	p.lock.Lock()
	defer p.lock.Unlock()

	copy(p.head[:], hash[:])
	p.heightEle = heightEle
	p.lampBaseHeight = lampHeight
	copy(p.lampBaseHash[:], lampHash[:])
	p.lampTD.Set(lampTD)
}

func (p *peer) SetLampHeight(lampHeight int64) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.lampHeight = lampHeight
}

func (p *peer) GetLampHeight() int64 {
	p.lock.Lock()
	defer p.lock.Unlock()

	return p.lampHeight
}

func (p *peer) GetRealShard() uint16 {
	p.lock.Lock()
	defer p.lock.Unlock()

	return p.realShard
}

// MarkBlock marks a block as known for the peer, ensuring that the block will
// never be propagated to this particular peer.
func (p *peer) MarkBlock(hashWithBft common.Hash) {
	// If we reached the memory allowance, drop a previously known block hash
	for p.knownBlocks.Size() >= maxKnownBlocks {
		p.knownBlocks.Pop()
	}
	p.knownBlocks.Add(hashWithBft)
}

// MarkTransaction marks a transaction as known for the peer, ensuring that it
// will never be propagated to this particular peer.
func (p *peer) MarkTransaction(hash common.Hash) {
	// If we reached the memory allowance, drop a previously known transaction hash
	for p.knownTxs.Size() >= maxKnownTxs {
		p.knownTxs.Pop()
	}
	p.knownTxs.Add(hash)
}

// MarkSHardingTx marks a sharding transaction as known for the peer, ensuring that it
// will never be propagated to this particular peer.
func (p *peer) MarkShardingTxBatch(hash common.Hash) {
	// If we reached the memory allowance, drop a previously known transaction hash
	for p.knownShardingTxBatches.Size() >= maxKnownTxs {
		p.knownShardingTxBatches.Pop()
	}
	p.knownShardingTxBatches.Add(hash)
}

// SendTransactions sends transactions to the peer and includes the hashes
// in its transaction hash set for future reference.
func (p *peer) SendTransactions(txs types.Transactions) error {
	for _, tx := range txs {
		p.knownTxs.Add(tx.Hash())
	}
	return p2p.Send(p.rw, TxMsg, txs)
}

// SendShardingTxBatches sends sharding transactions to the peer and includes the hashes
// in its transaction hash set for future reference.
func (p *peer) SendShardingTxBatches(shardingTxs types.ShardingTxBatches) error {
	for _, shardingTxBatch := range shardingTxs {
		p.knownShardingTxBatches.Add(shardingTxBatch.Hash())
	}
	return p2p.Send(p.rw, ShardingTxMsg, shardingTxs)
}

// SendVerifiedHeader sends a block header hash verified by a police officer to the peer and includes the hash with
// the signature of the officer in its verified header map for future reference.
func (p *peer) SendVerifiedHeader(header consensus.PoliceVerifiedHeader) error {
	p.knownVerifiedHeaders[header.Sign] = *header.Header
	return p2p.Send(p.rw, VerifiedHeaderMsg, header)
}

// SendNewBlockHashes announces the availability of a number of blocks through
// a hash notification.
func (p *peer) SendNewBlockHashes(hashWithBfts, hashes []common.Hash, numbers []uint64) error {
	for _, hashWithBft := range hashWithBfts {
		p.knownBlocks.Add(hashWithBft)
	}
	request := make(newBlockHashesData, len(hashes))
	for i := 0; i < len(hashes); i++ {
		request[i].Hash = hashes[i]
		request[i].HashWithBft = hashWithBfts[i]
		request[i].Number = numbers[i]
	}
	return p2p.Send(p.rw, NewBlockHashesMsg, request)
}

// SendNewBlock propagates an entire block to a remote peer.
func (p *peer) SendNewBlock(block *types.Block, td *big.Int, lampHash common.Hash, lampTD *big.Int) error {
	p.knownBlocks.Add(block.HashWithBft())
	return p2p.Send(p.rw, NewBlockMsg, []interface{}{block, td, lampHash, lampTD})
}

// SendBlockHeaders sends a batch of block headers to the remote peer.
func (p *peer) SendBlockHeaders(headers []*types.Header, forPolice bool) error {
	if !forPolice {
		return p2p.Send(p.rw, BlockHeadersMsg, headers)
	} else {
		return p2p.Send(p.rw, BlockHeadersForPoliceMsg, headers)
	}
}

// SendBlockBodies sends a batch of block contents to the remote peer.
func (p *peer) SendBlockBodies(bodies []*blockBody) error {
	return p2p.Send(p.rw, BlockBodiesMsg, blockBodiesData(bodies))
}

// SendBlockBodiesRLP sends a batch of block contents to the remote peer from
// an already RLP encoded format.
func (p *peer) SendBlockBodiesRLP(bodies []rlp.RawValue) error {
	return p2p.Send(p.rw, BlockBodiesMsg, bodies)
}

// SendNodeDataRLP sends a batch of arbitrary internal data, corresponding to the
// hashes requested.
func (p *peer) SendNodeData(data [][]byte) error {
	return p2p.Send(p.rw, NodeDataMsg, data)
}

// SendReceiptsRLP sends a batch of transaction receipts, corresponding to the
// ones requested from an already RLP encoded format.
func (p *peer) SendReceiptsRLP(receipts []rlp.RawValue) error {
	return p2p.Send(p.rw, ReceiptsMsg, receipts)
}

// RequestOneHeader is a wrapper around the header query functions to fetch a
// single header. It is used solely by the fetcher.
func (p *peer) RequestOneHeader(hash common.Hash) error {
	p.Log().Debug("Fetching single header", "hash", hash)
	return p2p.Send(p.rw, GetBlockHeadersMsg, &getBlockHeadersData{
		Origin:    hashOrNumber{Hash: hash},
		Amount:    uint64(1),
		Skip:      uint64(0),
		Reverse:   false,
		ForPolice: false,
	})
}

func (p *peer) RequestOneHeaderForPolice(hash common.Hash) error {
	p.Log().Debug("Fetching single header", "hash", hash)
	return p2p.Send(p.rw, GetBlockHeadersMsg, &getBlockHeadersData{
		Origin:    hashOrNumber{Hash: hash},
		Amount:    uint64(1),
		Skip:      uint64(0),
		Reverse:   false,
		ForPolice: true,
	})
}

// RequestHeadersByHash fetches a batch of blocks' headers corresponding to the
// specified header query, based on the hash of an origin block.
func (p *peer) RequestHeadersByHash(origin common.Hash, amount int, skip int, reverse bool) error {
	p.Log().Debug("Fetching batch of headers", "count", amount, "fromhash", origin, "skip", skip, "reverse", reverse)
	return p2p.Send(p.rw, GetBlockHeadersMsg, &getBlockHeadersData{Origin: hashOrNumber{Hash: origin}, Amount: uint64(amount), Skip: uint64(skip), Reverse: reverse})
}

// RequestHeadersByNumber fetches a batch of blocks' headers corresponding to the
// specified header query, based on the number of an origin block.
func (p *peer) RequestHeadersByNumber(origin uint64, amount int, skip int, reverse bool) error {
	p.Log().Debug("Fetching batch of headers", "count", amount, "fromnum", origin, "skip", skip, "reverse", reverse)
	return p2p.Send(p.rw, GetBlockHeadersMsg, &getBlockHeadersData{Origin: hashOrNumber{Number: origin}, Amount: uint64(amount), Skip: uint64(skip), Reverse: reverse})
}

func (p *peer) RequestHeadersByNumberForPolice(origin uint64, amount int, skip int, reverse bool) error {
	p.Log().Debug("Fetching batch of headers", "count", amount, "fromnum", origin, "skip", skip, "reverse", reverse)
	return p2p.Send(p.rw, GetBlockHeadersMsg, &getBlockHeadersData{
		Origin:    hashOrNumber{Number: origin},
		Amount:    uint64(amount),
		Skip:      uint64(skip),
		Reverse:   reverse,
		ForPolice: true,
	})
}

// RequestBodies fetches a batch of blocks' bodies corresponding to the hashes
// specified.
func (p *peer) RequestBodies(hashes []common.Hash) error {
	p.Log().Debug("Fetching batch of block bodies", "count", len(hashes))
	return p2p.Send(p.rw, GetBlockBodiesMsg, hashes)
}

// RequestNodeData fetches a batch of arbitrary data from a node's known state
// data, corresponding to the specified hashes.
func (p *peer) RequestNodeData(hashes []common.Hash) error {
	p.Log().Debug("Fetching batch of state data", "count", len(hashes))
	return p2p.Send(p.rw, GetNodeDataMsg, hashes)
}

// RequestReceipts fetches a batch of transaction receipts from a remote node.
func (p *peer) RequestReceipts(hashes []common.Hash) error {
	p.Log().Debug("Fetching batch of receipts", "count", len(hashes))
	return p2p.Send(p.rw, GetReceiptsMsg, hashes)
}

// Handshake executes the lamp protocol handshake, negotiating version number,
// network IDs, difficulties, head and genesis blocks.
func (p *peer) Handshake(network uint64, td *big.Int, height *big.Int, lampHeight *big.Int, lampBaseHeight *big.Int,
	lampHash common.Hash, lampTD *big.Int, head common.Hash, genesis common.Hash, realShard uint16) error {
	// Send out own handshake in a new thread
	errc := make(chan error, 2)
	var status statusData // safe to read after two values have been received from errc

	go func() {
		errc <- p2p.Send(p.rw, StatusMsg, &statusData{
			ProtocolVersion: uint32(p.version),
			NetworkId:       network,
			TD:              td,
			HeightEle:       height,
			LampHeight:      lampHeight,
			LampBaseHeight:  lampBaseHeight,
			LampBaseHash:    lampHash,
			LampTD:          lampTD,
			CurrentBlock:    head,
			GenesisBlock:    genesis,
			RealShard:       realShard,
		})
	}()
	go func() {
		errc <- p.readStatus(network, &status, genesis)
	}()
	timeout := time.NewTimer(handshakeTimeout)
	defer timeout.Stop()
	for i := 0; i < 2; i++ {
		select {
		case err := <-errc:
			if err != nil {
				log.Error("elephant errc", "err", err)
				return err
			}
		case <-timeout.C:
			log.Error("elephant Handshake timeout", "id", p.id)
			return p2p.DiscReadTimeout
		}
	}
	p.td, p.heightEle, p.lampHeight, p.lampBaseHeight, p.lampBaseHash, p.lampTD, p.head, p.realShard = status.TD, status.HeightEle.Int64(), status.LampHeight.Int64(), status.LampBaseHeight.Int64(), status.LampBaseHash, status.LampTD, status.CurrentBlock, status.RealShard
	log.Info("elephant Handshake end")
	return nil
}

func (p *peer) readStatus(network uint64, status *statusData, genesis common.Hash) (err error) {
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
	if status.GenesisBlock != genesis {
		return errResp(ErrGenesisBlockMismatch, "%x (!= %x)", status.GenesisBlock[:8], genesis[:8])
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
func (p *peer) String() string {
	return fmt.Sprintf("Peer %s [%s]", p.id,
		fmt.Sprintf("lamp/%2d", p.version),
	)
}

// peerSet represents the collection of active peers currently participating in
// the Ethereum sub-protocol.
type peerSet struct {
	peers  map[string]*peer
	lock   sync.RWMutex
	closed bool
}

// newPeerSet creates a new peer set to track the active participants.
func newPeerSet() *peerSet {
	return &peerSet{
		peers: make(map[string]*peer),
	}
}

// Register injects a new peer into the working set, or returns an error if the
// peer is already known.
func (ps *peerSet) Register(p *peer) error {
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
func (ps *peerSet) Peer(id string) *peer {
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

// PeersWithoutBlock retrieves a list of peers that do not have a given block in
// their set of known hashes.
func (ps *peerSet) PeersWithoutBlock(hashWithBft common.Hash) []*peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	list := make([]*peer, 0, len(ps.peers))
	for _, p := range ps.peers {
		if !p.knownBlocks.Has(hashWithBft) {
			list = append(list, p)
		}
	}
	return list
}

// PeersWithoutBlock retrieves a list of peers that do not have a given block in
// their set of known hashes.
func (ps *peerSet) GetSealerPeers(sealers []common.Address, nodesInfo map[discover.NodeID]common.Address) []*peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	list := make([]*peer, 0, len(ps.peers))
	for _, p := range ps.peers {
		if _, ok := nodesInfo[p.ID()]; ok {
			list = append(list, p)
		}
	}

	return list
}

// PeersWithoutTx retrieves a list of peers that do not have a given transaction
// in their set of known hashes.
func (ps *peerSet) PeersWithoutTx(hash common.Hash) []*peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	list := make([]*peer, 0, len(ps.peers))
	for _, p := range ps.peers {
		if !p.knownTxs.Has(hash) {
			list = append(list, p)
		}
	}
	return list
}

// PeersWithoutShardingTxBatch retrieves a list of peers that do not have a given sharding transactions
// in their set of known transaction root hashes of a mined block.
func (ps *peerSet) PeersWithoutShardingTxBatch(hash common.Hash) []*peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	list := make([]*peer, 0, len(ps.peers))
	for _, p := range ps.peers {
		if !p.knownShardingTxBatches.Has(hash) {
			list = append(list, p)
		}
	}
	return list
}

func (ps *peerSet) getSameShardingPeers(currentShard, totalShard uint16) []*peer {
	peers := make([]*peer, 0)
	for _, p := range ps.peers {
		real := p.GetRealShard()
		if (real % totalShard) == currentShard {
			peers = append(peers, p)
		}
	}
	return peers
}

func (ps *peerSet) GetInOutShardingPeers(shardingID uint16, lampHeight uint64) ([]*peer, []*peer) {
	totalShard := common.GetTotalSharding(lampHeight)
	insideShardingPeers := make([]*peer, 0)
	outsideShardingPeers := make([]*peer, 0)

	for _, p := range ps.peers {
		real := p.GetRealShard()
		if (real % totalShard) == shardingID {
			insideShardingPeers = append(insideShardingPeers, p)
		} else {
			outsideShardingPeers = append(outsideShardingPeers, p)
		}
	}

	return insideShardingPeers, outsideShardingPeers
}

// PeersWithoutVerifiedHeader retrieves a list of peers that do not have a given header verified by a police officer
// in their set of known verified header.
func (ps *peerSet) PeersWithoutVerifiedHeader(sig string, hash *common.Hash) []*peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	list := make([]*peer, 0, len(ps.peers))
	for _, p := range ps.peers {
		headerHash, ok := p.knownVerifiedHeaders[sig]
		if !ok || !headerHash.Equal(*hash) {
			list = append(list, p)
		}
	}
	return list
}

// jh: 从本地发现的所有 peer 中挑选一个td最大的（进行同步）
// zzr: 从本地发现的所有 peer 中挑选一个高度最高的（进行同步）
// BestPeer retrieves the known peer with the currently highest total difficulty.
func (ps *peerSet) BestPeer(realShard, totalShard uint16, ethHeight, eleHeight uint64) *peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	// First find the same sharding peers
	bestPeers := ps.getSameShardingPeers(realShard%totalShard, totalShard)
	p := selectPeer(bestPeers, ethHeight, eleHeight)
	if p != nil {
		//log.Info("Same sharding peer")
		return p
	}

	// Second find brother sharding peers
	curentShard := common.GetSharding2(realShard, totalShard)
	brother := common.GetBrotherSharding(curentShard, totalShard)
	if brother == -1 {
		return nil
	}
	bestPeers = ps.getSameShardingPeers(uint16(brother), totalShard)
	return selectPeer(bestPeers, ethHeight, eleHeight)
}

func selectPeer(peers []*peer, ethHeight, eleHeight uint64) *peer {
	var bestPeer *peer
	var highestLamp int64
	var highestEle int64
	for _, p := range peers {
		_, heightEle, _, _, _ := p.Head()
		heightLamp := p.GetLampHeight()
		if uint64(heightLamp) != ethHeight || uint64(heightEle) <= eleHeight {
			continue
		}
		if bestPeer == nil {
			bestPeer, highestLamp, highestEle = p, heightLamp, heightEle
		} else if highestLamp < heightLamp {
			bestPeer, highestLamp, highestEle = p, heightLamp, heightEle
		} else if highestEle < heightEle {
			bestPeer, highestLamp, highestEle = p, heightLamp, heightEle
		}
	}

	return bestPeer
}

func (ps *peerSet) Peers() []*peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	peers := make([]*peer, 0, len(ps.peers))
	for _, p := range ps.peers {
		peers = append(peers, p)
	}
	return peers
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

func (ps *peerSet) PeersWithoutChunkProvider(chunkId uint64) []*peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	list := make([]*peer, 0, len(ps.peers))
	for i, p := range ps.peers {
		if !p.knownChunkProvider.Has(chunkId) {
			list = append(list, p)
		}
		log.Info("range peers", "i", i, "p", p)
	}
	return list
}

func (p *peer) GetChunkProvider(chunkId uint64, queryId uint64) error {
	data := &storage.GetFarmerData{
		ChunkId: chunkId,
		QueryId: queryId,
	}
	return p2p.Send(p.rw, GetFarmersMsg, *data)
}

func (p *peer) SendFarmerMsg(data *storage.FarmerData) error {
	return p2p.Send(p.rw, FarmerMsg, data)
}

func (p *peer) SendDataHashMsg(data *storage.DataHashProto) error {
	return p2p.Send(p.rw, DataHashMsg, data)
}

// func (p *peer) SendWriteDataMsg(data *storage.WriteDataProto) error {
// 	return p2p.Send(p.rw, WriteDataMsg, data)
// }

func (p *peer) SendReadDataMsg(data *storage.ReadDataProto) error {
	return p2p.Send(p.rw, ReadDataMsg, data)
}

func (p *peer) SendBlizObjectMeta(data *storage.ObjectMetaData) error {
	return p2p.Send(p.rw, BlizObjectMetaMsg, data)
}

func (p *peer) SendBlizProtocol(msgcode uint64, params string) error {
	datas := strings.Split(params, storage.SplitStr)
	if len(datas) < 4 {
		return errParam
	}
	switch msgcode {
	case GetDataHashMsg:
		data := storage.GetDataHashProto{
			ChunkId: uint64(storage.AtoiExt(datas[0])),
			SliceId: uint32(storage.AtoiExt(datas[1])),
			Off:     uint32(storage.AtoiExt(datas[2])),
			Size:    uint32(storage.AtoiExt(datas[3])),
		}
		return p2p.Send(p.rw, msgcode, data)
	// case GetWriteDataMsg:
	// 	data := storage.GetWriteDataProto{
	// 		ChunkId: uint64(storage.AtoiExt(datas[0])),
	// 		SliceId: uint32(storage.AtoiExt(datas[1])),
	// 		Off:     uint32(storage.AtoiExt(datas[2])),
	// 		Size:    uint32(storage.AtoiExt(datas[3])),
	// 	}
	// 	return p2p.Send(p.rw, msgcode, data)
	case GetReadDataMsg:
		data := storage.GetReadDataProto{
			ChunkId: uint64(storage.AtoiExt(datas[0])),
			SliceId: uint32(storage.AtoiExt(datas[1])),
			Off:     uint32(storage.AtoiExt(datas[2])),
			Size:    uint32(storage.AtoiExt(datas[3])),
		}
		return p2p.Send(p.rw, msgcode, data)
	default:
		return errParam
	}
}

// func (p *peer) SendBlizReHandshakeMsg(data *storage.BlizReHandshakeProto) error {
// 	return p2p.Send(p.rw, BlizReHandshakeMsg, data)
// }
