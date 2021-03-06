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

package miner

import (
	//"bytes"
	"fmt"
	"math/big"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/contatract/go-contatract/baseparam"
	"github.com/contatract/go-contatract/blizparam"
	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/consensus"
	"github.com/contatract/go-contatract/core/types"
	"github.com/contatract/go-contatract/core/vm"
	core "github.com/contatract/go-contatract/core_eth"
	"github.com/contatract/go-contatract/core_eth/state"
	"github.com/contatract/go-contatract/ethdb"
	"github.com/contatract/go-contatract/event"
	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/params"
	"gopkg.in/fatih/set.v0"
)

const (
	resultQueueSize  = 10
	miningLogAtDepth = 5

	// txChanSize is the size of channel listening to TxPreEvent.
	// The number is referenced from the size of tx pool.
	txChanSize = 4096
	// electionChanSize is the size of channel listening to ElectionPreEvent.
	// The number is referenced from the size of election pool.
	electionChanSize = 4096
	// chainHeadChanSize is the size of channel listening to ChainHeadEvent.
	chainHeadChanSize = 10
	// chainSideChanSize is the size of channel listening to ChainSideEvent.
	chainSideChanSize = 10
)

// Agent can register themself with the worker
type Agent interface {
	Work() chan<- *Work
	SetReturnCh(chan<- *Result)
	Stop()
	Start()
	GetHashRate() int64
}

// Work is the workers current environment and holds
// all of the current state information
type Work struct {
	config *params.ChainConfig
	signer types.Signer

	state     *state.StateDB // apply state changes here
	ancestors *set.Set       // ancestor set (used for checking uncle parent validity)
	family    *set.Set       // family set (used for checking uncle invalidity)
	uncles    *set.Set       // uncle set
	tcount    int            // tx count in cycle

	Block *types.Block // the new block

	header   *types.Header
	txs      []*types.Transaction
	receipts []*types.Receipt

	createdAt time.Time
}

type Result struct {
	Work  *Work
	Block *types.Block
}

// worker is the main object which takes care of applying messages to the new state
type Worker struct {
	config *params.ChainConfig
	engine consensus.Engine

	mu sync.Mutex

	// update loop
	mux          *event.TypeMux
	txCh         chan core.TxPreEvent
	txSub        event.Subscription
	electionCh   chan core.ElectionPreEvent
	electionSub  event.Subscription
	chainHeadCh  chan core.ChainHeadEvent
	chainHeadSub event.Subscription
	chainSideCh  chan core.ChainSideEvent
	chainSideSub event.Subscription
	wg           sync.WaitGroup

	agents map[Agent]struct{}
	recv   chan *Result

	eth     Backend
	chain   *core.BlockChain
	proc    core.Validator
	chainDb ethdb.Database

	coinbase common.Address
	extra    []byte

	currentMu sync.Mutex
	current   *Work

	uncleMu        sync.Mutex
	possibleUncles map[common.Hash]*types.Block

	unconfirmed *unconfirmedBlocks // set of locally mined blocks pending canonicalness confirmations

	// atomic status counters
	mining int32
	atWork int32

	log log.Logger
}

