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

// Package eth implements the Ethereum protocol.
package eth

import (
	"errors"
	"fmt"
	"math/big"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/contatract/go-contatract/accounts"
	"github.com/contatract/go-contatract/baseparam"
	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/common/hexutil"
	"github.com/contatract/go-contatract/consensus"
	"github.com/contatract/go-contatract/consensus/clique"
	"github.com/contatract/go-contatract/consensus/ethash"
	"github.com/contatract/go-contatract/core/bloombits"
	"github.com/contatract/go-contatract/core/types"
	"github.com/contatract/go-contatract/core/vm"
	core "github.com/contatract/go-contatract/core_eth"
	"github.com/contatract/go-contatract/eth/downloader"
	"github.com/contatract/go-contatract/eth/filters"
	"github.com/contatract/go-contatract/eth/gasprice"
	"github.com/contatract/go-contatract/ethdb"
	"github.com/contatract/go-contatract/event"
	"github.com/contatract/go-contatract/internal/ethapi"
	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/miner"
	"github.com/contatract/go-contatract/node"
	"github.com/contatract/go-contatract/p2p"
	"github.com/contatract/go-contatract/p2p/discover"
	"github.com/contatract/go-contatract/params"
	"github.com/contatract/go-contatract/rlp"
	"github.com/contatract/go-contatract/rpc"
)

type LesServer interface {
	Start(srvr *p2p.Server)
	Stop()
	Protocols() []p2p.Protocol
	SetBloomBitsIndexer(bbIndexer *core.ChainIndexer)
}

// Ethereum implements the Ethereum full node service.
type Ethereum struct {
	config      *Config
	chainConfig *params.ChainConfig

	// Channel for shutting down the service
	shutdownChan chan bool // Channel for shutting down the ethereum
	// stopDbUpgrade func() error // stop chain db sequential key upgrade

	// Handlers
	txPool          *core.TxPool
	electionPool    *core.ElectionPool
	verifHeaderPool *core.VerifiedHeaderPool
	blockchain      *core.BlockChain
	protocolManager *ProtocolManager
	lesServer       LesServer

	// DB interfaces
	chainDb ethdb.Database // Block chain database

	eventMux       *event.TypeMux
	engine         consensus.Engine
	accountManager *accounts.Manager

	bloomRequests chan chan *bloombits.Retrieval // Channel receiving bloom data retrieval requests
	bloomIndexer  *core.ChainIndexer             // Bloom indexer operating during block imports

	ApiBackend *EthApiBackend

	miner     *miner.Miner
	gasPrice  *big.Int
	etherbase common.Address

	networkId     uint64
	netRPCService *ethapi.PublicNetAPI

	verifiedHeader chan *consensus.PoliceVerifiedHeader
	stopRcvHeader  chan bool

	node *discover.Node

	mainNodeForBFTFlag bool

	lock sync.RWMutex // Protects the variadic fields (e.g. gas price and etherbase)
}

func (s *Ethereum) AddLesServer(ls LesServer) {
	s.lesServer = ls
	ls.SetBloomBitsIndexer(s.bloomIndexer)
}

