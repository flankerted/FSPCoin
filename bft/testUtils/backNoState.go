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

package testUtils

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/contatract/go-contatract/accounts"
	"github.com/contatract/go-contatract/accounts/keystore"
	"github.com/contatract/go-contatract/bft/consensus/hbft"
	"github.com/contatract/go-contatract/bft/nodeHbft"
	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/consensus"
	types "github.com/contatract/go-contatract/core/types_elephant"
	core "github.com/contatract/go-contatract/core_elephant"
	"github.com/contatract/go-contatract/core_elephant/state"
	"github.com/contatract/go-contatract/eth"
	"github.com/contatract/go-contatract/event"
	"github.com/contatract/go-contatract/log"
	sealer "github.com/contatract/go-contatract/miner_elephant"
	"github.com/contatract/go-contatract/params"
)

type BackendForTestWithoutStateDB struct {
	// Node information
	NodeName  string
	NodeIndex int

	// Events subscription scope
	scope event.SubscriptionScope

	// Simulated elephant
	DataDir      string
	Accman       *accounts.Manager
	EtherBase    common.Address
	Blocks       []*types.Block
	BlockHashMap map[common.Hash]*types.Block
	BlockNumMap  map[uint64]*types.Block
	engineTest   consensus.EngineCtt
	blockChain   *core.BlockChain
	bftSealer    *sealer.Miner

	futureBFTBlocks []*types.Block // In block chain

	chainCh   chan core.ChainEvent // New block connected event in elephant
	chainSub  event.Subscription   // New block connected event in elephant
	chainFeed event.Feed           // New block connected event in elephant

	eventMux *event.TypeMux

	// Simulated sealer
	mu            sync.Mutex
	newBlockCh    chan *types.Block
	stop          chan struct{}
	quitCurrentOp chan struct{}
	found         chan *foundResult
	stopTest      chan struct{}

	futureParentBlock *types.Block // In sealer

	chainHeadCh   chan core.ChainHeadEvent // New block connected event in sealer
	chainHeadSub  event.Subscription       // New block connected event in sealer
	chainHeadFeed event.Feed               // New block connected event in sealer

	// New block net transmitting event for test
	chainNetSub event.Subscription // New block net transmitting event in elephant

	// BFT messages net transmitting event for test
	bftMsgNetSub event.Subscription  // BFT messages net transmitting event in elephant
	bftMsgNetCh  chan bftMsgNetEvent // BFT messages net transmitting event in elephant only for reply

	// Test controller parameter
	TestBlocksCntTotal uint64

	Lamp *eth.Ethereum

	Log log.Logger
}

func NewBackendForTestWithoutStateDB(nodeName string, index int, lamp *eth.Ethereum, logger log.Logger) (*BackendForTestWithoutStateDB, error) {
	backend := &BackendForTestWithoutStateDB{
		NodeName:      nodeName,
		NodeIndex:     index,
		DataDir:       defaultDataDir(nodeName),
		Blocks:        make([]*types.Block, 0),
		BlockHashMap:  make(map[common.Hash]*types.Block),
		BlockNumMap:   make(map[uint64]*types.Block),
		chainCh:       make(chan core.ChainEvent, core.ChainHeadChanSize),
		eventMux:      new(event.TypeMux),
		newBlockCh:    make(chan *types.Block),
		stop:          make(chan struct{}),
		quitCurrentOp: make(chan struct{}),
		found:         make(chan *foundResult),
		stopTest:      make(chan struct{}),
		chainHeadCh:   make(chan core.ChainHeadEvent, core.ChainHeadChanSize),
		bftMsgNetCh:   make(chan bftMsgNetEvent),
		Lamp:          lamp,
		Log:           logger,
	}

	if backend.DataDir != "" {
		absdatadir, err := filepath.Abs(backend.DataDir)
		if err != nil {
			return nil, err
		}
		backend.DataDir = absdatadir
	}

	// Ensure that the AccountManager method works before the node has started.
	am, err := makeAccountManager(backend.AccountConfig)
	if err != nil {
		return nil, err
	}
	backend.Accman = am
	account, errAcc := backend.GetAccount()
	if errAcc != nil {
		return nil, errAcc
	}
	backend.EtherBase = common.HexToAddress(account)
	BftNodeEtherBasesGlobal = append(BftNodeEtherBasesGlobal, backend.EtherBase)
	BftNodeMapAddressIndex[backend.EtherBase] = index

	genesisBlock := defaultGenesisBlock()
	backend.Blocks = append(backend.Blocks, genesisBlock)
	backend.BlockNumMap[0] = genesisBlock
	backend.BlockHashMap[genesisBlock.Hash()] = genesisBlock
	InsertGlobalBlock(genesisBlock)

	//backend.chainSub = backend.SubscribeChainEvent(backend.chainCh)
	backend.chainNetSub = backend.SubscribeChainNetEvent(chainNetCh)
	backend.bftMsgNetSub = backend.SubscribeBftMsgNetEvent(bftMsgNetCh)
	BftNodeBackNoStateGlobal[index] = backend

	// Create HBFT node
	bftNode, err := nodeHbft.NewNode(lamp, backend, logger)
	if err != nil {
		return nil, err
	}
	bftNode.InitNodeTable()
	bftNode.SubscribeChainHeadEvent()
	bftNode.SetPassphrase(nodeName)
	BftNodesGlobal[index] = bftNode

	// Create consensus engine
	backend.engineTest = CreateConsensusEngine(index, bftNode)

	// Create core.BlockChain
	backend.blockChain, err = core.NewBlockChainForTest(backend.engineTest, lamp, &backend.EtherBase)
	if err != nil {
		return nil, err
	}

	// Create sealer
	backend.bftSealer = sealer.New(backend, params.ElephantDefChainConfig, backend.EventMux(),
		backend.engineTest, lamp, logger)
	if backend.bftSealer == nil {
		return nil, errors.New("failed to create sealer")
	}
	BftSealersGlobal[index] = backend.bftSealer

	return backend, nil
}

