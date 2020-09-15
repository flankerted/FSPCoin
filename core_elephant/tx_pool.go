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

package core_elephant

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/contatract/go-contatract/common"
	types "github.com/contatract/go-contatract/core/types_elephant"
	state "github.com/contatract/go-contatract/core_elephant/state"
	"github.com/contatract/go-contatract/event"
	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/metrics"
	"github.com/contatract/go-contatract/params"
	"gopkg.in/karalabe/cookiejar.v2/collections/prque"
)

const (
	// ChainHeadChanSize is the size of channel listening to ChainHeadEvent.
	ChainHeadChanSize = 10
	// rmTxChanSize is the size of channel listening to RemovedTransactionEvent.
	rmTxChanSize = 10
)

var (
	evictionInterval    = time.Minute     // Time interval to check for evictable transactions
	statsReportInterval = 8 * time.Second // Time interval to report transaction pool stats
)

var (
	// Metrics for the pending pool
	pendingDiscardCounter   = metrics.NewCounter("txpool/pending/discard")
	pendingReplaceCounter   = metrics.NewCounter("txpool/pending/replace")
	pendingRateLimitCounter = metrics.NewCounter("txpool/pending/ratelimit") // Dropped due to rate limiting
	pendingNofundsCounter   = metrics.NewCounter("txpool/pending/nofunds")   // Dropped due to out-of-funds

	// Metrics for the queued pool
	queuedDiscardCounter   = metrics.NewCounter("txpool/queued/discard")
	queuedReplaceCounter   = metrics.NewCounter("txpool/queued/replace")
	queuedRateLimitCounter = metrics.NewCounter("txpool/queued/ratelimit") // Dropped due to rate limiting
	queuedNofundsCounter   = metrics.NewCounter("txpool/queued/nofunds")   // Dropped due to out-of-funds

	// General tx metrics
	invalidTxCounter     = metrics.NewCounter("txpool/invalid")
	underpricedTxCounter = metrics.NewCounter("txpool/underpriced")
)

// TxStatus is the current status of a transaction as seen by the pool.
type TxStatus uint

const (
	TxStatusUnknown TxStatus = iota
	TxStatusQueued
	TxStatusPending
	TxStatusIncluded
)

// blockChain provides the state of blockchain and current gas limit to do
// some pre checks in tx pool and event subscribers.
type blockChain interface {
	CurrentBlock() *types.Block
	GetBlock(hash common.Hash, number uint64) *types.Block
	StateAt(root common.Hash) (*state.StateDB, error)

	SubscribeChainHeadEvent(ch chan<- ChainHeadEvent) event.Subscription
}

// TxPoolConfig are the configuration parameters of the transaction pool.
type TxPoolConfig struct {
	NoLocals  bool          // Whether local transaction handling should be disabled
	Journal   string        // Journal of local transactions to survive node restarts
	Rejournal time.Duration // Time interval to regenerate the local transaction journal

	PriceLimit uint64 // Minimum gas price to enforce for acceptance into the pool
	PriceBump  uint64 // Minimum price bump percentage to replace an already existing transaction (nonce)

	AccountSlots uint64 // Minimum number of executable transaction slots guaranteed per account
	GlobalSlots  uint64 // Maximum number of executable transaction slots for all accounts
	AccountQueue uint64 // Maximum number of non-executable transaction slots permitted per account
	GlobalQueue  uint64 // Maximum number of non-executable transaction slots for all accounts

	Lifetime time.Duration // Maximum amount of time non-executable transaction are queued

	InputJournal string // Journal of input transactions to survive node restarts
}

// DefaultTxPoolConfig contains the default configurations for the transaction
// pool.
var DefaultTxPoolConfig = TxPoolConfig{
	Journal:   "transactions_elephant.rlp",
	Rejournal: time.Hour,

	PriceLimit: 1,
	PriceBump:  10,

	AccountSlots: 16,
	GlobalSlots:  4096,
	AccountQueue: 64,
	GlobalQueue:  1024,

	Lifetime: 3 * time.Hour,

	InputJournal: "transactions_input.rlp",
}

// sanitize checks the provided user configurations and changes anything that's
// unreasonable or unworkable.
func (config *TxPoolConfig) sanitize() TxPoolConfig {
	conf := *config
	if conf.Rejournal < time.Second {
		log.Warn("Sanitizing invalid txpool journal time", "provided", conf.Rejournal, "updated", time.Second)
		conf.Rejournal = time.Second
	}
	if conf.PriceLimit < 1 {
		log.Warn("Sanitizing invalid txpool price limit", "provided", conf.PriceLimit, "updated", DefaultTxPoolConfig.PriceLimit)
		conf.PriceLimit = DefaultTxPoolConfig.PriceLimit
	}
	if conf.PriceBump < 1 {
		log.Warn("Sanitizing invalid txpool price bump", "provided", conf.PriceBump, "updated", DefaultTxPoolConfig.PriceBump)
		conf.PriceBump = DefaultTxPoolConfig.PriceBump
	}
	return conf
}

// TxPool contains all currently known transactions. Transactions
// enter the pool when they are received from the network or submitted
// locally. They exit the pool when they are included in the blockchain.
//
// The pool separates processable transactions (which can be applied to the
// current state) and future transactions. Transactions move between those
// two states over time as they are received and processed.
type TxPool struct {
	config       TxPoolConfig
	chainconfig  *params.ElephantChainConfig
	chain        blockChain
	gasPrice     *big.Int
	txFeed       event.Feed
	scope        event.SubscriptionScope
	chainHeadCh  chan ChainHeadEvent
	chainHeadSub event.Subscription
	signer       types.Signer
	mu           sync.RWMutex
	inputMu      sync.RWMutex

	currentState  *state.StateDB      // Current state in the blockchain head
	pendingState  *state.ManagedState // Pending state tracking virtual nonces
	currentMaxGas uint64              // Current gas limit for transaction caps

	locals  *accountSet // Set of local transaction to exempt from eviction rules
	journal *txJournal  // Journal of local transaction to back up to disk

	pending map[common.Address]*txList         // All currently processable transactions
	queue   map[common.Address]*txList         // Queued but non-processable transactions
	beats   map[common.Address]time.Time       // Last heartbeat from each known account
	all     map[common.Hash]*types.Transaction // All transactions to allow lookups
	priced  *txPricedList                      // All transactions sorted by price

	wg           sync.WaitGroup // for shutdown sync
	allInput     map[common.Hash]*types.ShardingTxBatch
	inputJournal *inputJournal // Journal of input transaction to back up to disk
	EtherBase    *common.Address
}