// New creates a new Ethereum object (including the
// initialisation of the common Ethereum object)
func New(ctx *node.ServiceContext, config *Config) (*Ethereum, error) {
	if config.SyncMode == downloader.LightSync {
		return nil, errors.New("can't run eth.Ethereum in light sync mode, use les.LightEthereum")
	}
	if !config.SyncMode.IsValid() {
		return nil, fmt.Errorf("invalid sync mode %d", config.SyncMode)
	}
	chainDb, err := CreateDB(ctx, config, "chaindata")
	if err != nil {
		return nil, err
	}
	// stopDbUpgrade := upgradeDeduplicateData(chainDb)
	chainConfig, genesisHash, genesisErr := core.SetupGenesisBlock(chainDb, config.Genesis)
	if _, ok := genesisErr.(*params.ConfigCompatError); genesisErr != nil && !ok {
		return nil, genesisErr
	}
	log.Info("Initialised chain configuration", "config", chainConfig)

	eth := &Ethereum{
		config:         config,
		chainDb:        chainDb,
		chainConfig:    chainConfig,
		eventMux:       ctx.EventMux,
		accountManager: ctx.AccountManager,
		engine:         CreateConsensusEngine(ctx, &config.Ethash, chainConfig, chainDb),
		shutdownChan:   make(chan bool),
		verifiedHeader: make(chan *consensus.PoliceVerifiedHeader),
		// stopDbUpgrade:  stopDbUpgrade,
		networkId:     config.NetworkId,
		gasPrice:      config.GasPrice,
		etherbase:     config.Etherbase,
		bloomRequests: make(chan chan *bloombits.Retrieval),
		bloomIndexer:  NewBloomIndexer(chainDb, params.BloomBitsBlocks),
	}

	log.Info("Initialising Ethereum protocol", "versions", ProtocolVersions, "network", config.NetworkId)

	if !config.SkipBcVersionCheck {
		bcVersion := core.GetBlockChainVersion(chainDb)
		if bcVersion != core.BlockChainVersion && bcVersion != 0 {
			return nil, fmt.Errorf("Blockchain DB version mismatch (%d / %d). Run geth upgradedb.\n", bcVersion, core.BlockChainVersion)
		}
		core.WriteBlockChainVersion(chainDb, core.BlockChainVersion)
	}
	var (
		vmConfig    = vm.Config{EnablePreimageRecording: config.EnablePreimageRecording}
		cacheConfig = &core.CacheConfig{Disabled: config.NoPruning, TrieNodeLimit: config.TrieCache, TrieTimeLimit: config.TrieTimeout}
	)
	eth.blockchain, err = core.NewBlockChain(chainDb, cacheConfig, eth.chainConfig, eth.engine, vmConfig)
	if err != nil {
		return nil, err
	}

	if config.ElectionPool.SealersJournal != "" {
		config.ElectionPool.SealersJournal = ctx.ResolvePath(config.ElectionPool.SealersJournal)
	}
	if config.ElectionPool.PoliceJournal != "" {
		config.ElectionPool.PoliceJournal = ctx.ResolvePath(config.ElectionPool.PoliceJournal)
	}
	eth.electionPool = core.NewElectionPool(config.ElectionPool, eth.blockchain)
	eth.blockchain.SetElectionPool(eth.electionPool)

	eth.verifHeaderPool = core.NewVerifiedHeaderPool(eth.blockchain, eth)

	// Rewind the chain in case of an incompatible config upgrade.
	if compat, ok := genesisErr.(*params.ConfigCompatError); ok {
		log.Warn("Rewinding chain to upgrade configuration", "err", compat)
		eth.blockchain.SetHead(compat.RewindTo)
		core.WriteChainConfig(chainDb, genesisHash, chainConfig)
	}
	eth.bloomIndexer.Start(eth.blockchain)

	if config.TxPool.Journal != "" {
		config.TxPool.Journal = ctx.ResolvePath(config.TxPool.Journal)
	}
	eth.txPool = core.NewTxPool(config.TxPool, eth.chainConfig, eth.blockchain)

	if eth.protocolManager, err = NewProtocolManager(eth.chainConfig, config.SyncMode, config.NetworkId, eth.eventMux,
		eth.txPool, eth.electionPool, eth.engine, eth.blockchain, chainDb); err != nil {
		return nil, err
	}

	eth.ApiBackend = &EthApiBackend{eth, nil}

	eth.miner = miner.New(eth, eth.chainConfig, eth.EventMux(), eth.engine, nil)
	eth.miner.SetExtra(makeExtraData(config.ExtraData))
	if common.GetConsensusBftTest() {
		eb, err := eth.Etherbase()
		if err == nil {
			eth.etherbase = eb
			eth.miner.SetEtherbase(eb)
		}
	}

	gpoParams := config.GPO
	if gpoParams.Default == nil {
		gpoParams.Default = config.GasPrice
	}
	eth.ApiBackend.gpo = gasprice.NewOracle(eth.ApiBackend, gpoParams)

	return eth, nil
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
		db.Meter("eth/db/chaindata/")
	}
	return db, nil
}

