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
	"fmt"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/contatract/go-contatract/common"
	types "github.com/contatract/go-contatract/core/types_elephant"
	downloader "github.com/contatract/go-contatract/elephant/downloader"
	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/p2p/discover"
)

const (
	forceSyncCycle      = 5 * time.Second // Time interval to force syncs, even if few peers are available
	minDesiredPeerCount = 5               // Amount of peers desired to start syncing

	// This is the target size for the packs of transactions sent by txsyncLoop.
	// A pack can get larger than this if a single transactions exceeds this size.
	txsyncPackSize = 100 * 1024
)

type txsync struct {
	p   *peer
	txs []*types.Transaction
}

// JiangHan：将我们本地 pending 的所有交易发给 peer
// syncTransactions starts sending all currently pending transactions to the given peer.
func (pm *ProtocolManager) syncTransactions(p *peer) {
	var txs types.Transactions
	pending, _ := pm.txpool.Pending()
	for _, batch := range pending {
		txs = append(txs, batch...)
	}
	if len(txs) == 0 {
		return
	}
	select {
	case pm.txsyncCh <- &txsync{p, txs}:
	case <-pm.quitSync:
	}
}

// txsyncLoop takes care of the initial transaction sync for each new
// connection. When a new peer appears, we relay all currently pending
// transactions. In order to minimise egress bandwidth usage, we send
// the transactions in small packs to one peer at a time.
func (pm *ProtocolManager) txsyncLoop() {
	var (
		pending = make(map[discover.NodeID]*txsync)
		sending = false               // whether a send is active
		pack    = new(txsync)         // the pack that is being sent
		done    = make(chan error, 1) // result of the send
	)

	// send starts a sending a pack of transactions from the sync.
	send := func(s *txsync) {
		// Fill pack with transactions up to the target size.
		size := common.StorageSize(0)
		pack.p = s.p
		pack.txs = pack.txs[:0]
		for i := 0; i < len(s.txs) && size < txsyncPackSize; i++ {
			pack.txs = append(pack.txs, s.txs[i])
			size += s.txs[i].Size()
		}
		// Remove the transactions that will be sent.
		s.txs = s.txs[:copy(s.txs, s.txs[len(pack.txs):])]
		if len(s.txs) == 0 {
			delete(pending, s.p.ID())
		}
		// Send the pack in the background.
		s.p.Log().Trace("Sending batch of transactions", "count", len(pack.txs), "bytes", size)
		sending = true
		go func() { done <- pack.p.SendTransactions(pack.txs) }()
	}

	// pick chooses the next pending sync.
	pick := func() *txsync {
		if len(pending) == 0 {
			return nil
		}
		n := rand.Intn(len(pending)) + 1
		for _, s := range pending {
			if n--; n == 0 {
				return s
			}
		}
		return nil
	}

	for {
		select {
		case s := <-pm.txsyncCh:
			pending[s.p.ID()] = s
			if !sending {
				send(s)
			}
		case err := <-done:
			sending = false
			// Stop tracking peers that cause send failures.
			if err != nil {
				pack.p.Log().Debug("Transaction send failed", "err", err)
				delete(pending, pack.p.ID())
			}
			// Schedule the next send.
			if s := pick(); s != nil {
				send(s)
			}
		case <-pm.quitSync:
			return
		}
	}
}

// syncer is responsible for periodically synchronising with the network, both
// downloading hashes and blocks as well as handling the announcement handler.
func (pm *ProtocolManager) syncer() {
	// Start and ensure cleanup of sync mechanisms
	pm.fetcher.Start(pm.lamp)
	defer pm.fetcher.Stop()
	defer pm.downloader.Terminate()

	// Wait for different events to fire synchronisation operations
	forceSync := time.NewTicker(forceSyncCycle)
	defer forceSync.Stop()

	for {
		select {
		case <-pm.newPeerCh:
			// Make sure we have peers to select from, then sync
			if pm.peers.Len() < minDesiredPeerCount {
				break
			}
			peer := pm.selectPeer()
			go pm.synchronise(peer)

		case <-forceSync.C:
			// Force a sync even if not enough peers are present
			peer := pm.selectPeer()
			go pm.synchronise(peer)

		case <-pm.noMorePeers:
			return
		}
	}
}