func newWorker(config *params.ChainConfig, engine consensus.Engine, coinbase common.Address, eth Backend,
	mux *event.TypeMux, logger log.Logger) *Worker {
	worker := &Worker{
		config:         config,
		engine:         engine,
		eth:            eth,
		mux:            mux,
		txCh:           make(chan core.TxPreEvent, txChanSize),
		electionCh:     make(chan core.ElectionPreEvent, electionChanSize),
		chainHeadCh:    make(chan core.ChainHeadEvent, chainHeadChanSize),
		chainSideCh:    make(chan core.ChainSideEvent, chainSideChanSize),
		chainDb:        eth.ChainDb(),
		recv:           make(chan *Result, resultQueueSize),
		chain:          eth.BlockChain(),
		proc:           eth.BlockChain().Validator(),
		possibleUncles: make(map[common.Hash]*types.Block),
		coinbase:       coinbase,
		agents:         make(map[Agent]struct{}),
		unconfirmed:    newUnconfirmedBlocks(eth.BlockChain(), miningLogAtDepth),
		log:            logger,
	}
	if logger == nil {
		worker.log = log.New()
		worker.log.SetHandler(log.LvlFilterHandler(log.LvlInfo, log.StreamHandler(os.Stdout, log.TerminalFormat(false))))
	}

	// Subscribe TxPreEvent for tx pool
	worker.txSub = eth.TxPool().SubscribeTxPreEvent(worker.txCh)
	// Subscribe ElectionPreEvent for election pool
	worker.electionSub = eth.ElectionPool().SubscribeElectionPreEvent(worker.electionCh)
	// Subscribe events for blockchain
	worker.chainHeadSub = eth.BlockChain().SubscribeChainHeadEvent(worker.chainHeadCh)
	worker.chainSideSub = eth.BlockChain().SubscribeChainSideEvent(worker.chainSideCh)
	go worker.Update()

	go worker.wait()
	worker.commitNewWork()

	return worker
}

func (self *Worker) setEtherbase(addr common.Address) {
	self.mu.Lock()
	defer self.mu.Unlock()
	self.coinbase = addr
}

func (self *Worker) GetEtherbase() common.Address {
	return self.coinbase
}

func (self *Worker) setExtra(extra []byte) {
	self.mu.Lock()
	defer self.mu.Unlock()
	self.extra = extra
}

func (self *Worker) pending() (*types.Block, *state.StateDB) {
	self.currentMu.Lock()
	defer self.currentMu.Unlock()

	if atomic.LoadInt32(&self.mining) == 0 {
		return types.NewBlock(
			self.current.header,
			self.current.txs,
			nil,
			self.current.receipts,
		), self.current.state.Copy()
	}
	return self.current.Block, self.current.state.Copy()
}

func (self *Worker) pendingBlock() *types.Block {
	self.currentMu.Lock()
	defer self.currentMu.Unlock()

	if atomic.LoadInt32(&self.mining) == 0 {
		return types.NewBlock(
			self.current.header,
			self.current.txs,
			nil,
			self.current.receipts,
		)
	}
	return self.current.Block
}

func (self *Worker) start() {
	self.mu.Lock()
	defer self.mu.Unlock()

	atomic.StoreInt32(&self.mining, 1)

	// spin up agents
	for agent := range self.agents {
		agent.Start()
	}
}

func (self *Worker) stop() {
	self.wg.Wait()

	self.mu.Lock()
	defer self.mu.Unlock()
	if atomic.LoadInt32(&self.mining) == 1 {
		for agent := range self.agents {
			agent.Stop()
		}
	}
	atomic.StoreInt32(&self.mining, 0)
	atomic.StoreInt32(&self.atWork, 0)
}

func (self *Worker) register(agent Agent) {
	self.mu.Lock()
	defer self.mu.Unlock()
	self.agents[agent] = struct{}{}
	agent.SetReturnCh(self.recv)
}

func (self *Worker) unregister(agent Agent) {
	self.mu.Lock()
	defer self.mu.Unlock()
	delete(self.agents, agent)
	agent.Stop()
}

func (self *Worker) TxSub() event.Subscription {
	return self.txSub
}

func (self *Worker) ChainHeadSub() event.Subscription {
	return self.chainHeadSub
}

func (self *Worker) ChainSideSub() event.Subscription {
	return self.chainSideSub
}

func (self *Worker) ChainHeadCh() chan core.ChainHeadEvent {
	return self.chainHeadCh
}

func (self *Worker) ChainSideCh() chan core.ChainSideEvent {
	return self.chainSideCh
}

func (self *Worker) HandleChainSideCh(ev core.ChainSideEvent) {
	self.uncleMu.Lock()
	self.possibleUncles[ev.Block.Hash()] = ev.Block
	self.uncleMu.Unlock()
}

func (self *Worker) TxCh() chan core.TxPreEvent {
	return self.txCh
}