// NewTxPool creates a new transaction pool to gather, sort and filter inbound
// transactions from the network.
func NewTxPool(config TxPoolConfig, chainconfig *params.ElephantChainConfig, chain blockChain, ethHeight uint64, eBase *common.Address) *TxPool {
	// Sanitize the input to ensure no vulnerable gas prices are set
	config = (&config).sanitize()

	// Create the transaction pool with its initial settings
	pool := &TxPool{
		config:      config,
		chainconfig: chainconfig,
		chain:       chain,
		signer:      types.NewEIP155Signer(chainconfig.ChainId),
		pending:     make(map[common.Address]*txList),
		queue:       make(map[common.Address]*txList),
		beats:       make(map[common.Address]time.Time),
		all:         make(map[common.Hash]*types.Transaction),
		chainHeadCh: make(chan ChainHeadEvent, ChainHeadChanSize),
		gasPrice:    new(big.Int).SetUint64(config.PriceLimit),
		allInput:    make(map[common.Hash]*types.ShardingTxBatch),
		EtherBase:   eBase,
	}
	pool.locals = newAccountSet(pool.signer)
	pool.priced = newTxPricedList(&pool.all)
	pool.reset(nil, chain.CurrentBlock().Header(), ethHeight)

	// If local transactions and journaling is enabled, load from disk
	if !config.NoLocals && config.Journal != "" {
		pool.journal = newTxJournal(config.Journal)

		if err := pool.journal.load(pool.AddLocal, ethHeight); err != nil {
			log.Warn("Failed to load transaction journal", "err", err)
		}
		if err := pool.journal.rotate(pool.local()); err != nil {
			log.Warn("Failed to rotate transaction journal", "err", err)
		}
	}

	// If input transactions journaling is enabled, load from disk
	if config.InputJournal != "" {
		pool.inputJournal = newInputJournal(config.InputJournal)

		if err := pool.inputJournal.load(pool.AddInputTx, ethHeight); err != nil {
			log.Warn("Failed to load input transaction journal", "err", err)
		}
		if err := pool.inputJournal.rotate(pool.getAllInputTxs()); err != nil {
			log.Warn("Failed to rotate input transaction journal", "err", err)
		}
	}

	// Subscribe events from blockchain
	pool.chainHeadSub = pool.chain.SubscribeChainHeadEvent(pool.chainHeadCh)

	// Start the event loop and return
	pool.wg.Add(1)
	go pool.loop()

	return pool
}

// loop is the transaction pool's main event loop, waiting for and reacting to
// outside blockchain events as well as for various reporting and transaction
// eviction events.
func (pool *TxPool) loop() {
	defer pool.wg.Done()

	// Start the stats reporting and transaction eviction tickers
	var prevPending, prevQueued, prevStales int

	report := time.NewTicker(statsReportInterval)
	defer report.Stop()

	evict := time.NewTicker(evictionInterval)
	defer evict.Stop()

	journal := time.NewTicker(pool.config.Rejournal)
	defer journal.Stop()

	// Track the previous head headers for transaction reorgs
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
				pool.reset(head.Header(), ev.Block.Header(), ev.LampHeight)
				head = ev.Block

				pool.mu.Unlock()
			}
		// Be unsubscribed due to system stopped
		case <-pool.chainHeadSub.Err():
			return

		// Handle stats reporting ticks
		// 定时执行状态报告，比如报告pending状态，queue状态，stale过时状态
		case <-report.C:
			pool.mu.RLock()
			pending, queued := pool.stats()
			stales := pool.priced.stales
			pool.mu.RUnlock()

			if pending != prevPending || queued != prevQueued || stales != prevStales {
				log.Debug("Transaction pool status report", "executable", pending, "queued", queued, "stales", stales)
				prevPending, prevQueued, prevStales = pending, queued, stales
			}

		// Handle inactive account transaction eviction
		case <-evict.C:
			pool.mu.Lock()
			for addr := range pool.queue {
				// Skip local transactions from the eviction mechanism
				if pool.locals.contains(addr) {
					continue
				}
				// Any non-locals old enough should be removed
				if time.Since(pool.beats[addr]) > pool.config.Lifetime {
					for _, tx := range pool.queue[addr].Flatten() {
						pool.removeTx(tx.Hash())
					}
				}
			}
			pool.mu.Unlock()

		// Handle local transaction journal rotation
		case <-journal.C:
			if pool.journal != nil {
				pool.mu.Lock()
				if err := pool.journal.rotate(pool.local()); err != nil {
					log.Warn("Failed to rotate local tx journal", "err", err)
				}
				pool.mu.Unlock()
			}
			if pool.inputJournal != nil {
				pool.inputMu.Lock()
				if err := pool.inputJournal.rotate(pool.getAllInputTxs()); err != nil {
					log.Warn("Failed to rotate input transaction journal", "err", err)
				}
				pool.inputMu.Unlock()
			}
		}
	}
}

// JiangHan：线程安全锁下的 reset 操作
// lockedReset is a wrapper around reset to allow calling it in a thread safe
// manner. This method is only ever used in the tester!
// func (pool *TxPool) lockedReset(oldHead, newHead *types.Header) {
// 	pool.mu.Lock()
// 	defer pool.mu.Unlock()

// 	pool.reset(oldHead, newHead)
// }

