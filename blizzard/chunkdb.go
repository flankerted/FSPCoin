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

package blizzard

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"os"
	"sync"
	"time"

	//"github.com/contatract/go-contatract/crypto"
	blizstorage "github.com/contatract/go-contatract/blizzard/storage"
	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/p2p/discover"
	"github.com/contatract/go-contatract/rlp"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/storage"
	"github.com/syndtr/goleveldb/leveldb/util"
)

var (
	chunkDBNilChunkId     = blizstorage.ChunkIdentity{} // Special node ID to use as a nil element.
	chunkDBNodeExpiration = 24 * time.Hour              // Time after which an unseen node should be dropped.
	chunkDBCleanupCycle   = time.Hour                   // Time period for running the expiration task.
)

// nodeDB stores all nodes we know about.
type chunkDB struct {
	lvl    *leveldb.DB   // Interface to the database itself
	runner sync.Once     // Ensures we can start at most one expirer
	quit   chan struct{} // Channel to signal the expiring thread to stop
}

type ChunkDbInfo struct {
	Id       blizstorage.ChunkIdentity
	Partners []discover.Node

	// Time when the info was added..
	refreshAt time.Time
}

// Schema layout for the node database
var (
	chunkDBVersionKey      = []byte("version") // Version of the database to flush if changes
	chunkDBItemPrefix      = []byte("n:")      // Identifier to prefix node entries with
	chunkDBDiscoverRoot    = ":chunk"
	chunkDBDiscoverRefresh = chunkDBDiscoverRoot + ":refresh"
)

// newNodeDB creates a new node database for storing and retrieving infos about
// known peers in the network. If no path is given, an in-memory, temporary
// database is constructed.
func newChunkDB(path string, version int) (*chunkDB, error) {
	if path == "" {
		return newMemoryChunkDB()
	}
	return newPersistentChunkDB(path, version)
}

// newMemoryNodeDB creates a new in-memory node database without a persistent
// backend.
func newMemoryChunkDB() (*chunkDB, error) {
	db, err := leveldb.Open(storage.NewMemStorage(), nil)
	if err != nil {
		return nil, err
	}
	return &chunkDB{
		lvl:  db,
		quit: make(chan struct{}),
	}, nil
}

// newPersistentChunkDB creates/opens a leveldb backed persistent chunk database,
// also flushing its contents in case of a version mismatch.
func newPersistentChunkDB(path string, version int) (*chunkDB, error) {
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

	blob, err := db.Get(chunkDBVersionKey, nil)
	switch err {
	case leveldb.ErrNotFound:
		// Version not found (i.e. empty cache), insert it
		if err := db.Put(chunkDBVersionKey, currentVer, nil); err != nil {
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
			return newPersistentChunkDB(path, version)
		}
	}
	return &chunkDB{
		lvl:  db,
		quit: make(chan struct{}),
	}, nil
}

// makeKey generates the leveldb key-blob from a node id and its particular
// field of interest.
func makeKey(id blizstorage.ChunkIdentity, field string) []byte {
	if bytes.Equal(id[:], chunkDBNilChunkId[:]) {
		return []byte(field)
	}
	return append(chunkDBItemPrefix, append(id[:], field...)...)
}

// splitKey tries to split a database key into a node id and a field part.
func splitKey(key []byte) (id blizstorage.ChunkIdentity, field string) {
	// If the key is not of a node, return it plainly
	if !bytes.HasPrefix(key, chunkDBItemPrefix) {
		return blizstorage.ChunkIdentity{}, string(key)
	}
	// Otherwise split the id and field
	item := key[len(chunkDBItemPrefix):]
	copy(id[:], item[:len(id)])
	field = string(item[len(id):])

	return id, field
}

// fetchInt64 retrieves an integer instance associated with a particular
// database key.
func (db *chunkDB) fetchInt64(key []byte) int64 {
	blob, err := db.lvl.Get(key, nil)
	if err != nil {
		return 0
	}
	val, read := binary.Varint(blob)
	if read <= 0 {
		return 0
	}
	return val
}