func (self *Worker) HandleTxCh(ev core.TxPreEvent) {
	// Apply transaction to the pending state if we're not mining
	if atomic.LoadInt32(&self.mining) == 0 {
		self.currentMu.Lock()
		acc, _ := types.Sender(self.current.signer, ev.Tx)
		txs := map[common.Address]types.Transactions{acc: {ev.Tx}}
		txset := types.NewTransactionsByPriceAndNonce(self.current.signer, txs)

		self.current.commitTransactions(self.mux, txset, self.chain, self.coinbase)
		self.currentMu.Unlock()
	} else {
		// If we're mining, but nothing is being processed, wake on new transactions
		if self.config.Clique != nil && self.config.Clique.Period == 0 {
			self.commitNewWork()
		}
	}
}

func (self *Worker) CommitNewWork() {
	self.commitNewWork()
}

func (self *Worker) Update() {
	defer self.txSub.Unsubscribe()
	defer self.chainHeadSub.Unsubscribe()
	defer self.chainSideSub.Unsubscribe()

	for {
		// A real event arrived, process interesting content
		select {
		// Handle ChainHeadEvent
		case <-self.chainHeadCh:
			// 一个新区块挂上本地主干区块链，我们需要在此基础上做下一个新区块的挖掘
			// 并挂载到这个已经挂上主干的区块后面
			// 这里需要触发一下，过滤掉已经存在的 tx 交易，重构下一个新区块的信息
			// 再提交
			if common.GetConsensusBftTest() { // only for test
				blockDelay := uint64(5) //15
				if self.eth.BlockChain().CurrentBlock().NumberU64()%baseparam.ElectionBlockCount == 2 &&
					self.eth.BlockChain().CurrentBlock().NumberU64() >= blockDelay && !common.EmptyAddress(self.coinbase) {
					self.eth.RunForSealerElection(self.coinbase, blizparam.GetCsSelfPassphrase())
				}
				if !self.eth.GetMainNodeForBFTFlag() && self.eth.BlockChain().CurrentBlock().NumberU64() >= 1 {
					continue
				} else if self.eth.GetMainNodeForBFTFlag() && self.eth.BlockChain().CurrentBlock().NumberU64() > blockDelay+5 &&
					self.eth.BlockChain().CurrentBlock().NumberU64()%baseparam.ElectionBlockCount == 1 {
					continue
				}
			}
			self.commitNewWork()

		// Handle ChainSideEvent
		case ev := <-self.chainSideCh:
			// 收集了一个侧链分叉的交易区块，侧链是因为它的 difficult 难度值
			// 不大于主干canon的同一序号的新区块，于是被编到侧链位置，这里形成
			// Uncle叔区块
			self.HandleChainSideCh(ev)
			//self.uncleMu.Lock()
			//self.possibleUncles[ev.Block.Hash()] = ev.Block
			//self.uncleMu.Unlock()

		// Handle TxPreEvent
		case ev := <-self.txCh:
			self.HandleTxCh(ev)

		// Handle ElectionPreEvent
		//case ev := <-self.electionCh:
		// Apply election to the pending state if we're not mining
		//if atomic.LoadInt32(&self.mining) == 0 {
		//	self.currentMu.Lock()
		//	acc, _ := types.Sender(self.current.signer, ev.Elect)
		//	txs := map[common.Address]types.Transactions{acc: {ev.Elect}}
		//	txset := types.NewTransactionsByPriceAndNonce(self.current.signer, txs)
		//
		//	self.current.commitTransactions(self.mux, txset, self.chain, self.coinbase)
		//	self.currentMu.Unlock()
		//} else {
		//	// If we're mining, but nothing is being processed, wake on new transactions
		//	if self.config.Clique != nil && self.config.Clique.Period == 0 {
		//		self.commitNewWork()
		//	}
		//}

		// System stopped
		case <-self.txSub.Err():
			return
		case <-self.chainHeadSub.Err():
			return
		case <-self.chainSideSub.Err():
			return
		}
	}
}

