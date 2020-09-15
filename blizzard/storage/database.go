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

package storage

import (
	"github.com/contatract/go-contatract/log"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

const (
	storageDir = "config/chaindata/gctt/storage"
)

type StorageDB struct {
	db *leveldb.DB
}

func OpenDB() (db *StorageDB, err error) {
	handles := 0
	cache := 768
	file := storageDir
	// Open the db and recover any potential corruptions
	ldb, err := leveldb.OpenFile(storageDir, &opt.Options{
		OpenFilesCacheCapacity: handles,
		BlockCacheCapacity:     cache / 2 * opt.MiB,
		WriteBuffer:            cache / 4 * opt.MiB, // Two of these are used internally
		Filter:                 filter.NewBloomFilter(10),
	})

	if _, corrupted := err.(*errors.ErrCorrupted); corrupted {
		ldb, err = leveldb.RecoverFile(file, nil)
	}
	// (Re)check for errors and abort if opening of the db failed
	if err != nil {
		return nil, err
	}

	return &StorageDB{
		db: ldb,
	}, nil
}

func (sdb *StorageDB) Close() {
	err := sdb.db.Close()
	if err != nil {
		log.Error("Failed to close database", "err", err)
	}
}

// Get returns the given key if it's present.
func (sdb *StorageDB) Get(key []byte) ([]byte, error) {
	// Retrieve the key and increment the miss counter if not found
	dat, err := sdb.db.Get(key, nil)
	if err != nil {
		return nil, err
	}
	return dat, nil
}

// Put puts the given key / value to the queue
func (sdb *StorageDB) Put(key []byte, value []byte) error {
	return sdb.db.Put(key, value, nil)
}

// Delete deletes the key from the queue and database
func (sdb *StorageDB) Delete(key []byte) error {
	return sdb.db.Delete(key, nil)
}

func NewDB() *StorageDB {
	db, err := OpenDB()
	if err != nil {
		log.Error("open db", "err", err)
		return nil
	}
	return db
}