// storeInt64 update a specific database entry to the current time instance as a
// unix timestamp.
func (db *chunkDB) storeInt64(key []byte, n int64) error {
	blob := make([]byte, binary.MaxVarintLen64)
	blob = blob[:binary.PutVarint(blob, n)]

	return db.lvl.Put(key, blob, nil)
}

// node retrieves a node with a given id from the database.
func (db *chunkDB) chunk(id blizstorage.ChunkIdentity) *ChunkDbInfo {
	blob, err := db.lvl.Get(makeKey(id, chunkDBDiscoverRoot), nil)
	if err != nil {
		return nil
	}
	chunkInfo := new(ChunkDbInfo)
	if err := rlp.DecodeBytes(blob, chunkInfo); err != nil {
		log.Error("Failed to decode node RLP", "err", err)
		return nil
	}
	return chunkInfo
}

// updateNode inserts - potentially overwriting - a node into the peer database.
func (db *chunkDB) UpdateNode(chunkInfo *ChunkDbInfo) error {
	// asso.Log().Info("UpdateNode", "chunkInfo", chunkInfo)
	chunkInfo.refreshAt = time.Now()
	blob, err := rlp.EncodeToBytes(chunkInfo)
	if err != nil {
		log.Error("EncodeToBytes", "err", err)
		return err
	}

	db.updateRefreshTime(chunkInfo.Id, time.Now())
	return db.lvl.Put(makeKey(chunkInfo.Id, chunkDBDiscoverRoot), blob, nil)
}

// deleteNode deletes all information/keys associated with a node.
func (db *chunkDB) DeleteNode(id blizstorage.ChunkIdentity) error {
	deleter := db.lvl.NewIterator(util.BytesPrefix(makeKey(id, "")), nil)
	for deleter.Next() {
		if err := db.lvl.Delete(deleter.Key(), nil); err != nil {
			return err
		}
	}
	return nil
}

// ensureExpirer is a small helper method ensuring that the data expiration
// mechanism is running. If the expiration goroutine is already running, this
// method simply returns.
//
// The goal is to start the data evacuation only after the network successfully
// bootstrapped itself (to prevent dumping potentially useful seed nodes). Since
// it would require significant overhead to exactly trace the first successful
// convergence, it's simpler to "ensure" the correct state when an appropriate
// condition occurs (i.e. a successful bonding), and discard further events.
func (db *chunkDB) ensureExpirer() {
	db.runner.Do(func() { go db.expirer() })
}

// expirer should be started in a go routine, and is responsible for looping ad
// infinitum and dropping stale data from the database.
func (db *chunkDB) expirer() {
	tick := time.NewTicker(chunkDBCleanupCycle)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			if err := db.expireNodes(); err != nil {
				log.Error("Failed to expire nodedb items", "err", err)
			}
		case <-db.quit:
			return
		}
	}
}

// expireNodes iterates over the database and deletes all nodes that have not
// been seen (i.e. received a pong from) for some allotted time.
func (db *chunkDB) expireNodes() error {
	threshold := time.Now().Add(-chunkDBNodeExpiration)

	// Find discovered nodes that are older than the allowance
	it := db.lvl.NewIterator(nil, nil)
	defer it.Release()

	for it.Next() {
		// Skip the item if not a discovery node
		id, field := splitKey(it.Key())
		if field != chunkDBDiscoverRoot {
			continue
		}
		// Skip the node if not expired yet (and not self)
		if seen := db.refreshTime(id); seen.After(threshold) {
			continue
		}
		// Otherwise delete all associated information
		db.DeleteNode(id)
	}
	return nil
}

// bondTime retrieves the time of the last successful pong from remote node.
func (db *chunkDB) refreshTime(id blizstorage.ChunkIdentity) time.Time {
	return time.Unix(db.fetchInt64(makeKey(id, chunkDBDiscoverRefresh)), 0)
}

// hasBond reports whether the given node is considered bonded.
func (db *chunkDB) hasRefresh(id blizstorage.ChunkIdentity) bool {
	return time.Since(db.refreshTime(id)) < chunkDBNodeExpiration
}

