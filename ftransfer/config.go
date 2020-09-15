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

package ftransfer

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"

	"github.com/contatract/go-contatract/log"
)

// DefaultConfig contains default settings for use on the Ethereum main net.
var DefaultConfig = Config{
	NetworkId: 1,
}

func init() {
	home := os.Getenv("HOME")
	if home == "" {
		if user, err := user.Current(); err == nil {
			home = user.HomeDir
		}
	}
	if runtime.GOOS == "windows" {
		DefaultConfig.FilesDir = filepath.Join(home, "AppData", "EleWallet", "files")
	} else {
		DefaultConfig.FilesDir = filepath.Join(home, ".eleWallet", "files")
	}
}

type Config struct {
	// Name sets the instance name of the node. It must not contain the / character and is
	// used in the devp2p node identifier. The instance name of geth is "geth". If no
	// value is specified, the basename of the current executable is used.
	Name string `toml:"-"`

	// Version should be set to the version number of the program. It is used
	// in the devp2p node identifier.
	Version string `toml:"-"`

	// Protocol options
	NetworkId uint64 // Network ID to use for selecting peers to connect to

	// Files save path
	FilesDir string

	// remoteIP only use for client
	RemoteIP string

	// Logger is a custom logger to use with the blizzard Server.
	Logger log.Logger `toml:",omitempty"`
}
