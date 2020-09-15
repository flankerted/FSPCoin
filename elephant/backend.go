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

// Package lamp implements the Ethereum protocol.
package elephant

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	lru "github.com/hashicorp/golang-lru"

	//"math/big"
	"runtime"
	"sync"

	//"runtime/debug"
	"sync/atomic"

	"github.com/contatract/go-contatract/baseparam"
	"github.com/contatract/go-contatract/bft/consensus/hbft"
	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/common/hexutil"
	"github.com/contatract/go-contatract/consensus"
	"github.com/contatract/go-contatract/consensus/elephanthash"
	"github.com/contatract/go-contatract/core_elephant/state"
	"github.com/contatract/go-contatract/eth"
	"github.com/contatract/go-contatract/internal/elephantapi"
	"github.com/contatract/go-contatract/p2p/discover"

	//"github.com/contatract/go-contatract/consensus/elephanthash"
	core "github.com/contatract/go-contatract/core_elephant"
	//"github.com/contatract/go-contatract/core/bloombits"
	types "github.com/contatract/go-contatract/core/types_elephant"
	//"github.com/contatract/go-contatract/core/vm"
	downloader "github.com/contatract/go-contatract/elephant/downloader"
	"github.com/contatract/go-contatract/elephant/exporter"

	//"github.com/contatract/go-contatract/elephant/filters"
	//"github.com/contatract/go-contatract/elephant/gasprice"
	"github.com/contatract/go-contatract/ethdb"
	"github.com/contatract/go-contatract/event"

	//"github.com/contatract/go-contatract/internal/ethapi"
	"github.com/contatract/go-contatract/accounts"
	"github.com/contatract/go-contatract/accounts/keystore"
	"github.com/contatract/go-contatract/bft/nodeHbft"
	"github.com/contatract/go-contatract/blizparam"
	"github.com/contatract/go-contatract/blizzard/storage"
	"github.com/contatract/go-contatract/blizzard/storagecore"
	"github.com/contatract/go-contatract/crypto"
	"github.com/contatract/go-contatract/crypto/ecies"
	"github.com/contatract/go-contatract/log"
	miner "github.com/contatract/go-contatract/miner_elephant"
	"github.com/contatract/go-contatract/node"
	"github.com/contatract/go-contatract/p2p"
	"github.com/contatract/go-contatract/params"
	"github.com/contatract/go-contatract/police"
	"github.com/contatract/go-contatract/rlp"
	"github.com/contatract/go-contatract/rpc"
)

// Ethereum implements the Ethereum full node service.
type Elephant struct {
	config      *Config
	chainConfig *params.ElephantChainConfig

	// Channel for shutting down the service
	shutdownChan chan bool                    // Channel for shutting down the ethereum
	replyBftChan map[string]chan *hbft.BFTMsg // Channel for replying BFT message to the primary
	// stopDbUpgrade func() error // stop chain db sequential key upgrade

	// Handlers
	txPool          *core.TxPool
	blockchain      *core.BlockChain
	protocolManager *ProtocolManager

	// DB interfaces
	chainDb ethdb.Database // Block chain database

	eventMux       *event.TypeMux
	engine         consensus.EngineCtt
	accountManager *accounts.Manager
	ApiBackend     *ElephantApiBackend

	miner     *miner.Miner
	etherbase common.Address
	police    *police.Officer

	networkId uint64

	lock sync.RWMutex // Protects the variadic fields (e.g. gas price and etherbase)

	storageManager *storage.Manager
	node           *discover.Node
	hbftNode       *nodeHbft.Node

	policeIsWorking *bool // if true, the police is working and the node accepts other shard nodes

	lamp *eth.Ethereum
}

// New creates a new Ethereum object (including the
// initialisation of the common Elephant object)
func New(ctx *node.ServiceContext, config *Config) (*Elephant, error) {
	if config.SyncMode == downloader.LightSync {
		return nil, errors.New("can't run elephant.Elephant in light sync mode, use les.LightEthereum")
	}
	if !config.SyncMode.IsValid() {
		return nil, fmt.Errorf("invalid sync mode %d", config.SyncMode)
	}
	chainDb, err := CreateDB(ctx, config, "elephantdata")
	if err != nil {
		return nil, err
	}
	// stopDbUpgrade := upgradeDeduplicateData(chainDb)
	chainConfig, genesisHash, genesisErr := core.SetupGenesisBlock(chainDb, config.Genesis)
	if _, ok := genesisErr.(*params.ConfigCompatError); genesisErr != nil && !ok {
		return nil, genesisErr
	}
	log.Info("Initialised chain configuration", "config", chainConfig)

	var lamp *eth.Ethereum
	if err := ctx.Service(&lamp); err != nil {
		log.Error("[New Elephant Service failed!] : Ethereum Service not running: %v", err)
		return nil, err
	}

	elephant := &Elephant{
		config:         config,
		chainDb:        chainDb,
		chainConfig:    chainConfig,
		eventMux:       ctx.EventMux,
		accountManager: ctx.AccountManager,
		// engine:         CreateConsensusEngine(ctx, &config.Elephanthash, chainConfig, chainDb),
		shutdownChan: make(chan bool),
		replyBftChan: make(map[string]chan *hbft.BFTMsg),
		// stopDbUpgrade:  stopDbUpgrade,
		networkId: config.NetworkId,
		etherbase: config.Etherbase,
		lamp:      lamp,
	}

	cb, _ := elephant.Etherbase()

	var loggerBFT log.Logger = nil
	if hbft.IsDebugMode() {
		loggerBFT = createLogger(fmt.Sprintf("HBFT-%s.log", cb.String()))
	}
	elephant.hbftNode, err = nodeHbft.NewNode(lamp, elephant, loggerBFT)
	if err != nil {
		return nil, err
	}

	elephant.engine = CreateConsensusEngine(&config.Elephanthash, elephant.hbftNode)

	log.Info("Initialising Elephant protocol", "versions", ProtocolVersions, "network", config.NetworkId)

	// if !config.SkipBcVersionCheck {
	// 	bcVersion := core.GetBlockChainVersion(chainDb)
	// 	if bcVersion != core.BlockChainVersion && bcVersion != 0 {
	// 		return nil, fmt.Errorf("Blockchain DB version mismatch (%d / %d). Run geth upgradedb.\n", bcVersion, core.BlockChainVersion)
	// 	}
	// 	core.WriteBlockChainVersion(chainDb, core.BlockChainVersion)
	// }
	var (
		cacheConfig = &core.CacheConfig{Disabled: config.NoPruning, TrieNodeLimit: config.TrieCache, TrieTimeLimit: config.TrieTimeout}
	)
	elephant.blockchain, err = core.NewBlockChain(chainDb, cacheConfig, elephant.chainConfig, elephant.engine, lamp, &cb)
	if err != nil {
		return nil, err
	}
	// Rewind the chain in case of an incompatible config upgrade.
	if compat, ok := genesisErr.(*params.ConfigCompatError); ok {
		log.Warn("Rewinding chain to upgrade configuration", "err", compat)
		elephant.blockchain.SetHead(compat.RewindTo)
		core.WriteChainConfig(chainDb, genesisHash, chainConfig)
	}
	//elephant.bloomIndexer.Start(lamp.blockchain)

	if config.TxPool.Journal != "" {
		config.TxPool.Journal = ctx.ResolvePath(config.TxPool.Journal)
	}
	if config.TxPool.InputJournal != "" {
		config.TxPool.InputJournal = ctx.ResolvePath(config.TxPool.InputJournal)
	}
	elephant.txPool = core.NewTxPool(config.TxPool, elephant.chainConfig, elephant.blockchain, uint64(lamp.CurrBlockNum()), &cb)

	elephant.miner = miner.New(elephant, elephant.chainConfig, elephant.EventMux(), elephant.engine, lamp, loggerBFT)
	elephant.miner.SetExtra(makeExtraData(config.ExtraData))

	elephant.ApiBackend = &ElephantApiBackend{
		ele: elephant,
	}

	if common.GetConsensusBft() {
		elephant.hbftNode.SubscribeChainHeadEvent()
		elephant.hbftNode.InitNodeTable()
	}

	elephant.storageManager = storage.NewManager(&cb)
	elephant.storageManager.LoadClaimDatas()

	if elephant.protocolManager, err = NewProtocolManager(elephant.chainConfig, config.SyncMode, config.NetworkId, elephant.eventMux, elephant.txPool, elephant.engine, elephant.blockchain, chainDb, elephant, lamp); err != nil {
		return nil, err
	}
	// exporter.ExportObjectChanInit()

	elephant.police = police.New(elephant.protocolManager, lamp)

	return elephant, nil
}

