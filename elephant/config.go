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

package elephant

import (
	//"math/big"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"time"

	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/elephant/downloader"

	"github.com/contatract/go-contatract/consensus/elephanthash"
	"github.com/contatract/go-contatract/core_elephant"
	//"github.com/contatract/go-contatract/params"
)

// DefaultConfig contains default settings for use on the Elephant main net.
var DefaultConfig = Config{
	SyncMode:      elephant_downloader.FullSync, // qiweiyou: FastSync, to avoid "state node c0bf5c…3e3992 failed with all peers (1 tries, 1 peers)"
	Elephanthash:  elephanthash.Config{},
	NetworkId:     1,
	DatabaseCache: 768,
	TrieCache:     256,
	TrieTimeout:   5 * time.Minute,

	// gas 价格
	//GasPrice:      big.NewInt(18 * params.Shannon),

	TxPool: core_elephant.DefaultTxPoolConfig,
	//GPO: gasprice.Config{
	//	Blocks:     20,
	//	Percentile: 60,
	//},
}

func init() {
	home := os.Getenv("HOME")
	if home == "" {
		if user, err := user.Current(); err == nil {
			home = user.HomeDir
		}
	}
	if runtime.GOOS == "windows" {
		DefaultConfig.Elephanthash.DatasetDir = filepath.Join(home, "AppData", "Elephanthash")
	} else {
		DefaultConfig.Elephanthash.DatasetDir = filepath.Join(home, ".elephanthash")
	}
}

//go:generate gencodec -type Config -field-override configMarshaling -formats toml -out gen_config.go

type Config struct {
	// The genesis block, which is inserted if the database is empty.
	// If nil, the Ethereum main net block is used.
	Genesis *core_elephant.Genesis `toml:",omitempty"`

	SyncMode elephant_downloader.SyncMode

	// Protocol options
	NetworkId uint64 // Network ID to use for selecting peers to connect to
	NoPruning bool

	// Database options
	SkipBcVersionCheck bool `toml:"-"`
	DatabaseHandles    int  `toml:"-"`
	DatabaseCache      int
	TrieCache          int
	TrieTimeout        time.Duration

	// Mining-related options
	Etherbase    common.Address `toml:",omitempty"`
	MinerThreads int            `toml:",omitempty"`
	ExtraData    []byte         `toml:",omitempty"`
	//GasPrice     *big.Int

	// Ethash options
	Elephanthash elephanthash.Config

	// Transaction pool options
	TxPool core_elephant.TxPoolConfig

	// Miscellaneous options
	DocRoot string `toml:"-"`
}
