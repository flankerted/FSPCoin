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
	"github.com/contatract/go-contatract/baseparam"
	"github.com/contatract/go-contatract/bft/nodeHbft"
	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/consensus"
	"github.com/contatract/go-contatract/consensus/ethash"
	"github.com/contatract/go-contatract/core/types"
	"github.com/contatract/go-contatract/core/vm"
	core "github.com/contatract/go-contatract/core_eth"
	"github.com/contatract/go-contatract/core_eth/state"
	"github.com/contatract/go-contatract/eth"
	"github.com/contatract/go-contatract/ethdb"
	"github.com/contatract/go-contatract/event"
	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/miner"
	"github.com/contatract/go-contatract/params"
	"github.com/contatract/go-contatract/rpc"
)

type BackendForMinerTest struct {
	// Node information
	NodeName  string
	NodeIndex int

	// Events subscription scope
	scope event.SubscriptionScope

	// Simulated elephant
	DataDir   string
	Accman    *accounts.Manager
	EtherBase common.Address

	memDatabase     ethdb.Database // DB interfaces
	StateMemDB      *state.StateDB
	engine          consensus.Engine
	blockChain      *core.BlockChain
	miner           *miner.Miner
	txPool          *core.TxPool
	electionPool    *core.ElectionPool
	verifHeaderPool *core.VerifiedHeaderPool

	chainFeed event.Feed // Not use

	eventMux *event.TypeMux
	mu       sync.Mutex

	// New block net transmitting event for test
	minedBlockSub *event.TypeMuxSubscription
	chainNetSub   event.Subscription // New block net transmitting event in elephant

	// Test controller parameter
	TestBlocksCntTotal uint64

	Log log.Logger
}

