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

package blizcs

import (
	//"path/filepath"
	//"github.com/contatract/go-contatract/node"
	"github.com/contatract/go-contatract/log"
)

// Config represents a small collection of configuration values to fine tune the
// P2P network layer of a protocol stack. These values can be further extended by
// all registered services.
type Config struct {
	// Name sets the instance name of the node. It must not contain the / character and is
	// used in the devp2p node identifier. The instance name of geth is "geth". If no
	// value is specified, the basename of the current executable is used.
	Name string `toml:"-"`

	// UserIdent, if set, is used as an additional component in the devp2p node identifier.
	UserIdent string `toml:",omitempty"`

	// Version should be set to the version number of the program. It is used
	// in the devp2p node identifier.
	Version string `toml:"-"`

	// DataDir is the file system folder the node should use for any data storage
	// requirements. The configured data directory will not be directly shared with
	// registered services, instead those can use utility methods to create/access
	// databases or flat files. This enables ephemeral nodes which can fully reside
	// in memory.
	ChunkFileDir string

	// SCHost is the host interface on which to start the Server-Client data transfer.
	SCHost string 
	// SCPort is the host listen port on which to start the Server-Client data transfer.
	SCPort int

	// PartnerHost is the host interface on which to start the Communication over Partners.
	PartnerHost string 
	// PartnerPort is the host listen port on which to start the Communication over Partners.
	PartnerPort int

	// WSModules is a list of API modules to expose via the websocket RPC interface.
	// If the module list is empty, all RPC API endpoints designated public will be
	// exposed.
	WSModules []string `toml:",omitempty"`

	// Logger is a custom logger to use with the blizzard Server.
	Logger log.Logger `toml:",omitempty"`

	DatabaseHandles    	int  `toml:"-"`
	DatabaseCache      	int

	//ChunkDBPath			string

	NetworkId	uint64
}