// reset retrieves the current state of the blockchain and ensures the content
// of the transaction pool is valid with regard to the chain state.
func (pool *TxPool) reset(oldHead, newHead *types.Header, lampHeight uint64) {
	// If we're reorging an old state, reinject all dropped transactions
	var reinject types.Transactions

	if oldHead != nil && oldHead.Hash() != newHead.ParentHash {
		// jh: 如果 oldHeader 并不是 newHeaer 的 father 节点，那么我们需要看看是不是相差太远
		// 取两者的 number 进行比较
		// If the reorg is too deep, avoid doing it (will happen during fast sync)
		oldNum := oldHead.Number.Uint64()
		newNum := newHead.Number.Uint64()

		if depth := uint64(math.Abs(float64(oldNum) - float64(newNum))); depth > 64 {
			// 相差64层。。。太远了
			log.Debug("Skipping deep transaction reorg", "depth", depth)
		} else {
			// 相差的深度还比较浅，可以考虑将相差的所有交易都拉入内存
			// Reorg seems shallow enough to pull in all transactions into memory
			var discarded, included types.Transactions

			var (
				rem = pool.chain.GetBlock(oldHead.Hash(), oldHead.Number.Uint64())
				add = pool.chain.GetBlock(newHead.Hash(), newHead.Number.Uint64())
			)

			// 如果新header number<旧header number, 就将旧的超出新的部分都加入到 discarded
			// 列表，全部丢弃
			for rem.NumberU64() > add.NumberU64() {
				discarded = append(discarded, rem.Transactions()...)
				if rem = pool.chain.GetBlock(rem.ParentHash(), rem.NumberU64()-1); rem == nil {
					log.Error("Unrooted old chain seen by tx pool", "block", oldHead.Number, "hash", oldHead.Hash())
					return
				}
			}

			// 如果旧header number<新header number, 就将新加入的部分都加入到 included 列表
			// 全部加进来
			for add.NumberU64() > rem.NumberU64() {
				included = append(included, add.Transactions()...)
				if add = pool.chain.GetBlock(add.ParentHash(), add.NumberU64()-1); add == nil {
					log.Error("Unrooted new chain seen by tx pool", "block", newHead.Number, "hash", newHead.Hash())
					return
				}

				// Remove input shard transactions that included in the new head
				for _, txBatch := range add.InputTxs() {
					if _, ok := pool.allInput[txBatch.Hash()]; ok {
						pool.RemoveInputTxs(txBatch.Hash())
					}
				}
			}

			// 最后处理新旧两者本身，将旧的加入discarded，干掉； 将新的加入included, 加入
			for rem.Hash() != add.Hash() {
				discarded = append(discarded, rem.Transactions()...)
				if rem = pool.chain.GetBlock(rem.ParentHash(), rem.NumberU64()-1); rem == nil {
					log.Error("Unrooted old chain seen by tx pool", "block", oldHead.Number, "hash", oldHead.Hash())
					return
				}
				included = append(included, add.Transactions()...)
				if add = pool.chain.GetBlock(add.ParentHash(), add.NumberU64()-1); add == nil {
					log.Error("Unrooted new chain seen by tx pool", "block", newHead.Number, "hash", newHead.Hash())
					return
				}
			}

			// 将included 中有可能跟 discarded 中重合的交易去除
			reinject = types.TxDifference(discarded, included)
		}
	}
	// Initialize the internal state to the current head
	if newHead == nil {
		newHead = pool.chain.CurrentBlock().Header() // Special case during testing
	}
	statedb, err := pool.chain.StateAt(newHead.Root)
	if err != nil {
		log.Error("Failed to reset txpool state", "err", err)
		return
	}
	pool.currentState = statedb
	pool.pendingState = state.ManageState(statedb)
	pool.currentMaxGas = newHead.GasLimit

	// Inject any transactions discarded due to reorgs
	log.Debug("Reinjecting stale transactions", "count", len(reinject))
	pool.addTxsLocked(reinject, false, lampHeight)

	// validate the pool of pending transactions, this will remove
	// any transactions that have been included in the block or
	// have been invalidated because of another transaction (e.g.
	// higher gas price)
	pool.demoteUnexecutables(lampHeight)

	// Update all accounts to the latest known pending nonce
	for addr, list := range pool.pending {
		txs := list.Flatten() // Heavy but will be cached and is needed by the miner anyway
		pool.pendingState.SetNonce(addr, txs[len(txs)-1].Nonce()+1)
	}
	// Check the queue and move transactions over to the pending if possible
	// or remove those that have become invalid
	pool.promoteExecutables(nil, lampHeight)
}

// Stop terminates the transaction pool.
func (pool *TxPool) Stop() {
	// Unsubscribe all subscriptions registered from txpool
	pool.scope.Close()

	// Unsubscribe subscriptions registered from blockchain
	pool.chainHeadSub.Unsubscribe()
	pool.wg.Wait()

	if pool.journal != nil {
		pool.journal.close()
	}
	log.Info("Transaction pool stopped")
}

// SubscribeTxPreEvent registers a subscription of TxPreEvent and
// starts sending event to the given channel.
func (pool *TxPool) SubscribeTxPreEvent(ch chan<- TxPreEvent) event.Subscription {
	return pool.scope.Track(pool.txFeed.Subscribe(ch))
}

// GasPrice returns the current gas price enforced by the transaction pool.
func (pool *TxPool) GasPrice() *big.Int {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	return new(big.Int).Set(pool.gasPrice)
}

// SetGasPrice updates the minimum price required by the transaction pool for a
// new transaction, and drops all transactions below this threshold.
func (pool *TxPool) SetGasPrice(price *big.Int) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pool.gasPrice = price
	for _, tx := range pool.priced.Cap(price, pool.locals) {
		pool.removeTx(tx.Hash())
	}
	log.Info("Transaction pool price threshold updated", "price", price)
}

// State returns the virtual managed state of the transaction pool.
func (pool *TxPool) State() *state.ManagedState {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	return pool.pendingState
}

// Stats retrieves the current pool stats, namely the number of pending and the
// number of queued (non-executable) transactions.
func (pool *TxPool) Stats() (int, int) {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	return pool.stats()
}

// stats retrieves the current pool stats, namely the number of pending and the
// number of queued (non-executable) transactions.
func (pool *TxPool) stats() (int, int) {
	pending := 0
	for _, list := range pool.pending {
		pending += list.Len()
	}
	queued := 0
	for _, list := range pool.queue {
		queued += list.Len()
	}
	return pending, queued
}

// Content retrieves the data content of the transaction pool, returning all the
// pending as well as queued transactions, grouped by account and sorted by nonce.
func (pool *TxPool) Content() (map[common.Address]types.Transactions, map[common.Address]types.Transactions) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pending := make(map[common.Address]types.Transactions)
	for addr, list := range pool.pending {
		pending[addr] = list.Flatten()
	}
	queued := make(map[common.Address]types.Transactions)
	for addr, list := range pool.queue {
		queued[addr] = list.Flatten()
	}
	return pending, queued
}