func (self *Worker) wait() {
	for {
		mustCommitNewWork := true
		// JiangHan：这里表示 worker 的 recv 通道等到了挖矿成功的新block通知
		// 早先将 recv 通道注册给了agent.SetReturnCh(self.recv)，于是agent
		// 在 returnCh 上将新挖掘到的区块返回，这个新区块已经完成了POW计算，即将向
		// 全网广播（新区块就是之前在 self.update()的commitNewWork函数中提交的
		// 本地主干canon block）
		for result := range self.recv {
			atomic.AddInt32(&self.atWork, -1)

			if result == nil {
				continue
			}
			block := result.Block
			work := result.Work

			// Update the block hash in all logs since it is now available and not when the
			// receipt/log of individual transactions were created.
			for _, r := range work.receipts {
				for _, l := range r.Logs {
					l.BlockHash = block.Hash()
				}
			}
			for _, log := range work.state.Logs() {
				log.BlockHash = block.Hash()
			}
			stat, err := self.chain.WriteBlockWithState(block, work.receipts, work.state)
			if err != nil {
				self.log.Error("Failed writing block to chain", "err", err)
				continue
			}
			// check if canon block and write transactions
			if stat == core.CanonStatTy {
				// implicit by posting ChainHeadEvent
				mustCommitNewWork = false
			}
			// Broadcast the block and announce chain insertion event
			self.mux.Post(core.NewMinedBlockEvent{Block: block})
			var (
				events []interface{}
				logs   = work.state.Logs()
			)
			events = append(events, core.ChainEvent{Block: block, Hash: block.Hash(), Logs: logs})
			if stat == core.CanonStatTy {
				events = append(events, core.ChainHeadEvent{Block: block})
			}
			self.chain.PostChainEvents(events, logs)

			if block.IsElectionBlock() {
				totalShard := common.GetTotalSharding(block.NumberU64())
				lastElectionBlock := self.eth.BlockChain().GetBlockByNumber(block.NumberU64() - baseparam.ElectionBlockCount)
				lastVerified := lastElectionBlock.GetValidHeaders()
				verified := block.GetValidHeaders()
				for id := uint16(0); id < totalShard; id++ {
					if verified[id] != nil && lastVerified[id] != nil && verified[id].BlockHeight > lastVerified[id].BlockHeight {
						self.eth.VerifiedHeaderPool().ClearSharding(id)
					}
				}
			}

			// Insert the block into the set of pending ones to wait for confirmations
			self.unconfirmed.Insert(block.NumberU64(), block.Hash())

			if mustCommitNewWork {
				self.commitNewWork()
			}
		}
	}
}

// push sends a new work task to currently live miner agents.
func (self *Worker) push(work *Work) {
	if atomic.LoadInt32(&self.mining) != 1 {
		return
	}
	for agent := range self.agents {
		atomic.AddInt32(&self.atWork, 1)
		if ch := agent.Work(); ch != nil {
			ch <- work
		}
	}
}

// makeCurrent creates a new environment for the current cycle.
func (self *Worker) makeCurrent(parent *types.Block, header *types.Header) error {
	state, err := self.chain.StateAt(parent.Root())
	if err != nil {
		return err
	}
	work := &Work{
		config:    self.config,
		signer:    types.NewEIP155Signer(self.config.ChainId),
		state:     state,
		ancestors: set.New(),
		family:    set.New(),
		uncles:    set.New(),
		header:    header,
		createdAt: time.Now(),
	}

	// when 08 is processed ancestors contain 07 (quick block)
	for _, ancestor := range self.chain.GetBlocksFromHash(parent.Hash(), 7) {
		for _, uncle := range ancestor.Uncles() {
			work.family.Add(uncle.Hash())
		}
		work.family.Add(ancestor.Hash())
		work.ancestors.Add(ancestor.Hash())
	}

	// Keep track of transactions which return errors so they can be removed
	work.tcount = 0
	self.current = work
	return nil
}

