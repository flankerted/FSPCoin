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
	"fmt"
	"io"
	"math/big"
	"os"
	"sync"
	"time"

	"github.com/contatract/go-contatract/baseparam"
	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/core/types"
	"github.com/contatract/go-contatract/core_eth/state"
	"github.com/contatract/go-contatract/event"
	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/rlp"
)

//var (
//	// Metrics for the pending pool
//	pendingElectDiscardCounter = metrics.NewCounter("elctionpool/pendingSealers/discard")
//
//	// Metrics for the queued pool
//	queuedElectDiscardCounter = metrics.NewCounter("elctionpool/queuedSealers/discard")
//)

// ElectionStatus is the current status of a transaction as seen by the pool.
type ElectionStatus uint

const (
	ElectionStatusUnknown ElectionStatus = iota
	ElectionStatusQueued
	ElectionStatusPending
	ElectionStatusIncluded
)

// ElectionPoolCfg are the configuration parameters of the election pool.
type ElectionPoolCfg struct {
	//NoLocals  bool          // Whether local election handling should be disabled
	SealersJournal string // Elephant sealers journal of local elections to survive node restarts
	PoliceJournal  string // Police journal of local elections to survive node restarts

	Rejournal time.Duration // Time interval to regenerate the local election journalSealers

	//PriceLimit uint64 // Minimum gas price to enforce for acceptance into the pool
	//PriceBump  uint64 // Minimum price bump percentage to replace an already existing election (nonce)

	GlobalSlots uint64 // Maximum number of executable election slots for all accounts
	GlobalQueue uint64 // Maximum number of non-executable election slots for all accounts

	//Lifetime time.Duration // Maximum amount of time non-executable election are queued
}

// DefaultElectionPoolCfg contains the default configurations for the election pool.
var DefaultElectionPoolCfg = ElectionPoolCfg{
	SealersJournal: "sealersElectionPool.rlp",
	PoliceJournal:  "policeElectionPool.rlp",
	Rejournal:      time.Hour,

	//PriceLimit: 1,
	//PriceBump:  10,

	GlobalSlots: 21,
	GlobalQueue: 42,

	//Lifetime: 3 * time.Hour,
}

// sanitize checks the provided user configurations and changes anything that's
// unreasonable or unworkable.
func (config *ElectionPoolCfg) sanitize() ElectionPoolCfg {
	conf := *config
	if conf.Rejournal < time.Second {
		log.Warn("Sanitizing invalid electionpool journalSealers time", "provided", conf.Rejournal, "updated", time.Second)
		conf.Rejournal = time.Second
	}
	//if conf.PriceLimit < 1 {
	//	log.Warn("Sanitizing invalid electionpool price limit", "provided", conf.PriceLimit, "updated", DefaultTxPoolConfig.PriceLimit)
	//	conf.PriceLimit = DefaultElectionPoolCfg.PriceLimit
	//}
	//if conf.PriceBump < 1 {
	//	log.Warn("Sanitizing invalid electionpool price bump", "provided", conf.PriceBump, "updated", DefaultTxPoolConfig.PriceBump)
	//	conf.PriceBump = DefaultElectionPoolCfg.PriceBump
	//}
	return conf
}

// errNoActiveJournal is returned if a election is attempted to be inserted
// into the journalSealers, but no such file is currently open.
var errNoActiveElectionJournal = errors.New("no active journalSealers")

// electionJournal is a rotating log of elections with the aim of storing locally
// created elections to allow non-executed ones to survive node restarts.
type electionJournal struct {
	path   string         // Filesystem path to store the elections at
	writer io.WriteCloser // Output stream to write new elections into
}

// newElectionJournal creates a new election journal
func newElectionJournal(path string) *electionJournal {
	return &electionJournal{
		path: path,
	}
}

// load parses a election journal dump from disk, loading its contents into
// the specified pool.
func (journal *electionJournal) load(add func(*types.AddressElection, uint64) error) error {
	// Skip the parsing if the journal file doens't exist at all
	if _, err := os.Stat(journal.path); os.IsNotExist(err) {
		return nil
	}
	// Open the journal for loading any past elections
	input, err := os.Open(journal.path)
	if err != nil {
		return err
	}
	defer input.Close()

	// Temporarily discard any journal additions (don't double add on load)
	journal.writer = new(devNull)
	defer func() { journal.writer = nil }()

	// Inject all elections from the journal into the pool
	stream := rlp.NewStream(input, 0)
	total, dropped := 0, 0

	var failure error = nil
	for {
		// Parse the next election and terminate on error
		elect := new(types.AddressElection)
		if err = stream.Decode(elect); err != nil {
			if err != io.EOF {
				failure = err
			}
			break
		}
		// Import the election and bump the appropriate progress counters
		total++
		if err = add(elect, 0); err != nil {
			log.Debug("Failed to add journaled election", "err", err)
			dropped++
			continue
		}
	}
	log.Info("Loaded local election journal", "elections", total, "dropped", dropped)

	return failure
}

// insert adds the specified election to the local disk journalSealers.
func (journal *electionJournal) insert(elect *types.AddressElection) error {
	if journal.writer == nil {
		return errNoActiveJournal
	}
	if err := rlp.Encode(journal.writer, elect); err != nil {
		return err
	}
	return nil
}