// Pending retrieves all currently processable transactions, groupped by origin
// account and sorted by nonce. The returned transaction set is a copy and can be
// freely modified by calling code.
func (pool *TxPool) Pending() (map[common.Address]types.Transactions, error) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pending := make(map[common.Address]types.Transactions)
	for addr, list := range pool.pending {
		pending[addr] = list.Flatten()
	}
	return pending, nil
}

// local retrieves all currently known local transactions, groupped by origin
// account and sorted by nonce. The returned transaction set is a copy and can be
// freely modified by calling code.
func (pool *TxPool) local() map[common.Address]types.Transactions {
	txs := make(map[common.Address]types.Transactions)
	for addr := range pool.locals.accounts {
		if pending := pool.pending[addr]; pending != nil {
			txs[addr] = append(txs[addr], pending.Flatten()...)
		}
		if queued := pool.queue[addr]; queued != nil {
			txs[addr] = append(txs[addr], queued.Flatten()...)
		}
	}
	return txs
}

// validateTx checks whether a transaction is valid according to the consensus
// rules and adheres to some heuristic limits of the local node (price and size).
func (pool *TxPool) validateTx(tx *types.Transaction, local bool, lampHeight uint64) error {
	// Heuristic limit, reject transactions over 32KB to prevent DOS attacks
	// jh: 这里是说智能合约不能超过 32k 吗？（其实是不能超过24k）
	if tx.Size() > 32*1024 {
		return ErrOversizedData
	}
	// jh: 金额不能是负数，只能是From账户转给To账户，如果是负数就变成从别人那里拿钱了。。
	// Transactions can't be negative. This may never happen using RLP decoded
	// transactions but may occur if you create a transaction using the RPC.
	if tx.Value().Sign() < 0 {
		return ErrNegativeValue
	}
	// Ensure the transaction doesn't exceed the current block limit gas.
	if pool.currentMaxGas < tx.Gas() {
		return ErrGasLimit
	}
	// Make sure the transaction is signed properly
	from, err := types.Sender(pool.signer, tx)
	if err != nil {
		return ErrInvalidSender
	}
	// Drop non-local transactions under our own minimal accepted gas price
	local = local || pool.locals.contains(from) // account may be local even if the transaction arrived from the network
	if !local && pool.gasPrice.Cmp(tx.GasPrice()) > 0 {
		return ErrUnderpriced
	}
	// Ensure the transaction adheres to nonce ordering
	if pool.currentState.GetNonce(from) > tx.Nonce() {
		return ErrNonceTooLow
	}
	// Transactor should have enough funds to cover the costs
	// cost == V + GP * GL
	if !tx.IsFreeAct(lampHeight) && pool.currentState.GetBalance(from).Cmp(tx.Cost()) < 0 {
		return ErrInsufficientFunds
	}
	intrGas, err := IntrinsicGas(tx.Data(), tx.To() == nil)
	if err != nil {
		return err
	}
	if tx.Gas() < intrGas {
		return ErrIntrinsicGas
	}
	return nil
}

// JiangHan: （重点）交易池增加交易的逻辑，策略，如何替代已有的交易
// 一个交易的入池逻辑：首先判断等待执行的 pending list 中是否已经存在该交易，如果已经存在，就直接
// 替代old; 如果没有替换，则放入 queue 队列，等待被promoteTx在合适时机将它们“升级”到 pending
// list, 而pending list 则是等待 worker, miner 的commitNewWork 将它们打包执行，并将执行结果
// 通过POW的计算后seal到新区块发布给其他节点

// add validates a transaction and inserts it into the non-executable queue for
// later pending promotion and execution. If the transaction is a replacement for
// an already pending or queued one, it overwrites the previous and returns this
// so outer code doesn't uselessly call promote.
//
// If a newly added transaction is marked as local, its sending account will be
// whitelisted, preventing any associated transaction from being dropped out of
// the pool due to pricing constraints.
func (pool *TxPool) add(tx *types.Transaction, local bool, lampHeight uint64) (bool, error) {
	if !tx.IsLocalOrOutputTx(lampHeight, pool.EtherBase) {
		tips := "Tx pool add, type of tx isn't local or output"
		log.Info(tips, "tx", tx)
		return false, errors.New(tips)
	}
	// If the transaction is already known, discard it
	hash := tx.Hash()
	if pool.all[hash] != nil {
		log.Trace("Discarding already known transaction", "hash", hash)
		return false, fmt.Errorf("known transaction: %x", hash)
	}

	// jh: 如果交易不符合基本验证，丢弃
	// 比如gas不够，尺寸错误，from方金额balance不足支付等等
	// If the transaction fails basic validation, discard it
	if err := pool.validateTx(tx, local, lampHeight); err != nil {
		log.Trace("Discarding invalid transaction", "hash", hash, "err", err)
		invalidTxCounter.Inc(1)
		return false, err
	}

	// jh: 如果交易池满了，我们将低价值的交易丢弃
	// If the transaction pool is full, discard underpriced transactions
	if uint64(len(pool.all)) >= pool.config.GlobalSlots+pool.config.GlobalQueue {
		// If the new transaction is underpriced, don't accept it
		if pool.priced.Underpriced(tx, pool.locals) {
			log.Trace("Discarding underpriced transaction", "hash", hash, "price", tx.GasPrice())
			underpricedTxCounter.Inc(1)
			return false, ErrUnderpriced
		}
		// New transaction is better than our worse ones, make room for it
		drop := pool.priced.Discard(len(pool.all)-int(pool.config.GlobalSlots+pool.config.GlobalQueue-1), pool.locals)
		for _, tx := range drop {
			log.Trace("Discarding freshly underpriced transaction", "hash", tx.Hash(), "price", tx.GasPrice())
			underpricedTxCounter.Inc(1)
			pool.removeTx(tx.Hash())
		}
	}

	// jh: 查看这个 signer (交易源发地址）的交易链,看看新交易是否会替换已经 pending的交易
	// If the transaction is replacing an already pending one, do directly
	from, _ := types.Sender(pool.signer, tx) // already validated
	if list := pool.pending[from]; list != nil && list.Overlaps(tx) {
		// Nonce already pending, check if required price bump is met
		inserted, old := list.Add(tx, pool.config.PriceBump)
		if !inserted {
			pendingDiscardCounter.Inc(1)
			return false, ErrReplaceUnderpriced
		}
		// New transaction is better, replace old one
		if old != nil {
			delete(pool.all, old.Hash())
			pool.priced.Removed()
			pendingReplaceCounter.Inc(1)
		}
		pool.all[tx.Hash()] = tx
		pool.priced.Put(tx)
		pool.journalTx(from, tx)

		log.Trace("Pooled new executable transaction", "hash", hash, "from", from, "to", tx.To())

		// 我们成功将一个交易插入到交易池，需要通知一下 Feed 模块，让它通告所有等待 TxPreEvent 事件
		// 的订购模块，让它们执行自己的逻辑（广播，执行等等）
		// We've directly injected a replacement transaction, notify subsystems
		go pool.txFeed.Send(TxPreEvent{tx})

		// 如果 old != nil, 这里就表示发生了替换
		return old != nil, nil
	}
	// New transaction isn't replacing a pending one, push into queue
	replace, err := pool.enqueueTx(hash, tx)
	if err != nil {
		return false, err
	}
	// Mark local addresses and journal local transactions
	if local {
		pool.locals.add(from)
	}
	pool.journalTx(from, tx)

	log.Trace("Pooled new future transaction", "hash", hash, "from", from, "to", tx.To())
	return replace, nil
}