// AccountConfig determines the settings for scrypt and keydirectory
func (b *BackendForTestWithoutStateDB) AccountConfig() (int, int, string) {
	scryptN := keystore.StandardScryptN
	scryptP := keystore.StandardScryptP
	keydir := filepath.Join(b.DataDir, datadirDefaultKeyStore)
	return scryptN, scryptP, keydir
}

// GetAccAddresses returns the collection of accounts this node manages
func (b *BackendForTestWithoutStateDB) GetAccAddresses() []string {
	addresses := make([]string, 0) // return [] instead of nil if empty
	for _, wallet := range b.Accman.Wallets() {
		for _, account := range wallet.Accounts() {
			addresses = append(addresses, account.Address.String())
		}
	}
	return addresses
}

func (b *BackendForTestWithoutStateDB) GetAccount() (string, error) {
	accs := b.GetAccAddresses()
	if len(accs) != 0 {
		strAccounts := ""
		for _, acc := range accs {
			strAccounts += acc
			break
		}
		return strAccounts, nil
	} else {
		newAcc, err := newAccount(b.Accman, b.NodeName)
		if err != nil {
			return "", err
		} else {
			return newAcc, nil
		}
	}
}

func (b *BackendForTestWithoutStateDB) BlockChain() *core.BlockChain {
	return b.blockChain
}

func (b *BackendForTestWithoutStateDB) TxPool() *core.TxPool {
	return nil
}

func (b *BackendForTestWithoutStateDB) GenerateShardingTxBatches(block *types.Block, stateDB *state.StateDB) (types.ShardingTxBatches,
	map[uint16]uint64, error) {
	return nil, nil, nil
}

func (b *BackendForTestWithoutStateDB) EventMux() *event.TypeMux {
	return b.eventMux
}

func (b *BackendForTestWithoutStateDB) CommitNewWork() {
	if uint64(len(b.Blocks)) <= b.TestBlocksCntTotal && !BftTestCaseStopped {
		BftSealersGlobal[b.NodeIndex].Worker().CommitNewWork()
	} else {
		BftSealersGlobal[b.NodeIndex].Stop()
		BftTestCaseStopped = true
	}
}

func (b *BackendForTestWithoutStateDB) GetBlockByNumber(num uint64) *types.Block {
	b.mu.Lock()
	defer b.mu.Unlock()

	if block, ok := b.BlockNumMap[num]; ok {
		return block
	} else {
		return nil
	}
}

func (b *BackendForTestWithoutStateDB) GetBlockByHash(hash common.Hash) *types.Block {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.BlockHashMap[hash]
}

// BFTNewestFutureBlock retrieves the newest future head in BFT consensus of the block chain.
func (b *BackendForTestWithoutStateDB) BFTNewestFutureBlock() *types.Block {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.futureBFTBlocks) > 0 {
		return b.futureBFTBlocks[len(b.futureBFTBlocks)-1]
	} else {
		return nil
	}
}

