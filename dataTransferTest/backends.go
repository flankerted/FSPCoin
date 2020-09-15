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

package dataTransferTest

import (
	"github.com/contatract/go-contatract/accounts"
	"github.com/contatract/go-contatract/accounts/keystore"
	"github.com/contatract/go-contatract/blizcs"
	"github.com/contatract/go-contatract/blizzard"
	"github.com/contatract/go-contatract/blizzard/storage"
	"github.com/contatract/go-contatract/blizzard/storagecore"
	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/common/hexutil"
	types "github.com/contatract/go-contatract/core/types_elephant"
	core "github.com/contatract/go-contatract/core_elephant"
	"github.com/contatract/go-contatract/core_elephant/state"
	"github.com/contatract/go-contatract/elephant/exporter"
	"github.com/contatract/go-contatract/eth"
	"github.com/contatract/go-contatract/ethdb"
	"github.com/contatract/go-contatract/event"
	"github.com/contatract/go-contatract/ftransfer"
	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/p2p"
	"github.com/contatract/go-contatract/p2p/discover"
	"github.com/contatract/go-contatract/params"
	"github.com/contatract/go-contatract/rlp"
	"math/big"
	"math/rand"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

const (
	rinkebyAllocData           = "\xf9\x01\x54\xe1\x94\x20\x5a\xe0\x14\x85\x59\xa8\x2f\xef\x17\xf9\xdd\x81\xe3\xc2\xe6\xf9\xb7\x58\x5b\x8b\x52\xb7\xd2\xdc\xc8\x0c\xd2\xe4\x00\x00\x00\xe1\x94\xa5\x57\x35\x21\x78\xf3\x16\x3e\x6a\x28\xd0\xad\x07\xee\xfc\xc3\x9d\xcc\xf0\x94\x8b\x29\x5b\xe9\x6e\x64\x06\x69\x72\x00\x00\x00\xe1\x94\xc1\x17\x37\x1d\x0b\xc9\xf0\xa3\x5f\x5a\xa0\xb7\x8d\x03\x53\xb7\xd4\x0b\x68\x5d\x8b\x18\xd0\xbf\x42\x3c\x03\xd8\xde\x00\x00\x00\xe1\x94\x97\x64\xb9\x47\x60\x00\xeb\xef\x9e\xf5\x13\xd3\xd4\x88\x71\x6f\xb4\xbd\x94\x67\x8b\x10\x8b\x2a\x2c\x28\x02\x90\x94\x00\x00\x00\xe1\x94\x97\x83\x09\x24\xf9\x83\x4c\xee\xa9\x6a\x42\xc3\x0c\xbd\xbb\x72\xd8\xc4\x39\x87\x8b\xa5\x6f\xa5\xb9\x90\x19\xa5\xc8\x00\x00\x00\xe1\x94\x90\x53\x37\xbf\x24\xaa\xa5\xfc\xc5\x18\x19\xe6\x24\xa6\x32\x30\xa8\xd0\x74\xf8\x8b\xa5\x6f\xa5\xb9\x90\x19\xa5\xc8\x00\x00\x00\xe1\x94\x82\x7e\x76\xe4\x92\x49\xdc\x25\xdd\xed\x79\x38\x30\xbf\x01\x81\x2e\x2e\xb3\xa9\x8b\x52\xb7\xd2\xdc\xc8\x0c\xd2\xe4\x00\x00\x00\xe1\x94\x6d\x40\x9f\xf2\xbf\x0f\x70\xc5\x86\xea\x87\x9e\xc2\xca\x4f\xe0\x0d\xf5\x9f\x5e\x8b\x52\xb7\xd2\xdc\xc8\x0c\xd2\xe4\x00\x00\x00\xe1\x94\x0f\x60\x50\x58\x5a\x44\x4e\x09\xca\xca\x37\x87\xec\x50\x7f\x88\xd9\x05\xbd\x42\x8b\x52\xb7\xd2\xdc\xc8\x0c\xd2\xe4\x00\x00\x00\xe1\x94\x2b\x3e\x0e\x8b\xb2\x69\x35\x38\x83\x46\x20\x95\xba\x90\x31\x99\x61\x8a\x0e\x5a\x8b\x52\xb7\xd2\xdc\xc8\x0c\xd2\xe4\x00\x00\x00"
	datadirDefaultKeyStoreServ = "keystoreServ" // Path within the datadir to the keystore
	datadirDefaultKeyStoreCli  = "keystoreCli"  // Path within the datadir to the keystore
)

func newAccount(am *accounts.Manager, pwd string) (string, error) {
	acc, err := fetchKeystore(am).NewAccount(pwd)
	if err == nil {
		return acc.Address.String(), nil
	} else {
		return "", err
	}
}

// fetchKeystore retrives the encrypted keystore from the account manager.
func fetchKeystore(am *accounts.Manager) *keystore.KeyStore {
	return am.Backends(keystore.KeyStoreType)[0].(*keystore.KeyStore)
}

func defaultDataTestDir(nodeName, testType string) string {
	// Try to place the data folder in the user's home dir
	home := homeDir()
	if nodeName != "" {
		if home != "" {
			if runtime.GOOS == "darwin" {
				return filepath.Join(filepath.Join(filepath.Join(home, "Library", "ContatractTest"), testType), nodeName)
			} else if runtime.GOOS == "windows" {
				return filepath.Join(filepath.Join(filepath.Join(home, "AppData", "Roaming", "ContatractTest"), testType), nodeName)
			} else {
				return filepath.Join(filepath.Join(filepath.Join(home, ".contatractTest"), testType), nodeName)
			}
		}
	} else {
		if home != "" {
			if runtime.GOOS == "darwin" {
				return filepath.Join(filepath.Join(home, "Library", "ContatractTest"), testType)
			} else if runtime.GOOS == "windows" {
				return filepath.Join(filepath.Join(home, "AppData", "Roaming", "ContatractTest"), testType)
			} else {
				return filepath.Join(filepath.Join(home, ".contatractTest"), testType)
			}
		}
	}
	// As we cannot guess a stable location, return empty and handle later
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

// defaultGenesisBlock returns the Elephant main net genesis block.
func defaultGenesisBlock() *types.Block {
	g := &core.Genesis{
		Config:     params.ElephantDefChainConfig,
		ExtraData:  hexutil.MustDecode("0x11bbe8db4e347b4e8c937c1c8370e4b5ed33adb3db69cbdb7a38e1e50b1b82fa"),
		GasLimit:   params.GenesisGasLimit, // 5000,
		Difficulty: big.NewInt(400000),
		Alloc:      decodePrealloc(rinkebyAllocData),
		//Alloc:      decodePrealloc(mainnetAllocData),
	}

	head := &types.Header{
		Number:         new(big.Int).SetUint64(g.Number),
		Time:           new(big.Int).SetUint64(g.Timestamp),
		ParentHash:     g.ParentHash,
		Extra:          g.ExtraData,
		GasLimit:       g.GasLimit,
		GasUsed:        g.GasUsed,
		Difficulty:     g.Difficulty,
		MixDigest:      g.Mixhash,
		Coinbase:       g.Coinbase,
		Root:           common.Hash{},
		LampBaseHash:   types.EmptyLampBaseHash,
		LampBaseNumber: new(big.Int).SetUint64(0),
	}

	if g.GasLimit == 0 {
		head.GasLimit = params.GenesisGasLimit
	}
	if g.Difficulty == nil {
		head.Difficulty = params.GenesisDifficulty
	}

	return types.NewBlock(head, nil, nil, nil, nil, nil, nil)
}

func decodePrealloc(data string) core.GenesisAlloc {
	var p []struct{ Addr, Balance *big.Int }
	if err := rlp.NewStream(strings.NewReader(data), 0).Decode(&p); err != nil {
		panic(err)
	}
	ga := make(core.GenesisAlloc, len(p))
	for _, account := range p {
		ga[common.BigToAddress(account.Addr)] = core.GenesisAccount{Balance: account.Balance}
	}
	return ga
}

func defaultChunkDataTestDir(nodeName, testType string) string {
	// Try to place the data folder in the user's home dir
	home := homeDir()
	if home != "" {
		if runtime.GOOS == "darwin" {
			return filepath.Join(home, "Library", "ContatractTest", testType, nodeName, "Chunkdata")
		} else if runtime.GOOS == "windows" {
			return filepath.Join(home, "AppData", "Roaming", "ContatractTest", testType, nodeName, "Chunkdata")
		} else {
			return filepath.Join(home, ".contatractTest", testType, nodeName, "chunkdata")
		}
	}
	// As we cannot guess a stable location, return empty and handle later
	return ""
}

func defaultBlizzardTestConfig(nodeName, testType string) blizzard.Config {
	cfg := blizzard.DefaultConfig
	cfg.Name = "gcttTest"
	cfg.Version = "1.0"
	cfg.WSModules = append(cfg.WSModules, "blizzard", "shh")
	cfg.NetworkId = 627
	cfg.ChunkFileDir = defaultChunkDataTestDir(nodeName, testType)

	return cfg
}

func defaultBlizCsTestConfig() blizcs.Config {
	cfg := blizcs.Config{}
	cfg.Name = "gcttTest"
	cfg.Version = "1.0"
	cfg.NetworkId = 627

	//if ctx.GlobalIsSet(MinCopyCntFlag.Name) {
	//	num := ctx.GlobalInt(MinCopyCntFlag.Name)
	//	blizparam.SetObjMetaQueryCount(num)
	//	blizparam.SetMinCopyCount(num)
	//}
	//if ctx.GlobalIsSet(MaxCopyCntFlag.Name) {
	//	num := ctx.GlobalInt(MaxCopyCntFlag.Name)
	//	blizparam.SetMaxCopyCount(num)
	//}
	//if ctx.GlobalIsSet(MinRWFarmerCntFlag.Name) {
	//	num := ctx.GlobalInt(MinRWFarmerCntFlag.Name)
	//	blizcs.SetRWCheckCount(num)
	//}

	return cfg
}

func defaultFtransTestConfig() ftransfer.Config {
	cfg := ftransfer.DefaultConfig
	cfg.Name = "gcttTest"
	cfg.Version = "1.0"
	cfg.NetworkId = 627
	//if ctx.GlobalIsSet(DataDirFlag.Name) {
	//	dir := ctx.GlobalString(DataDirFlag.Name)
	//	cfg.FilesDir = filepath.Join(dir, "files")
	//}

	return cfg
}

func CreateLogger(nodeName, testType, fileName string) (logger log.Logger) {
	logger = log.New()
	path := filepath.Join(defaultDataTestDir(nodeName, testType), fileName)
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

type BackendNoDiskDBOrBlock struct {
	// Node information
	NodeName  string
	NodeIndex int
	IsServ    bool

	// Events subscription scope
	scope event.SubscriptionScope

	// Simulated elephant
	DataDir   string
	Accman    *accounts.Manager
	EtherBase common.Address

	memDatabase ethdb.Database // DB interfaces
	StateMemDB  *state.StateDB

	//Blocks       []*types.Block
	//BlockHashMap map[common.Hash]*types.Block
	//BlockNumMap  map[uint64]*types.Block
	//engine       consensus.EngineCtt
	//blockChain   *core.BlockChain
	Bliz      *blizzard.Bliz
	BlizCs    *blizcs.BlizCS
	Ftransfer *ftransfer.FileTransfer
	NodeId    discover.NodeID
	Peers     map[discover.NodeID]*ftransfer.Peer

	BlizCsApi *blizcs.PublicBlizCSAPI
	FTransAPI *ftransfer.PrivateFTransAPI

	chainCh   chan core.ChainEvent // New block connected event in elephant
	chainSub  event.Subscription   // New block connected event in elephant
	chainFeed event.Feed           // New block connected event in elephant

	eventMux *event.TypeMux

	// Simulated sealer
	mu            sync.Mutex
	newBlockCh    chan *types.Block
	stop          chan struct{}
	quitCurrentOp chan struct{}
	stopTest      chan struct{}

	futureParentBlock *types.Block // In sealer

	chainHeadCh   chan core.ChainHeadEvent // New block connected event in sealer
	chainHeadSub  event.Subscription       // New block connected event in sealer
	chainHeadFeed event.Feed               // New block connected event in sealer

	// New block net transmitting event for test
	chainNetSub event.Subscription // New block net transmitting event in elephant

	// Test controller parameter
	//TestTimes      uint64
	TestTimesTotal uint64

	Lamp *eth.Ethereum

	Log log.Logger
}

func NewSimBackendForIntTestWithoutDiskDBOrBlock(isServ bool, nodeName, testType string, index int, logger log.Logger) (*BackendNoDiskDBOrBlock, error) {
	backend := &BackendNoDiskDBOrBlock{
		NodeName:  nodeName,
		NodeIndex: index,
		IsServ:    isServ,
		DataDir:   defaultDataTestDir(nodeName, testType),
		Peers:     make(map[discover.NodeID]*ftransfer.Peer),
		Log:       logger,
	}

	if backend.DataDir != "" {
		absdatadir, err := filepath.Abs(backend.DataDir)
		if err != nil {
			return nil, err
		}
		backend.DataDir = absdatadir
	}

	// Crate account management of server
	if isServ {
		amServ, err := makeAccountManager(backend, true)
		if err != nil {
			return nil, err
		}
		backend.Accman = amServ
		accountServ, errAcc := backend.GetAccount(nodeName, amServ)
		if errAcc != nil {
			return nil, errAcc
		}
		backend.EtherBase = common.HexToAddress(accountServ)

		// Create blizzard
		cfgBliz := defaultBlizzardTestConfig(nodeName, testType)
		backend.Bliz, err = blizzard.NewForTest(backend, &cfgBliz)
		if err != nil {
			return nil, err
		}

		// Create blizCs
		cfgBlizCs := defaultBlizCsTestConfig()
		backend.BlizCs, err = blizcs.NewForTest(backend.Bliz, backend.Accman, backend.DataDir, nodeName, &cfgBlizCs)
		if err != nil {
			return nil, err
		}
		backend.BlizCsApi = blizcs.NewPublicBlizCSAPI(backend.BlizCs)

		// Create ftransfer server
		cfgFtans := defaultFtransTestConfig()
		backend.Ftransfer, err = ftransfer.New(backend.BlizCs, &cfgFtans, backend.Accman)
		if err != nil {
			return nil, err
		}
	} else {
		// Crate account management of client
		amCli, err := makeAccountManager(backend, false)
		if err != nil {
			return nil, err
		}
		backend.Accman = amCli
		accountCli, errAcc := backend.GetAccount(nodeName, amCli)
		if errAcc != nil {
			return nil, errAcc
		}
		backend.EtherBase = common.HexToAddress(accountCli)

		// Create ftransfer of client
		cfgFtans := defaultFtransTestConfig()
		backend.Ftransfer, err = ftransfer.New(nil, &cfgFtans, backend.Accman)
		if err != nil {
			return nil, err
		}
		backend.FTransAPI = ftransfer.NewPrivateFTransAPI(backend.Ftransfer)
	}

	rand.Read(backend.NodeId[:])

	memDatabase, err := ethdb.NewMemDatabase()
	if err != nil {
		return nil, err
	}
	backend.memDatabase = memDatabase
	backend.StateMemDB, err = state.New(common.Hash{}, state.NewDatabase(memDatabase), nil, &backend.EtherBase)
	if err != nil {
		return nil, err
	}

	return backend, nil
}

func makeAccountManager(backend *BackendNoDiskDBOrBlock, serv bool) (*accounts.Manager, error) {
	var err error
	scryptN, scryptP, keydir := backend.AccountConfig(serv)

	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(keydir, 0700); err != nil {
		return nil, err
	}
	// Assemble the account manager and supported backends
	backends := []accounts.Backend{
		keystore.NewKeyStore(keydir, scryptN, scryptP),
	}

	return accounts.NewManager(backends...), nil
}

// AccountConfig determines the settings for scrypt and keydirectory
func (b *BackendNoDiskDBOrBlock) AccountConfig(serv bool) (int, int, string) {
	scryptN := keystore.StandardScryptN
	scryptP := keystore.StandardScryptP
	if serv {
		return scryptN, scryptP, filepath.Join(b.DataDir, datadirDefaultKeyStoreServ)
	} else {
		return scryptN, scryptP, filepath.Join(b.DataDir, datadirDefaultKeyStoreCli)
	}
}

// GetAccAddresses returns the collection of accounts this node manages
func (b *BackendNoDiskDBOrBlock) GetAccAddresses(accMan *accounts.Manager) []string {
	addresses := make([]string, 0) // return [] instead of nil if empty
	for _, wallet := range accMan.Wallets() {
		for _, account := range wallet.Accounts() {
			addresses = append(addresses, account.Address.String())
		}
	}
	return addresses
}

func (b *BackendNoDiskDBOrBlock) GetAccount(nodeName string, accMan *accounts.Manager) (string, error) {
	accs := b.GetAccAddresses(accMan)
	if len(accs) != 0 {
		strAccounts := ""
		for _, acc := range accs {
			strAccounts += acc
			break
		}
		return strAccounts, nil
	} else {
		newAcc, err := newAccount(accMan, nodeName)
		if err != nil {
			return "", err
		} else {
			return newAcc, nil
		}
	}
}

func (b *BackendNoDiskDBOrBlock) CreateMsgPipeWithTheServer(remoteId discover.NodeID) (discover.NodeID, p2p.MsgReadWriter, error) {
	rwOther, rw := p2p.MsgPipe()
	b.Peers[remoteId] = ftransfer.NewPeer(0, p2p.NewPeer(remoteId, b.NodeName, nil), rw)
	err := b.Ftransfer.GetProtocolMan().RegisterPeer(b.Peers[remoteId])
	if err != nil {
		return discover.NodeID{}, nil, err
	}
	return b.NodeId, rwOther, nil
}

func (b *BackendNoDiskDBOrBlock) ConnectToServer(serv *BackendNoDiskDBOrBlock) error {
	remoteId, rw, err := serv.CreateMsgPipeWithTheServer(b.NodeId)
	if err != nil {
		return err
	}
	b.Peers[remoteId] = ftransfer.NewPeer(0, p2p.NewPeer(remoteId, b.NodeName, nil), rw)
	err = b.Ftransfer.GetProtocolMan().RegisterPeer(b.Peers[remoteId])
	if err != nil {
		return err
	}
	return nil
}

func (b *BackendNoDiskDBOrBlock) StartFtansferHandler() {
	for _, p := range b.Peers {
		go b.FTransAPI.GetProtocolMan().HandleMsgForTest(p)
	}
}

func (b *BackendNoDiskDBOrBlock) GetStorageManager() *storage.Manager {
	return nil
}

func (b *BackendNoDiskDBOrBlock) CheckFarmerAward(eleHeight uint64, blockHash common.Hash, chunkID uint64, h uint64) {

}

func (b *BackendNoDiskDBOrBlock) GetLeftRentSize() uint64 {
	return 0
}

func (b *BackendNoDiskDBOrBlock) GetUnusedChunkCnt(address common.Address, cIds []uint64) uint32 {
	return 0
}

func (b *BackendNoDiskDBOrBlock) GetBalance(addr common.Address) *big.Int {
	return b.StateMemDB.GetBalance(addr)
}

func (b *BackendNoDiskDBOrBlock) ObjMetaQueryFunc(query *exporter.ObjMetaQuery) {

}

func (b *BackendNoDiskDBOrBlock) ObjFarmerQueryFunc(query *exporter.ObjFarmerQuery) {

}

func (b *BackendNoDiskDBOrBlock) ObjClaimChunkQueryFunc(query *exporter.ObjClaimChunkQuery) (uint64, error) {
	return 0, nil
}

func (b *BackendNoDiskDBOrBlock) ObjGetObjectFunc(query *exporter.ObjGetObjectQuery) exporter.ObjectsInfo {
	return exporter.ObjectsInfo{}
}

func (b *BackendNoDiskDBOrBlock) ObjGetCliAddrFunc(query *exporter.ObjGetCliAddr) []common.CSAuthData {
	return nil
}

func (b *BackendNoDiskDBOrBlock) ObjGetElephantDataFunc(query *exporter.ObjGetElephantData) {

}

func (b *BackendNoDiskDBOrBlock) ObjGetElephantDataRspFunc(query *exporter.ObjGetElephantData) *exporter.ObjGetElephantDataRsp {
	return nil
}

func (b *BackendNoDiskDBOrBlock) GetSliceFromState(address common.Address, objId uint64) []*storagecore.Slice {
	return nil
}

func (b *BackendNoDiskDBOrBlock) GetClaim(addr common.Address) (int, error) {
	return 0, nil
}

func (b *BackendNoDiskDBOrBlock) BlockChain() *core.BlockChain {
	return nil
}
