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
	"math/big"
	"path/filepath"
	"sync"
	"time"

	"github.com/contatract/go-contatract/accounts"
	"github.com/contatract/go-contatract/accounts/keystore"
	"github.com/contatract/go-contatract/bft/consensus/hbft"
	"github.com/contatract/go-contatract/bft/nodeHbft"
	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/consensus"
	"github.com/contatract/go-contatract/consensus/elephanthash"
	types "github.com/contatract/go-contatract/core/types_elephant"
	core "github.com/contatract/go-contatract/core_elephant"
	"github.com/contatract/go-contatract/core_elephant/state"
	"github.com/contatract/go-contatract/eth"
	"github.com/contatract/go-contatract/ethdb"
	"github.com/contatract/go-contatract/event"
	"github.com/contatract/go-contatract/log"
	sealer "github.com/contatract/go-contatract/miner_elephant"
	"github.com/contatract/go-contatract/params"

	"github.com/hashicorp/golang-lru"
)

type BackendForTestWithStateDB struct {
	// Node information
	NodeName  string
	NodeIndex int

	// Events subscription scope
	scope event.SubscriptionScope

	// Simulated elephant
	DataDir   string
	Accman    *accounts.Manager
	EtherBase common.Address

	memDatabase ethdb.Database // DB interfaces
	StateMemDB  *state.StateDB
	engine      consensus.EngineCtt
	blockChain  *core.BlockChain
	bftSealer   *sealer.Miner
	txPool      *core.TxPool

	chainFeed event.Feed // Not use

	eventMux *event.TypeMux
	mu       sync.Mutex

	// New block net transmitting event for test
	minedBlockSub *event.TypeMuxSubscription
	chainNetSub   event.Subscription // New block net transmitting event in elephant

	// BFT messages net transmitting event for test
	bftMsgNetSub event.Subscription  // BFT messages net transmitting event in elephant
	bftMsgNetCh  chan bftMsgNetEvent // BFT messages net transmitting event in elephant only for reply

	// Test controller parameter
	TestBlocksCntTotal uint64

	Lamp *eth.Ethereum

	Log log.Logger
}

func NewBackendForTestWithStateDB(nodeName string, index int, lamp *eth.Ethereum, logger log.Logger) (*BackendForTestWithStateDB, error) {
	backend := &BackendForTestWithStateDB{
		NodeName:    nodeName,
		NodeIndex:   index,
		DataDir:     defaultDataDir(nodeName),
		eventMux:    new(event.TypeMux),
		bftMsgNetCh: make(chan bftMsgNetEvent),
		Lamp:        lamp,
		Log:         logger,
	}

	if backend.DataDir != "" {
		absdatadir, err := filepath.Abs(backend.DataDir)
		if err != nil {
			return nil, err
		}
		backend.DataDir = absdatadir
	}

	// Create account manager
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

	// Create state database
	memDatabase, err := ethdb.NewMemDatabase()
	if err != nil {
		return nil, err
	}
	backend.memDatabase = memDatabase
	backend.StateMemDB, err = state.New(common.Hash{}, state.NewDatabase(memDatabase), nil, &backend.EtherBase)
	if err != nil {
		return nil, err
	}

	// Create genesis block
	var initBalAddrs []struct{ Addr, Balance *big.Int }
	if index == 0 {
		initBalAddrs = append(initBalAddrs, struct{ Addr, Balance *big.Int }{
			backend.EtherBase.Big(), big.NewInt(int64(1000000000000000000))})
	} else {
		initBalAddrs = append(initBalAddrs, struct{ Addr, Balance *big.Int }{
			BftNodeEtherBasesGlobal[0].Big(), big.NewInt(int64(1000000000000000000))})
	}
	genesis := defaultGenesis(initBalAddrs)
	genesisBlock, errG := genesis.Commit(backend.memDatabase)
	if errG != nil {
		return nil, errG
	}
	InsertGlobalBlock(genesisBlock)

	// Create HBFT node
	bftNode, err := nodeHbft.NewNode(lamp, backend, logger)
	if err != nil {
		return nil, err
	}
	BftNodesGlobal[index] = bftNode

	// Create consensus engine
	backend.engine = elephanthash.NewTesterForHBFT(bftNode)

	// Create core.BlockChain
	chainConfig := &params.ElephantChainConfig{ChainId: big.NewInt(627)}
	backend.blockChain, err = core.NewBlockChain(backend.memDatabase, nil, chainConfig,
		backend.engine, lamp, &backend.EtherBase)
	if err != nil {
		return nil, err
	}

	// Create transaction pool
	txCfg := core.TxPoolConfig{NoLocals: true, InputJournal: "", GlobalSlots: 100000, GlobalQueue: 100000}
	backend.txPool = core.NewTxPool(txCfg, chainConfig, backend.blockChain, uint64(lamp.CurrBlockNum()), &backend.EtherBase)

	// Create sealer
	backend.bftSealer = sealer.New(backend, params.ElephantDefChainConfig, backend.EventMux(),
		backend.engine, lamp, logger)
	if backend.bftSealer == nil {
		return nil, errors.New("failed to create sealer")
	}
	BftSealersGlobal[index] = backend.bftSealer

	//backend.chainSub = backend.SubscribeChainEvent(backend.chainCh)
	backend.chainNetSub = backend.SubscribeChainNetEvent(chainNetCh)
	backend.minedBlockSub = backend.eventMux.Subscribe(core.NewMinedBlockEvent{})
	go backend.minedBroadcastLoop()
	backend.bftMsgNetSub = backend.SubscribeBftMsgNetEvent(bftMsgNetCh)
	BftNodeBackWithStateGlobal[index] = backend

	bftNode.InitNodeTable()
	bftNode.SubscribeChainHeadEvent()
	bftNode.SetPassphrase(nodeName)

	return backend, nil
}