// ClearBFTFutureBlock clears the future block in BFT consensus of the block chain.
func (b *BackendForTestWithoutStateDB) ClearBFTFutureBlock(hash common.Hash) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for i := 0; i < len(b.futureBFTBlocks); {
		if hash.Equal(b.futureBFTBlocks[i].Hash()) {
			b.futureBFTBlocks = append(b.futureBFTBlocks[:i], b.futureBFTBlocks[i+1:]...)
		} else {
			i++
		}
	}
}

// ClearOutDatedBFTFutureBlock clears the outdated future blocks in BFT consensus of the block chain.
func (b *BackendForTestWithoutStateDB) ClearOutDatedBFTFutureBlock(height uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for i := 0; i < len(b.futureBFTBlocks); {
		if b.futureBFTBlocks[i].NumberU64() <= height {
			b.futureBFTBlocks = append(b.futureBFTBlocks[:i], b.futureBFTBlocks[i+1:]...)
		} else {
			i++
		}
	}
}

func (b *BackendForTestWithoutStateDB) StartBlockSyncLoop() {
	for {
		if BftTestCaseStopped {
			return
		}

		BftNodeGlobalMu.Lock()
		bftNodeBlockNumMapLength := len(BftNodeBlockNumMap)
		BftNodeGlobalMu.Unlock()

		blockLength := len(b.Blocks)
		if blockLength < bftNodeBlockNumMapLength {
			LogWarn("---The length of blocks is not consistent with the global's", "len(b.Blocks)", blockLength,
				"len(BftNodeBlockNumMap)", bftNodeBlockNumMapLength)
			num := bftNodeBlockNumMapLength - 1
			for i := 0; i < bftNodeBlockNumMapLength-blockLength; i++ {
				go func(j int) {
					BftNodeGlobalMu.Lock()
					block, ok := BftNodeBlockNumMap[uint64(num+j)]
					BftNodeGlobalMu.Unlock()
					if !ok {
						return
					}

					BftNodesGlobal[b.NodeIndex].Info(fmt.Sprintf("Syncronising the block to connect, Hash: %s",
						block.Hash().TerminalString()))
					LogInfo(fmt.Sprintf("---Syncronising the block to connect, Hash: %s",
						block.Hash().TerminalString()))
					if parent := b.blockChain.GetBlock(block.ParentHash(), block.NumberU64()-1); parent == nil {
						b.waitForParentBlock(block.ParentHash())
					}

					b.connectReceivedBlock(block)
				}(i)
			}
		}

		time.Sleep(time.Second * 3)
	}
}

func (b *BackendForTestWithoutStateDB) StartBlockEventLoop() {
	defer b.chainNetSub.Unsubscribe()

	for {
		select {
		case ev := <-chainNetCh: // from simulated net
			go func() {
				b.mu.Lock()
				blockWeHave, ok := b.BlockHashMap[ev.Block.Hash()]
				b.mu.Unlock()

				if !ok {
					LogInfo(fmt.Sprintf("---Receive new block in %s", b.NodeName),
						"Hash", ev.Block.Hash().TerminalString())
					if parent := b.blockChain.GetBlock(ev.Block.ParentHash(), ev.Block.NumberU64()-1); parent == nil {
						b.waitForParentBlock(ev.Block.ParentHash())
					}

					b.connectReceivedBlock(ev.Block)
				} else {
					blockHashWithBft := blockWeHave.HashWithBft()
					if !blockHashWithBft.Equal(ev.HashWithBft) {
						if ev.Block.HBFTStageChain()[1].ViewID > blockWeHave.HBFTStageChain()[1].ViewID ||
							(ev.Block.HBFTStageChain()[1].ViewID == blockWeHave.HBFTStageChain()[1].ViewID &&
								ev.Block.HBFTStageChain()[1].Timestamp > blockWeHave.HBFTStageChain()[1].Timestamp) {
							b.mu.Lock()
							b.Blocks[ev.Block.NumberU64()] = ev.Block
							b.BlockHashMap[ev.Block.Hash()] = ev.Block
							b.BlockNumMap[ev.Block.NumberU64()] = ev.Block

							InsertGlobalBlock(ev.Block)
							b.mu.Unlock()

							BftNodesGlobal[b.NodeIndex].Info(fmt.Sprintf("Replaced new block in %s instead of the block", b.NodeName),
								"Hash", ev.Block.Hash().TerminalString())
							LogInfo(fmt.Sprintf("---Replaced new block in %s instead of the block successfully", b.NodeName),
								"Hash", ev.Block.Hash().TerminalString(), "HashWithBft", ev.Block.HashWithBft())

							b.BroadcastBlock(ev.Block)
						}
					}
				}
			}()

		// System stopped
		case <-b.chainNetSub.Err():
			LogInfo("---Quit Loop of StartBlockEventLoop(), due to b.chainNetSub.Err()", "node", b.EtherBase.String())
			return
		}
	}
}

