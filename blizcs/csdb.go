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

// Contains the node database, storing previously seen nodes and any collected
// metadata about them for QoS purposes.

package blizcs

import (
	"bytes"
	"encoding/binary"
	"os"

	"github.com/contatract/go-contatract/common"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/storage"
)

// csDB stores all nodes we know about.
type csDB struct {
	lvl *leveldb.DB // Interface to the database itself
}

var (
	csDBVersionKey      = []byte("version") // Version of the database to flush if changes
	usedFlowPrefix      = []byte("usedflow:")
	authAllowFlowPrefix = []byte("authallowflow:")
)

// newNodeDB creates a new node database for storing and retrieving infos about
// known peers in the network. If no path is given, an in-memory, temporary
// database is constructed.
func newCsDB(path string, version int) (*csDB, error) {
	if path == "" {
		return newMemoryCsDB()
	}
	return newPersistentCsDB(path, version)
}

// newMemoryNodeDB creates a new in-memory node database without a persistent
// backend.
func newMemoryCsDB() (*csDB, error) {
	db, err := leveldb.Open(storage.NewMemStorage(), nil)
	if err != nil {
		return nil, err
	}
	return &csDB{
		lvl: db,
	}, nil
}

// newPersistentChunkDB creates/opens a leveldb backed persistent chunk database,
// also flushing its contents in case of a version mismatch.
func newPersistentCsDB(path string, version int) (*csDB, error) {
	opts := &opt.Options{OpenFilesCacheCapacity: 5}
	db, err := leveldb.OpenFile(path, opts)
	if _, iscorrupted := err.(*errors.ErrCorrupted); iscorrupted {
		db, err = leveldb.RecoverFile(path, nil)
	}
	if err != nil {
		return nil, err
	}
	// The nodes contained in the cache correspond to a certain protocol version.
	// Flush all nodes if the version doesn't match.
	currentVer := make([]byte, binary.MaxVarintLen64)
	currentVer = currentVer[:binary.PutVarint(currentVer, int64(version))]

	blob, err := db.Get(csDBVersionKey, nil)
	switch err {
	case leveldb.ErrNotFound:
		// Version not found (i.e. empty cache), insert it
		if err := db.Put(csDBVersionKey, currentVer, nil); err != nil {
			db.Close()
			return nil, err
		}

	case nil:
		// Version present, flush if different
		if !bytes.Equal(blob, currentVer) {
			db.Close()
			if err = os.RemoveAll(path); err != nil {
				return nil, err
			}
			return newPersistentCsDB(path, version)
		}
	}
	return &csDB{
		lvl: db,
	}, nil
}

// Get returns the given key if it's present.
func (db *csDB) Get(key []byte) ([]byte, error) {
	// Retrieve the key and increment the miss counter if not found
	return db.lvl.Get(key, nil)
}

// Put puts the given key / value to the queue
func (db *csDB) Put(key []byte, value []byte) error {
	return db.lvl.Put(key, value, nil)
}

// Delete deletes the key from the queue and database
func (db *csDB) Delete(key []byte) error {
	return db.lvl.Delete(key, nil)
}

// close flushes and closes the database files.
func (db *csDB) Close() {
	db.lvl.Close()
}

func getUsedFlowKey(addr common.Address) []byte {
	return append(usedFlowPrefix, addr[:]...)
}

func getAuthAllowFlowKey(addr common.Address) []byte {
	return append(authAllowFlowPrefix, addr[:]...)
}
