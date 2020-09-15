// Copyright 2016 The go-contatract Authors
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

package blizzard

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"
)

const (
	DefaultHTTPHost 	= "localhost" 	// Default host interface for the HTTP RPC server
	DefaultHTTPPort 	= 8601        	// Default TCP port for the HTTP RPC server
	DefaultSCHost   	= "localhost" 	// Default host interface for the Server-Client interface
	DefaultSCPort   	= 8610        	// Default TCP port for the Server-Client server
	DefaultPartnerHost  = "localhost"   // Default host interface for the Partners communication
	DefaultPartnerPort  = 9611        	// Default TCP port for the Partners communication
)

// DefaultConfig contains reasonable default settings.
var DefaultConfig = Config{
	ChunkFileDir:     	DefaultChunkDataDir(),
	//ChunkDBPath:		DefaultChunkDBPath(),
	SCHost:				DefaultSCHost,
	SCPort:      		DefaultSCPort,
	PartnerHost:		DefaultPartnerHost,
	PartnerPort:    	DefaultPartnerPort,
	DatabaseCache: 		768,
	NetworkId:			5,
}

// DefaultDataDir is the default data directory to use for the databases and other
// persistence requirements.
func DefaultChunkDataDir() string {
	// Try to place the data folder in the user's home dir
	home := homeDir()
	if home != "" {
		if runtime.GOOS == "darwin" {
			return filepath.Join(home, "Library", "Contatract", "Elephant", "Chunkdata")
		} else if runtime.GOOS == "windows" {
			return filepath.Join(home, "AppData", "Roaming", "Contatract", "Elephant", "Chunkdata")
		} else {
			return filepath.Join(home, ".contatract", "elephant", "chunkdata")
		}
	}
	// As we cannot guess a stable location, return empty and handle later
	return ""
}

// DefaultDataDir is the default data directory to use for the databases and other
// persistence requirements.
func DefaultChunkDBPath() string {
	// Try to place the data folder in the user's home dir
	home := homeDir()
	if home != "" {
		if runtime.GOOS == "darwin" {
			return filepath.Join(home, "Library", "Contatract", "Elephant", "Chunks")
		} else if runtime.GOOS == "windows" {
			return filepath.Join(home, "AppData", "Roaming", "Contatract", "Elephant", "Chunks")
		} else {
			return filepath.Join(home, ".contatract", "elephant", "chunks")
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