func (b *BackendForTestWithoutStateDB) waitForParentBlock(hash common.Hash) {
	BftNodesGlobal[b.NodeIndex].Info(fmt.Sprintf("Waiting for the block to connect, Hash: %s", hash.TerminalString()))
	for {
		if BftTestCaseStopped {
			time.Sleep(time.Second)
			return
		}
		time.Sleep(nodeHbft.BlockSealingBeat)
		if _, ok := b.BlockHashMap[hash]; ok {
			BftNodesGlobal[b.NodeIndex].Info(fmt.Sprintf("The block connected, Hash: %s, stop waiting", hash.TerminalString()))
			return
		}
		BftNodeGlobalMu.Lock()
		block, ok := BftNodeBlockHashMap[hash]
		BftNodeGlobalMu.Unlock()
		if ok {
			if parent := b.blockChain.GetBlock(block.ParentHash(), block.NumberU64()-1); parent == nil {
				b.waitForParentBlock(block.ParentHash())
			}
			b.connectReceivedBlock(block)
			BftNodesGlobal[b.NodeIndex].Info(fmt.Sprintf("The block connected, Hash: %s, stop waiting", hash.TerminalString()))
			return
		}
	}
}

func (b *BackendForTestWithoutStateDB) StartSealerUpdateLoop(self *sealer.Worker) {
	defer self.TxSub().Unsubscribe()
	defer self.ChainHeadSub().Unsubscribe()
	defer self.ChainSideSub().Unsubscribe()

	for {
		select {
		//case ev := <-b.chainCh:
		case ev := <-self.ChainHeadCh(): //sealer internal
			go func() {
				b.BroadcastBlock(ev.Block)
				b.CommitNewWork()
			}()

		// System stopped
		case <-self.ChainHeadSub().Err():
			return
		case <-self.ChainSideSub().Err():
			return
		}
	}
}

func (b *BackendForTestWithoutStateDB) BroadcastBlock(block *types.Block) {
	ev := core.ChainEvent{Block: block, HashWithBft: block.HashWithBft()}
	chainNetFeed.Send(ev)

	BftNodesGlobal[b.NodeIndex].Info(fmt.Sprintf("Broadcast new block in %s", b.NodeName),
		"Hash", ev.Block.Hash().TerminalString())
	LogInfo(fmt.Sprintf("---Broadcast new block in %s", b.NodeName),
		"Hash", ev.Block.Hash().TerminalString())
}

func (b *BackendForTestWithoutStateDB) StartBFTMsgEventLoop() {
	for {
		select {
		case ev := <-bftMsgNetCh:
			if ev.BFTMsg.ViewPrimary != b.EtherBase.String() {
				go b.handleBftMsg(ev.BFTMsg)
			}

		case ev := <-b.bftMsgNetCh:
			go b.handleBftMsg(ev.BFTMsg)

		// Be unsubscribed due to system stopped
		case <-b.bftMsgNetSub.Err():
			LogInfo("---Quit Loop of StartBFTMsgEventLoop(), due to b.bftMsgNetSub.Err()", "node", b.EtherBase.String())
			return
		}
	}
}

func (b *BackendForTestWithoutStateDB) handleBftMsg(msg *hbft.BFTMsg) {
	if DisableNode(b.NodeIndex) {
		return
	}
	if !RandomDelayMsForHBFT(b.NodeIndex, "handleBftMsg", true) {
		return
	}

	if msg.MsgType == hbft.MsgOldestFuture {
		go BftNodesGlobal[b.NodeIndex].HbftProcess(msg.MsgOldestFuture)
		return
	}

	if !BftNodesGlobal[b.NodeIndex].IsBusy() {
		switch msg.MsgType {
		case hbft.MsgPreConfirm:
			//*viewPrimary = msg.MsgPreConfirm.ViewPrimary
			go BftNodesGlobal[b.NodeIndex].HbftProcess(msg.MsgPreConfirm)

		case hbft.MsgConfirm:
			//*viewPrimary = msg.MsgConfirm.ViewPrimary
			go BftNodesGlobal[b.NodeIndex].HbftProcess(msg.MsgConfirm)

		}
		//b.CreateBftMsgChan(*viewPrimary)
		//break waitReceive

	} else {
		switch msg.MsgType {
		case hbft.MsgReply:
			if msg.ViewPrimary == b.EtherBase.String() {
				go BftNodesGlobal[b.NodeIndex].HbftProcess(msg.MsgReply)
			}
		}
	}
}