// JiangHan：收集本地的 pending tx, 并打包新区块，提交给minor agent做seal封装
// minor agent 的默认实现 CpuAgent 的 work 队列只容纳一个元素，所以提交不了的就
// 堵塞排队，如果 agent 比较多，则可以分发并行处理（也就是提到的所谓矿池）
func (self *Worker) commitNewWork() {
	self.mu.Lock()
	defer self.mu.Unlock()
	self.uncleMu.Lock()
	defer self.uncleMu.Unlock()
	self.currentMu.Lock()
	defer self.currentMu.Unlock()

	tstart := time.Now()
	parent := self.chain.CurrentBlock()

	tstamp := tstart.UnixNano() / 1e6
	if parent.Time().Cmp(new(big.Int).SetInt64(tstamp)) >= 0 {
		tstamp = parent.Time().Int64() + 1
	}
	// this will ensure we're not going off too far in the future
	if now := time.Now().UnixNano() / 1e6; tstamp > now+1 {
		wait := time.Duration(tstamp-now) * time.Millisecond
		self.log.Info("Mining too far in the future", "wait", common.PrettyDuration(wait))
		time.Sleep(wait)
	}

	num := parent.Number()
	header := &types.Header{
		ParentHash: parent.Hash(),
		Number:     num.Add(num, common.Big1),
		GasLimit:   core.CalcGasLimit(parent),
		Extra:      self.extra,
		Time:       big.NewInt(tstamp),
	}
	// Only set the coinbase if we are mining (avoid spurious block rewards)
	if atomic.LoadInt32(&self.mining) == 1 {
		header.Coinbase = self.coinbase
	}
	if err := self.engine.Prepare(self.chain, header); err != nil {
		self.log.Error("Failed to prepare header for mining", "err", err)
		return
	}
	// Could potentially happen if starting to mine in an odd state.
	err := self.makeCurrent(parent, header)
	if err != nil {
		self.log.Error("Failed to create mining context", "err", err)
		return
	}
	// Create the current work task and check any fork transitions needed
	work := self.current

	// JiangHan:从本地 pending 交易池 txpool 中选取交易进行签名并执行 commitTransactions
	// 将它们挂在新work上
	pending, err := self.eth.TxPool().Pending()
	if err != nil {
		self.log.Error("Failed to fetch pending transactions", "err", err)
		return
	}
	txs := types.NewTransactionsByPriceAndNonce(self.current.signer, pending)
	work.commitTransactions(self.mux, txs, self.chain, self.coinbase)

	// compute uncles for the new block.
	var (
		uncles    []*types.Header
		badUncles []common.Hash
	)
	for hash, uncle := range self.possibleUncles {
		if len(uncles) == 2 {
			break
		}
		if err := self.commitUncle(work, uncle.Header()); err != nil {
			log.Trace("Bad uncle found and will be removed", "hash", hash)
			log.Trace(fmt.Sprint(uncle))

			badUncles = append(badUncles, hash)
		} else {
			log.Debug("Committing new uncle to block", "hash", hash)
			uncles = append(uncles, uncle.Header())
		}
	}
	for _, hash := range badUncles {
		delete(self.possibleUncles, hash)
	}

	// JiangHan：收集并执行完所有当前transaction, 这里进行区块封装
	// Create the new block to seal with the consensus engine
	if work.Block, err = self.engine.Finalize(self.chain, header, work.state, work.txs, uncles, work.receipts); err != nil {
		self.log.Error("Failed to finalize block for sealing", "err", err)
		return
	}

	if header.IsElectionBlock() {
		electionPool := self.eth.ElectionPool()
		currentBlockNum := self.chain.CurrentBlock().NumberU64()
		// Handle the sealers and police election
		electionSealers := electionPool.PendingSealers()
		newSealers := make([]common.AddressNode, 0)
		for addr, elect := range electionSealers {
			if err := electionPool.ElectionIsValid(&elect, currentBlockNum); err != nil {
				self.log.Warn("The election in election pool is not valid, discarded")
				continue
			}
			newSealers = append(newSealers, common.AddressNode{Address: addr, NodeStr: elect.NodeStr})
		}
		electionPolice := electionPool.PendingPolice()
		newPolice := make([]common.AddressNode, 0)
		for addr, elect := range electionPolice {
			if err := electionPool.ElectionIsValid(&elect, currentBlockNum); err != nil {
				self.log.Warn("The election in election pool is not valid, discarded")
				continue
			}
			newPolice = append(newPolice, common.AddressNode{Address: addr, NodeStr: elect.NodeStr})
		}
		self.commitElections(newSealers, baseparam.TypeSealersElection)
		self.commitElections(newPolice, baseparam.TypePoliceElection)

		// Update block heights verified by police
		verifiedHeights, useLast, err := self.eth.VerifiedHeaderPool().Verified(header.Number.Uint64())
		if verifiedHeights != nil && err == nil {
			work.Block.SetValidHeaders(verifiedHeights)
		} else {
			self.log.Error("Failed to set verified valid headers", "err", err)
			return
		}
		for i := 0; i < len(useLast); i++ {
			if useLast[i] {
				self.log.Info(fmt.Sprintf("No new verified headers in shard %d, use the latest instead", i))
			}
		}
	}
	// We only care about logging if we're actually mining.
	if atomic.LoadInt32(&self.mining) == 1 {
		self.log.Info("Commit new mining work", "number", work.Block.Number(), "txs", work.tcount, "uncles", len(uncles), "elapsed", common.PrettyDuration(time.Since(tstart)))
		self.unconfirmed.Shift(work.Block.NumberU64() - 1)
	}
	self.push(work)
}