// rotate regenerates the election journal based on the current contents of
// the election pool.
func (journal *electionJournal) rotate(pending map[common.Address]types.AddressElection) error {
	// Close the current journal (if any is open)
	if journal.writer != nil {
		if err := journal.writer.Close(); err != nil {
			return err
		}
		journal.writer = nil
	}
	// Generate a new journal with the contents of the current pool
	replacement, err := os.OpenFile(journal.path+".new", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	journaled := 0
	for _, elect := range pending {
		if err = rlp.Encode(replacement, elect); err != nil {
			replacement.Close()
			return err
		}
		journaled += 1
	}
	replacement.Close()

	// Replace the live journal with the newly generated one
	if err = os.Rename(journal.path+".new", journal.path); err != nil {
		return err
	}
	sink, err := os.OpenFile(journal.path, os.O_WRONLY|os.O_APPEND, 0755)
	if err != nil {
		return err
	}
	journal.writer = sink
	log.Info("Regenerated local election journal", "elections", journaled, "accounts", len(pending))

	return nil
}

// close flushes the election journal contents to disk and closes the file.
func (journal *electionJournal) close() error {
	var err error

	if journal.writer != nil {
		err = journal.writer.Close()
		journal.writer = nil
	}
	return err
}

// ElectionPool contains all currently known elections. elections
// enter the pool when they are received from the network or submitted
// locally. They exit the pool when they are included in the blockchain.
//
// The pool separates processable elections (which can be applied to the
// current state) and future elections. elections move between those
// two states over time as they are received and processed.
type ElectionPool struct {
	config ElectionPoolCfg
	//chainconfig  *params.ChainConfig
	chain blockChain
	//gasPrice     *big.Int
	electionFeed event.Feed
	scope        event.SubscriptionScope
	chainHeadCh  chan ChainHeadEvent
	chainHeadSub event.Subscription
	//signer       types.Signer
	mu sync.RWMutex

	currentState *state.StateDB // Current state in the blockchain head
	//pendingState  *state.ManagedState // Pending state tracking virtual nonces
	//currentMaxGas uint64              // Current gas limit for election caps

	//locals  *accountSet // Set of local election to exempt from eviction rules
	journalSealers *electionJournal // Journal of local elephant sealer elections to back up to disk
	journalPolice  *electionJournal // Journal of local police elections to back up to disk

	pendingSealers map[common.Address]*types.AddressElection // All currently valid elected address for sealers
	queueSealers   map[common.Address]*types.AddressElection // Queued but non-processable elections for sealers
	pendingPolice  map[common.Address]*types.AddressElection // All currently valid elected address for police
	queuePolice    map[common.Address]*types.AddressElection // Queued but non-processable elections for police
	//beats   map[common.Address]time.Time            // Last heartbeat from each known account
	all map[common.Hash]*types.AddressElection // All elected address to allow lookups
	//priced  *txPricedList                           // All elections sorted by price

	wg sync.WaitGroup // for shutdown sync

	//homestead bool
}

// ElectionPool creates a new election pool to gather, sort and filter inbound
// elections from the network.
func NewElectionPool(config ElectionPoolCfg, chain blockChain) *ElectionPool {
	// Sanitize the input to ensure no vulnerable gas prices are set
	config = (&config).sanitize()

	// Create the election pool with its initial settings
	pool := &ElectionPool{
		config: config,
		//chainconfig: chainconfig,
		chain: chain,
		//signer:      types.NewEIP155Signer(chainconfig.ChainId),
		pendingSealers: make(map[common.Address]*types.AddressElection),
		queueSealers:   make(map[common.Address]*types.AddressElection),
		pendingPolice:  make(map[common.Address]*types.AddressElection),
		queuePolice:    make(map[common.Address]*types.AddressElection),
		//beats:       make(map[common.Address]time.Time),
		all:         make(map[common.Hash]*types.AddressElection),
		chainHeadCh: make(chan ChainHeadEvent, chainHeadChanSize),
		//gasPrice:    new(big.Int).SetUint64(config.PriceLimit),
	}
	//pool.locals = newAccountSet(pool.signer)
	//pool.priced = newTxPricedList(&pool.all)
	currentBlock := chain.CurrentBlock()
	if currentBlock == nil {
		pool.reset(nil, nil)
	} else {
		pool.reset(nil, currentBlock.Header())
	}

	// If journaling is enabled, load from disk
	if config.SealersJournal != "" {
		pool.journalSealers = newElectionJournal(config.SealersJournal)
		if err := pool.journalSealers.load(pool.Add); err != nil {
			log.Warn("Failed to load election journalSealers", "err", err)
		}
		if err := pool.journalSealers.rotate(pool.storedSealers()); err != nil {
			log.Warn("Failed to rotate election journalSealers for sealers", "err", err)
		}
	}
	if config.PoliceJournal != "" {
		pool.journalPolice = newElectionJournal(config.PoliceJournal)
		if err := pool.journalPolice.load(pool.Add); err != nil {
			log.Warn("Failed to load election journalPolice", "err", err)
		}
		if err := pool.journalPolice.rotate(pool.storedPolice()); err != nil {
			log.Warn("Failed to rotate election journalPolice for police", "err", err)
		}
	}

	// Subscribe events from blockchain
	pool.chainHeadSub = pool.chain.SubscribeChainHeadEvent(pool.chainHeadCh)

	// Start the event loop and return
	pool.wg.Add(1)
	go pool.loop()

	return pool
}

// loop is the election pool's main event loop, waiting for and reacting to
// outside blockchain events as well as for various reporting and election
// eviction events.
func (pool *ElectionPool) loop() {
	defer pool.wg.Done()

	// Start the stats reporting and election eviction tickers
	var prevPending, prevQueued int

	report := time.NewTicker(statsReportInterval)
	defer report.Stop()

	//evict := time.NewTicker(evictionInterval)
	//defer evict.Stop()

	journal := time.NewTicker(pool.config.Rejournal)
	defer journal.Stop()

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

		// Handle stats reporting ticks
		case <-report.C:
			pool.mu.RLock()
			pending, queued := pool.statsSealers()
			//stales := pool.priced.stales
			pool.mu.RUnlock()

			if pending != prevPending || queued != prevQueued {
				log.Debug("election pool status report", "executable", pending, "queued", queued)
				prevPending, prevQueued = pending, queued
			}

		// Handle local election journalSealers rotation
		case <-journal.C:
			if pool.journalSealers != nil {
				pool.mu.Lock()
				if err := pool.journalSealers.rotate(pool.storedSealers()); err != nil {
					log.Warn("Failed to rotate sealers election journalSealers", "err", err)
				}
				pool.mu.Unlock()
			}
			if pool.journalPolice != nil {
				pool.mu.Lock()
				if err := pool.journalPolice.rotate(pool.storedPolice()); err != nil {
					log.Warn("Failed to rotate police election journalSealers", "err", err)
				}
				pool.mu.Unlock()
			}
		}
	}
}