// JiangHan: (下载)同步入口，必须保证peer的td比我们本地大，不然就没有意义了
// synchronise tries to sync up our local block chain with a remote peer.
func (pm *ProtocolManager) synchronise(peer *peer) {
	// Short circuit if no peers are available
	if peer == nil {
		return
	}

	err := pm.checkSync(peer)
	if err != nil {
		// log.Info("Synchronise fail", "err", err)
		return
	}
	pHead, pHeight, _, _, _ := peer.Head()

	// not genesis block
	currentBlock := pm.blockchain.CurrentBlock()
	//if currentBlock.NumberU64() != 0 {
	//	eleSealers := pm.lamp.GetNewestEleSealers()
	//	log.Info("synchronise", "last eleSealers len", len(eleSealers))
	//	ret := currentBlock.CheckBFTStages()
	//	if !ret {
	//		return
	//	}
	//}

	// Otherwise try to sync with the downloader
	mode := downloader.FullSync
	if atomic.LoadUint32(&pm.fastSync) == 1 {
		// Fast sync was explicitly requested, and explicitly granted
		mode = downloader.FastSync
	} else if currentBlock.NumberU64() == 0 && pm.blockchain.CurrentFastBlock().NumberU64() > 0 {
		// The database seems empty as the current block is the genesis. Yet the fast
		// block is ahead, so fast sync was enabled for this node at a certain point.
		// The only scenario where this can happen is if the user manually (or via a
		// bad block) rolled back a fast sync node below the sync point. In this case
		// however it's safe to reenable fast sync.
		atomic.StoreUint32(&pm.fastSync, 1)
		mode = downloader.FastSync
	}

	// Run the sync cycle, and disable fast sync if we've went past the pivot block
	//if err := pm.downloader.Synchronise(peer.id, pHead, pTd, mode); err != nil {
	if err := pm.downloader.Synchronise(peer.id, pHead, pHeight, mode); err != nil {
		return
	}
	if atomic.LoadUint32(&pm.fastSync) == 1 {
		log.Info("Fast sync complete, auto disabling")
		atomic.StoreUint32(&pm.fastSync, 0)
	}
	atomic.StoreUint32(&pm.acceptTxs, 1) // Mark initial sync done
	if head := pm.blockchain.CurrentBlock(); head.NumberU64() > 0 {
		// We've completed a sync cycle, notify all peers of new state. This path is
		// essential in star-topology networks where a gateway node needs to notify
		// all its out-of-date peers of the availability of a new block. This failure
		// scenario will most often crop up in private and hackathon networks with
		// degenerate connectivity, but it should be healthy for the mainnet too to
		// more reliably update peers or the local TD state.
		go pm.BroadcastBlock(head, false, false)
	}
}

// selectPeer returns the peer from which we can synchronise
func (pm *ProtocolManager) selectPeer() *peer {
	peers := pm.peers.Peers()
	for _, p := range peers {
		err := pm.checkSync(p)
		if err == nil {
			return p
		}
	}
	return nil
}

// checkSync returns whether we can synchronise from the peer.
// We will recheck when downloading
func (pm *ProtocolManager) checkSync(p *peer) error {
	// Check lamp height
	pLamp := p.GetLampHeight()
	lLamp := int64(pm.lamp.CurrBlockNum())
	if pLamp != lLamp {
		return fmt.Errorf("checkSync failed because of lamp height, peer: %d, local: %d", pLamp, lLamp)
	}
	_, pHeight, pLampHeight, _, pLampTD := p.Head()
	currentBlock := pm.blockchain.CurrentBlock()
	lHeight := int64(currentBlock.NumberU64())
	// The elepant height of peer is more than local's
	if pHeight > lHeight {
		lampHash := currentBlock.LampHash()
		lampHeight := currentBlock.LampBaseNumberU64()
		// Check local's lamp td
		ethTD := pm.lamp.BlockChain().GetTd(lampHash, lampHeight)
		if ethTD == nil {
			return nil
		} else if pLampTD.Cmp(ethTD) >= 0 {
			// Check Sharding
			pRealShard := p.GetRealShard()
			lRealShard := common.GetRealSharding(pm.elephant.etherbase)
			maxLamp := common.GetEleShardHeight(pRealShard, lRealShard)
			if uint64(pLampHeight) > maxLamp && lampHeight > maxLamp {
				return fmt.Errorf("Fail because of different sharding")
			}
			return nil
		}
		return fmt.Errorf("Fail because of lamp td")
	} else if pHeight == 0 {
		return fmt.Errorf("Fail because of peer elephant height is 0")
	} else {
		// Check local's elephant block
		block := pm.blockchain.GetBlockByNumber(uint64(pHeight))
		if block == nil {
			log.Info("Elephant block not exist", "height", pHeight)
			return nil
		}
		lampHash := block.LampHash()
		lampHeight := block.LampBaseNumberU64()
		// Check local's lamp block
		ethBlock := pm.lamp.BlockChain().GetBlock(lampHash, lampHeight)
		if ethBlock == nil {
			log.Info("Eth block not exist", "height", lampHeight)
			return nil
		}
		return fmt.Errorf("Fail because of elephant height, peer: %d, local: %d", pHeight, lHeight)
	}
}
