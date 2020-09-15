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
	core "github.com/contatract/go-contatract/core_eth"
	"github.com/contatract/go-contatract/params"
	"github.com/contatract/go-contatract/rlp"
)

const (
	testnetAllocData       = "\xf9\x03\xa4\u0080\x01\xc2\x01\x01\xc2\x02\x01\xc2\x03\x01\xc2\x04\x01\xc2\x05\x01\xc2\x06\x01\xc2\a\x01\xc2\b\x01\xc2\t\x01\xc2\n\x80\xc2\v\x80\xc2\f\x80\xc2\r\x80\xc2\x0e\x80\xc2\x0f\x80\xc2\x10\x80\xc2\x11\x80\xc2\x12\x80\xc2\x13\x80\xc2\x14\x80\xc2\x15\x80\xc2\x16\x80\xc2\x17\x80\xc2\x18\x80\xc2\x19\x80\xc2\x1a\x80\xc2\x1b\x80\xc2\x1c\x80\xc2\x1d\x80\xc2\x1e\x80\xc2\x1f\x80\xc2 \x80\xc2!\x80\xc2\"\x80\xc2#\x80\xc2$\x80\xc2%\x80\xc2&\x80\xc2'\x80\xc2(\x80\xc2)\x80\xc2*\x80\xc2+\x80\xc2,\x80\xc2-\x80\xc2.\x80\xc2/\x80\xc20\x80\xc21\x80\xc22\x80\xc23\x80\xc24\x80\xc25\x80\xc26\x80\xc27\x80\xc28\x80\xc29\x80\xc2:\x80\xc2;\x80\xc2<\x80\xc2=\x80\xc2>\x80\xc2?\x80\xc2@\x80\xc2A\x80\xc2B\x80\xc2C\x80\xc2D\x80\xc2E\x80\xc2F\x80\xc2G\x80\xc2H\x80\xc2I\x80\xc2J\x80\xc2K\x80\xc2L\x80\xc2M\x80\xc2N\x80\xc2O\x80\xc2P\x80\xc2Q\x80\xc2R\x80\xc2S\x80\xc2T\x80\xc2U\x80\xc2V\x80\xc2W\x80\xc2X\x80\xc2Y\x80\xc2Z\x80\xc2[\x80\xc2\\\x80\xc2]\x80\xc2^\x80\xc2_\x80\xc2`\x80\xc2a\x80\xc2b\x80\xc2c\x80\xc2d\x80\xc2e\x80\xc2f\x80\xc2g\x80\xc2h\x80\xc2i\x80\xc2j\x80\xc2k\x80\xc2l\x80\xc2m\x80\xc2n\x80\xc2o\x80\xc2p\x80\xc2q\x80\xc2r\x80\xc2s\x80\xc2t\x80\xc2u\x80\xc2v\x80\xc2w\x80\xc2x\x80\xc2y\x80\xc2z\x80\xc2{\x80\xc2|\x80\xc2}\x80\xc2~\x80\xc2\u007f\x80\u00c1\x80\x80\u00c1\x81\x80\u00c1\x82\x80\u00c1\x83\x80\u00c1\x84\x80\u00c1\x85\x80\u00c1\x86\x80\u00c1\x87\x80\u00c1\x88\x80\u00c1\x89\x80\u00c1\x8a\x80\u00c1\x8b\x80\u00c1\x8c\x80\u00c1\x8d\x80\u00c1\x8e\x80\u00c1\x8f\x80\u00c1\x90\x80\u00c1\x91\x80\u00c1\x92\x80\u00c1\x93\x80\u00c1\x94\x80\u00c1\x95\x80\u00c1\x96\x80\u00c1\x97\x80\u00c1\x98\x80\u00c1\x99\x80\u00c1\x9a\x80\u00c1\x9b\x80\u00c1\x9c\x80\u00c1\x9d\x80\u00c1\x9e\x80\u00c1\x9f\x80\u00c1\xa0\x80\u00c1\xa1\x80\u00c1\xa2\x80\u00c1\xa3\x80\u00c1\xa4\x80\u00c1\xa5\x80\u00c1\xa6\x80\u00c1\xa7\x80\u00c1\xa8\x80\u00c1\xa9\x80\u00c1\xaa\x80\u00c1\xab\x80\u00c1\xac\x80\u00c1\xad\x80\u00c1\xae\x80\u00c1\xaf\x80\u00c1\xb0\x80\u00c1\xb1\x80\u00c1\xb2\x80\u00c1\xb3\x80\u00c1\xb4\x80\u00c1\xb5\x80\u00c1\xb6\x80\u00c1\xb7\x80\u00c1\xb8\x80\u00c1\xb9\x80\u00c1\xba\x80\u00c1\xbb\x80\u00c1\xbc\x80\u00c1\xbd\x80\u00c1\xbe\x80\u00c1\xbf\x80\u00c1\xc0\x80\u00c1\xc1\x80\u00c1\u0080\u00c1\u00c0\u00c1\u0100\u00c1\u0140\u00c1\u0180\u00c1\u01c0\u00c1\u0200\u00c1\u0240\u00c1\u0280\u00c1\u02c0\u00c1\u0300\u00c1\u0340\u00c1\u0380\u00c1\u03c0\u00c1\u0400\u00c1\u0440\u00c1\u0480\u00c1\u04c0\u00c1\u0500\u00c1\u0540\u00c1\u0580\u00c1\u05c0\u00c1\u0600\u00c1\u0640\u00c1\u0680\u00c1\u06c0\u00c1\u0700\u00c1\u0740\u00c1\u0780\u00c1\u07c0\u00c1\xe0\x80\u00c1\xe1\x80\u00c1\xe2\x80\u00c1\xe3\x80\u00c1\xe4\x80\u00c1\xe5\x80\u00c1\xe6\x80\u00c1\xe7\x80\u00c1\xe8\x80\u00c1\xe9\x80\u00c1\xea\x80\u00c1\xeb\x80\u00c1\xec\x80\u00c1\xed\x80\u00c1\xee\x80\u00c1\xef\x80\u00c1\xf0\x80\u00c1\xf1\x80\u00c1\xf2\x80\u00c1\xf3\x80\u00c1\xf4\x80\u00c1\xf5\x80\u00c1\xf6\x80\u00c1\xf7\x80\u00c1\xf8\x80\u00c1\xf9\x80\u00c1\xfa\x80\u00c1\xfb\x80\u00c1\xfc\x80\u00c1\xfd\x80\u00c1\xfe\x80\u00c1\xff\x80\u3507KT\xa8\xbd\x15)f\xd6?pk\xae\x1f\xfe\xb0A\x19!\xe5\x8d\f\x9f,\x9c\xd0Ft\xed\xea@\x00\x00\x00"
	datadirDefaultKeyStore = "keystore" // Path within the datadir to the keystore
)