func createLogger(fileName string) (logger log.Logger) {
	logger = log.New()
	path := filepath.Join(defaultDataDir(""), fileName)
	dir := filepath.Join(defaultDataDir(""))
	if err := os.MkdirAll(dir, 0700); err != nil {
		log.Warn("createLogger failed", "err", err)
		return nil
	}
	if ok, _ := pathExists(path); ok {
		bak := path + ".bak"
		if okBak, _ := pathExists(bak); okBak {
			os.Remove(bak)
		}
		os.Rename(path, bak)
	}
	fileHandle := log.LvlFilterHandler(log.LvlInfo, log.GetDefaultFileHandle(path))
	logger.SetHandler(fileHandle)
	//logger.SetHandler(log.LvlFilterHandler(log.LvlInfo, log.StreamHandler(os.Stdout, log.TerminalFormat(false))))

	return
}

func defaultDataDir(nodeName string) string {
	// Try to place the data folder in the user's home dir
	home := homeDir()
	if home != "" {
		if runtime.GOOS == "darwin" {
			return filepath.Join(home, "Library", "Contatract", "HBFTDebugLog")
		} else if runtime.GOOS == "windows" {
			return filepath.Join(home, "AppData", "Roaming", "Contatract", "HBFTDebugLog")
		} else {
			return filepath.Join(home, ".contatract", "HBFTDebugLog")
		}
	}

	return ""
}

func homeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if usr, err := user.Current(); err == nil {
		return usr.HomeDir
	}
	return ""
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func makeExtraData(extra []byte) []byte {
	if len(extra) == 0 {
		// create default extradata
		extra, _ = rlp.EncodeToBytes([]interface{}{
			uint(params.VersionMajor<<16 | params.VersionMinor<<8 | params.VersionPatch),
			"gctt",
			runtime.Version(),
			runtime.GOOS,
		})
	}
	if uint64(len(extra)) > params.MaximumExtraDataSize {
		log.Warn("Miner extra data exceed limit", "extra", hexutil.Bytes(extra), "limit", params.MaximumExtraDataSize)
		extra = nil
	}
	return extra
}

// CreateDB creates the chain database.
func CreateDB(ctx *node.ServiceContext, config *Config, name string) (ethdb.Database, error) {
	db, err := ctx.OpenDatabase(name, config.DatabaseCache, config.DatabaseHandles)
	if err != nil {
		return nil, err
	}
	if db, ok := db.(*ethdb.LDBDatabase); ok {
		db.Meter("elephant/db/chaindata/")
	}
	return db, nil
}

// CreateConsensusEngine creates the required type of consensus engine instance for an Ethereum service
//func CreateConsensusEngine(ctx *node.ServiceContext, config *elephanthash.Config, chainConfig *params.ElephantChainConfig, db ethdb.Database, hbftNode *nodeHbft.Node) consensus.EngineCtt {
func CreateConsensusEngine(config *elephanthash.Config, hbftNode *nodeHbft.Node) consensus.EngineCtt {
	// Otherwise assume proof-of-work
	switch {
	case config.PowMode == elephanthash.ModeFake:
		log.Warn("Elephanthash used in fake mode")
		return nil //elephanthash.NewFaker()
	case config.PowMode == elephanthash.ModeTest:
		log.Warn("Elephanthash used in test mode")
		return nil //elephanthash.NewTester()
	case config.PowMode == elephanthash.ModeShared:
		log.Warn("Ethash used in shared mode")
		return nil //elephanthash.NewShared()
	default:
		engine := elephanthash.New(elephanthash.Config{
			DatasetDir: config.DatasetDir,
		}, hbftNode)
		return engine
	}
}

