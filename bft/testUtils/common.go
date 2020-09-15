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
	"math/big"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/contatract/go-contatract/accounts"
	"github.com/contatract/go-contatract/accounts/keystore"
	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/common/hexutil"
	types "github.com/contatract/go-contatract/core/types_elephant"
	core "github.com/contatract/go-contatract/core_elephant"
	"github.com/contatract/go-contatract/params"
	"github.com/contatract/go-contatract/rlp"
)

const (
	rinkebyAllocData       = "\xf9\x01\x54\xe1\x94\x20\x5a\xe0\x14\x85\x59\xa8\x2f\xef\x17\xf9\xdd\x81\xe3\xc2\xe6\xf9\xb7\x58\x5b\x8b\x52\xb7\xd2\xdc\xc8\x0c\xd2\xe4\x00\x00\x00\xe1\x94\xa5\x57\x35\x21\x78\xf3\x16\x3e\x6a\x28\xd0\xad\x07\xee\xfc\xc3\x9d\xcc\xf0\x94\x8b\x29\x5b\xe9\x6e\x64\x06\x69\x72\x00\x00\x00\xe1\x94\xc1\x17\x37\x1d\x0b\xc9\xf0\xa3\x5f\x5a\xa0\xb7\x8d\x03\x53\xb7\xd4\x0b\x68\x5d\x8b\x18\xd0\xbf\x42\x3c\x03\xd8\xde\x00\x00\x00\xe1\x94\x97\x64\xb9\x47\x60\x00\xeb\xef\x9e\xf5\x13\xd3\xd4\x88\x71\x6f\xb4\xbd\x94\x67\x8b\x10\x8b\x2a\x2c\x28\x02\x90\x94\x00\x00\x00\xe1\x94\x97\x83\x09\x24\xf9\x83\x4c\xee\xa9\x6a\x42\xc3\x0c\xbd\xbb\x72\xd8\xc4\x39\x87\x8b\xa5\x6f\xa5\xb9\x90\x19\xa5\xc8\x00\x00\x00\xe1\x94\x90\x53\x37\xbf\x24\xaa\xa5\xfc\xc5\x18\x19\xe6\x24\xa6\x32\x30\xa8\xd0\x74\xf8\x8b\xa5\x6f\xa5\xb9\x90\x19\xa5\xc8\x00\x00\x00\xe1\x94\x82\x7e\x76\xe4\x92\x49\xdc\x25\xdd\xed\x79\x38\x30\xbf\x01\x81\x2e\x2e\xb3\xa9\x8b\x52\xb7\xd2\xdc\xc8\x0c\xd2\xe4\x00\x00\x00\xe1\x94\x6d\x40\x9f\xf2\xbf\x0f\x70\xc5\x86\xea\x87\x9e\xc2\xca\x4f\xe0\x0d\xf5\x9f\x5e\x8b\x52\xb7\xd2\xdc\xc8\x0c\xd2\xe4\x00\x00\x00\xe1\x94\x0f\x60\x50\x58\x5a\x44\x4e\x09\xca\xca\x37\x87\xec\x50\x7f\x88\xd9\x05\xbd\x42\x8b\x52\xb7\xd2\xdc\xc8\x0c\xd2\xe4\x00\x00\x00\xe1\x94\x2b\x3e\x0e\x8b\xb2\x69\x35\x38\x83\x46\x20\x95\xba\x90\x31\x99\x61\x8a\x0e\x5a\x8b\x52\xb7\xd2\xdc\xc8\x0c\xd2\xe4\x00\x00\x00"
	datadirDefaultKeyStore = "keystore" // Path within the datadir to the keystore
)

var (
	BftTestCaseStopped bool
	useRandomDelay     bool
	testTimes          int
	recoverDalayTimes  int
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

func defaultDataDir(nodeName string) string {
	// Try to place the data folder in the user's home dir
	home := homeDir()
	if nodeName != "" {
		if home != "" {
			if runtime.GOOS == "darwin" {
				return filepath.Join(filepath.Join(filepath.Join(home, "Library", "ContatractTest"), "HBFTTest"), nodeName)
			} else if runtime.GOOS == "windows" {
				return filepath.Join(filepath.Join(filepath.Join(home, "AppData", "Roaming", "ContatractTest"), "HBFTTest"), nodeName)
			} else {
				return filepath.Join(filepath.Join(filepath.Join(home, ".contatractTest"), "HBFTTest"), nodeName)
			}
		}
	} else {
		if home != "" {
			if runtime.GOOS == "darwin" {
				return filepath.Join(filepath.Join(home, "Library", "ContatractTest"), "HBFTTest")
			} else if runtime.GOOS == "windows" {
				return filepath.Join(filepath.Join(home, "AppData", "Roaming", "ContatractTest"), "HBFTTest")
			} else {
				return filepath.Join(filepath.Join(home, ".contatractTest"), "HBFTTest")
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

func makeAccountManager(accCfg func() (int, int, string)) (*accounts.Manager, error) {
	var err error
	scryptN, scryptP, keydir := accCfg()

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

func defaultGenesis(initBalAddrs []struct{ Addr, Balance *big.Int }) *core.Genesis {
	ga := make(core.GenesisAlloc, len(initBalAddrs))
	for _, account := range initBalAddrs {
		ga[common.BigToAddress(account.Addr)] = core.GenesisAccount{Balance: account.Balance}
	}

	g := &core.Genesis{
		Config:     params.ElephantDefChainConfig,
		ExtraData:  hexutil.MustDecode("0x11bbe8db4e347b4e8c937c1c8370e4b5ed33adb3db69cbdb7a38e1e50b1b82fa"),
		GasLimit:   params.GenesisGasLimit, // 5000,
		Difficulty: big.NewInt(400000),
		Alloc:      ga, //decodePrealloc(rinkebyAllocData),
		//Alloc:      decodePrealloc(mainnetAllocData),
	}
	return g
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