// JiangHan：线程安全锁下的 reset 操作
// lockedReset is a wrapper around reset to allow calling it in a thread safe
// manner. This method is only ever used in the tester!
func (pool *ElectionPool) lockedReset(oldHead, newHead *types.Header) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pool.reset(oldHead, newHead)
}

// Clear clears all elections in the election pool
func (pool *ElectionPool) Clear() {
	pool.pendingSealers = make(map[common.Address]*types.AddressElection)
	pool.queueSealers = make(map[common.Address]*types.AddressElection)
	pool.pendingPolice = make(map[common.Address]*types.AddressElection)
	pool.queuePolice = make(map[common.Address]*types.AddressElection)
	pool.all = make(map[common.Hash]*types.AddressElection)
}

// reset retrieves the current state of the blockchain and ensures the content
// of the election pool is valid with regard to the chain state.
func (pool *ElectionPool) reset(oldHead, newHead *types.Header) {
	// If we're reorging an old state, reinject all dropped elections
	//var reinject types.AddressElection

	if newHead.IsElectionBlock() {
		pool.Clear()
	}

	//if oldHead != nil {
	//	newNum := big.NewInt(0)
	//	if newNum.Sub(newHead.Number, oldHead.Number).Int64() > baseparam.ElectionBlockCount ||
	//		newHead.IsElectionBlock() {
	//		pool.Clear()
	//	}
	//}

	//if oldHead != nil && oldHead.Hash() != newHead.ParentHash {
	//	// jh: 如果 oldHeader 并不是 newHeaer 的 father 节点，那么我们需要看看是不是相差太远
	//	// 取两者的 number 进行比较
	//	// If the reorg is too deep, avoid doing it (will happen during fast sync)
	//	oldNum := oldHead.Number.Uint64()
	//	newNum := newHead.Number.Uint64()
	//
	//	if depth := uint64(math.Abs(float64(oldNum) - float64(newNum))); depth > 64 {
	//		// 相差64层。。。太远了
	//		log.Debug("Skipping deep election reorg", "depth", depth)
	//	} else {
	//		// 相差的深度还比较浅，可以考虑将相差的所有交易都拉入内存
	//		// Reorg seems shallow enough to pull in all elections into memory
	//		var discarded, included types.AddressElection
	//
	//		var (
	//			rem = pool.chain.GetBlock(oldHead.Hash(), oldHead.Number.Uint64())
	//			add = pool.chain.GetBlock(newHead.Hash(), newHead.Number.Uint64())
	//		)
	//
	//		// 如果新header number<旧header number, 就将旧的超出新的部分都加入到 discarded
	//		// 列表，全部丢弃
	//		for rem.NumberU64() > add.NumberU64() {
	//			discarded = append(discarded, rem.Transactions()...)
	//			if rem = pool.chain.GetBlock(rem.ParentHash(), rem.NumberU64()-1); rem == nil {
	//				log.Error("Unrooted old chain seen by tx pool", "block", oldHead.Number, "hash", oldHead.Hash())
	//				return
	//			}
	//		}
	//
	//		// 如果旧header number<新header number, 就将新加入的部分都加入到 included 列表
	//		// 全部加进来
	//		for add.NumberU64() > rem.NumberU64() {
	//			included = append(included, add.Transactions()...)
	//			if add = pool.chain.GetBlock(add.ParentHash(), add.NumberU64()-1); add == nil {
	//				log.Error("Unrooted new chain seen by tx pool", "block", newHead.Number, "hash", newHead.Hash())
	//				return
	//			}
	//		}
	//
	//		// 最后处理新旧两者本身，将旧的加入discarded，干掉； 将新的加入included, 加入
	//		for rem.Hash() != add.Hash() {
	//			discarded = append(discarded, rem.Transactions()...)
	//			if rem = pool.chain.GetBlock(rem.ParentHash(), rem.NumberU64()-1); rem == nil {
	//				log.Error("Unrooted old chain seen by tx pool", "block", oldHead.Number, "hash", oldHead.Hash())
	//				return
	//			}
	//			included = append(included, add.Transactions()...)
	//			if add = pool.chain.GetBlock(add.ParentHash(), add.NumberU64()-1); add == nil {
	//				log.Error("Unrooted new chain seen by tx pool", "block", newHead.Number, "hash", newHead.Hash())
	//				return
	//			}
	//		}
	//
	//		// 将included 中有可能跟 discarded 中重合的交易去除
	//		reinject = types.TxDifference(discarded, included)
	//	}
	//}

	// Initialize the internal state to the current head
	if newHead == nil {
		newHead = pool.chain.CurrentBlock().Header() // Special case during testing
	}
	statedb, err := pool.chain.StateAt(newHead.Root)
	if err != nil {
		log.Error("Failed to reset election pool state", "err", err)
		return
	}
	pool.currentState = statedb
	//pool.pendingState = state.ManageState(statedb)
	//pool.currentMaxGas = newHead.GasLimit

	//// Inject any elections discarded due to reorgs
	//log.Debug("Reinjecting stale elections", "count", len(reinject))
	//pool.addElectionsLocked(reinject, false)

	//// validate the pool of pending elections, this will remove
	//// any elections that have been included in the block or
	//// have been invalidated because of another election (e.g.
	//// higher gas price)
	//pool.demoteUnexecutables()

	//// Update all accounts to the latest known pending nonce
	//for addr, list := range pool.pending {
	//	txs := list.Flatten() // Heavy but will be cached and is needed by the miner anyway
	//	pool.pendingState.SetNonce(addr, txs[len(txs)-1].Nonce()+1)
	//}
	//// Check the queue and move elections over to the pending if possible
	//// or remove those that have become invalid
	//pool.promoteExecutables(nil)
}