// APIs returns the collection of RPC services the ethereum package offers.
// NOTE, some of these services probably need to be moved to somewhere else.
func (s *Elephant) APIs() []rpc.API {
	apis := elephantapi.GetAPIs(s.ApiBackend)
	apis = append(apis, []rpc.API{
		{
			Namespace: "elephant",
			Version:   "1.0",
			Service:   NewPublicElephantAPI(s),
			Public:    true,
		},
		{
			Namespace: "eleminer",
			Version:   "1.0",
			Service:   NewPublicMinerAPI(s),
			Public:    true,
		},
		{
			Namespace: "police",
			Version:   "1.0",
			Service:   NewPublicPoliceAPI(s),
			Public:    true,
		},
		{
			Namespace: "storage",
			Version:   "1.0",
			Service:   NewStorageAPI(s),
			Public:    true,
		},
	}...)

	// Append any APIs exposed explicitly by the consensus engine
	apis = append(apis, s.engine.APIs(s.BlockChain())...)

	// Append all the local APIs and return
	return append(apis, []rpc.API{}...)
}

func (s *Elephant) ResetWithGenesisBlock(gb *types.Block) {
	s.blockchain.ResetWithGenesisBlock(gb)
}

func (s *Elephant) Etherbase() (eb common.Address, err error) {
	s.lock.RLock()
	etherbase := s.etherbase
	s.lock.RUnlock()

	if etherbase != (common.Address{}) {
		return etherbase, nil
	}
	if wallets := s.AccountManager().Wallets(); len(wallets) > 0 {
		if accounts := wallets[0].Accounts(); len(accounts) > 0 {
			etherbase := accounts[0].Address

			s.lock.Lock()
			s.etherbase = etherbase
			s.lock.Unlock()

			log.Info("Etherbase automatically configured", "address", etherbase)
			return etherbase, nil
		}
	}
	return common.Address{}, fmt.Errorf("etherbase must be explicitly specified")
}

// set in js console via admin interface or wrapper from cli flags
func (self *Elephant) SetEtherbase(etherbase common.Address) {
	self.lock.Lock()
	self.etherbase = etherbase
	self.lock.Unlock()

	self.miner.SetEtherbase(etherbase)
	self.police.SetEtherbase(etherbase)
}

func (s *Elephant) StartMining(local bool) error {
	eb, err := s.Etherbase()
	if err != nil {
		log.Error("Cannot start mining without etherbase", "err", err)
		return fmt.Errorf("etherbase missing: %v", err)
	}

	if local {
		// If local (CPU) mining is started, we can disable the transaction rejection
		// mechanism introduced to speed sync times. CPU mining on mainnet is ludicrous
		// so noone will ever hit this path, whereas marking sync done on CPU mining
		// will ensure that private networks work in single miner mode too.
		atomic.StoreUint32(&s.protocolManager.acceptTxs, 1)
	}
	go s.miner.Start(eb)
	return nil
}

func (s *Elephant) StopMining()         { s.miner.Stop() }
func (s *Elephant) IsMining() bool      { return s.miner.Mining() }
func (s *Elephant) Miner() *miner.Miner { return s.miner }

func (s *Elephant) StartPoliceWorking() error {
	eb, err := s.Etherbase()
	if err != nil {
		log.Error("Cannot start police officer working without etherbase", "err", err)
		return fmt.Errorf("etherbase missing: %v", err)
	}

	atomic.StoreUint32(&s.protocolManager.acceptTxs, 1)
	go s.police.Start(eb)
	log.Info("Police officer verification work started", "etherbase", eb.String())

	if s.policeIsWorking != nil {
		*s.policeIsWorking = true
	}

	return nil
}

func (s *Elephant) StopPoliceWorking() {
	s.police.Stop()
	if s.policeIsWorking != nil {
		*s.policeIsWorking = false
	}
}
func (s *Elephant) IsPoliceWorking() bool          { return *s.policeIsWorking }
func (s *Elephant) PoliceOfficer() *police.Officer { return s.police }

func (s *Elephant) AccountManager() *accounts.Manager   { return s.accountManager }
func (s *Elephant) BlockChain() *core.BlockChain        { return s.blockchain }
func (s *Elephant) TxPool() *core.TxPool                { return s.txPool }
func (s *Elephant) EventMux() *event.TypeMux            { return s.eventMux }
func (s *Elephant) Engine() consensus.EngineCtt         { return s.engine }
func (s *Elephant) ChainDb() ethdb.Database             { return s.chainDb }
func (s *Elephant) IsListening() bool                   { return true } // Always listening
func (s *Elephant) EthVersion() int                     { return int(s.protocolManager.SubProtocols[0].Version) }
func (s *Elephant) NetVersion() uint64                  { return s.networkId }
func (s *Elephant) Downloader() *downloader.Downloader  { return s.protocolManager.downloader }
func (s *Elephant) GetEthHeight() uint64                { return uint64(s.protocolManager.lamp.CurrBlockNum()) }
func (s *Elephant) GetEBase() *common.Address           { return &s.etherbase }
func (s *Elephant) GetStorageManager() *storage.Manager { return s.storageManager }

// Protocols implements node.Service, returning all the currently configured
// network protocols to start.
func (s *Elephant) Protocols() []p2p.Protocol {
	return s.protocolManager.SubProtocols
}

// Start implements node.Service, starting all internal goroutines needed by the
// Ethereum protocol implementation.
func (s *Elephant) Start(srvr *p2p.Server) error {

	// Start the RPC service
	// s.netRPCService = ethapi.NewPublicNetAPI(srvr, s.NetVersion())

	s.node = srvr.Self()
	s.policeIsWorking = srvr.GetPoliceStatus()

	// Figure out a max peers count based on the server limits
	maxPeers := srvr.MaxPeers

	// Start the networking layer and the light server if requested
	s.protocolManager.Start(maxPeers)
	return nil
}

// Stop implements node.Service, terminating all internal goroutines used by the
// Ethereum protocol.
func (s *Elephant) Stop() error {
	//debug.PrintStack()

	// if s.stopDbUpgrade != nil {
	// 	s.stopDbUpgrade()
	// }

	s.blockchain.Stop()
	s.protocolManager.Stop()

	s.txPool.Stop()
	s.miner.Stop()
	s.eventMux.Stop()

	s.chainDb.Close()
	close(s.shutdownChan)

	for _, ch := range s.replyBftChan {
		close(ch)
	}

	return nil
}

func (s *Elephant) CreateBftMsgChan(viewPrimary string) {
	s.replyBftChan[viewPrimary] = make(chan *hbft.BFTMsg)
}

func (s *Elephant) ReplyBftChan(viewPrimary string) chan *hbft.BFTMsg {
	return s.replyBftChan[viewPrimary]
}

