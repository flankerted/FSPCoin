// Copyright 2014 The go-contatract Authors
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

package core_eth

import (
	"errors"
	"sync"
	"time"

	"github.com/contatract/go-contatract/baseparam"
	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/consensus"
	"github.com/contatract/go-contatract/core/types"
	"github.com/contatract/go-contatract/crypto"
	"github.com/contatract/go-contatract/crypto/sha3"
	"github.com/contatract/go-contatract/event"
	"github.com/contatract/go-contatract/rlp"
)

var (
	halfPoliceCnt        = 1 // baseparam.MaxElectionPoolLen / 2
	ignoreGap     uint64 = 5
)

// Backend wraps all methods required for verifying a police officer.
type Backend interface {
	GetPoliceByHash(lampBlockHash *common.Hash) []common.AddressNode
	GetNextPolice() []common.AddressNode
	GetLastPolice() []common.AddressNode
	GetNewestValidHeaders() map[uint16]*types.VerifiedValid
}

type VerifiedHeaderPool struct {
	//config VerifHeadPoolCfg
	chain   blockChain
	backend Backend
	//gasPrice     *big.Int
	electionFeed event.Feed
	scope        event.SubscriptionScope
	chainHeadCh  chan ChainHeadEvent
	chainHeadSub event.Subscription
	//signer       types.Signer
	mu sync.RWMutex

	pending map[uint16]map[common.Hash][]*consensus.PoliceVerifiedHeader // All currently valid header without over half confirmations
	valid   map[uint16]map[common.Hash][]*consensus.PoliceVerifiedHeader // All valid header with over half confirmations

	newest map[uint16]uint64 // The newest header of a sharding

	wg sync.WaitGroup // for shutdown sync
}

func NewVerifiedHeaderPool(chain blockChain, eth Backend) *VerifiedHeaderPool {
	if chain == nil || eth == nil {
		return nil
	}

	pool := &VerifiedHeaderPool{
		chain:       chain,
		backend:     eth,
		pending:     make(map[uint16]map[common.Hash][]*consensus.PoliceVerifiedHeader),
		valid:       make(map[uint16]map[common.Hash][]*consensus.PoliceVerifiedHeader),
		newest:      make(map[uint16]uint64),
		chainHeadCh: make(chan ChainHeadEvent, chainHeadChanSize),
	}

	// Subscribe events from blockchain
	pool.chainHeadSub = pool.chain.SubscribeChainHeadEvent(pool.chainHeadCh)

	// Start the event loop and return
	pool.wg.Add(1)
	go pool.loop()

	return pool
}

func (pool *VerifiedHeaderPool) loop() {
	defer pool.wg.Done()

	//// Start the stats reporting and election eviction tickers
	//var prevPending, prevQueued int

	report := time.NewTicker(statsReportInterval)
	defer report.Stop()

	// Track the previous head headers for election reorgs
	head := pool.chain.CurrentBlock()

	// Keep waiting for and reacting to the various events
	for {
		select {
		// Handle ChainHeadEvent
		// 新区块头通知
		case ev := <-pool.chainHeadCh:
			if ev.Block != nil {
				pool.mu.Lock()

				// 用新区块头替换旧区块头，需要重整 pool
				pool.reset(head.Header(), ev.Block.Header())
				head = ev.Block

				pool.mu.Unlock()
			}
		// Be unsubscribed due to system stopped
		case <-pool.chainHeadSub.Err():
			return
		}
	}
}

func (pool *VerifiedHeaderPool) Clear() {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pool.pending = make(map[uint16]map[common.Hash][]*consensus.PoliceVerifiedHeader)
	pool.valid = make(map[uint16]map[common.Hash][]*consensus.PoliceVerifiedHeader)
	pool.newest = make(map[uint16]uint64)
}

func (pool *VerifiedHeaderPool) ClearSharding(shardingID uint16) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pool.pending[shardingID] = make(map[common.Hash][]*consensus.PoliceVerifiedHeader)
	pool.valid[shardingID] = make(map[common.Hash][]*consensus.PoliceVerifiedHeader)
	pool.newest = make(map[uint16]uint64)
}

func (pool *VerifiedHeaderPool) reset(oldHead, newHead *types.Header) {
	// Initialize the internal state to the current head
	//if newHead.IsElectionBlock() {
	//	pool.Clear()
	//}
	//statedb, err := pool.chain.StateAt(newHead.Root)
	//if err != nil {
	//	log.Error("Failed to reset election pool state", "err", err)
	//	return
	//}
	//pool.currentState = statedb
}

func (pool *VerifiedHeaderPool) Stop() {
	// Unsubscribe all subscriptions registered from electionpool
	pool.scope.Close()

	// Unsubscribe subscriptions registered from blockchain
	pool.chainHeadSub.Unsubscribe()
	pool.wg.Wait()
}