// Stop terminates the election pool.
func (pool *ElectionPool) Stop() {
	// Unsubscribe all subscriptions registered from electionpool
	pool.scope.Close()

	// Unsubscribe subscriptions registered from blockchain
	pool.chainHeadSub.Unsubscribe()
	pool.wg.Wait()

	if pool.journalSealers != nil {
		pool.journalSealers.close()
	}
	if pool.journalPolice != nil {
		pool.journalPolice.close()
	}
	log.Info("election pool stopped")
}

// SubscribeElectionPreEvent registers a subscription of ElectionPreEvent and
// starts sending event to the given channel.
func (pool *ElectionPool) SubscribeElectionPreEvent(ch chan<- ElectionPreEvent) event.Subscription {
	return pool.scope.Track(pool.electionFeed.Subscribe(ch))
}

//// GasPrice returns the current gas price enforced by the election pool.
//func (pool *ElectionPool) GasPrice() *big.Int {
//	pool.mu.RLock()
//	defer pool.mu.RUnlock()
//
//	return new(big.Int).Set(pool.gasPrice)
//}

//// SetGasPrice updates the minimum price required by the election pool for a
//// new election, and drops all elections below this threshold.
//func (pool *ElectionPool) SetGasPrice(price *big.Int) {
//	pool.mu.Lock()
//	defer pool.mu.Unlock()
//
//	pool.gasPrice = price
//	for _, tx := range pool.priced.Cap(price, pool.locals) {
//		pool.removeElection(tx.Hash())
//	}
//	log.Info("election pool price threshold updated", "price", price)
//}

//// State returns the virtual managed state of the election pool.
//func (pool *ElectionPool) State() *state.ManagedState {
//	pool.mu.RLock()
//	defer pool.mu.RUnlock()
//
//	return pool.pendingState
//}

// Stats retrieves the current pool stats, namely the number of pending and the
// number of queued (non-executable) elections.
func (pool *ElectionPool) StatsSealers() (int, int) {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	return pool.statsSealers()
}

func (pool *ElectionPool) StatsPolice() (int, int) {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	return pool.statsPolice()
}

// stats retrieves the current pool stats, namely the number of pending and the
// number of queued (non-executable) elections.
func (pool *ElectionPool) statsSealers() (int, int) {
	return len(pool.pendingSealers), len(pool.queueSealers)
}

func (pool *ElectionPool) statsPolice() (int, int) {
	return len(pool.pendingPolice), len(pool.queuePolice)
}

// Content retrieves the data content of the election pool, returning all the
// pending as well as queued elections, grouped by account and sorted by nonce.
func (pool *ElectionPool) ContentSealers() (map[common.Address]types.AddressElection, map[common.Address]types.AddressElection) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pending := make(map[common.Address]types.AddressElection)
	for addr, elect := range pool.pendingSealers {
		pending[addr] = *elect
	}
	queued := make(map[common.Address]types.AddressElection)
	for addr, elect := range pool.queueSealers {
		queued[addr] = *elect
	}
	return pending, queued
}

func (pool *ElectionPool) ContentPolice() (map[common.Address]types.AddressElection, map[common.Address]types.AddressElection) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pending := make(map[common.Address]types.AddressElection)
	for addr, elect := range pool.pendingPolice {
		pending[addr] = *elect
	}
	queued := make(map[common.Address]types.AddressElection)
	for addr, elect := range pool.queuePolice {
		queued[addr] = *elect
	}
	return pending, queued
}