// JiangHan：enqueueTx函数是将一个新交易插入到一个“非执行”队列
// enqueueTx inserts a new transaction into the non-executable transaction queue.
//
// Note, this method assumes the pool lock is held!
func (pool *TxPool) enqueueTx(hash common.Hash, tx *types.Transaction) (bool, error) {
	// Try to insert the transaction into the future queue
	from, err := types.Sender(pool.signer, tx) // already validated
	if err != nil {
		log.Error("Enqueue tx", "err", err)
		return false, err
	}
	if pool.queue[from] == nil {
		pool.queue[from] = newTxList(false)
	}
	inserted, old := pool.queue[from].Add(tx, pool.config.PriceBump)
	if !inserted {
		// An older transaction was better, discard this
		queuedDiscardCounter.Inc(1)
		return false, ErrReplaceUnderpriced
	}
	// Discard any previous transaction and mark this
	if old != nil {
		delete(pool.all, old.Hash())
		pool.priced.Removed()
		queuedReplaceCounter.Inc(1)
	}
	pool.all[hash] = tx
	pool.priced.Put(tx)
	return old != nil, nil
}

// journalTx adds the specified transaction to the local disk journal if it is
// deemed to have been sent from a local account.
func (pool *TxPool) journalTx(from common.Address, tx *types.Transaction) {
	// Only journal if it's enabled and the transaction is local
	if pool.journal == nil || !pool.locals.contains(from) {
		return
	}
	if err := pool.journal.insert(tx); err != nil {
		log.Warn("Failed to journal local transaction", "err", err)
	}
}

// JiangHan: 将交易从“非执行”队列移到"pending"队列等待执行，并发送 TxPreEvent 事件通知所有事件监听者
// promoteTx adds a transaction to the pending (processable) list of transactions.
//
// Note, this method assumes the pool lock is held!
func (pool *TxPool) promoteTx(addr common.Address, hash common.Hash, tx *types.Transaction) {
	// Try to insert the transaction into the pending queue
	if pool.pending[addr] == nil {
		pool.pending[addr] = newTxList(true)
	}
	list := pool.pending[addr]

	inserted, old := list.Add(tx, pool.config.PriceBump)
	if !inserted {
		// An older transaction was better, discard this
		delete(pool.all, hash)
		pool.priced.Removed()

		pendingDiscardCounter.Inc(1)
		return
	}
	// Otherwise discard any previous transaction and mark this
	if old != nil {
		delete(pool.all, old.Hash())
		pool.priced.Removed()

		pendingReplaceCounter.Inc(1)
	}
	// Failsafe to work around direct pending inserts (tests)
	if pool.all[hash] == nil {
		pool.all[hash] = tx
		pool.priced.Put(tx)
	}
	// Set the potentially new pending nonce and notify any subsystems of the new tx
	pool.beats[addr] = time.Now()
	pool.pendingState.SetNonce(addr, tx.Nonce()+1)

	go pool.txFeed.Send(TxPreEvent{tx})
}

// AddLocal enqueues a single transaction into the pool if it is valid, marking
// the sender as a local one in the mean time, ensuring it goes around the local
// pricing constraints.
func (pool *TxPool) AddLocal(tx *types.Transaction, ethHeight uint64) error {
	return pool.addTx(tx, !pool.config.NoLocals, ethHeight)
}

// AddRemote enqueues a single transaction into the pool if it is valid. If the
// sender is not among the locally tracked ones, full pricing constraints will
// apply.
func (pool *TxPool) AddRemote(tx *types.Transaction, ethHeight uint64) error {
	return pool.addTx(tx, false, ethHeight)
}

// AddLocals enqueues a batch of transactions into the pool if they are valid,
// marking the senders as a local ones in the mean time, ensuring they go around
// the local pricing constraints.
func (pool *TxPool) AddLocals(txs []*types.Transaction, ethHeight uint64) []error {
	return pool.addTxs(txs, !pool.config.NoLocals, ethHeight)
}

// JiangHan: 来自其他 peer 的 TxMsg 消息，我们将它们带来的交易加入到 remote 组
// AddRemotes enqueues a batch of transactions into the pool if they are valid.
// If the senders are not among the locally tracked ones, full pricing constraints
// will apply.
func (pool *TxPool) AddRemotes(txs []*types.Transaction, ethHeight uint64) []error {
	return pool.addTxs(txs, false, ethHeight)
}

// addTx enqueues a single transaction into the pool if it is valid.
func (pool *TxPool) addTx(tx *types.Transaction, local bool, ethHeight uint64) error {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	// Try to inject the transaction and update any state
	replace, err := pool.add(tx, local, ethHeight)
	if err != nil {
		return err
	}
	// If we added a new transaction, run promotion checks and return
	// jh: 如果不是替换，而是新增交易到交易池，这里执行一下提升池性能的操作
	// 将那些nonce比较低（过时），balance不足，gas不足的交易通通从池中删除
	if !replace {
		from, _ := types.Sender(pool.signer, tx) // already validated
		pool.promoteExecutables([]common.Address{from}, ethHeight)
	}
	return nil
}