func (s *Elephant) SendBFTMsg(msg hbft.MsgHbftConsensus, msgType uint) {
	bftMsg := hbft.NewBFTMsg(msg, msgType)
	if bftMsg == nil {
		return
	}

	if !bftMsg.IsReply() {
		s.protocolManager.BroadcastBFTMsg(*bftMsg)
	} else {
		ticker := time.NewTicker(time.Millisecond * 100)
		defer ticker.Stop()
		num := 0

		for {
			select {
			case <-ticker.C:
				if num >= 5 {
					return
				}
				num++

			case s.replyBftChan[bftMsg.ViewPrimary] <- bftMsg:
				return
			}
		}
	}
}

// GetCurRealSealers returns current real elephant sealers
func (s *Elephant) GetCurRealSealers() ([]common.Address, bool) {
	if sealers, useNew := s.GetRealSealersByNum(s.blockchain.CurrentBlock().NumberU64()); sealers != nil {
		return sealers, useNew
	} else {
		log.Error("GetCurRealSealers failed, can not find current block")
		return nil, useNew
	}
}

// GetRealSealersByNum returns real elephant sealers of the number of the block given
func (s *Elephant) GetRealSealersByNum(number uint64) ([]common.Address, bool) {
	useNew := true
	block := s.blockchain.GetBlockByNumber(number)
	if block == nil {
		return nil, false
	}

	lamp := block.LampHash()
	if lamp.Equal(types.EmptyLampBaseHash) {
		return s.lamp.GetNewestEleSealers(), useNew
	}

	cnt := 0
	for num := block.Number().Uint64(); cnt < baseparam.BFTSealersWorkDelay; num-- {
		if num == 0 {
			return s.lamp.GetNewestEleSealers(), useNew
		}

		if num != block.Number().Uint64() && !lamp.Equal(s.blockchain.GetBlockByNumber(num).LampHash()) {
			useNew = false
			break
		}
		cnt++
	}

	if useNew {
		return s.lamp.GetNewestEleSealers(), useNew
	} else {
		return s.lamp.GetLastEleSealers(), useNew
	}
}

// Get2fRealSealersCnt returns the count of 2f in PBFT paper
func (s *Elephant) Get2fRealSealersCnt() int {
	sealers, _ := s.GetCurRealSealers()
	f := (len(sealers) - 1) / 3
	for offset := 0; ; offset++ {
		if 3*(f+offset)+1 >= len(sealers) {
			return 2 * (f + offset)
		}
	}
}

func (s *Elephant) GetBlockByHash(hash common.Hash) *types.Block {
	return s.blockchain.GetBlockByHash(hash)
}

func (s *Elephant) GetBlockByNumber(number uint64) *types.Block {
	return s.blockchain.GetBlockByNumber(number)
}

func (s *Elephant) GetCurrentBlock() *types.Block {
	return s.blockchain.CurrentBlock()
}

func (s *Elephant) AddBFTFutureBlock(b *types.Block) {
	s.blockchain.AddBFTFutureBlock(b)
}

func (s *Elephant) BFTFutureBlocks() []*types.Block {
	return s.blockchain.BFTFutureBlocks()
}

func (s *Elephant) BFTOldestFutureBlock() *types.Block {
	return s.blockchain.BFTOldestFutureBlock()
}

func (s *Elephant) ClearAllBFTFutureBlock() {
	s.blockchain.ClearAllBFTFutureBlock()
}

func (s *Elephant) ClearOutDatedBFTFutureBlock(height uint64) {
	s.blockchain.ClearOutDatedBFTFutureBlock(height)
}

func (s *Elephant) ClearWorkerFutureParent() {
	s.miner.ClearWorkerFutureParent()
}

// NextIsNewSealersFirstBlock returns whether the next block after the input block is first one to be produced by new sealers
func (s *Elephant) NextIsNewSealersFirstBlock(block *types.Block) bool {
	lamp := common.Hash{}
	cnt := 0
	for num := block.NumberU64(); cnt < baseparam.BFTSealersWorkDelay+1; num-- {
		if num == 0 && cnt == 0 {
			return true
		} else if num == 0 {
			return false
		}

		if num == block.Number().Uint64() {
			lamp = block.LampHash()
		} else if !lamp.Equal(s.blockchain.GetBlockByNumber(num).LampHash()) {
			break
		}
		cnt++

	}

	if cnt == baseparam.BFTSealersWorkDelay {
		return true
	}

	return false
}

func (s *Elephant) ValidateNewBftBlocks(blocks []*types.Block) error {
	headers := make([]*types.Header, len(blocks))
	for i, block := range blocks {
		headers[i] = block.Header()
	}

	var stateDb *state.StateDB
	futureBlocks, _ := lru.New(4)
	abort, results := s.blockchain.Engine().VerifyHeaders(s.blockchain, headers)
	defer close(abort)

	for i, block := range blocks {
		futureBlock := false
		if core.BadHashes[block.Hash()] {
			return core.ErrBlacklistedHash
		}
		err := <-results
		if err != nil {
			return err
		}
		futureBlocks.Add(block.Hash(), block)
		err = s.blockchain.Validator().ValidateBody(block, false)
		if err == consensus.ErrUnknownAncestor && futureBlocks.Contains(block.ParentHash()) {
			futureBlock = true
		} else if err != nil {
			return err
		}
		err = s.blockchain.ValidateInputShardingTxs(block)
		if err != nil {
			return err
		}

		// Create a new stateDb using the parent block and report an
		// error if it fails.
		var parent *types.Block
		if i == 0 {
			parent = s.blockchain.GetBlock(block.ParentHash(), block.NumberU64()-1)
		} else {
			parent = blocks[i-1]
		}

		if !futureBlock {
			stateDb, err = state.New(parent.Root(), *s.blockchain.StateCache(), s.blockchain.GetFeeds(), s.blockchain.GetElebase())
			if err != nil {
				return err
			}
		}

		// Process block using the parent state as reference point.
		receipts, usedGas, err := s.blockchain.Processor().Process(block, stateDb, uint64(s.lamp.CurrBlockNum()), &s.etherbase)
		if err != nil {
			return err
		}
		err = s.blockchain.Validator().ValidateState(block, parent, stateDb, receipts, usedGas)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Elephant) SendHBFTCurrentBlock(currentBlock *types.Block) {
	s.protocolManager.BroadcastBlock(currentBlock, true, true)
}

func (s *Elephant) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return s.BlockChain().SubscribeChainHeadEvent(ch)
}