// stored retrieves all currently known pending elections, groupped by origin
// account and sorted by nonce. The returned election set is a copy and can be
// freely modified by calling code.
func (pool *ElectionPool) storedSealers() map[common.Address]types.AddressElection {
	pending := make(map[common.Address]types.AddressElection)
	for addr, elect := range pool.pendingSealers {
		pending[addr] = *elect
	}
	queued := make(map[common.Address]types.AddressElection)
	for addr, elect := range pool.queueSealers {
		queued[addr] = *elect
	}
	return pending
}

func (pool *ElectionPool) storedPolice() map[common.Address]types.AddressElection {
	pending := make(map[common.Address]types.AddressElection)
	for addr, elect := range pool.pendingPolice {
		pending[addr] = *elect
	}
	queued := make(map[common.Address]types.AddressElection)
	for addr, elect := range pool.queuePolice {
		queued[addr] = *elect
	}
	return pending
}

// Pending retrieves all currently processable elections, groupped by origin
// account and sorted by nonce. The returned election set is a copy and can be
// freely modified by calling code.
func (pool *ElectionPool) PendingSealers() map[common.Address]types.AddressElection {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pending := make(map[common.Address]types.AddressElection)
	for addr, elect := range pool.pendingSealers {
		pending[addr] = *elect
	}
	return pending
}

func (pool *ElectionPool) PendingPolice() map[common.Address]types.AddressElection {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pending := make(map[common.Address]types.AddressElection)
	for addr, elect := range pool.pendingPolice {
		pending[addr] = *elect
	}
	return pending
}

func (pool *ElectionPool) Pending(electionType uint8) (map[common.Address]types.AddressElection, error) {
	if electionType == baseparam.TypeSealersElection {
		return pool.PendingSealers(), nil
	} else if electionType == baseparam.TypePoliceElection {
		return pool.PendingPolice(), nil
	} else {
		return nil, errors.New("wrong type of election when getting pending elections")
	}
}

func (pool *ElectionPool) ElectionIsValid(elect *types.AddressElection, curBlockNum uint64) error {
	electNum := elect.ForBlockNum.Int64()
	lastElectionBlock := curBlockNum - curBlockNum%baseparam.ElectionBlockCount
	nextElectionBlock := lastElectionBlock + baseparam.ElectionBlockCount
	if electNum != int64(nextElectionBlock) {
		return fmt.Errorf("wrong next-election block number: %d, discard the election", electNum)
	}

	return nil
}

// add validates a election and inserts it into the non-executable queue for
// later pending promotion and execution. If the election is a replacement for
// an already pending or queued one, it overwrites the previous and returns this
// so outer code doesn't uselessly call promote.
func (pool *ElectionPool) add(elect *types.AddressElection, applyInBlock uint64) (bool, error) {
	// If the election is already known, discard it
	hash := elect.Hash()
	if pool.all[hash] != nil {
		log.Trace("Discarding already known election", "hash", hash)
		return true, nil
		//return false, fmt.Errorf("known election: %x", hash)
	}

	// Check if election block number is wrong
	//currentBlockNum := pool.chain.CurrentBlock().NumberU64()
	//if applyInBlock != 0 {
	//	currentBlockNum = applyInBlock
	//}
	//if err := pool.ElectionIsValid(elect, currentBlockNum); err != nil {
	//	return false, err
	//}

	//// Check balance
	//electionNeedBalance := getElectionReqBalance()
	//leftBalance := pool.currentState.GetBalance(*(elect.Address))
	//if leftBalance.Cmp(electionNeedBalance) < 0 {
	//	return false, fmt.Errorf("not enough balance to elect: %s, discard the election", elect.Address.String())
	//}

	// If the election pool is full, discard underpriced election
	if uint64(len(pool.all)) >= (pool.config.GlobalSlots + pool.config.GlobalQueue) {
		// If the new election is underpriced, don't accept it
		return false, errors.New("the election pool is full, discard the election")
	}

	// New election isn't replacing a pending one, push into queue
	replace := false
	if uint64(len(pool.all)) >= pool.config.GlobalSlots {
		return false, errors.New("the election pool is full, discard the election")
		//replace, err := pool.enqueueElection(hash, tx)
		//if err != nil {
		//	return false, err
		//}
	}

	// Mark these election addresses and journalSealers them
	pool.journalElection(elect)

	log.Trace("Pooled new future election", "hash", hash)
	return replace, nil
}

func getElectionReqBalance() *big.Int {
	return new(big.Int).SetUint64(baseparam.BalanceReqForElection)
}

// enqueueElection inserts a new election into the non-executable election queue.
//
// Note, this method assumes the pool lock is held!
func (pool *ElectionPool) enqueueElection(hash common.Hash, elect *types.AddressElection) (bool, error) {
	// Try to insert the election into the future queue
	from := *elect.Address // already validated
	if pool.queueSealers[from] == nil {
		pool.queueSealers[from] = elect
	} else {
		// The election was already in the pool, discard this
		queuedDiscardCounter.Inc(1)
		return false, errors.New("repeated election added to queue of the election pool")
	}

	pool.all[hash] = elect

	return true, nil
}

// journalElection adds the specified election to the local disk journalSealers
func (pool *ElectionPool) journalElection(elect *types.AddressElection) {
	// Only journalSealers if it's enabled
	if elect.ElectionType == baseparam.TypeSealersElection {
		if pool.journalSealers == nil {
			return
		}
		if err := pool.journalSealers.insert(elect); err != nil {
			log.Warn("Failed to journalSealers election", "err", err)
		}
	} else if elect.ElectionType == baseparam.TypePoliceElection {
		if pool.journalPolice == nil {
			return
		}
		if err := pool.journalPolice.insert(elect); err != nil {
			log.Warn("Failed to journalPolice election", "err", err)
		}
	}
}