// addTxs attempts to queue a batch of transactions if they are valid.
func (pool *TxPool) addTxs(txs []*types.Transaction, local bool, ethHeight uint64) []error {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	// Add the batch of transaction, tracking the accepted ones
	dirty := make(map[common.Address]struct{})
	errs := make([]error, len(txs))

	for i, tx := range txs {
		var replace bool
		if replace, errs[i] = pool.add(tx, local, ethHeight); errs[i] == nil {
			if !replace {
				// jh: 对于新加入的交易（非替换）对应的发送者，我们将它们记录到dirty组中
				from, _ := types.Sender(pool.signer, tx) // already validated
				dirty[from] = struct{}{}
			}
		}
	}
	// Only reprocess the internal state if something was actually added
	if len(dirty) > 0 {
		addrs := make([]common.Address, 0, len(dirty))
		for addr := range dirty {
			addrs = append(addrs, addr)
		}
		// jh: 对这些涉及新加入交易的sender,我们提升他们拥有的所有交易的执行级别
		// 将它们都加入到 pending 队列
		pool.promoteExecutables(addrs, ethHeight)
	}
	return errs
}

func (pool *TxPool) addTxLocked(tx *types.Transaction, local bool, ethHeight uint64) error {
	replace, err := pool.add(tx, local, ethHeight)
	if err != nil {
		return err
	}
	if !replace {
		from, _ := types.Sender(pool.signer, tx) // already validated
		pool.promoteExecutables([]common.Address{from}, ethHeight)
	}
	return nil
}

// addTxsLocked attempts to queue a batch of transactions if they are valid,
// whilst assuming the transaction pool lock is already held.
func (pool *TxPool) addTxsLocked(txs []*types.Transaction, local bool, lampHeight uint64) []error {
	// Add the batch of transaction, tracking the accepted ones
	dirty := make(map[common.Address]struct{})
	errs := make([]error, len(txs))

	for i, tx := range txs {
		var replace bool
		if replace, errs[i] = pool.add(tx, local, lampHeight); errs[i] == nil {
			if !replace {
				// jh: 对于新加入的交易（非替换）对应的发送者，我们将它们记录到dirty组中
				from, _ := types.Sender(pool.signer, tx) // already validated
				dirty[from] = struct{}{}
			}
		}
	}
	// Only reprocess the internal state if something was actually added
	if len(dirty) > 0 {
		addrs := make([]common.Address, 0, len(dirty))
		for addr := range dirty {
			addrs = append(addrs, addr)
		}
		// jh: 对这些涉及新加入交易的sender,我们提升他们拥有的所有交易的执行级别
		// 将它们都加入到 pending 队列
		pool.promoteExecutables(addrs, lampHeight)
	}
	return errs
}

// Status returns the status (unknown/pending/queued) of a batch of transactions
// identified by their hashes.
func (pool *TxPool) Status(hashes []common.Hash) []TxStatus {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	status := make([]TxStatus, len(hashes))
	for i, hash := range hashes {
		if tx := pool.all[hash]; tx != nil {
			from, _ := types.Sender(pool.signer, tx) // already validated
			if pool.pending[from] != nil && pool.pending[from].txs.items[tx.Nonce()] != nil {
				status[i] = TxStatusPending
			} else {
				status[i] = TxStatusQueued
			}
		}
	}
	return status
}

// Get returns a transaction if it is contained in the pool
// and nil otherwise.
func (pool *TxPool) Get(hash common.Hash) *types.Transaction {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	return pool.all[hash]
}

// removeTx removes a single transaction from the queue, moving all subsequent
// transactions back to the future queue.
func (pool *TxPool) removeTx(hash common.Hash) {
	// Fetch the transaction we wish to delete
	tx, ok := pool.all[hash]
	if !ok {
		return
	}
	addr, _ := types.Sender(pool.signer, tx) // already validated during insertion

	// Remove it from the list of known transactions
	delete(pool.all, hash)
	pool.priced.Removed()

	// Remove the transaction from the pending lists and reset the account nonce
	if pending := pool.pending[addr]; pending != nil {
		if removed, invalids := pending.Remove(tx); removed {
			// If no more transactions are left, remove the list
			if pending.Empty() {
				delete(pool.pending, addr)
				delete(pool.beats, addr)
			} else {
				// Otherwise postpone any invalidated transactions
				for _, tx := range invalids {
					pool.enqueueTx(tx.Hash(), tx)
				}
			}
			// Update the account nonce if needed
			if nonce := tx.Nonce(); pool.pendingState.GetNonce(addr) > nonce {
				pool.pendingState.SetNonce(addr, nonce)
			}
			return
		}
	}
	// Transaction is in the future queue
	if future := pool.queue[addr]; future != nil {
		future.Remove(tx)
		if future.Empty() {
			delete(pool.queue, addr)
		}
	}
}

// JiangHan: promoteExecutables 是将“非执行”队列中的交易提升到"pending"队列，在这个过程中
// 将执行过滤操作，将不合法交易（low nonce, low balance）都剔除掉
// 还会清理 pending 的所有交易，将超出配置的部分根据算法清除出部分
// 还会清理 queue 的所有交易，将超出配置的部分根据心跳排序清除出部分