func (s *Elephant) GetStateDB() *state.StateDB {
	return s.miner.GetStateDB()
}

func (s *Elephant) GetCurrentBlockNumber() uint64 {
	block := s.blockchain.CurrentBlock()
	if block != nil {
		return block.NumberU64()
	}
	return 0
}

func (s *Elephant) GetCurrentBlockHash() common.Hash {
	block := s.blockchain.CurrentBlock()
	if block != nil {
		return block.Hash()
	}
	return common.Hash{}
}

func (s *Elephant) GetFarmerDataRes(chunkId uint64) *storage.FarmerData {
	return s.GetStateDB().GetFarmerDataRes(chunkId)
}

// 需要先申请chunk
func (s *Elephant) GetSliceFromState(address common.Address, objId uint64) []*storagecore.Slice {
	return s.GetStateDB().GetStateSlice(address, objId)
}

func (s *Elephant) ClientCreateObject(address common.Address, objId uint64, size uint64, password string) {
	renter := address
	if common.EmptyAddress(renter) {
		log.Error("etherbase unset")
		return
	}
	addrLocker := new(elephantapi.AddrLocker)
	privateAccountAPI := elephantapi.NewPrivateAccountAPI(s.ApiBackend, addrLocker)
	ctx := context.Background()
	var args elephantapi.SendTxArgs
	args.From = renter
	to := common.HexToAddress("0x04cf866b90f9e65d9c2cc30ae8bcbadec27d26b8")
	args.To = &to
	args.Value = (*hexutil.Big)(big.NewInt(5555))
	args.ActType = blizparam.TypeActRent

	rentData := storage.RentData{
		Size:  size,
		ObjId: objId,
	}
	var err error
	if args.ActData, err = rlp.EncodeToBytes(rentData); err != nil {
		log.Error("Client create object", "err", err)
		return
	}

	if strings.Compare(password, "") == 0 {
		password = "block"
	}

	privateAccountAPI.SendTransaction(ctx, args, password)
}

func (s *Elephant) ClientClaimChunk(password string, address common.Address, node *discover.Node, blockNumber uint64) (uint64, error) {
	// chunkId := s.GetStateDB().GetClaimChunk(address, node, blockNumber)
	// log.Info("ClientClaimChunk", "chunkId", chunkId)
	// if chunkId != 0 {
	// 	return chunkId, s.ClaimChunk(password, address, chunkId)
	// }
	// return 0, errors.New("Getting claim chunk fails")
	var chunkId uint64
	return chunkId, s.ClaimChunk(password, address, chunkId)
}

func (s *Elephant) ClaimChunk(password string, address common.Address, chunkId uint64) error {
	claimer := address
	var err error
	// if !s.storageManager.CheckClaimFile(claimer, chunkId) {
	// 	return errors.New("check claim file fail")
	// }
	// s.storageManager.LoadClaimData(claimer, chunkId)
	// content, hashs, err := s.storageManager.GetVerifyData(claimer, chunkId)
	// if err != nil {
	// 	log.Error("GetVerifyData", "err", err)
	// 	return err
	// }
	addrLocker := new(elephantapi.AddrLocker)
	privateAccountAPI := elephantapi.NewPrivateAccountAPI(s.ApiBackend, addrLocker)
	ctx := context.Background()
	var args elephantapi.SendTxArgs
	args.From = claimer
	args.To = &args.From
	args.Value = blizparam.TxCost0
	args.ActType = blizparam.TypeActClaim

	claimData := storage.ClaimData{
		ChunkId: chunkId,
		// Content: content,
		// Hashs:   hashs,
		Node: s.node,
	}
	if args.ActData, err = rlp.EncodeToBytes(claimData); err != nil {
		log.Error("Claim chunk", "err", err)
		return err
	}

	if strings.Compare(password, "") == 0 {
		password = "block"
	}

	_, err = privateAccountAPI.SendTransaction(ctx, args, password)
	if err == nil {
		err = s.AddClaim(claimer)
	}
	return err
}

func (s *Elephant) AddClaim(addr common.Address) error {
	cnt, err := s.GetClaim(addr)
	if err != nil {
		return err
	}
	new := strconv.Itoa(cnt + 1)
	return s.storageManager.PutClaim(addr, []byte(new))
}

func (s *Elephant) GetClaim(addr common.Address) (int, error) {
	v, err := s.storageManager.GetClaim(addr)
	if err != nil {
		return 0, err
	}
	cnt, err := strconv.Atoi(string(v))
	if err != nil {
		return 0, err
	}
	return cnt, nil
}

func (s *Elephant) ClientGetBlizCSObject(address common.Address, ty uint8) exporter.ObjectsInfo {
	object := make(exporter.ObjectsInfo, 0)
	if (ty & 1) != 0 {
		selfObject := s.GetStateDB().GetBlizCSSelfObject(address)
		for _, s := range selfObject {
			object = append(object, s)
		}
	}
	if (ty & 2) != 0 {
		shareObject := s.GetStateDB().GetBlizCSShareObject(address)
		for _, s := range shareObject {
			object = append(object, s)
		}
	}
	return object
}

func fileTransfer(pathName string) []byte {
	content, err := ioutil.ReadFile(pathName)
	if err != nil {
		log.Error("ReadFile", "err", err)
		return nil
	}
	_, name := filepath.Split(pathName)
	type FileStrans struct {
		Name    string
		Content []byte
	}
	data := FileStrans{
		Name:    name,
		Content: content,
	}
	ret, err := json.Marshal(data)
	if err != nil {
		log.Error("Marshal", "err", err)
		return nil
	}
	err = os.Remove(pathName)
	if err != nil {
		log.Error("Remove", "err", err)
	}
	return ret
}