func (self *Worker) commitElections(newElectors []common.AddressNode, electionType uint8) {
	if electionType == baseparam.TypeSealersElection {
		self.log.Info("commit new election for sealers", "newSealers len", len(newElectors))
	} else {
		self.log.Info("commit new election for police", "newPolice len", len(newElectors))
	}

	work := self.current
	if len(newElectors) >= baseparam.MinElectionPoolLen /*&& len(newSealers) <= baseparam.MaxElectionPoolLen*/ {
		work.Block.SetNextEleElectors(newElectors, electionType)
	} else {
		self.log.Info("The election in this block is invalid without enough candidates", "we have", len(newElectors))

		lastElectors := self.getLastElectors(electionType)
		if len(lastElectors) >= baseparam.MinElectionPoolLen /*&& len(newSealers) <= baseparam.MaxElectionPoolLen*/ {
			work.Block.SetNextEleElectors(lastElectors, electionType)
		}
	}
}

func (self *Worker) getLastElectors(electionType uint8) []common.AddressNode {
	lastElectors := make([]common.AddressNode, 0)
	currentBlockNum := self.chain.CurrentBlock().NumberU64() + 1
	lastElectionBlockNum := currentBlockNum - currentBlockNum%baseparam.ElectionBlockCount -
		baseparam.ElectionBlockCount

	if lastElectionBlockNum >= 0 {
		block := self.chain.GetBlockByNumber(uint64(lastElectionBlockNum))
		if block != nil {
			if electionType == baseparam.TypeSealersElection {
				lastElectors = block.NextSealers()
			} else if electionType == baseparam.TypePoliceElection {
				lastElectors = block.NextPolice()
			}
		}
	}

	return lastElectors
}

func (self *Worker) commitUncle(work *Work, uncle *types.Header) error {
	hash := uncle.Hash()
	if work.uncles.Has(hash) {
		return fmt.Errorf("uncle not unique")
	}
	if !work.ancestors.Has(uncle.ParentHash) {
		return fmt.Errorf("uncle's parent unknown (%x)", uncle.ParentHash[0:4])
	}
	if work.family.Has(hash) {
		return fmt.Errorf("uncle already in family (%x)", hash)
	}
	work.uncles.Add(uncle.Hash())
	return nil
}