// promoteExecutables moves transactions that have become processable from the
// future queue to the set of pending transactions. During this process, all
// invalidated transactions (low nonce, low balance) are deleted.
func (pool *TxPool) promoteExecutables(accounts []common.Address, lampHeight uint64) {
	// Gather all the accounts potentially needing updates
	if accounts == nil {
		accounts = make([]common.Address, 0, len(pool.queue))
		for addr := range pool.queue {
			accounts = append(accounts, addr)
		}
	}
	// Iterate over all accounts and promote any executable transactions
	for _, addr := range accounts {
		list := pool.queue[addr]
		if list == nil {
			continue // Just in case someone calls with a non existing account
		}
		// jh: 去除list中所有nonce<currentNonce的交易
		// Drop all transactions that are deemed too old (low nonce)
		for _, tx := range list.Forward(pool.currentState.GetNonce(addr)) {
			hash := tx.Hash()
			log.Trace("Removed old queued transaction", "hash", hash)
			delete(pool.all, hash)
			pool.priced.Removed()
		}

		// jh: 设置该list的消耗帽和gas帽，并以这两个帽（cap）去除list中所有昂贵的交易（超过池中现有的最大Gas)
		// Drop all transactions that are too costly (low balance or out of gas)
		drops, _ := list.Filter(pool.currentState.GetBalance(addr), pool.currentMaxGas, lampHeight)
		for _, tx := range drops {
			hash := tx.Hash()
			log.Trace("Removed unpayable queued transaction", "hash", hash)
			delete(pool.all, hash)
			pool.priced.Removed()
			queuedNofundsCounter.Inc(1)
		}

		// jh: 收集该list中Ready的所有交易（tx.nonce > pool.pendingState.GetNonce(addr) )
		// Gather all executable transactions and promote them
		for _, tx := range list.Ready(pool.pendingState.GetNonce(addr)) {
			hash := tx.Hash()
			log.Trace("Promoting queued transaction", "hash", hash)
			pool.promoteTx(addr, hash, tx)
		}
		// Drop all transactions over the allowed limit
		if !pool.locals.contains(addr) {
			for _, tx := range list.Cap(int(pool.config.AccountQueue)) {
				hash := tx.Hash()
				delete(pool.all, hash)
				pool.priced.Removed()
				queuedRateLimitCounter.Inc(1)
				log.Trace("Removed cap-exceeding queued transaction", "hash", hash)
			}
		}
		// Delete the entire queue entry if it became empty.
		if list.Empty() {
			delete(pool.queue, addr)
		}
	}
	// If the pending limit is overflown, start equalizing allowances
	pending := uint64(0)
	for _, list := range pool.pending {
		pending += uint64(list.Len())
	}
	if pending > pool.config.GlobalSlots {
		pendingBeforeCap := pending
		// Assemble a spam order to penalize large transactors first
		spammers := prque.New()
		for addr, list := range pool.pending {
			// Only evict transactions from high rollers
			if !pool.locals.contains(addr) && uint64(list.Len()) > pool.config.AccountSlots {
				// jh: 不是本地节点的account, 并且它存在本地的交易list长度超过了配置的最大slot限制
				// 我们将它记录到 spammers（垃圾邮件发送者:<） 中
				spammers.Push(addr, float32(list.Len()))
			}
		}
		// Gradually drop transactions from offenders
		offenders := []common.Address{}
		for pending > pool.config.GlobalSlots && !spammers.Empty() {
			// Retrieve the next offender if not local address
			// jh： 我们在一个for循环中逐步清除掉 spammer的交易，最终使得
			// pending 总数 <= pool.config.GlobalSlots （全局slot）
			offender, _ := spammers.Pop()
			offenders = append(offenders, offender.(common.Address))

			// Equalize balances until all the same or below threshold
			if len(offenders) > 1 {
				// Calculate the equalization threshold for all current offenders
				// jh: 当offenders容纳超过2个spammer时，我们使用最后遍历到的spammer的txList的Len
				// 当作threshold(阈值)
				//
				threshold := pool.pending[offender.(common.Address)].Len()

				// 之所以有上述设计，目的是为了将最近一个spammer的交易列表跟前一个的threshold进行一个运算
				// 使得它们能够被保留的交易数量按照一定规则递减
				// Iteratively reduce all offenders until below limit or threshold reached
				for pending > pool.config.GlobalSlots && pool.pending[offenders[len(offenders)-2]].Len() > threshold {
					for i := 0; i < len(offenders)-1; i++ {
						list := pool.pending[offenders[i]]
						for _, tx := range list.Cap(list.Len() - 1) {
							// Drop the transaction from the global pools too
							hash := tx.Hash()
							delete(pool.all, hash)
							pool.priced.Removed()

							// Update the account nonce to the dropped transaction
							if nonce := tx.Nonce(); pool.pendingState.GetNonce(offenders[i]) > nonce {
								pool.pendingState.SetNonce(offenders[i], nonce)
							}
							log.Trace("Removed fairness-exceeding pending transaction", "hash", hash)
						}
						pending--
					}
				}
			}
		}
		// If still above threshold, reduce to limit or min allowance
		if pending > pool.config.GlobalSlots && len(offenders) > 0 {
			for pending > pool.config.GlobalSlots && uint64(pool.pending[offenders[len(offenders)-1]].Len()) > pool.config.AccountSlots {
				for _, addr := range offenders {
					list := pool.pending[addr]
					for _, tx := range list.Cap(list.Len() - 1) {
						// Drop the transaction from the global pools too
						hash := tx.Hash()
						delete(pool.all, hash)
						pool.priced.Removed()

						// Update the account nonce to the dropped transaction
						if nonce := tx.Nonce(); pool.pendingState.GetNonce(addr) > nonce {
							pool.pendingState.SetNonce(addr, nonce)
						}
						log.Trace("Removed fairness-exceeding pending transaction", "hash", hash)
					}
					pending--
				}
			}
		}
		pendingRateLimitCounter.Inc(int64(pendingBeforeCap - pending))
	}

	// 最后收拾一下 queue 队列中的交易，将非本地的交易根据对端跟本地心跳接触次数进行排序
	// 将排在队尾的address对应的交易list全部清除出去
	// If we've queued more transactions than the hard limit, drop oldest ones
	queued := uint64(0)
	for _, list := range pool.queue {
		queued += uint64(list.Len())
	}
	if queued > pool.config.GlobalQueue {
		// Sort all accounts with queued transactions by heartbeat
		addresses := make(addresssByHeartbeat, 0, len(pool.queue))
		for addr := range pool.queue {
			if !pool.locals.contains(addr) { // don't drop locals
				addresses = append(addresses, addressByHeartbeat{addr, pool.beats[addr]})
			}
		}
		sort.Sort(addresses)

		// Drop transactions until the total is below the limit or only locals remain
		for drop := queued - pool.config.GlobalQueue; drop > 0 && len(addresses) > 0; {
			addr := addresses[len(addresses)-1]
			list := pool.queue[addr.address]

			addresses = addresses[:len(addresses)-1]

			// Drop all transactions if they are less than the overflow
			if size := uint64(list.Len()); size <= drop {
				for _, tx := range list.Flatten() {
					pool.removeTx(tx.Hash())
				}
				drop -= size
				queuedRateLimitCounter.Inc(int64(size))
				continue
			}
			// Otherwise drop only last few transactions
			txs := list.Flatten()
			for i := len(txs) - 1; i >= 0 && drop > 0; i-- {
				pool.removeTx(txs[i].Hash())
				drop--
				queuedRateLimitCounter.Inc(1)
			}
		}
	}
}