func (s *Elephant) ExportKS(addr common.Address, passphrase string) string {
	am := s.accountManager
	acc := accounts.Account{Address: addr}
	_, err := am.Find(acc)
	if err != nil {
		log.Error("find", "err", err)
		return ""
	}
	ks := fetchKeystore(am)
	keyJson, err := ks.Export(acc, passphrase, passphrase)
	if err != nil {
		log.Error("Export", "err", err)
		return ""
	}
	log.Info("Export", "keyJson len", len(keyJson), "keyJson", string(keyJson))
	//acc, err = ks.Import(keyJson, passphrase, passphrase)
	//if err != nil {
	//	log.Error("Import", "err", err)
	//	return ""
	//}
	//log.Info("Import", "addr", acc.Address, "Path", acc.URL.Path)
	//return string(fileTransfer(acc.URL.Path))
	return hex.EncodeToString(keyJson)
}

// fetchKeystore retrives the encrypted keystore from the account manager.
func fetchKeystore(am *accounts.Manager) *keystore.KeyStore {
	return am.Backends(keystore.KeyStoreType)[0].(*keystore.KeyStore)
}

func (s *Elephant) GetServerCliAddr(addr common.Address) []common.CSAuthData {
	return s.GetStateDB().GetServerCliAddr(addr)
}

func (s *Elephant) GetSignData(query *exporter.ObjGetElephantData) []interface{} {
	switch query.Ty {
	case exporter.TypeElephantSign:
		var csAddr common.Address
		if _, ok := query.Params[0].(common.Address); ok {
			csAddr = query.Params[0].(common.Address)
		}
		if common.EmptyAddress(csAddr) {
			return nil
		}
		return s.GetStateDB().GetSignData(query.Address, csAddr)
	}
	return nil
}

func (s *Elephant) CsGetServeCost(csbase, user common.Address, usedFlow uint64, authAllowFlow uint64, sign []byte) {
	addrLocker := new(elephantapi.AddrLocker)
	privateAccountAPI := elephantapi.NewPrivateAccountAPI(s.ApiBackend, addrLocker)
	ctx := context.Background()
	var args elephantapi.SendTxArgs
	args.From = csbase
	to := common.HexToAddress(blizparam.DestAddrStr)
	args.To = &to
	args.Value = blizparam.TxDefaultCost
	args.ActType = blizparam.TypeActPickupCost
	param := blizparam.CsPickupCostParam{
		Addr:          user.Hex(),
		UsedFlow:      usedFlow,
		AuthAllowFlow: authAllowFlow,
		Sign:          sign,
		Now:           uint64(time.Now().Unix()),
	}
	ret, err := rlp.EncodeToBytes(param)
	if err != nil {
		log.Error("Cs get serve cost", "err", err)
		return
	}
	args.ActData = ret
	password := blizparam.GetCsSelfPassphrase()

	_, err = privateAccountAPI.SendTransaction(ctx, args, password)
	if err != nil {
		log.Error("SendTransaction", "err", err)
	}
	return
}

func (s *Elephant) UserAuthAllowFlow(user, csbase common.Address, flow uint64, sign []byte) {
	addrLocker := new(elephantapi.AddrLocker)
	privateAccountAPI := elephantapi.NewPrivateAccountAPI(s.ApiBackend, addrLocker)
	ctx := context.Background()
	var args elephantapi.SendTxArgs
	args.From = csbase
	to := common.HexToAddress(blizparam.DestAddrStr)
	args.To = &to
	args.Value = blizparam.TxDefaultCost
	args.ActType = blizparam.TypeActUsedFlow
	param := blizparam.CsUsedFlowParam{
		Addr: user.Hex(),
		Flow: flow,
	}
	ret, err := rlp.EncodeToBytes(param)
	if err != nil {
		log.Error("User auth allow flow", "err", err)
		return
	}
	args.ActData = ret
	password := blizparam.GetCsSelfPassphrase()

	_, err = privateAccountAPI.SendTransaction(ctx, args, password)
	if err != nil {
		log.Error("SendTransaction", "err", err)
	}
	return
}

func (s *Elephant) CsAddUsedFlow(csbase, user common.Address, flow uint64) {
	addrLocker := new(elephantapi.AddrLocker)
	privateAccountAPI := elephantapi.NewPrivateAccountAPI(s.ApiBackend, addrLocker)
	ctx := context.Background()
	var args elephantapi.SendTxArgs
	args.From = csbase
	to := common.HexToAddress(blizparam.DestAddrStr)
	args.To = &to
	args.Value = blizparam.TxDefaultCost
	args.ActType = blizparam.TypeActUsedFlow
	param := blizparam.CsUsedFlowParam{
		Addr: user.Hex(),
		Flow: flow,
	}
	ret, err := rlp.EncodeToBytes(param)
	if err != nil {
		log.Error("Cs add used flow", "err", err)
		return
	}
	args.ActData = ret
	password := blizparam.GetCsSelfPassphrase()

	_, err = privateAccountAPI.SendTransaction(ctx, args, password)
	if err != nil {
		log.Error("SendTransaction", "err", err)
	}
	return
}

func (s *Elephant) CsGetPickupFlow(csbase, user common.Address, flow uint64) {
	addrLocker := new(elephantapi.AddrLocker)
	privateAccountAPI := elephantapi.NewPrivateAccountAPI(s.ApiBackend, addrLocker)
	ctx := context.Background()
	var args elephantapi.SendTxArgs
	args.From = csbase
	to := common.HexToAddress(blizparam.DestAddrStr)
	args.To = &to
	args.Value = blizparam.TxDefaultCost
	args.ActType = blizparam.TypeActPickupFlow
	param := blizparam.PickupFlowParam{
		Addr:       user.Hex(),
		PickupFlow: flow,
	}
	ret, err := rlp.EncodeToBytes(param)
	if err != nil {
		log.Error("Cs get pickup flow", "err", err)
		return
	}
	args.ActData = ret
	password := blizparam.GetCsSelfPassphrase()

	_, err = privateAccountAPI.SendTransaction(ctx, args, password)
	return
}

func (s *Elephant) CsGetPickupTime(csbase, user common.Address, seconds uint64) {
	addrLocker := new(elephantapi.AddrLocker)
	privateAccountAPI := elephantapi.NewPrivateAccountAPI(s.ApiBackend, addrLocker)
	ctx := context.Background()
	var args elephantapi.SendTxArgs
	args.From = csbase
	to := common.HexToAddress(blizparam.DestAddrStr)
	args.To = &to
	args.Value = (*hexutil.Big)(blizparam.GetTimeBalance(seconds))
	args.ActType = blizparam.TypeActPickupTime
	param := blizparam.PickupTimeParam{
		Addr:       user.Hex(),
		PickupTime: seconds,
	}
	ret, err := rlp.EncodeToBytes(param)
	if err != nil {
		log.Error("Cs get pickup time", "err", err)
		return
	}
	args.ActData = ret
	password := blizparam.GetCsSelfPassphrase()

	_, err = privateAccountAPI.SendTransaction(ctx, args, password)
	return
}