// Note, this method assumes the pool lock is held!
func (pool *ElectionPool) promoteElection(addr common.Address, hash common.Hash, elect *types.AddressElection) {
	// Try to insert the election into the pending queue
	if pool.pendingSealers[addr] == nil && elect.ElectionType == baseparam.TypeSealersElection {
		pool.pendingSealers[addr] = elect
	} else if pool.pendingPolice[addr] == nil && elect.ElectionType == baseparam.TypePoliceElection {
		pool.pendingPolice[addr] = elect
	} else {
		//pendingElectDiscardCounter.Inc(1)
		return
	}

	if pool.all[hash] == nil {
		pool.all[hash] = elect
		//pool.priced.Put(elect)
	}

	go pool.electionFeed.Send(ElectionPreEvent{elect})
}

// Add enqueues a single election into the pool if it is valid
func (pool *ElectionPool) Add(elect *types.AddressElection, applyInBlock uint64) error {
	return pool.addElection(elect, applyInBlock)
}

// addElection enqueues a single elected address into the pool if it is valid.
func (pool *ElectionPool) addElection(elect *types.AddressElection, applyInBlock uint64) error {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	// Try to inject the election and update any state
	replace, err := pool.add(elect, applyInBlock)
	if err != nil {
		return err
	}

	// If we added a new election, run promotion checks and return
	if !replace {
		pool.promoteExecutables([]*types.AddressElection{elect})
	}
	return nil
}

// AddElections attempts to queue a batch of elections if they are valid.
func (pool *ElectionPool) AddElections(elects []*types.AddressElection) []error {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	return pool.addElectionsLocked(elects)
}

// addElectionsLocked attempts to queue a batch of elections if they are valid,
// whilst assuming the election pool lock is already held.
func (pool *ElectionPool) addElectionsLocked(elects []*types.AddressElection) []error {
	// Add the batch of election, tracking the accepted ones
	//dirty := make(map[common.Address]struct{})
	errs := make([]error, len(elects))

	//electsPromoted := make([]*types.AddressElection, 0)
	//for i, elect := range elects {
	//	var replace bool
	//	if replace, errs[i] = pool.add(elect); errs[i] == nil {
	//		if !replace {
	//			// jh: 对于新加入的交易（非替换）对应的发送者，我们将它们记录到dirty组中
	//			electsPromoted = append(electsPromoted, elect)
	//		}
	//	}
	//}
	//
	//// Only reprocess the internal state if something was actually added
	//if len(electsPromoted) > 0 {
	//	// jh: 对这些涉及新加入交易的sender,我们提升他们拥有的所有交易的执行级别
	//	// 将它们都加入到 pending 队列
	//	pool.promoteExecutables(electsPromoted)
	//}
	return errs
}

// Status returns the status (unknown/pending/queued) of a batch of elections
// identified by their hashes.
func (pool *ElectionPool) StatusSealers(hashes []common.Hash) []ElectionStatus {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	status := make([]ElectionStatus, len(hashes))
	for i, hash := range hashes {
		if elect := pool.all[hash]; elect != nil {
			from := elect.Address // already validated
			if pool.pendingSealers[*from] != nil {
				status[i] = ElectionStatusPending
			} else {
				status[i] = ElectionStatusQueued
			}
		}
	}
	return status
}

func (pool *ElectionPool) StatusPolice(hashes []common.Hash) []ElectionStatus {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	status := make([]ElectionStatus, len(hashes))
	for i, hash := range hashes {
		if elect := pool.all[hash]; elect != nil {
			from := elect.Address // already validated
			if pool.pendingPolice[*from] != nil {
				status[i] = ElectionStatusPending
			} else {
				status[i] = ElectionStatusQueued
			}
		}
	}
	return status
}

// Get returns a election if it is contained in the pool
// and nil otherwise.
func (pool *ElectionPool) Get(hash common.Hash) *types.AddressElection {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	return pool.all[hash]
}

// removeElection removes a single election from the queue, moving all subsequent
// elections back to the future queue.
func (pool *ElectionPool) removeElection(hash common.Hash) {
	// Fetch the election we wish to delete
	elect, ok := pool.all[hash]
	if !ok {
		return
	}
	addr := elect.Address // already validated during insertion

	// Remove it from the list of known elections
	delete(pool.all, hash)
	//pool.priced.Removed()

	// Remove the election from the pending lists
	delete(pool.pendingSealers, *addr)
	delete(pool.pendingPolice, *addr)

	// Remove the election from the pending lists
	//if pending := pool.pending[*addr]; pending != nil {
	//	if removed, invalids := pending.Remove(elect); removed {
	//		// If no more elections are left, remove the list
	//		if pending.Empty() {
	//			delete(pool.pending, addr)
	//			delete(pool.beats, addr)
	//		} else {
	//			// Otherwise postpone any invalidated elections
	//			for _, tx := range invalids {
	//				pool.enqueueElection(tx.Hash(), tx)
	//			}
	//		}
	//		// Update the account nonce if needed
	//		if nonce := elect.Nonce(); pool.pendingState.GetNonce(addr) > nonce {
	//			pool.pendingState.SetNonce(addr, nonce)
	//		}
	//		return
	//	}
	//}
	//// election is in the future queue
	//if future := pool.queue[addr]; future != nil {
	//	future.Remove(elect)
	//	if future.Empty() {
	//		delete(pool.queue, addr)
	//	}
	//}
}