var (
	LampMinerTestStopped bool
	useRandomDelay       bool
	testTimes            int
	recoverDalayTimes    int
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
				return filepath.Join(filepath.Join(filepath.Join(home, "Library", "ContatractTest"), "MinerTest"), nodeName)
			} else if runtime.GOOS == "windows" {
				return filepath.Join(filepath.Join(filepath.Join(home, "AppData", "Roaming", "ContatractTest"), "MinerTest"), nodeName)
			} else {
				return filepath.Join(filepath.Join(filepath.Join(home, ".contatractTest"), "MinerTest"), nodeName)
			}
		}
	} else {
		if home != "" {
			if runtime.GOOS == "darwin" {
				return filepath.Join(filepath.Join(home, "Library", "ContatractTest"), "MinerTest")
			} else if runtime.GOOS == "windows" {
				return filepath.Join(filepath.Join(home, "AppData", "Roaming", "ContatractTest"), "MinerTest")
			} else {
				return filepath.Join(filepath.Join(home, ".contatractTest"), "MinerTest")
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

func defaultGenesis() *core.Genesis {
	g := &core.Genesis{
		Config:     params.TestnetChainConfig,
		Nonce:      66,
		ExtraData:  hexutil.MustDecode("0x3535353535353535353535353535353535353535353535353535353535353535"),
		GasLimit:   16777216,
		Difficulty: big.NewInt(1048576),
		Alloc:      decodePrealloc(testnetAllocData),
	}
	return g
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