func (s *Elephant) GenerateShardingTxBatches(block *types.Block, stateDB *state.StateDB) (types.ShardingTxBatches, map[uint16]uint64, error) {
	shardingTxBatches := make([]*types.ShardingTxBatch, 0)
	mapBatchNonce := make(map[uint16]uint64)

	outPutTxs := block.OutputTxs(&s.etherbase)
	ethHeight := block.LampBaseNumberU64()
	txsByShardID := types.GetOutTxsByShardingID(ethHeight, outPutTxs)
	txsHashesByShardID := make(map[uint16]common.Hash)
	for id, txs := range txsByShardID {
		txsHashesByShardID[id] = types.DeriveSha(txs)
	}
	trieDb := types.DeriveTrieForOutTxs(txsHashesByShardID)
	if trieDb == nil {
		return nil, nil, errors.New("failed Generating transactions trie when making the proof")
	}

	blockHash := block.Hash()
	outTxRoot := block.OutTxHash()
	shardingID := common.GetEleSharding(&s.etherbase, ethHeight)
	blockHashPath := block.HashNoOutTxRoot()
	for _, shardingTxs := range txsByShardID {
		shardingTxBatch := types.NewMinedShardingTxBatch(shardingTxs, &outTxRoot, &blockHash, block.Number().Uint64())

		toShardingID := shardingTxBatch.ToShardingID()
		batchNonce := stateDB.GetBatchNonce(toShardingID) + 1
		shardingTxBatch.SetBatchNonce(batchNonce)
		mapBatchNonce[toShardingID] = batchNonce

		proof, _ := ethdb.NewMemDatabase()
		errP := trieDb.Prove(shardingTxBatch.TxsHash().Bytes(), 0, proof)
		if errP != nil {
			log.Error("Failed Generating sharding transactions when making the proof", "err", errP)
			return nil, nil, errors.New(fmt.Sprintf("Failed Generating sharding transactions when making the proof, %s", errP.Error()))
		}
		shardingTxBatch.SetProof(proof)

		shardingTxBatch.SetBlockHashPath(&blockHashPath)
		shardingTxBatch.SetShardingInfo(&block.Header().LampBaseHash, block.Header().LampBaseNumber.Uint64(), shardingID)

		if !shardingTxBatch.IsValid() {
			return nil, nil, errors.New("failed Generating sharding transactions when verifying the proof")
		}
		shardingTxBatches = append(shardingTxBatches, shardingTxBatch)
	}
	return shardingTxBatches, mapBatchNonce, nil
}

func (e *Elephant) Verify(password string, chunkId uint64, h uint64) bool {
	e.Etherbase()
	verifyer := e.etherbase
	if common.EmptyAddress(verifyer) {
		log.Error("etherbase unset")
		return false
	}
	content, hashs, err := e.storageManager.GetVerifyData(verifyer, chunkId)
	if err != nil {
		log.Error("get verify data", "err", err)
		return false
	}
	// for i, h := range hashs {
	// 	fmt.Println(i, ", hash: ", h)
	// }
	// fmt.Println("content: ", string(content))
	// rootHash := storage.GetRootHash(content, hashs)
	// fmt.Println("roothash: ", rootHash)

	addrLocker := new(elephantapi.AddrLocker)
	privateAccountAPI := elephantapi.NewPrivateAccountAPI(e.ApiBackend, addrLocker)
	ctx := context.Background()
	var args elephantapi.SendTxArgs
	args.From = verifyer
	args.To = &args.From
	args.Value = blizparam.TxCost0
	args.ActType = blizparam.TypeActVerify

	verifyData := storage.VerifyData{
		ChunkId: chunkId,
		Content: content,
		Hashs:   hashs,
		H:       h,
	}
	if args.ActData, err = rlp.EncodeToBytes(verifyData); err != nil {
		log.Error("Verify", "err", err)
		return false
	}

	if strings.Compare(password, "") == 0 {
		password = "block"
	}

	privateAccountAPI.SendTransaction(ctx, args, password)
	return true
}

func (e *Elephant) CheckFarmerAward(eleHeight uint64, blockHash common.Hash, chunkID uint64, h uint64) {
	return
	if eleHeight%20 != 0 {
		return
	}
	// Algorithm: todo
	// if blockHash[0] % 2 != 0 {
	// 	return
	// }
	log.Info("Check farmer award pass", "chunkid", chunkID)
	passwd := blizparam.GetCsSelfPassphrase()
	e.Verify(passwd, chunkID, h)
}

func (e *Elephant) GetLeftRentSize() uint64 {
	return e.GetStateDB().GetLeftRentSize()
}

func (e *Elephant) GetUnusedChunkCnt(address common.Address, cIds []uint64) uint32 {
	return e.GetStateDB().GetUnusedChunkCnt(address, cIds)
}

func (e *Elephant) GetBalance(addr common.Address) *big.Int {
	return e.GetStateDB().GetBalance(addr)
}

func (e *Elephant) ObjMetaQueryFunc(query *exporter.ObjMetaQuery) {
	peer := e.protocolManager.peers.Peer(query.Peer.PeersKey())
	log.Info("exporterLoop", "query.Peer", query.Peer, "peer", peer)
	if peer != nil {
		// TODO:使用 peer 发起查询请求
		atomic.AddUint64(&queryId, 1)
		queryMap[queryId] = query.RspFeed
		queryTs[queryId] = time.Now().Unix()
		// 将queryId带到查询请求中，接收peer处理消息后原样返回
		// 这里就能找到合适的请求者chan进行通知
		peer.SendBlizObjectMetaQuery(&storage.GetObjectMetaData{Address: query.Base, ObjectId: query.ObjId, QueryId: queryId})
	}
}

func (e *Elephant) ObjFarmerQueryFunc(query *exporter.ObjFarmerQuery) {
	peer := e.protocolManager.peers.Peer(query.Peer.PeersKey())
	if peer != nil {
		// TODO:使用 peer 发起查询请求
		atomic.AddUint64(&queryId, 1)
		queryMap[queryId] = query.RspFeed
		queryTs[queryId] = time.Now().Unix()
		// 将queryId带到查询请求中，接收peer处理消息后原样返回
		// 这里就能找到合适的请求者chan进行通知
		peer.GetChunkProvider(storage.ChunkIdentity2Int(query.ChunkId), queryId)
	}
}