// JiangHan: promoteExecutables 是将“非执行”队列中的交易提升到"pending"队列，在这个过程中
// 将执行过滤操作，将不合法交易（low nonce, low balance）都剔除掉
// 还会清理 pending 的所有交易，将超出配置的部分根据算法清除出部分
// 还会清理 queue 的所有交易，将超出配置的部分根据心跳排序清除出部分

// promoteExecutables moves elections that have become processable from the
// future queue to the set of pending elections. During this process, all
// invalidated elections (low nonce, low balance) are deleted.
func (pool *ElectionPool) promoteExecutables(elects []*types.AddressElection) {
	for _, elect := range elects {
		pool.promoteElection(*elect.Address, elect.Hash(), elect)
		//pool.pendingSealers[*elect.Address] = elect
	}

	//// Gather all the elects potentially needing updates
	//if elects == nil {
	//	elects = make([]*types.AddressElection, 0, len(pool.queue))
	//	for elect := range pool.queue {
	//		elects = append(elects, elect)
	//	}
	//}
	// Iterate over all elects and promote any executable elections
	//for _, elect := range elects {
	//list := pool.queue[elect]
	//if list == nil {
	//	continue // Just in case someone calls with a non existing account
	//}
	//// jh: 去除list中所有nonce<currentNonce的交易
	//// Drop all elections that are deemed too old (low nonce)
	//for _, tx := range list.Forward(pool.currentState.GetNonce(elect)) {
	//	hash := tx.Hash()
	//	log.Trace("Removed old queued election", "hash", hash)
	//	delete(pool.all, hash)
	//	pool.priced.Removed()
	//}
	//
	//// jh: 设置该list的消耗帽和gas帽，并以这两个帽（cap）去除list中所有昂贵的交易（超过池中现有的最大Gas)
	//// Drop all elections that are too costly (low balance or out of gas)
	//drops, _ := list.Filter(pool.currentState.GetBalance(elect), pool.currentMaxGas)
	//for _, tx := range drops {
	//	hash := tx.Hash()
	//	log.Trace("Removed unpayable queued election", "hash", hash)
	//	delete(pool.all, hash)
	//	pool.priced.Removed()
	//	queuedNofundsCounter.Inc(1)
	//}
	//
	//// jh: 收集该list中Ready的所有交易（tx.nonce > pool.pendingState.GetNonce(elect) )
	//// Gather all executable elections and promote them
	//for _, tx := range list.Ready(pool.pendingState.GetNonce(elect)) {
	//	hash := tx.Hash()
	//	log.Trace("Promoting queued election", "hash", hash)
	//	pool.promoteElection(elect, hash, tx)
	//}
	// Drop all elections over the allowed limit
	//	if !pool.locals.contains(elect) {
	//		for _, tx := range list.Cap(int(pool.config.AccountQueue)) {
	//			hash := tx.Hash()
	//			delete(pool.all, hash)
	//			pool.priced.Removed()
	//			queuedRateLimitCounter.Inc(1)
	//			log.Trace("Removed cap-exceeding queued election", "hash", hash)
	//		}
	//	}
	//	// Delete the entire queue entry if it became empty.
	//	if list.Empty() {
	//		delete(pool.queue, elect)
	//	}
	//}
	// If the pending limit is overflown, start equalizing allowances
	//pending := uint64(len(pool.pending))
	//if pending > pool.config.GlobalSlots {
	//	pendingBeforeCap := pending
	//	// Assemble a spam order to penalize large transactors first
	//	spammers := prque.New()
	//	for addr, list := range pool.pending {
	//		// Only evict elections from high rollers
	//		if !pool.locals.contains(addr) && uint64(list.Len()) > pool.config.AccountSlots {
	//			// jh: 不是本地节点的account, 并且它存在本地的交易list长度超过了配置的最大slot限制
	//			// 我们将它记录到 spammers（垃圾邮件发送者:<） 中
	//			spammers.Push(addr, float32(list.Len()))
	//		}
	//	}
	//	// Gradually drop elections from offenders
	//	offenders := []common.Address{}
	//	for pending > pool.config.GlobalSlots && !spammers.Empty() {
	//		// Retrieve the next offender if not local address
	//		// jh： 我们在一个for循环中逐步清除掉 spammer的交易，最终使得
	//		// pending 总数 <= pool.config.GlobalSlots （全局slot）
	//		offender, _ := spammers.Pop()
	//		offenders = append(offenders, offender.(common.Address))
	//
	//		// Equalize balances until all the same or below threshold
	//		if len(offenders) > 1 {
	//			// Calculate the equalization threshold for all current offenders
	//			// jh: 当offenders容纳超过2个spammer时，我们使用最后遍历到的spammer的txList的Len
	//			// 当作threshold(阈值)
	//			//
	//			threshold := pool.pending[offender.(common.Address)].Len()
	//
	//			// 之所以有上述设计，目的是为了将最近一个spammer的交易列表跟前一个的threshold进行一个运算
	//			// 使得它们能够被保留的交易数量按照一定规则递减
	//			// Iteratively reduce all offenders until below limit or threshold reached
	//			for pending > pool.config.GlobalSlots && pool.pending[offenders[len(offenders)-2]].Len() > threshold {
	//				for i := 0; i < len(offenders)-1; i++ {
	//					list := pool.pending[offenders[i]]
	//					for _, tx := range list.Cap(list.Len() - 1) {
	//						// Drop the election from the global pools too
	//						hash := tx.Hash()
	//						delete(pool.all, hash)
	//						pool.priced.Removed()
	//
	//						// Update the account nonce to the dropped election
	//						if nonce := tx.Nonce(); pool.pendingState.GetNonce(offenders[i]) > nonce {
	//							pool.pendingState.SetNonce(offenders[i], nonce)
	//						}
	//						log.Trace("Removed fairness-exceeding pending election", "hash", hash)
	//					}
	//					pending--
	//				}
	//			}
	//		}
	//	}
	//	// If still above threshold, reduce to limit or min allowance
	//	if pending > pool.config.GlobalSlots && len(offenders) > 0 {
	//		for pending > pool.config.GlobalSlots && uint64(pool.pending[offenders[len(offenders)-1]].Len()) > pool.config.AccountSlots {
	//			for _, addr := range offenders {
	//				list := pool.pending[addr]
	//				for _, tx := range list.Cap(list.Len() - 1) {
	//					// Drop the election from the global pools too
	//					hash := tx.Hash()
	//					delete(pool.all, hash)
	//					pool.priced.Removed()
	//
	//					// Update the account nonce to the dropped election
	//					if nonce := tx.Nonce(); pool.pendingState.GetNonce(addr) > nonce {
	//						pool.pendingState.SetNonce(addr, nonce)
	//					}
	//					log.Trace("Removed fairness-exceeding pending election", "hash", hash)
	//				}
	//				pending--
	//			}
	//		}
	//	}
	//	pendingRateLimitCounter.Inc(int64(pendingBeforeCap - pending))
	//}
	//
	//// 最后收拾一下 queue 队列中的交易，将非本地的交易根据对端跟本地心跳接触次数进行排序
	//// 将排在队尾的address对应的交易list全部清除出去
	//// If we've queued more elections than the hard limit, drop oldest ones
	//queued := uint64(0)
	//for _, list := range pool.queue {
	//	queued += uint64(list.Len())
	//}
	//if queued > pool.config.GlobalQueue {
	//	// Sort all elects with queued elections by heartbeat
	//	addresses := make(addresssByHeartbeat, 0, len(pool.queue))
	//	for addr := range pool.queue {
	//		if !pool.locals.contains(addr) { // don't drop locals
	//			addresses = append(addresses, addressByHeartbeat{addr, pool.beats[addr]})
	//		}
	//	}
	//	sort.Sort(addresses)
	//
	//	// Drop elections until the total is below the limit or only locals remain
	//	for drop := queued - pool.config.GlobalQueue; drop > 0 && len(addresses) > 0; {
	//		addr := addresses[len(addresses)-1]
	//		list := pool.queue[addr.address]
	//
	//		addresses = addresses[:len(addresses)-1]
	//
	//		// Drop all elections if they are less than the overflow
	//		if size := uint64(list.Len()); size <= drop {
	//			for _, tx := range list.Flatten() {
	//				pool.removeElection(tx.Hash())
	//			}
	//			drop -= size
	//			queuedRateLimitCounter.Inc(int64(size))
	//			continue
	//		}
	//		// Otherwise drop only last few elections
	//		txs := list.Flatten()
	//		for i := len(txs) - 1; i >= 0 && drop > 0; i-- {
	//			pool.removeElection(txs[i].Hash())
	//			drop--
	//			queuedRateLimitCounter.Inc(1)
	//		}
	//	}
	//}
}