// updateBondTime updates the last pong time of a node.
func (db *chunkDB) updateRefreshTime(id blizstorage.ChunkIdentity, instance time.Time) error {
	return db.storeInt64(makeKey(id, chunkDBDiscoverRefresh), instance.Unix())
}

// querySeeds retrieves random nodes to be used as potential seed nodes
// for bootstrapping.
func (db *chunkDB) querySeeds(n int, maxAge time.Duration) []*ChunkDbInfo {
	var (
		now   = time.Now()
		nodes = make([]*ChunkDbInfo, 0, n)
		it    = db.lvl.NewIterator(nil, nil)
		id    blizstorage.ChunkIdentity
	)
	defer it.Release()

seek:
	for seeks := 0; len(nodes) < n && seeks < n*5; seeks++ {
		// Seek to a random entry. The first byte is incremented by a
		// random amount each time in order to increase the likelihood
		// of hitting all existing nodes in very small databases.
		ctr := id[0]
		rand.Read(id[:])
		id[0] = ctr + id[0]%16
		it.Seek(makeKey(id, chunkDBDiscoverRoot))

		n := nextNode(it)
		if n == nil {
			id[0] = 0
			continue seek // iterator exhausted
		}
		if now.Sub(db.refreshTime(n.Id)) > maxAge {
			continue seek
		}
		for i := range nodes {
			if nodes[i].Id == n.Id {
				continue seek // duplicate
			}
		}
		nodes = append(nodes, n)
	}
	return nodes
}

func (db *chunkDB) AllChunks2(cIds []blizstorage.ChunkIdentity, maxAge time.Duration) []*ChunkDbInfo {
	infos := make([]*ChunkDbInfo, 0)
	for _, cId := range cIds {
		info := db.chunk(cId)
		if info == nil {
			continue
		}
		infos = append(infos, info)
	}
	return infos
}

// querySeeds retrieves random nodes to be used as potential seed nodes
// for bootstrapping.
func (db *chunkDB) AllChunks(total int, maxAge time.Duration) []*ChunkDbInfo { // qiweiyou: 暂时不使用，for会死循环，用AllChunks2代替
	var (
		cnt   = 0
		now   = time.Now()
		nodes = make([]*ChunkDbInfo, 0)
		it    = db.lvl.NewIterator(nil, nil)
	)
	defer it.Release()

seek:
	for {
		// Seek to a random entry. The first byte is incremented by a
		// random amount each time in order to increase the likelihood
		// of hitting all existing nodes in very small databases.
		n := nextNode(it)
		if n == nil {
			log.Info("AllChunks nextNode nil")
			break seek // iterator exhausted
		}
		if blizstorage.ChunkIdentity2Int(n.Id) == 0 {
			log.Warn("AllChunks n.Id 0")
			continue
		}
		if maxAge > 0 && now.Sub(db.refreshTime(n.Id)) > maxAge {
			log.Info("AllChunks", "time", now.Sub(db.refreshTime(n.Id)))
			continue seek
		}
		for i := range nodes {
			if nodes[i].Id == n.Id {
				log.Warn("AllChunks", "i", i, "id", nodes[i].Id, "Partners", nodes[i].Partners)
				continue seek // duplicate
			}
		}
		nodes = append(nodes, n)
		cnt++
		if total > 0 && cnt >= total {
			break seek
		}
	}
	return nodes
}

// reads the next node record from the iterator, skipping over other
// database entries.
func nextNode(it iterator.Iterator) *ChunkDbInfo {
	for end := false; !end; end = !it.Next() {
		id, field := splitKey(it.Key())
		if field != chunkDBDiscoverRoot {
			continue
		}
		var chunk ChunkDbInfo
		if err := rlp.DecodeBytes(it.Value(), &chunk); err != nil {
			log.Warn("Failed to decode node RLP", "id", id, "err", err)
			continue
		}
		return &chunk
	}
	return nil
}

// close flushes and closes the database files.
func (db *chunkDB) close() {
	close(db.quit)
	db.lvl.Close()
}