func (b *BackendForTestWithoutStateDB) connectReceivedBlock(block *types.Block) {
	b.insertBlock(block)

	// BFT node handle
	evH := core.ChainHeadEvent{Block: block} // To BFT node
	b.chainHeadFeed.Send(evH)

	// Sealer handle
	b.CommitNewWork()
}

func (b *BackendForTestWithoutStateDB) insertBlock(block *types.Block) {
	if err := b.engineTest.VerifyBFT(b.blockChain, block); err != nil {
		fmt.Println("---Insert block failed")
		BftNodesGlobal[b.NodeIndex].Error(fmt.Sprintf("InsertBlock failed, %s", err.Error()))
		LogCrit(fmt.Sprintf("---InsertBlock failed, %s", err.Error()), "node", b.EtherBase.String())
	}

	b.mu.Lock()
	if _, ok := b.BlockHashMap[block.Hash()]; !ok {
		b.Blocks = append(b.Blocks, block)
		b.BlockHashMap[block.Hash()] = block
		b.BlockNumMap[block.NumberU64()] = block

		InsertGlobalBlock(block)
		BftNodesGlobal[b.NodeIndex].Info(fmt.Sprintf("Inserted new block in %s", b.NodeName),
			"Hash", block.Hash().TerminalString())
		LogInfo(fmt.Sprintf("---Inserted new block in %s successfully", b.NodeName),
			"Hash", block.Hash().TerminalString(), "number", block.NumberU64(), "HashWithBft", block.HashWithBft())
	}
	b.mu.Unlock()

	b.ClearBFTFutureBlock(block.Hash())
	b.ClearOutDatedBFTFutureBlock(block.NumberU64())

	for i, future := range b.BFTFutureBlocks() {
		BftNodesGlobal[b.NodeIndex].Info(fmt.Sprintf("BFT Future block %d: in %s", i, future.Hash().TerminalString()))
	}
}

func (b *BackendForTestWithoutStateDB) ConnectBlock(self *sealer.Worker, block *types.Block, isFutureBlock bool, mustCommitNewWork *bool) {
	b.mu.Lock()
	_, ok := b.BlockHashMap[block.Hash()]
	b.mu.Unlock()

	if !ok {
		if parent := b.blockChain.GetBlock(block.ParentHash(), block.NumberU64()-1); parent == nil {
			b.Log.Info("ConnectBlock failed, missing parent block", "hash", block.Hash(), "parent", block.ParentHash())
		}

		b.insertBlock(block)
	} else {
		b.ClearBFTFutureBlock(block.Hash())
		b.ClearOutDatedBFTFutureBlock(block.NumberU64())

		for i, future := range b.BFTFutureBlocks() {
			BftNodesGlobal[b.NodeIndex].Info(fmt.Sprintf("BFT Future block %d: in %s", i, future.Hash().TerminalString()))
		}
	}

	var eventsSealer []interface{}
	eventsSealer = append(eventsSealer, core.ChainHeadEvent{Block: block}) // To sealer
	self.BlockChain().PostChainEvents(eventsSealer)

	var events []interface{}
	events = append(events, core.ChainHeadEvent{Block: block}) // To BFT node
	b.PostChainEvents(events)

	if !isFutureBlock && *mustCommitNewWork {
		b.CommitNewWork()
	}
}

func (b *BackendForTestWithoutStateDB) PostChainEvents(events []interface{}) {
	// post event logs for further processing
	for _, event := range events {
		switch ev := event.(type) {
		case core.ChainEvent:
			b.chainFeed.Send(ev)

		case core.ChainHeadEvent:
			b.chainHeadFeed.Send(ev)
		}
	}
}

func (b *BackendForTestWithoutStateDB) Etherbase() (eb common.Address, err error) {
	return b.EtherBase, nil
}

func (b *BackendForTestWithoutStateDB) AccountManager() *accounts.Manager {
	return b.Accman
}