// AccountConfig determines the settings for scrypt and keydirectory
func (b *BackendForTestWithStateDB) AccountConfig() (int, int, string) {
	scryptN := keystore.StandardScryptN
	scryptP := keystore.StandardScryptP
	keydir := filepath.Join(b.DataDir, datadirDefaultKeyStore)
	return scryptN, scryptP, keydir
}

// GetAccAddresses returns the collection of accounts this node manages
func (b *BackendForTestWithStateDB) GetAccAddresses() []string {
	addresses := make([]string, 0) // return [] instead of nil if empty
	for _, wallet := range b.Accman.Wallets() {
		for _, account := range wallet.Accounts() {
			addresses = append(addresses, account.Address.String())
		}
	}
	return addresses
}

func (b *BackendForTestWithStateDB) GetAccount() (string, error) {
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

func (b *BackendForTestWithStateDB) GetAccountMan() *accounts.Manager {
	return b.Accman
}

func (b *BackendForTestWithStateDB) QuitBlockBroadcastLoop() {
	b.minedBlockSub.Unsubscribe() // quits blockBroadcastLoop
}

func (b *BackendForTestWithStateDB) BlockChain() *core.BlockChain {
	return b.blockChain
}

func (b *BackendForTestWithStateDB) TxPool() *core.TxPool {
	return b.txPool
}

func (b *BackendForTestWithStateDB) GenerateShardingTxBatches(block *types.Block, stateDB *state.StateDB) (types.ShardingTxBatches,
	map[uint16]uint64, error) {
	return nil, nil, nil
}

func (b *BackendForTestWithStateDB) EventMux() *event.TypeMux {
	return b.eventMux
}

func (b *BackendForTestWithStateDB) GetBlockByNumber(num uint64) *types.Block {
	return b.blockChain.GetBlockByNumber(num)
}

func (b *BackendForTestWithStateDB) GetBlockByHash(hash common.Hash) *types.Block {
	return b.blockChain.GetBlockByHash(hash)
}

func (b *BackendForTestWithStateDB) GetCurrentBlockNumber() uint64 {
	block := b.blockChain.CurrentBlock()
	if block != nil {
		return block.NumberU64()
	}
	return 0
}

// BFTNewestFutureBlock retrieves the newest future head in BFT consensus of the block chain.
func (b *BackendForTestWithStateDB) BFTNewestFutureBlock() *types.Block {
	return b.blockChain.BFTNewestFutureBlock()
}

// ClearBFTFutureBlock clears the future block in BFT consensus of the block chain.
func (b *BackendForTestWithStateDB) ClearBFTFutureBlock(hash common.Hash) {
	b.blockChain.ClearBFTFutureBlock(hash)
}

// ClearOutDatedBFTFutureBlock clears the outdated future blocks in BFT consensus of the block chain.
func (b *BackendForTestWithStateDB) ClearOutDatedBFTFutureBlock(height uint64) {
	b.blockChain.ClearOutDatedBFTFutureBlock(height)
}

func (b *BackendForTestWithStateDB) StartSealerUpdateLoop(self *sealer.Worker) {
	defer self.TxSub().Unsubscribe()
	defer self.ChainHeadSub().Unsubscribe()
	defer self.ChainSideSub().Unsubscribe()

	for {
		// A real event arrived, process interesting content
		select {
		// Handle ChainHeadEvent
		case <-self.ChainHeadCh():
			self.CommitNewWork()

			// Handle TxPreEvent
		case ev := <-self.TxCh():
			self.HandleTxCh(ev)

		case <-self.ChainHeadSub().Err():
			return
		case <-self.ChainSideSub().Err():
			return
		}
	}
}

func (b *BackendForTestWithStateDB) StartBlockSyncLoop() {
	for {
		if BftTestCaseStopped {
			return
		}

		BftNodeGlobalMu.Lock()
		bftNodeBlockNumMapLength := len(BftNodeBlockNumMap)
		BftNodeGlobalMu.Unlock()

		blockLength := int(b.GetCurrentBlockNumber() + 1)
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

// Mined broadcast loop
func (b *BackendForTestWithStateDB) minedBroadcastLoop() {
	// automatically stops if unsubscribe
	for obj := range b.minedBlockSub.Chan() {
		switch ev := obj.Data.(type) {
		case core.NewMinedBlockEvent:
			//eleHeight := ev.Block.NumberU64()
			//peers := self.peers.Peers()
			//for _, p := range peers {
			//	p.SendEleHeight(&eleHeight)
			//}
			//self.BroadcastBlock(ev.Block, true, false)  // First propagate block to peers
			//self.BroadcastBlock(ev.Block, false, false) // Only then announce to the rest
			//log.Info("Mined broadcast loop", "shardtxlen", len(ev.ShardTxs))
			//if ev.ShardTxs != nil && len(ev.ShardTxs) > 0 {
			//	self.BroadcastShardingTxs(ev.ShardTxs)
			//	self.DeliverShardingTxs(ev.ShardTxs)
			//}
			InsertGlobalBlock(ev.Block)

			BftNodesGlobal[b.NodeIndex].Info(fmt.Sprintf("Inserted new block in %s", b.NodeName),
				"Hash", ev.Block.Hash().TerminalString())
			LogInfo(fmt.Sprintf("---Inserted new block in %s successfully", b.NodeName),
				"Hash", ev.Block.Hash().TerminalString(), "number", ev.Block.NumberU64(), "HashWithBft", ev.Block.HashWithBft())

			go b.BroadcastBlock(ev.Block)
		}
	}
}

func (b *BackendForTestWithStateDB) StartBlockEventLoop() {
	defer b.chainNetSub.Unsubscribe()

	for {
		select {
		case ev := <-chainNetCh: // from simulated net
			go func() {
				if !RandomDelayMsForHBFT(b.NodeIndex, "ReceiveBlock", false) {
					return
				}

				b.mu.Lock()
				blockWeHave := b.blockChain.GetBlockByHash(ev.Block.Hash()) //b.BlockHashMap[ev.Block.Hash()]
				b.mu.Unlock()

				if blockWeHave == nil {
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

							if err := b.blockChain.ReplaceBlockWithoutState(ev.Block); err != nil {
								LogCrit(fmt.Sprintf("---Write block without state failed when replacing the old block in %s", b.NodeName),
									"Hash", ev.Block.Hash().TerminalString(), "HashWithBft", ev.Block.HashWithBft())
							}
							InsertGlobalBlock(ev.Block)

							BftNodesGlobal[b.NodeIndex].Info(fmt.Sprintf("Replaced new block in %s instead of the old one", b.NodeName),
								"Hash", ev.Block.Hash().TerminalString(), "HashWithBft", ev.Block.HashWithBft())
							LogInfo(fmt.Sprintf("---Replaced new block in %s instead of the block successfully", b.NodeName),
								"Hash", ev.Block.Hash().TerminalString(), "HashWithBft", ev.Block.HashWithBft())

							go b.BroadcastBlock(ev.Block)
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

func (b *BackendForTestWithStateDB) waitForParentBlock(hash common.Hash) {
	BftNodesGlobal[b.NodeIndex].Info(fmt.Sprintf("Waiting for the block to connect, Hash: %s", hash.TerminalString()))
	for {
		if BftTestCaseStopped {
			time.Sleep(time.Second)
			return
		}
		time.Sleep(nodeHbft.BlockSealingBeat)
		//if _, ok := b.BlockHashMap[hash]; ok {
		if exist := b.blockChain.GetBlockByHash(hash); exist != nil {
			BftNodesGlobal[b.NodeIndex].Info(fmt.Sprintf("The block connected, Hash: %s, stop waiting", hash.TerminalString()))
			return
		}
		if block, ok := BftNodeBlockHashMap[hash]; ok {
			if parent := b.blockChain.GetBlock(block.ParentHash(), block.NumberU64()-1); parent == nil {
				b.waitForParentBlock(block.ParentHash())
			}
			b.connectReceivedBlock(block)
			BftNodesGlobal[b.NodeIndex].Info(fmt.Sprintf("The block connected, Hash: %s, stop waiting", hash.TerminalString()))
			return
		}
	}
}

func (b *BackendForTestWithStateDB) BroadcastBlock(block *types.Block) {
	if !RandomDelayMsForHBFT(b.NodeIndex, "BroadcastBlock", false) {
		return
	}

	ev := core.ChainEvent{Block: block, HashWithBft: block.HashWithBft()}
	chainNetFeed.Send(ev)

	BftNodesGlobal[b.NodeIndex].Info(fmt.Sprintf("Broadcast new block in %s", b.NodeName),
		"Hash", ev.Block.Hash().TerminalString())
	LogInfo(fmt.Sprintf("---Broadcast new block in %s", b.NodeName),
		"Hash", ev.Block.Hash().TerminalString())
}

func (b *BackendForTestWithStateDB) StartBFTMsgEventLoop() {
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

func (b *BackendForTestWithStateDB) handleBftMsg(msg *hbft.BFTMsg) {
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

func (b *BackendForTestWithStateDB) connectReceivedBlock(block *types.Block) {
	//b.insertBlock(block)
	if _, err := b.blockChain.InsertChain([]*types.Block{block}); err != nil {
		fmt.Println(fmt.Sprintf("---InsertBlock failed, %s", err.Error()), "node", b.EtherBase.String())
		LogCrit(fmt.Sprintf("---InsertBlock failed, %s", err.Error()), "node", b.EtherBase.String())
	}
	InsertGlobalBlock(block)

	BftNodesGlobal[b.NodeIndex].Info(fmt.Sprintf("Inserted new block in %s", b.NodeName),
		"Hash", block.Hash().TerminalString())
	LogInfo(fmt.Sprintf("---Inserted new block in %s successfully", b.NodeName),
		"Hash", block.Hash().TerminalString(), "number", block.NumberU64(), "HashWithBft", block.HashWithBft())

	if uint64(b.blockChain.CurrentBlock().NumberU64()+1) >= b.TestBlocksCntTotal || BftTestCaseStopped {
		BftSealersGlobal[b.NodeIndex].Stop()
		BftTestCaseStopped = true
	}

	for i, future := range b.BFTFutureBlocks() {
		BftNodesGlobal[b.NodeIndex].Info(fmt.Sprintf("BFT Future block %d: in %s", i, future.Hash().TerminalString()))
	}
}

func (b *BackendForTestWithStateDB) Etherbase() (eb common.Address, err error) {
	return b.EtherBase, nil
}

func (b *BackendForTestWithStateDB) AccountManager() *accounts.Manager {
	return b.Accman
}

func (b *BackendForTestWithStateDB) SendBFTMsg(msg hbft.MsgHbftConsensus, msgType uint) {
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
				BftNodeBackWithStateGlobal[i].GetBFTMsgChanForReply() <- ev
				return
			}
		}
	}
}

func (b *BackendForTestWithStateDB) GetBFTMsgChanForReply() chan bftMsgNetEvent {
	return b.bftMsgNetCh
}

func (b *BackendForTestWithStateDB) GetCurRealSealers() ([]common.Address, bool) {
	return BftNodeEtherBasesGlobal, true
}

func (b *BackendForTestWithStateDB) GetRealSealersByNum(number uint64) ([]common.Address, bool) {
	return BftNodeEtherBasesGlobal, false
}

func (b *BackendForTestWithStateDB) Get2fRealSealersCnt() int {
	sealers, _ := b.GetCurRealSealers()
	f := (len(sealers) - 1) / 3
	for offset := 0; ; offset++ {
		if 3*(f+offset)+1 >= len(sealers) {
			return 2 * (f + offset)
		}
	}
}

func (b *BackendForTestWithStateDB) GetBlock(hash common.Hash, number uint64) *types.Block {
	return b.blockChain.GetBlock(hash, number)
}

func (b *BackendForTestWithStateDB) GetCurrentBlock() *types.Block {
	return b.blockChain.CurrentBlock()
}

func (b *BackendForTestWithStateDB) AddBFTFutureBlock(block *types.Block) {
	b.blockChain.AddBFTFutureBlock(block)
}

func (b *BackendForTestWithStateDB) BFTFutureBlocks() []*types.Block {
	return b.blockChain.BFTFutureBlocks()
}

func (b *BackendForTestWithStateDB) BFTOldestFutureBlock() *types.Block {
	return b.blockChain.BFTOldestFutureBlock()
}

func (b *BackendForTestWithStateDB) ClearAllBFTFutureBlock() {
	b.blockChain.ClearAllBFTFutureBlock()
}

func (b *BackendForTestWithStateDB) ClearWorkerFutureParent() {
	b.bftSealer.ClearWorkerFutureParent()
}

func (b *BackendForTestWithStateDB) NextIsNewSealersFirstBlock(block *types.Block) bool {
	return false
}

func (b *BackendForTestWithStateDB) ValidateNewBftBlocks(blocks []*types.Block) error {
	headers := make([]*types.Header, len(blocks))
	for i, block := range blocks {
		headers[i] = block.Header()
	}

	var stateDb *state.StateDB
	futureBlocks, _ := lru.New(4)
	abort, results := b.blockChain.Engine().VerifyHeaders(b.blockChain, headers)
	defer close(abort)

	for i, block := range blocks {
		BftNodesGlobal[b.NodeIndex].HBFTDebugInfo(fmt.Sprintf("ValidateNewBftBlocks, block: %s, root: %s",
			block.Hash().TerminalString(), block.Root().TerminalString()))
		futureBlock := false
		if core.BadHashes[block.Hash()] {
			return core.ErrBlacklistedHash
		}
		err := <-results
		if err != nil {
			return err
		}
		futureBlocks.Add(block.Hash(), block)
		err = b.blockChain.Validator().ValidateBody(block, false)
		if err == consensus.ErrUnknownAncestor && futureBlocks.Contains(block.ParentHash()) {
			futureBlock = true
		} else if err != nil {
			return err
		}
		err = b.blockChain.ValidateInputShardingTxs(block)
		if err != nil {
			return err
		}

		// Create a new stateDb using the parent block and report an
		// error if it fails.
		var parent *types.Block
		if i == 0 {
			parent = b.blockChain.GetBlock(block.ParentHash(), block.NumberU64()-1)
		} else {
			parent = blocks[i-1]
		}

		if !futureBlock {
			stateDb, err = state.New(parent.Root(), *b.blockChain.StateCache(), b.blockChain.GetFeeds(), b.blockChain.GetElebase())
			if err != nil {
				return err
			}
		}

		// Process block using the parent state as reference point.
		receipts, usedGas, err := b.blockChain.Processor().Process(block, stateDb, uint64(b.Lamp.CurrBlockNum()), &b.EtherBase)
		if err != nil {
			return err
		}
		err = b.blockChain.Validator().ValidateState(block, parent, stateDb, receipts, usedGas)
		if err != nil {
			return err
		}
	}

	return nil
}

func (b *BackendForTestWithStateDB) SendHBFTCurrentBlock(currentBlock *types.Block) {
	go b.BroadcastBlock(currentBlock)
}

func (b *BackendForTestWithStateDB) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return b.scope.Track(b.blockChain.SubscribeChainHeadEvent(ch))
}

func (b *BackendForTestWithStateDB) SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription {
	return b.scope.Track(b.chainFeed.Subscribe(ch))
}

func (b *BackendForTestWithStateDB) SubscribeChainNetEvent(ch chan<- core.ChainEvent) event.Subscription {
	return b.scope.Track(chainNetFeed.Subscribe(ch))
}

func (b *BackendForTestWithStateDB) SubscribeBftMsgNetEvent(ch chan<- bftMsgNetEvent) event.Subscription {
	return b.scope.Track(bftMsgNetFeed.Subscribe(ch))
}