// CreateConsensusEngine creates the required type of consensus engine instance for an Ethereum service
func CreateConsensusEngine(ctx *node.ServiceContext, config *ethash.Config, chainConfig *params.ChainConfig, db ethdb.Database) consensus.Engine {
	// If proof-of-authority is requested, set it up
	if chainConfig.Clique != nil {
		return clique.New(chainConfig.Clique, db)
	}
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
			CacheDir:       ctx.ResolvePath(config.CacheDir),
			CachesInMem:    config.CachesInMem,
			CachesOnDisk:   config.CachesOnDisk,
			DatasetDir:     config.DatasetDir,
			DatasetsInMem:  config.DatasetsInMem,
			DatasetsOnDisk: config.DatasetsOnDisk,
		})
		engine.SetThreads(-1) // Disable CPU mining
		return engine
	}
}

// APIs returns the collection of RPC services the ethereum package offers.
// NOTE, some of these services probably need to be moved to somewhere else.
func (s *Ethereum) APIs() []rpc.API {
	apis := ethapi.GetAPIs(s.ApiBackend)

	// Append any APIs exposed explicitly by the consensus engine
	apis = append(apis, s.engine.APIs(s.BlockChain())...)

	// Append all the local APIs and return
	return append(apis, []rpc.API{
		{
			Namespace: "eth",
			Version:   "1.0",
			Service:   NewPublicEthereumAPI(s),
			Public:    true,
		}, {
			Namespace: "eth",
			Version:   "1.0",
			Service:   NewPublicMinerAPI(s),
			Public:    true,
		}, {
			Namespace: "eth",
			Version:   "1.0",
			Service:   downloader.NewPublicDownloaderAPI(s.protocolManager.downloader, s.eventMux),
			Public:    true,
		}, {
			Namespace: "miner",
			Version:   "1.0",
			Service:   NewPrivateMinerAPI(s),
			Public:    false,
		}, {
			Namespace: "eth",
			Version:   "1.0",
			Service:   filters.NewPublicFilterAPI(s.ApiBackend, false),
			Public:    true,
		}, {
			Namespace: "admin",
			Version:   "1.0",
			Service:   NewPrivateAdminAPI(s),
		}, {
			Namespace: "debug",
			Version:   "1.0",
			Service:   NewPublicDebugAPI(s),
			Public:    true,
		}, {
			Namespace: "debug",
			Version:   "1.0",
			Service:   NewPrivateDebugAPI(s.chainConfig, s),
		}, {
			Namespace: "net",
			Version:   "1.0",
			Service:   s.netRPCService,
			Public:    true,
		},
	}...)
}

func (s *Ethereum) ResetWithGenesisBlock(gb *types.Block) {
	s.blockchain.ResetWithGenesisBlock(gb)
}