func (b *BackendForTestWithoutStateDB) SendBFTMsg(msg hbft.MsgHbftConsensus, msgType uint) {
	if DisableNode(b.NodeIndex) {
		return
	}
	if !RandomDelayMsForHBFT(b.NodeIndex, "SendBFTMsg", true) {
		return
	}

	bftMsg := hbft.NewBFTMsg(msg, msgType)
	if bftMsg == nil {
		return
	}

	ev := bftMsgNetEvent{bftMsg, bftMsg.ViewPrimary}
	if !bftMsg.IsReply() {
		bftMsgNetFeed.Send(ev)
	} else {
		for i, nodeTest := range BftNodesGlobal {
			if nodeTest.CoinBase.String() == bftMsg.ViewPrimary {
				BftNodeBackNoStateGlobal[i].GetBFTMsgChanForReply() <- ev
				return
			}
		}
	}
}

func (b *BackendForTestWithoutStateDB) GetBFTMsgChanForReply() chan bftMsgNetEvent {
	return b.bftMsgNetCh
}

func (b *BackendForTestWithoutStateDB) GetCurRealSealers() ([]common.Address, bool) {
	return BftNodeEtherBasesGlobal, true
}

func (b *BackendForTestWithoutStateDB) GetRealSealersByNum(number uint64) ([]common.Address, bool) {
	return BftNodeEtherBasesGlobal, false
}

func (b *BackendForTestWithoutStateDB) Get2fRealSealersCnt() int {
	sealers, _ := b.GetCurRealSealers()
	f := (len(sealers) - 1) / 3
	for offset := 0; ; offset++ {
		if 3*(f+offset)+1 >= len(sealers) {
			return 2 * (f + offset)
		}
	}
}

func (b *BackendForTestWithoutStateDB) GetBlock(hash common.Hash, number uint64) *types.Block {
	return b.blockChain.GetBlock(hash, number)
}

func (b *BackendForTestWithoutStateDB) GetCurrentBlock() *types.Block {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.Blocks[len(b.Blocks)-1]
}

func (b *BackendForTestWithoutStateDB) AddBFTFutureBlock(block *types.Block) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.futureBFTBlocks = append(b.futureBFTBlocks, block)
}

func (b *BackendForTestWithoutStateDB) BFTFutureBlocks() []*types.Block {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.futureBFTBlocks
}

func (b *BackendForTestWithoutStateDB) BFTOldestFutureBlock() *types.Block {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.futureBFTBlocks) > 0 {
		return b.futureBFTBlocks[0]
	} else {
		return nil
	}
}

func (b *BackendForTestWithoutStateDB) ClearAllBFTFutureBlock() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.futureBFTBlocks = make([]*types.Block, 0)
}

func (b *BackendForTestWithoutStateDB) ClearWorkerFutureParent() {
	b.bftSealer.ClearWorkerFutureParent()
}

func (b *BackendForTestWithoutStateDB) NextIsNewSealersFirstBlock(block *types.Block) bool {
	return false
}

func (b *BackendForTestWithoutStateDB) ValidateNewBftBlocks(blocks []*types.Block) error {
	for i, block := range blocks {
		if i == 0 {
			currtBlockHash := b.GetCurrentBlock().Hash()
			if !currtBlockHash.Equal(block.ParentHash()) {
				return errors.New(fmt.Sprintf("the block %d's parent hash %s is not equal with the hash of the last block %v",
					i, block.ParentHash().TerminalString(), currtBlockHash.TerminalString()))
			}
		} else {
			lastBlockHash := blocks[i-1].Hash()
			if !lastBlockHash.Equal(block.ParentHash()) {
				return errors.New(fmt.Sprintf("the block %d's parent hash %s is not equal with the hash of the last block %v",
					i, block.ParentHash().TerminalString(), lastBlockHash.TerminalString()))
			}
		}
	}
	return nil
}

func (b *BackendForTestWithoutStateDB) SendHBFTCurrentBlock(currentBlock *types.Block) {
	b.BroadcastBlock(currentBlock)
}

func (b *BackendForTestWithoutStateDB) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return b.scope.Track(b.chainHeadFeed.Subscribe(ch))
}

func (b *BackendForTestWithoutStateDB) SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription {
	return b.scope.Track(b.chainFeed.Subscribe(ch))
}

func (b *BackendForTestWithoutStateDB) SubscribeChainNetEvent(ch chan<- core.ChainEvent) event.Subscription {
	return b.scope.Track(chainNetFeed.Subscribe(ch))
}

func (b *BackendForTestWithoutStateDB) SubscribeBftMsgNetEvent(ch chan<- bftMsgNetEvent) event.Subscription {
	return b.scope.Track(bftMsgNetFeed.Subscribe(ch))
}