func (env *Work) commitTransactions(mux *event.TypeMux, txs *types.TransactionsByPriceAndNonce, bc *core.BlockChain, coinbase common.Address) {
	gp := new(core.GasPool).AddGas(env.header.GasLimit)

	var coalescedLogs []*types.Log

	for {
		// If we don't have enough gas for any further transactions then we're done
		if gp.Gas() < params.TxGas {
			log.Trace("Not enough gas for further transactions", "gp", gp)
			break
		}
		// Retrieve the next transaction and abort if all done
		tx := txs.Peek()
		if tx == nil {
			break
		}
		// Error may be ignored here. The error has already been checked
		// during transaction acceptance is the transaction pool.
		//
		// We use the eip155 signer regardless of the current hf.
		from, _ := types.Sender(env.signer, tx)
		// Check whether the tx is replay protected. If we're not in the EIP155 hf
		// phase, start ignoring the sender until we do.
		if tx.Protected() && !env.config.IsEIP155(env.header.Number) {
			log.Trace("Ignoring reply protected transaction", "hash", tx.Hash(), "eip155", env.config.EIP155Block)

			txs.Pop()
			continue
		}
		// Start executing the transaction
		// jh -- tx:transaction hash;
		// jh -- common.Hash:block hash byte 数组；
		// jh -- env.tcount: current traction total(index)
		env.state.Prepare(tx.Hash(), common.Hash{}, env.tcount)

		// JiangHan: 开始执行交易，带入coinbase(主账户），tx（交易信息），bc(当前区块链），gp(用户提供的gaspool,用户扣gas)
		err, logs := env.commitTransaction(tx, bc, coinbase, gp)
		switch err {
		case core.ErrGasLimitReached:
			// Pop the current out-of-gas transaction without shifting in the next from the account
			log.Trace("Gas limit exceeded for current block", "sender", from)
			txs.Pop()

		case core.ErrNonceTooLow:
			// New head notification data race between the transaction pool and miner, shift
			log.Trace("Skipping transaction with low nonce", "sender", from, "nonce", tx.Nonce())
			txs.Shift()

		case core.ErrNonceTooHigh:
			// Reorg notification data race between the transaction pool and miner, skip account =
			log.Trace("Skipping account with hight nonce", "sender", from, "nonce", tx.Nonce())
			txs.Pop()

		case nil:
			// Everything ok, collect the logs and shift in the next transaction from the same account
			coalescedLogs = append(coalescedLogs, logs...)
			env.tcount++
			txs.Shift()

		default:
			// Strange error, discard the transaction and get the next in line (note, the
			// nonce-too-high clause will prevent us from executing in vain).
			log.Debug("Transaction failed, account skipped", "hash", tx.Hash(), "err", err)
			txs.Shift()
		}
	}

	if len(coalescedLogs) > 0 || env.tcount > 0 {
		// make a copy, the state caches the logs and these logs get "upgraded" from pending to mined
		// logs by filling in the block hash when the block was mined by the local miner. This can
		// cause a race condition if a log was "upgraded" before the PendingLogsEvent is processed.
		cpy := make([]*types.Log, len(coalescedLogs))
		for i, l := range coalescedLogs {
			cpy[i] = new(types.Log)
			*cpy[i] = *l
		}
		go func(logs []*types.Log, tcount int) {
			if len(logs) > 0 {
				mux.Post(core.PendingLogsEvent{Logs: logs})
			}
			if tcount > 0 {
				mux.Post(core.PendingStateEvent{})
			}
		}(cpy, env.tcount)
	}
}

func (env *Work) commitTransaction(tx *types.Transaction, bc *core.BlockChain, coinbase common.Address, gp *core.GasPool) (error, []*types.Log) {
	// JiangHan: 执行交易前先拍个快照，发生错误就恢复快照内容还原
	snap := env.state.Snapshot()

	// JiangHan: （重点：交易执行点一）这里应该是 work 从其他用户节点收集交易请求，并执行它们
	receipt, _, err := core.ApplyTransaction(env.config, bc, &coinbase, gp, env.state, env.header, tx, &env.header.GasUsed, vm.Config{})
	if err != nil {
		// 有错误发生，还原快照
		env.state.RevertToSnapshot(snap)
		return err, nil
	}
	env.txs = append(env.txs, tx)
	env.receipts = append(env.receipts, receipt)

	return nil, receipt.Logs
}