func (e *Elephant) ObjClaimChunkQueryFunc(query *exporter.ObjClaimChunkQuery) (uint64, error) {
	return e.ClientClaimChunk(query.Password, query.Address, query.Node, e.GetCurrentBlockNumber())
}

func (e *Elephant) ObjGetObjectFunc(query *exporter.ObjGetObjectQuery) exporter.ObjectsInfo {
	return e.ClientGetBlizCSObject(query.Base, query.Ty)
}

func (e *Elephant) ObjGetCliAddrFunc(query *exporter.ObjGetCliAddr) []common.CSAuthData {
	return e.GetServerCliAddr(query.Address)
}

func (e *Elephant) ObjGetElephantDataRspFunc(query *exporter.ObjGetElephantData) *exporter.ObjGetElephantDataRsp {
	if query.Ty != exporter.TypeElephantSign {
		return nil
	}
	params := e.GetSignData(query)
	if len(query.Params) >= 2 {
		var peerId string
		if _, ok := query.Params[1].(string); ok {
			peerId = query.Params[1].(string)
		}
		params = append(params, peerId)
	}
	return &exporter.ObjGetElephantDataRsp{Ty: query.Ty, RspParams: params}
}

func (e *Elephant) ObjGetElephantDataFunc(query *exporter.ObjGetElephantData) {
	switch query.Ty {
	case exporter.TypeElephantCSGetFlow:
		csbase := query.Address
		user := query.Params[0].(common.Address)
		flow := query.Params[1].(uint64)
		e.CsGetPickupFlow(csbase, user, flow)

	case exporter.TypeCsToElephantServeCost:
		csbase := query.Params[0].(common.Address)
		cliAddr := query.Params[1].(common.Address)
		payMethod := query.Params[2].(uint8)
		var usedFlow uint64
		var authAllowFlow uint64
		var sign []byte
		if payMethod == 1 {
			usedFlow = query.Params[3].(uint64)
			authAllowFlow = query.Params[4].(uint64)
			sign = query.Params[5].([]byte)
		}
		e.CsGetServeCost(csbase, cliAddr, usedFlow, authAllowFlow, sign)

	case exporter.TypeCsToElephantAuthAllowFlow:
		user := query.Params[0].(common.Address)
		csbase := query.Params[1].(common.Address)
		flow := query.Params[2].(uint64)
		sign := query.Params[3].([]byte)
		e.UserAuthAllowFlow(user, csbase, flow, sign)

	case exporter.TypeCsToElephantAddUsedFlow:
		csbase := query.Params[0].(common.Address)
		user := query.Params[1].(common.Address)
		flow := query.Params[2].(uint64)
		e.CsAddUsedFlow(csbase, user, flow)
	}
}

func (e *Elephant) Encrypt(str string, passwd string) {
	log.Info("Encrypt", "origin", str, "len", len([]byte(str)))
	acc := accounts.Account{Address: e.etherbase}
	ks := fetchKeystore(e.accountManager)
	priKey, err := ks.GetMailPriKey(acc, passwd)
	if err != nil {
		log.Info("Get mail pri key", "err", err)
		return
	}
	defer zeroKey(priKey)
	data, err := ecies.EncryptBytes(&priKey.PublicKey, []byte(str))
	if err != nil {
		log.Info("Encrypt bytes", "err", err)
		return
	}
	log.Info("Encrypt", "encryptedlen", len(data))
	ret, err := ecies.DecryptBytes(priKey, data)
	if err != nil {
		log.Info("Decrypt bytes", "err", err)
		return
	}
	result := "fail"
	if str == string(ret) {
		result = "success"
	}
	log.Info("Encrypt " + result)
}

func (e *Elephant) DesEncrypt(str string, passwd string) {
	log.Info("Des encrypt", "origin", str, "len", len(str))
	data, err := crypto.Encrypt([]byte(str), []byte(passwd), true)
	if err != nil {
		log.Info("Des encrypt", "err", err)
		return
	}
	log.Info("Des encrypt", "desencryptedlen", len(data))
	ret, err := crypto.Decrypt(data, []byte(passwd), true)
	if err != nil {
		log.Info("Des decrypt", "err", err)
		return
	}
	result := "fail"
	if str == string(ret) {
		result = "success"
	}
	log.Info("Des encrypt " + result)
}

func (e *Elephant) SendTransaction(password string, to string, value int64) (common.Hash, error) {
	addrLocker := new(elephantapi.AddrLocker)
	privateAccountAPI := elephantapi.NewPrivateAccountAPI(e.ApiBackend, addrLocker)
	ctx := context.Background()
	var args elephantapi.SendTxArgs
	args.From = e.etherbase
	toAddr := common.HexToAddress(to)
	args.To = &toAddr
	a := new(big.Int).SetInt64(value)
	b := new(big.Int).SetInt64(params.Ether)
	v := new(big.Int).Mul(a, b)
	args.Value = (*hexutil.Big)(v)
	return privateAccountAPI.SendTransaction(ctx, args, password)
}

func (e *Elephant) RegMail(addr common.Address, name string, password string) error {
	addrLocker := new(elephantapi.AddrLocker)
	privateAccountAPI := elephantapi.NewPrivateAccountAPI(e.ApiBackend, addrLocker)
	ctx := context.Background()
	var args elephantapi.SendTxArgs
	args.From = e.etherbase
	to := common.HexToAddress(blizparam.DestAddrStr)
	args.To = &to
	args.Value = (*hexutil.Big)(blizparam.TxDefaultValue)
	args.ActType = blizparam.TypeActRegMail

	acc := accounts.Account{Address: addr}
	ks := fetchKeystore(e.accountManager)
	pubKey, err := ks.GetMailPubKey(acc, password)
	if err != nil {
		return err
	}
	data := blizparam.RegMailData{
		Name:   name,
		PubKey: crypto.CompressPubkey(pubKey),
	}
	args.ActData, err = rlp.EncodeToBytes(data)
	if err != nil {
		return err
	}

	privateAccountAPI.SendTransaction(ctx, args, password)
	return nil
}