func (pool *VerifiedHeaderPool) AddVerified(verified *consensus.PoliceVerifiedHeader) error {
	// Discard the too old one
	if newestHeight, ok := pool.newest[verified.ShardingID]; ok {
		if newestHeight > ignoreGap && verified.BlockNum <= newestHeight-ignoreGap {
			return nil
		}
	}

	signature := common.FromHex(verified.Sign)
	dataHash := rlpHash([]interface{}{verified.ShardingID, verified.LampBase, verified.Header, verified.BlockNum, verified.Previous}).Bytes()
	rpk, err := crypto.Ecrecover(dataHash, signature)
	if err != nil {
		return err
	}
	pubKey := crypto.ToECDSAPub(rpk)
	recoveredAddr := crypto.PubkeyToAddress(*pubKey)
	police := pool.backend.GetPoliceByHash(verified.LampBase)

	pool.mu.Lock()
	defer pool.mu.Unlock()

	for _, officer := range police {
		if recoveredAddr.Equal(officer.Address) {
			sID := verified.ShardingID
			dataHash := verified.Hash()
			if pool.pending[sID] == nil {
				pool.pending[sID] = make(map[common.Hash][]*consensus.PoliceVerifiedHeader, 0)
			}
			if pool.pending[sID][dataHash] == nil {
				pool.pending[sID][dataHash] = make([]*consensus.PoliceVerifiedHeader, 0)
			}
			pool.pending[sID][dataHash] = append(pool.pending[sID][dataHash], verified)

			if len(pool.pending[sID][dataHash]) > halfPoliceCnt {
				if pool.valid[sID] == nil {
					pool.valid[sID] = make(map[common.Hash][]*consensus.PoliceVerifiedHeader, 0)
				}
				if pool.valid[sID][dataHash] == nil {
					pool.valid[sID][dataHash] = make([]*consensus.PoliceVerifiedHeader, 0)
					pool.valid[sID][dataHash] = append(pool.valid[sID][dataHash], pool.pending[sID][dataHash]...)
					pool.newest[verified.ShardingID] = verified.BlockNum
					return nil
				}
				pool.valid[sID][dataHash] = append(pool.valid[sID][dataHash], verified)
				pool.newest[verified.ShardingID] = verified.BlockNum
			}
			return nil
		}
	}

	return errors.New("invalid police officer")
}

func rlpHash(x interface{}) (h common.Hash) {
	hw := sha3.NewKeccak256()
	rlp.Encode(hw, x)
	hw.Sum(h[:0])
	return h
}

func (pool *VerifiedHeaderPool) Verified(lampHeight uint64) ([]*types.VerifiedValid, []bool, error) {
	useLast := make([]bool, 0)
	verifieds := make([]*types.VerifiedValid, 0)
	totalShard := common.GetTotalSharding(lampHeight)
	lastTotalShard := common.GetTotalSharding(lampHeight - baseparam.ElectionBlockCount)

	pool.mu.Lock()
	defer pool.mu.Unlock()

shardingID:
	for id := uint16(0); id < totalShard; id++ {
		useLast = append(useLast, false)
		height, ok := pool.newest[id]
		if ok {
			newestValidHeights := pool.backend.GetNewestValidHeaders()
			if newestValidHeights != nil && newestValidHeights[id] != nil {
				last := true
				for _, values := range pool.valid[id] {
					if newestValidHeights[id].BlockHeight < pool.newest[id] && len(values) > halfPoliceCnt {
						last = false
						break
					}
				}
				if last {
					if lastTotalShard < totalShard {
						height = newestValidHeights[uint16(common.GetParentSharding(id, totalShard))].BlockHeight
					} else {
						height = newestValidHeights[id].BlockHeight
					}
					useLast[id] = true
				}
			}
		} else {
			newestValidHeights := pool.backend.GetNewestValidHeaders()
			if newestValidHeights != nil && newestValidHeights[id] != nil {
				if lastTotalShard < totalShard {
					height = newestValidHeights[uint16(common.GetParentSharding(id, totalShard))].BlockHeight
				} else {
					height = newestValidHeights[id].BlockHeight
				}
				useLast[id] = true
			} else {
				emptyHash := common.Hash{}
				nilVal := &types.VerifiedValid{ShardingID: id, BlockHeight: 0, LampBase: &emptyHash, Header: &emptyHash}
				verifieds = append(verifieds, nilVal)
				continue shardingID
			}
		}

		if !useLast[id] {
			values, ok := pool.valid[id]
			if !ok || len(values) == 0 {
				return nil, nil, errors.New("verifiedHeaderPool must have valid sharding info, but there is none")
			}

			heightest := uint64(0)
			heightestHash := common.Hash{}
			for hash, values := range pool.valid[id] {
				if values[0].BlockNum > heightest && len(values) > halfPoliceCnt {
					heightest = values[0].BlockNum
					heightestHash = hash
				}
			}
			if heightest != height {
				return nil, nil, errors.New("the var heightest must be equal to var height in verifiedHeaderPool, but it is not")
			}

			verified := &types.VerifiedValid{
				ShardingID:      id,
				BlockHeight:     height,
				LampBase:        pool.valid[id][heightestHash][0].LampBase,
				Header:          pool.valid[id][heightestHash][0].Header,
				PreviousHeaders: pool.valid[id][heightestHash][0].Previous,
				Signs:           make([]string, 0),
			}
			for _, val := range pool.valid[id][heightestHash] {
				verified.Signs = append(verified.Signs, val.Sign)
			}
			if len(verified.Signs) > halfPoliceCnt {
				verifieds = append(verifieds, verified)
			} else {
				return nil, nil, errors.New("the var len(verified.Signs) must be more than var halfPoliceCnt in verifiedHeaderPool, but it is not")
			}
		} else {
			verifieds = append(verifieds, pool.backend.GetNewestValidHeaders()[id])
		}
	}

	return verifieds, useLast, nil
}