// demoteUnexecutables removes invalid and processed elections from the pools
// executable/pending queue and any subsequent elections that become unexecutable
// are moved back into the future queue.
func (pool *ElectionPool) demoteUnexecutables() {

	//// Iterate over all accounts and demote any non-executable elections
	//for addr, list := range pool.pending {
	//	nonce := pool.currentState.GetNonce(addr)
	//
	//	// Drop all elections that are deemed too old (low nonce)
	//	for _, tx := range list.Forward(nonce) {
	//		hash := tx.Hash()
	//		log.Trace("Removed old pending election", "hash", hash)
	//		delete(pool.all, hash)
	//		pool.priced.Removed()
	//	}
	//	// Drop all elections that are too costly (low balance or out of gas), and queue any invalids back for later
	//	drops, invalids := list.Filter(pool.currentState.GetBalance(addr), pool.currentMaxGas)
	//	for _, tx := range drops {
	//		hash := tx.Hash()
	//		log.Trace("Removed unpayable pending election", "hash", hash)
	//		delete(pool.all, hash)
	//		pool.priced.Removed()
	//		pendingNofundsCounter.Inc(1)
	//	}
	//	for _, tx := range invalids {
	//		hash := tx.Hash()
	//		log.Trace("Demoting pending election", "hash", hash)
	//		pool.enqueueElection(hash, tx)
	//	}
	//	// If there's a gap in front, warn (should never happen) and postpone all elections
	//	if list.Len() > 0 && list.txs.Get(nonce) == nil {
	//		for _, tx := range list.Cap(0) {
	//			hash := tx.Hash()
	//			log.Error("Demoting invalidated election", "hash", hash)
	//			pool.enqueueElection(hash, tx)
	//		}
	//	}
	//	// Delete the entire queue entry if it became empty.
	//	if list.Empty() {
	//		delete(pool.pending, addr)
	//		delete(pool.beats, addr)
	//	}
	//}
}