func NewBackendForLampMinerTest(nodeName string, index int, logger log.Logger) (*BackendForMinerTest, error) {
	backend := &BackendForMinerTest{
		NodeName:  nodeName,
		NodeIndex: index,
		DataDir:   defaultDataDir(nodeName),
		eventMux:  new(event.TypeMux),
		Log:       logger,
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
	LampNodeAddressGlobal = append(LampNodeAddressGlobal, backend.EtherBase)
	LampNodeMapAddressIndex[backend.EtherBase] = index

	// Create state database
	memDatabase, err := ethdb.NewMemDatabase()
	if err != nil {
		return nil, err
	}
	backend.memDatabase = memDatabase
	backend.StateMemDB, err = state.New(common.Hash{}, state.NewDatabase(memDatabase))
	if err != nil {
		return nil, err
	}

	// Create genesis block
	genesis := defaultGenesis()
	genesis.Config.ChainId = big.NewInt(627)
	genesisBlock, errG := genesis.Commit(backend.memDatabase)
	if errG != nil {
		return nil, errG
	}
	InsertGlobalBlock(genesisBlock)

	// Create consensus engine
	config := &eth.DefaultConfig
	backend.engine = backend.createConsensusEngine(&config.Ethash, genesis.Config, backend.memDatabase)

	// Create core.BlockChain
	var (
		vmConfig    = vm.Config{EnablePreimageRecording: config.EnablePreimageRecording}
		cacheConfig = &core.CacheConfig{Disabled: config.NoPruning, TrieNodeLimit: config.TrieCache, TrieTimeLimit: config.TrieTimeout}
	)
	backend.blockChain, err = core.NewBlockChain(backend.memDatabase, cacheConfig, genesis.Config,
		backend.engine, vmConfig)
	if err != nil {
		return nil, err
	}

	// Create election pool
	backend.electionPool = core.NewElectionPool(config.ElectionPool, backend.blockChain)
	backend.blockChain.SetElectionPool(backend.electionPool)

	backend.verifHeaderPool = core.NewVerifiedHeaderPool(backend.blockChain, backend)

	// Create transaction pool
	txCfg := core.TxPoolConfig{NoLocals: true, Journal: "", GlobalSlots: 100000, GlobalQueue: 100000}
	backend.txPool = core.NewTxPool(txCfg, genesis.Config, backend.blockChain)

	// Create miner
	backend.miner = miner.New(backend, genesis.Config, backend.EventMux(), backend.engine, logger)
	if backend.miner == nil {
		return nil, errors.New("failed to create miner")
	}
	LampMinersGlobal[index] = backend.miner

	//backend.chainSub = backend.SubscribeChainEvent(backend.chainCh)
	backend.chainNetSub = backend.SubscribeChainNetEvent(chainNetCh)
	backend.minedBlockSub = backend.eventMux.Subscribe(core.NewMinedBlockEvent{})
	go backend.minedBroadcastLoop()
	LampNodeBackGlobal[index] = backend

	return backend, nil
}

// AccountConfig determines the settings for scrypt and keydirectory
func (b *BackendForMinerTest) AccountConfig() (int, int, string) {
	scryptN := keystore.StandardScryptN
	scryptP := keystore.StandardScryptP
	keydir := filepath.Join(b.DataDir, datadirDefaultKeyStore)
	return scryptN, scryptP, keydir
}

// GetAccAddresses returns the collection of accounts this node manages
func (b *BackendForMinerTest) GetAccAddresses() []string {
	addresses := make([]string, 0) // return [] instead of nil if empty
	for _, wallet := range b.Accman.Wallets() {
		for _, account := range wallet.Accounts() {
			addresses = append(addresses, account.Address.String())
		}
	}
	return addresses
}

func (b *BackendForMinerTest) GetAccount() (string, error) {
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

// CreateConsensusEngine creates the required type of consensus engine instance for an Ethereum service
func (b *BackendForMinerTest) createConsensusEngine(config *ethash.Config, chainConfig *params.ChainConfig, db ethdb.Database) consensus.Engine {
	// Otherwise assume proof-of-work
	switch {
	case config.PowMode == ethash.ModeFake:
		log.Warn("Ethash used in fake mode")
		return ethash.NewFaker()
	case config.PowMode == ethash.ModeTest:
		log.Warn("Ethash used in test mode")
		return ethash.NewTester()
	case config.PowMode == ethash.ModeShared:
		log.Warn("Ethash used in shared mode")
		return ethash.NewShared()
	default:
		engine := ethash.New(ethash.Config{
			CacheDir:       "ethash",
			CachesInMem:    config.CachesInMem,
			CachesOnDisk:   config.CachesOnDisk,
			DatasetDir:     config.DatasetDir,
			DatasetsInMem:  config.DatasetsInMem,
			DatasetsOnDisk: config.DatasetsOnDisk,
		})
		engine.SetThreads(1)
		return engine
	}
}

func (b *BackendForMinerTest) LastEleSealersBlockNum() rpc.BlockNumber {
	currentBlockNum := b.blockChain.CurrentBlock().NumberU64()
	lastEleSealersBlockNum := currentBlockNum - currentBlockNum%baseparam.ElectionBlockCount -
		baseparam.ElectionBlockCount
	if lastEleSealersBlockNum > 0 {
		return rpc.BlockNumber(lastEleSealersBlockNum)
	} else {
		return rpc.BlockNumber(0)
	}
}

func (b *BackendForMinerTest) CurrEleSealersBlockNum() rpc.BlockNumber {
	currentBlockNum := b.blockChain.CurrentBlock().NumberU64()
	currEleSealersBlockNum := currentBlockNum - currentBlockNum%baseparam.ElectionBlockCount
	if currEleSealersBlockNum > 0 {
		return rpc.BlockNumber(currEleSealersBlockNum)
	} else {
		return rpc.BlockNumber(0)
	}
}

func (b *BackendForMinerTest) GetPoliceByHash(lampBlockHash *common.Hash) []common.AddressNode {
	block := b.blockChain.GetBlockByHash(*lampBlockHash)
	if block.IsElectionBlock() {
		return block.NextPolice()
	}

	return []common.AddressNode{}
}

func (b *BackendForMinerTest) GetNextPolice() []common.AddressNode {
	block := b.blockChain.GetBlockByNumber(uint64(b.CurrEleSealersBlockNum()))
	return block.NextPolice()
}

func (b *BackendForMinerTest) GetLastPolice() []common.AddressNode {
	if lastEleSealersBlockNum := b.LastEleSealersBlockNum(); lastEleSealersBlockNum > 0 {
		block := b.blockChain.GetBlockByNumber(uint64(lastEleSealersBlockNum))
		return block.NextPolice()
	} else {
		return nil
	}
}

func (b *BackendForMinerTest) GetNewestValidHeaders() map[uint16]*types.VerifiedValid {
	block := b.blockChain.GetBlockByNumber(uint64(b.CurrEleSealersBlockNum()))
	if block == nil {
		return nil
	}
	return block.GetValidHeaders()
}

func (b *BackendForMinerTest) ElectionPool() *core.ElectionPool { return b.electionPool }
func (b *BackendForMinerTest) VerifiedHeaderPool() *core.VerifiedHeaderPool {
	return b.verifHeaderPool
}
func (b *BackendForMinerTest) ChainDb() ethdb.Database { return b.memDatabase }

func (b *BackendForMinerTest) RunForSealerElection(addr common.Address, passwd string) {

}

func (b *BackendForMinerTest) GetMainNodeForBFTFlag() bool {
	return false
}

func (b *BackendForMinerTest) GetAccountMan() *accounts.Manager {
	return b.Accman
}

func (b *BackendForMinerTest) QuitBlockBroadcastLoop() {
	b.minedBlockSub.Unsubscribe() // quits blockBroadcastLoop
}

func (b *BackendForMinerTest) BlockChain() *core.BlockChain {
	return b.blockChain
}

func (b *BackendForMinerTest) TxPool() *core.TxPool {
	return b.txPool
}

func (b *BackendForMinerTest) EventMux() *event.TypeMux {
	return b.eventMux
}

func (b *BackendForMinerTest) GetBlockByNumber(num uint64) *types.Block {
	return b.blockChain.GetBlockByNumber(num)
}

func (b *BackendForMinerTest) GetBlockByHash(hash common.Hash) *types.Block {
	return b.blockChain.GetBlockByHash(hash)
}

func (b *BackendForMinerTest) GetCurrentBlockNumber() uint64 {
	block := b.blockChain.CurrentBlock()
	if block != nil {
		return block.NumberU64()
	}
	return 0
}

func (b *BackendForMinerTest) StartMinerUpdateLoop(self *miner.Worker) {
	defer self.TxSub().Unsubscribe()
	defer self.ChainHeadSub().Unsubscribe()
	defer self.ChainSideSub().Unsubscribe()

	for {
		// A real event arrived, process interesting content
		select {
		// Handle ChainHeadEvent
		case <-self.ChainHeadCh():
			if uint64(b.blockChain.CurrentBlock().NumberU64()+1) > b.TestBlocksCntTotal || LampMinerTestStopped {
				LampMinersGlobal[b.NodeIndex].Stop()
				LampMinerTestStopped = true
				return
			}
			self.CommitNewWork()

			// Handle ChainSideEvent
		case ev := <-self.ChainSideCh():
			self.HandleChainSideCh(ev)

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

func (b *BackendForMinerTest) StartBlockSyncLoop() {
	for {
		if LampMinerTestStopped {
			return
		}

		LampNodeGlobalMu.Lock()
		bftNodeBlockNumMapLength := len(LampNodeBlockNumMap)
		LampNodeGlobalMu.Unlock()

		blockLength := int(b.GetCurrentBlockNumber() + 1)
		if blockLength < bftNodeBlockNumMapLength {
			LogWarn("---The length of blocks is not consistent with the global's", "len(b.Blocks)", blockLength,
				"len(LampNodeBlockNumMap)", bftNodeBlockNumMapLength)
			num := bftNodeBlockNumMapLength - 1
			for i := 0; i < bftNodeBlockNumMapLength-blockLength; i++ {
				go func(j int) {
					LampNodeGlobalMu.Lock()
					block, ok := LampNodeBlockNumMap[uint64(num+j)]
					LampNodeGlobalMu.Unlock()
					if !ok {
						return
					}

					LampMinersGlobal[b.NodeIndex].Info(fmt.Sprintf("Syncronising the block to connect, Hash: %s",
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
func (b *BackendForMinerTest) minedBroadcastLoop() {
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

			LampMinersGlobal[b.NodeIndex].Info(fmt.Sprintf("Inserted new block in %s", b.NodeName),
				"Hash", ev.Block.Hash().TerminalString())
			LogInfo(fmt.Sprintf("---Inserted new block in %s successfully", b.NodeName),
				"Hash", ev.Block.Hash().TerminalString(), "number", ev.Block.NumberU64())

			go b.BroadcastBlock(ev.Block)
		}
	}
}

func (b *BackendForMinerTest) StartBlockEventLoop() {
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
					//blockHashWithBft := blockWeHave.HashWithBft()
					//if !blockHashWithBft.Equal(ev.HashWithBft) {
					//	if ev.Block.HBFTStageChain()[1].ViewID > blockWeHave.HBFTStageChain()[1].ViewID ||
					//		(ev.Block.HBFTStageChain()[1].ViewID == blockWeHave.HBFTStageChain()[1].ViewID &&
					//			ev.Block.HBFTStageChain()[1].Timestamp > blockWeHave.HBFTStageChain()[1].Timestamp) {
					//
					//		if err := b.blockChain.ReplaceBlockWithoutState(ev.Block); err != nil {
					//			LogCrit(fmt.Sprintf("---Write block without state failed when replacing the old block in %s", b.NodeName),
					//				"Hash", ev.Block.Hash().TerminalString(), "HashWithBft", ev.Block.HashWithBft())
					//		}
					//		InsertGlobalBlock(ev.Block)
					//
					//		LampMinerNodesGlobal[b.NodeIndex].Info(fmt.Sprintf("Replaced new block in %s instead of the old one", b.NodeName),
					//			"Hash", ev.Block.Hash().TerminalString(), "HashWithBft", ev.Block.HashWithBft())
					//		LogInfo(fmt.Sprintf("---Replaced new block in %s instead of the block successfully", b.NodeName),
					//			"Hash", ev.Block.Hash().TerminalString(), "HashWithBft", ev.Block.HashWithBft())
					//
					//		go b.BroadcastBlock(ev.Block)
					//	}
					//}
				}
			}()

		// System stopped
		case <-b.chainNetSub.Err():
			LogInfo("---Quit Loop of StartBlockEventLoop(), due to b.chainNetSub.Err()", "node", b.EtherBase.String())
			return
		}
	}
}

func (b *BackendForMinerTest) waitForParentBlock(hash common.Hash) {
	LampMinersGlobal[b.NodeIndex].Info(fmt.Sprintf("Waiting for the block to connect, Hash: %s", hash.TerminalString()))
	for {
		if LampMinerTestStopped {
			time.Sleep(time.Second)
			return
		}
		time.Sleep(nodeHbft.BlockSealingBeat)
		//if _, ok := b.BlockHashMap[hash]; ok {
		if exist := b.blockChain.GetBlockByHash(hash); exist != nil {
			LampMinersGlobal[b.NodeIndex].Info(fmt.Sprintf("The block connected, Hash: %s, stop waiting", hash.TerminalString()))
			return
		}
		if block, ok := LampNodeBlockHashMap[hash]; ok {
			if parent := b.blockChain.GetBlock(block.ParentHash(), block.NumberU64()-1); parent == nil {
				b.waitForParentBlock(block.ParentHash())
			}
			b.connectReceivedBlock(block)
			LampMinersGlobal[b.NodeIndex].Info(fmt.Sprintf("The block connected, Hash: %s, stop waiting", hash.TerminalString()))
			return
		}
	}
}

func (b *BackendForMinerTest) BroadcastBlock(block *types.Block) {
	if !RandomDelayMsForHBFT(b.NodeIndex, "BroadcastBlock", false) {
		return
	}

	ev := core.ChainEvent{Block: block, Hash: block.Hash(), Logs: nil}
	chainNetFeed.Send(ev)

	LampMinersGlobal[b.NodeIndex].Info(fmt.Sprintf("Broadcast new block in %s", b.NodeName),
		"Hash", ev.Block.Hash().TerminalString())
	LogInfo(fmt.Sprintf("---Broadcast new block in %s", b.NodeName),
		"Hash", ev.Block.Hash().TerminalString())
}

func (b *BackendForMinerTest) connectReceivedBlock(block *types.Block) {
	if _, err := b.blockChain.InsertChain([]*types.Block{block}); err != nil {
		fmt.Println(fmt.Sprintf("---InsertBlock failed, %s", err.Error()), "node", b.EtherBase.String())
		LogCrit(fmt.Sprintf("---InsertBlock failed, %s", err.Error()), "node", b.EtherBase.String())
	}
	InsertGlobalBlock(block)

	LampMinersGlobal[b.NodeIndex].Info(fmt.Sprintf("Inserted new block in %s", b.NodeName),
		"Hash", block.Hash().TerminalString())
	LogInfo(fmt.Sprintf("---Inserted new block in %s successfully", b.NodeName),
		"Hash", block.Hash().TerminalString(), "number", block.NumberU64())

	if uint64(b.blockChain.CurrentBlock().NumberU64()+1) > b.TestBlocksCntTotal || LampMinerTestStopped {
		LampMinersGlobal[b.NodeIndex].Stop()
		LampMinerTestStopped = true
	}
}

func (b *BackendForMinerTest) Etherbase() (eb common.Address, err error) {
	return b.EtherBase, nil
}

func (b *BackendForMinerTest) AccountManager() *accounts.Manager {
	return b.Accman
}

func (b *BackendForMinerTest) GetCurRealSealers() ([]common.Address, bool) {
	return LampNodeAddressGlobal, true
}

func (b *BackendForMinerTest) GetRealSealersByNum(number uint64) ([]common.Address, bool) {
	return LampNodeAddressGlobal, false
}

func (b *BackendForMinerTest) Get2fRealSealersCnt() int {
	sealers, _ := b.GetCurRealSealers()
	f := (len(sealers) - 1) / 3
	for offset := 0; ; offset++ {
		if 3*(f+offset)+1 >= len(sealers) {
			return 2 * (f + offset)
		}
	}
}

func (b *BackendForMinerTest) GetBlock(hash common.Hash, number uint64) *types.Block {
	return b.blockChain.GetBlock(hash, number)
}

func (b *BackendForMinerTest) GetCurrentBlock() *types.Block {
	return b.blockChain.CurrentBlock()
}

func (b *BackendForMinerTest) NextIsNewSealersFirstBlock(block *types.Block) bool {
	return false
}

func (b *BackendForMinerTest) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return b.scope.Track(b.blockChain.SubscribeChainHeadEvent(ch))
}

func (b *BackendForMinerTest) SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription {
	return b.scope.Track(b.chainFeed.Subscribe(ch))
}

func (b *BackendForMinerTest) SubscribeChainNetEvent(ch chan<- core.ChainEvent) event.Subscription {
	return b.scope.Track(chainNetFeed.Subscribe(ch))
}