func (s *Ethereum) Etherbase() (eb common.Address, err error) {
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
func (self *Ethereum) SetEtherbase(etherbase common.Address) {
	self.lock.Lock()
	self.etherbase = etherbase
	self.lock.Unlock()

	self.miner.SetEtherbase(etherbase)
}

func (s *Ethereum) StartMining(local bool) error {
	eb, err := s.Etherbase()
	if err != nil {
		log.Error("Cannot start mining without etherbase", "err", err)
		return fmt.Errorf("etherbase missing: %v", err)
	}
	if clique, ok := s.engine.(*clique.Clique); ok {
		wallet, err := s.accountManager.Find(accounts.Account{Address: eb})
		if wallet == nil || err != nil {
			log.Error("Etherbase account unavailable locally", "err", err)
			return fmt.Errorf("signer missing: %v", err)
		}
		clique.Authorize(eb, wallet.SignHash)
	}
	if local {
		// If local (CPU) mining is started, we can disable the transaction rejection
		// mechanism introduced to speed sync times. CPU mining on mainnet is ludicrous
		// so noone will ever hit this path, whereas marking sync done on CPU mining
		// will ensure that private networks work in single miner mode too.
		atomic.StoreUint32(&s.protocolManager.acceptTxs, 1)
	}
	go s.miner.Start(eb)
	s.startReceiveVerifiedHeader()
	return nil
}

func (s *Ethereum) StopMining()         { s.miner.Stop(); s.stopReceiveVerifiedHeader() }
func (s *Ethereum) IsMining() bool      { return s.miner.Mining() }
func (s *Ethereum) Miner() *miner.Miner { return s.miner }

func (s *Ethereum) AccountManager() *accounts.Manager            { return s.accountManager }
func (s *Ethereum) BlockChain() *core.BlockChain                 { return s.blockchain }
func (s *Ethereum) TxPool() *core.TxPool                         { return s.txPool }
func (s *Ethereum) ElectionPool() *core.ElectionPool             { return s.electionPool }
func (s *Ethereum) VerifiedHeaderPool() *core.VerifiedHeaderPool { return s.verifHeaderPool }
func (s *Ethereum) EventMux() *event.TypeMux                     { return s.eventMux }
func (s *Ethereum) Engine() consensus.Engine                     { return s.engine }
func (s *Ethereum) ChainDb() ethdb.Database                      { return s.chainDb }
func (s *Ethereum) IsListening() bool                            { return true } // Always listening
func (s *Ethereum) EthVersion() int                              { return int(s.protocolManager.SubProtocols[0].Version) }
func (s *Ethereum) NetVersion() uint64                           { return s.networkId }
func (s *Ethereum) Downloader() *downloader.Downloader           { return s.protocolManager.downloader }

func (s *Ethereum) VerifiedHeaderChan() chan *consensus.PoliceVerifiedHeader {
	return s.verifiedHeader
}

func (s *Ethereum) CurrBlockNum() rpc.BlockNumber {
	return rpc.BlockNumber(s.blockchain.CurrentBlock().NumberU64())
}

func (s *Ethereum) CurrentBlock() *types.Block {
	return s.blockchain.CurrentBlock()
}

func (s *Ethereum) CurrentHeader() *types.Header {
	return s.blockchain.CurrentHeader()
}

func (s *Ethereum) CurrEleSealersBlockNum() rpc.BlockNumber {
	currentBlockNum := s.blockchain.CurrentBlock().NumberU64()
	currEleSealersBlockNum := currentBlockNum - currentBlockNum%baseparam.ElectionBlockCount
	if currEleSealersBlockNum > 0 {
		return rpc.BlockNumber(currEleSealersBlockNum)
	} else {
		return rpc.BlockNumber(0)
	}
}

func (s *Ethereum) CurrEleSealersBlockHash() common.Hash {
	return s.blockchain.GetBlockByNumber(uint64(s.CurrEleSealersBlockNum())).Hash()
}

func (s *Ethereum) LastEleSealersBlockNum() rpc.BlockNumber {
	currentBlockNum := s.blockchain.CurrentBlock().NumberU64()
	lastEleSealersBlockNum := currentBlockNum - currentBlockNum%baseparam.ElectionBlockCount -
		baseparam.ElectionBlockCount
	if lastEleSealersBlockNum > 0 {
		return rpc.BlockNumber(lastEleSealersBlockNum)
	} else {
		return rpc.BlockNumber(0)
	}
}

func (s *Ethereum) LastEleSealersBlockHash() common.Hash {
	if lastEleSealersBlockNum := s.LastEleSealersBlockNum(); lastEleSealersBlockNum > 0 {
		block := s.blockchain.GetBlockByNumber(uint64(lastEleSealersBlockNum))
		return block.Hash()
	} else {
		return common.Hash{}
	}
}

// GetNewestEleSealers returns the newest elephant sealers in the election block of the lamp chain
func (s *Ethereum) GetNewestEleSealers() []common.Address {
	block := s.blockchain.GetBlockByNumber(uint64(s.CurrEleSealersBlockNum()))
	if block == nil {
		return []common.Address{}
	}

	sealers := block.NextSealers()
	addrs := make([]common.Address, 0, len(sealers))
	for _, sealer := range sealers {
		addrs = append(addrs, sealer.Address)
	}
	return addrs
}

// GetNewestEleSealersNode returns the node info string of newest elephant sealers in the election block of the lamp chain
func (s *Ethereum) GetNewestEleNodeSealers() []string {
	block := s.blockchain.GetBlockByNumber(uint64(s.CurrEleSealersBlockNum()))
	if block == nil {
		return []string{}
	}

	sealers := block.NextSealers()
	nodes := make([]string, 0, len(sealers))
	for _, sealer := range sealers {
		nodes = append(nodes, sealer.NodeStr)
	}
	return nodes
}

// func (s *Ethereum) GetCurShardNextSealers() []common.Address {
// 	ethHeight := s.blockchain.CurrentBlock().NumberU64()
// 	curShard := common.GetSharding(common.GetElephantAddr(), ethHeight)

// 	block, _ := s.ApiBackend.BlockByNumber(context.Background(), s.CurrEleSealersBlockNum())
// 	sealers := block.NextSealers()
// 	addrs := make([]common.Address, 0, len(sealers))
// 	for _, sealer := range sealers {
// 		shard := common.GetSharding(sealer.Address, ethHeight)
// 		if shard != curShard {
// 			continue
// 		}
// 		addrs = append(addrs, sealer.Address)
// 	}
// 	return addrs
// }

func (s *Ethereum) GetLastEleSealers() []common.Address {
	if lastEleSealersBlockNum := s.LastEleSealersBlockNum(); lastEleSealersBlockNum > 0 {
		block := s.blockchain.GetBlockByNumber(uint64(lastEleSealersBlockNum))
		if block == nil {
			return []common.Address{}
		}

		sealers := block.NextSealers()
		addrs := make([]common.Address, 0, len(sealers))
		for _, sealer := range sealers {
			addrs = append(addrs, sealer.Address)
		}
		return addrs
	} else {
		return nil
	}
}

func (s *Ethereum) GetLastEleNodeSealers() []string {
	if lastEleSealersBlockNum := s.LastEleSealersBlockNum(); lastEleSealersBlockNum > 0 {
		block := s.blockchain.GetBlockByNumber(uint64(lastEleSealersBlockNum))
		if block == nil {
			return []string{}
		}

		sealers := block.NextSealers()
		nodes := make([]string, 0, len(sealers))
		for _, sealer := range sealers {
			nodes = append(nodes, sealer.NodeStr)
		}
		return nodes
	} else {
		return []string{}
	}
}

func (s *Ethereum) GetPoliceByHash(lampBlockHash *common.Hash) []common.AddressNode {
	block := s.blockchain.GetBlockByHash(*lampBlockHash)
	if block.IsElectionBlock() {
		return block.NextPolice()
	}

	return []common.AddressNode{}
}

func (s *Ethereum) GetNextPolice() []common.AddressNode {
	block := s.blockchain.GetBlockByNumber(uint64(s.CurrEleSealersBlockNum()))
	return block.NextPolice()
}

func (s *Ethereum) GetLastPolice() []common.AddressNode {
	if lastEleSealersBlockNum := s.LastEleSealersBlockNum(); lastEleSealersBlockNum > 0 {
		block := s.blockchain.GetBlockByNumber(uint64(lastEleSealersBlockNum))
		return block.NextPolice()
	} else {
		return nil
	}
}

func (s *Ethereum) GetValidHeadersByBlockNum(blockNum uint64) map[uint16]*types.VerifiedValid {
	block := s.blockchain.GetBlockByNumber(blockNum)
	if block == nil {
		return nil
	}
	return block.GetValidHeaders()
}

func (s *Ethereum) GetNewestValidHeaders() map[uint16]*types.VerifiedValid {
	block := s.blockchain.GetBlockByNumber(uint64(s.CurrEleSealersBlockNum()))
	if block == nil {
		return nil
	}
	return block.GetValidHeaders()
}

func (s *Ethereum) GetValidPrevious(shardingID uint16, lampHeight, blockHeight uint64) common.Hash {
	if lampHeight == 0 {
		return common.Hash{}
	}

	validHeaders := s.GetValidHeadersByBlockNum(lampHeight)
	for id, valid := range validHeaders {
		if id == shardingID {
			if len(valid.PreviousHeaders) == 0 {
				return s.GetValidPrevious(shardingID, lampHeight-baseparam.ElectionBlockCount, blockHeight)
			}
			if blockHeight >= valid.BlockHeight {
				return common.Hash{}
			} else {
				heightDiff := valid.BlockHeight - blockHeight
				previousLen := uint64(len(valid.PreviousHeaders))
				if heightDiff > previousLen {
					return s.GetValidPrevious(shardingID, lampHeight-baseparam.ElectionBlockCount, blockHeight)
				} else {
					return *valid.PreviousHeaders[heightDiff-1]
				}
			}
		}
	}

	return common.Hash{}
}

// Protocols implements node.Service, returning all the currently configured
// network protocols to start.
func (s *Ethereum) Protocols() []p2p.Protocol {
	if s.lesServer == nil {
		return s.protocolManager.SubProtocols
	}
	return append(s.protocolManager.SubProtocols, s.lesServer.Protocols()...)
}

// Start implements node.Service, starting all internal goroutines needed by the
// Ethereum protocol implementation.
func (s *Ethereum) Start(srvr *p2p.Server) error {
	s.node = srvr.Self()

	// Start the bloom bits servicing goroutines
	s.startBloomHandlers()

	// Start the RPC service
	s.netRPCService = ethapi.NewPublicNetAPI(srvr, s.NetVersion())

	// Figure out a max peers count based on the server limits
	maxPeers := srvr.MaxPeers
	if s.config.LightServ > 0 {
		if s.config.LightPeers >= srvr.MaxPeers {
			return fmt.Errorf("invalid peer config: light peer count (%d) >= total peer count (%d)", s.config.LightPeers, srvr.MaxPeers)
		}
		maxPeers -= s.config.LightPeers
	}
	// Start the networking layer and the light server if requested
	s.protocolManager.Start(maxPeers)
	if s.lesServer != nil {
		s.lesServer.Start(srvr)
	}

	s.blockchain.SetClearNetPoliceOnlyListFunc(srvr.ClearNetPoliceOnlyList)

	s.blockchain.SetInterconnectFunc(srvr.Interconnect)

	return nil
}

// Stop implements node.Service, terminating all internal goroutines used by the
// Ethereum protocol.
func (s *Ethereum) Stop() error {
	// if s.stopDbUpgrade != nil {
	// 	s.stopDbUpgrade()
	// }
	s.bloomIndexer.Close()
	s.blockchain.Stop()
	s.protocolManager.Stop()
	if s.lesServer != nil {
		s.lesServer.Stop()
	}
	s.txPool.Stop()
	s.miner.Stop()
	s.eventMux.Stop()

	s.chainDb.Close()
	close(s.shutdownChan)

	return nil
}

func (s *Ethereum) RunForSealerElection(addr common.Address, passwd string) {
	for _, api := range s.APIs() {
		if api.Namespace == "personal" {
			_, err := api.Service.(*ethapi.PrivateAccountAPI).RunForSealerElection(nil, addr, passwd)
			if err != nil {
				log.Error("auto RunForSealerElection failed", "err", err)
			}
			break
		}
	}
}

func (s *Ethereum) SetMainNodeForBFTFlag(flag bool) {
	s.mainNodeForBFTFlag = flag
}

func (s *Ethereum) GetMainNodeForBFTFlag() bool {
	return s.mainNodeForBFTFlag
}

func (s *Ethereum) startReceiveVerifiedHeader() {
	if s.stopRcvHeader == nil {
		s.stopRcvHeader = make(chan bool)
		go s.verifiedHeaderPoolUpdate()
	}
}

func (s *Ethereum) stopReceiveVerifiedHeader() {
	if s.stopRcvHeader != nil {
		s.stopRcvHeader <- true

		close(s.stopRcvHeader)
		s.stopRcvHeader = nil
	}
}

func (s *Ethereum) verifiedHeaderPoolUpdate() {
out:
	for {
		select {
		case header := <-s.verifiedHeader:
			go func(h *consensus.PoliceVerifiedHeader) {
				if err := s.verifHeaderPool.AddVerified(h); err != nil {
					log.Info(fmt.Sprintf("Adding verified header pool failed, err = %s", err.Error()))
				}
			}(header)

		case <-s.stopRcvHeader:
			break out
		}
	}
}