// demoteUnexecutables removes invalid and processed transactions from the pools
// executable/pending queue and any subsequent transactions that become unexecutable
// are moved back into the future queue.
func (pool *TxPool) demoteUnexecutables(lampHeight uint64) {
	// Iterate over all accounts and demote any non-executable transactions
	for addr, list := range pool.pending {
		nonce := pool.currentState.GetNonce(addr)

		// Drop all transactions that are deemed too old (low nonce)
		for _, tx := range list.Forward(nonce) {
			hash := tx.Hash()
			log.Trace("Removed old pending transaction", "hash", hash)
			delete(pool.all, hash)
			pool.priced.Removed()
		}
		// Drop all transactions that are too costly (low balance or out of gas), and queue any invalids back for later
		drops, invalids := list.Filter(pool.currentState.GetBalance(addr), pool.currentMaxGas, lampHeight)
		for _, tx := range drops {
			hash := tx.Hash()
			log.Trace("Removed unpayable pending transaction", "hash", hash)
			delete(pool.all, hash)
			pool.priced.Removed()
			pendingNofundsCounter.Inc(1)
		}
		for _, tx := range invalids {
			hash := tx.Hash()
			log.Trace("Demoting pending transaction", "hash", hash)
			pool.enqueueTx(hash, tx)
		}
		// If there's a gap in front, warn (should never happen) and postpone all transactions
		if list.Len() > 0 && list.txs.Get(nonce) == nil {
			for _, tx := range list.Cap(0) {
				hash := tx.Hash()
				log.Error("Demoting invalidated transaction", "hash", hash)
				pool.enqueueTx(hash, tx)
			}
		}
		// Delete the entire queue entry if it became empty.
		if list.Empty() {
			delete(pool.pending, addr)
			delete(pool.beats, addr)
		}
	}
}

// journalInput adds the specified input transaction to the local disk journal if it is
// deemed to have been sent from a local account.
func (pool *TxPool) journalInput(batch *types.ShardingTxBatch) {
	hash := batch.Hash()
	if _, ok := pool.allInput[hash]; ok {
		log.Info("Journal input, already exist")
		return
	}
	// Only journal if it's enabled and the input transaction is input type
	if pool.inputJournal == nil {
		log.Warn("Tx pool input journal nil")
		return
	}
	if err := pool.inputJournal.insert(batch); err != nil {
		log.Warn("Failed to journal input transaction", "err", err)
	}
}

func (pool *TxPool) AddInputTx(batch *types.ShardingTxBatch, ethHeight uint64) error {
	pool.inputMu.RLock()
	defer pool.inputMu.RUnlock()

	hash := batch.Hash()
	if _, ok := pool.allInput[hash]; ok {
		log.Info("Add input tx, already exist")
		return nil
	}
	if len(batch.Txs) == 0 {
		err := errors.New("Has not input tx")
		log.Error("Add input tx", "err", err)
		return err
	}
	batchNonce := pool.currentState.GetBatchNonce(batch.ToShardingID())
	if batch.BatchNonce != batchNonce+1 {
		return errors.New("Block nonce not match")
	}
	for _, tx := range batch.Txs {
		if !tx.IsInputTx(ethHeight, pool.EtherBase) {
			err := errors.New("Including not input tx")
			log.Error("Add input tx", "err", err)
			return err
		}
	}
	pool.allInput[hash] = batch
	return nil
}

func (pool *TxPool) AddTxBatch(batch *types.ShardingTxBatch, etherbase common.Address) error {
	toShardingID := batch.ToShardingID()
	shardingID := common.GetSharding(etherbase, batch.LampNum)
	if toShardingID != shardingID {
		return errors.New("Sharding id not match")
	}
	if !batch.IsValid() {
		return errors.New("Batch not valid")
	}
	blockHeight := pool.currentState.GetShardingHeight(toShardingID)
	if batch.BlockHeight <= blockHeight {
		return errors.New("Block height not match")
	}
	if err := pool.AddInputTx(batch, batch.LampNum); err == nil {
		pool.journalInput(batch)
		log.Info("Add tx batch", "sharding id", batch.ShardingID)
	}
	return nil
}

func (pool *TxPool) getAllInputTxs() map[common.Hash]*types.ShardingTxBatch {
	return pool.allInput
}

func (pool *TxPool) ContentInputTxs() map[common.Hash]*types.ShardingTxBatch {
	pool.inputMu.RLock()
	defer pool.inputMu.RUnlock()

	return pool.allInput
}

func (pool *TxPool) RemoveInputTxs(hash common.Hash) {
	pool.inputMu.RLock()
	defer pool.inputMu.RUnlock()

	delete(pool.allInput, hash)
}

func (pool *TxPool) PendingInputTxs() types.ShardingTxBatches {
	pool.inputMu.RLock()
	defer pool.inputMu.RUnlock()

	batches := make(types.ShardingTxBatches, 0)
	// One commit, sharding tx batch max count
	const maxCount = 100
	for _, batch := range pool.allInput {
		batches = append(batches, batch)
		if len(batches) == maxCount {
			break
		}
	}
	sort.Sort(batches)
	return batches
}

// addressByHeartbeat is an account address tagged with its last activity timestamp.
type addressByHeartbeat struct {
	address   common.Address
	heartbeat time.Time
}

type addresssByHeartbeat []addressByHeartbeat

func (a addresssByHeartbeat) Len() int           { return len(a) }
func (a addresssByHeartbeat) Less(i, j int) bool { return a[i].heartbeat.Before(a[j].heartbeat) }
func (a addresssByHeartbeat) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

// accountSet is simply a set of addresses to check for existence, and a signer
// capable of deriving addresses from transactions.
type accountSet struct {
	accounts map[common.Address]struct{}
	signer   types.Signer
}

// newAccountSet creates a new address set with an associated signer for sender
// derivations.
func newAccountSet(signer types.Signer) *accountSet {
	return &accountSet{
		accounts: make(map[common.Address]struct{}),
		signer:   signer,
	}
}

// contains checks if a given address is contained within the set.
func (as *accountSet) contains(addr common.Address) bool {
	_, exist := as.accounts[addr]
	return exist
}

// containsTx checks if the sender of a given tx is within the set. If the sender
// cannot be derived, this method returns false.
func (as *accountSet) containsTx(tx *types.Transaction) bool {
	if addr, err := types.Sender(as.signer, tx); err == nil {
		return as.contains(addr)
	}
	return false
}

// add inserts a new address into the set to track.
func (as *accountSet) add(addr common.Address) {
	as.accounts[addr] = struct{}{}
}