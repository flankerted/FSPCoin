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
	"crypto/rand"
	"encoding/binary"
	"fmt"
	mrand "math/rand"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	//"github.com/contatract/go-contatract/crypto"
	"github.com/contatract/go-contatract/blizcore"
	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/rlp"

	//"github.com/contatract/go-contatract/p2p/discover"
	blizstorage "github.com/contatract/go-contatract/blizzard/storage"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/storage"
	"github.com/syndtr/goleveldb/leveldb/util"
)

var (
	sliceDBNilChunkId     = blizstorage.ChunkIdentity{} // Special node ID to use as a nil element.
	sliceDBNodeExpiration = 24 * time.Hour              // Time after which an unseen node should be dropped.
	sliceDBCleanupCycle   = time.Hour                   // Time period for running the expiration task.
)

// sliceDB stores all nodes we know about.
type sliceDB struct {
	lvl    *leveldb.DB   // Interface to the database itself
	runner sync.Once     // Ensures we can start at most one expirer
	quit   chan struct{} // Channel to signal the expiring thread to stop
}

type SliceDbInfo struct {
	WopInfo blizcore.WopInfo

	// Time when the info was added..
	refreshAt time.Time
}

// Schema layout for the node database
var (
	sliceDBVersionKey      = []byte("version") // Version of the database to flush if changes
	sliceDBItemPrefix      = []byte("n:")      // Identifier to prefix node entries with
	sliceDBDiscoverRoot    = ":slice"
	sliceDBDiscoverRefresh = sliceDBDiscoverRoot + ":refresh"
)

// newNodeDB creates a new node database for storing and retrieving infos about
// known peers in the network. If no path is given, an in-memory, temporary
// database is constructed.
func newSliceDB(path string, version int) (*sliceDB, error) {
	if path == "" {
		return newMemorySliceDB()
	}
	return newPersistentSliceDB(path, version)
}

// newMemoryNodeDB creates a new in-memory node database without a persistent
// backend.
func newMemorySliceDB() (*sliceDB, error) {
	db, err := leveldb.Open(storage.NewMemStorage(), nil)
	if err != nil {
		return nil, err
	}
	return &sliceDB{
		lvl:  db,
		quit: make(chan struct{}),
	}, nil
}

// newPersistentChunkDB creates/opens a leveldb backed persistent chunk database,
// also flushing its contents in case of a version mismatch.
func newPersistentSliceDB(path string, version int) (*sliceDB, error) {
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

	blob, err := db.Get(sliceDBVersionKey, nil)
	switch err {
	case leveldb.ErrNotFound:
		// Version not found (i.e. empty cache), insert it
		if err := db.Put(sliceDBVersionKey, currentVer, nil); err != nil {
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
			return newPersistentSliceDB(path, version)
		}
	}
	return &sliceDB{
		lvl:  db,
		quit: make(chan struct{}),
	}, nil
}

// makeKey generates the leveldb key-blob from a node id and its particular
// field of interest.
func makeKey(objId string, sliceId uint32, id blizstorage.ChunkIdentity, field string) []byte {

	if bytes.Equal(id[:], sliceDBNilChunkId[:]) {
		return []byte(field)
	}

	// blizstorage.ChunkIdentity2Int(id)
	part := fmt.Sprintf("%s:%d:%s", objId, sliceId, field)

	return append(sliceDBItemPrefix, append(id[:], part...)...)
}

// splitKey tries to split a database key into a node id and a field part.
func splitKey(key []byte) (objId string, id blizstorage.ChunkIdentity, sliceId int, field string) {
	// If the key is not of a node, return it plainly
	if !bytes.HasPrefix(key, sliceDBItemPrefix) {
		return "", blizstorage.ChunkIdentity{}, -1, string(key)
	}
	// Otherwise split the id and field
	item := key[len(sliceDBItemPrefix):]
	copy(id[:], item[:len(id)])

	sliceAfield := string(item[len(id):])

	a := strings.Split(sliceAfield, ":")
	if len(a) < 3 {
		return "", blizstorage.ChunkIdentity{}, -1, string(key)
	}

	obj := a[0]
	slice, err := strconv.Atoi(a[1])
	if err != nil {
		return "", blizstorage.ChunkIdentity{}, -1, string(key)
	}

	return obj, id, slice, a[2]
}

// fetchInt64 retrieves an integer instance associated with a particular
// database key.
func (db *sliceDB) fetchInt64(key []byte) int64 {
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
func (db *sliceDB) storeInt64(key []byte, n int64) error {
	blob := make([]byte, binary.MaxVarintLen64)
	blob = blob[:binary.PutVarint(blob, n)]

	return db.lvl.Put(key, blob, nil)
}

// node retrieves a node with a given id from the database.
func (db *sliceDB) Slice(objId string, id blizstorage.ChunkIdentity, sliceId uint32) *SliceDbInfo {
	blob, err := db.lvl.Get(makeKey(objId, sliceId, id, sliceDBDiscoverRoot), nil)
	if err != nil {
		return nil
	}
	sliceInfo := new(SliceDbInfo)
	if err := rlp.DecodeBytes(blob, sliceInfo); err != nil {
		log.Error("Failed to decode node RLP", "err", err)
		debug.PrintStack()
		return nil
	}
	return sliceInfo
}

// updateNode inserts - potentially overwriting - a node into the peer database.
func (db *sliceDB) UpdateNode(objId string, sliceInfo *SliceDbInfo) error {
	sliceInfo.refreshAt = time.Now()
	blob, err := rlp.EncodeToBytes(sliceInfo)
	if err != nil {
		return err
	}

	chunkId := blizstorage.Int2ChunkIdentity(sliceInfo.WopInfo.Chunk)
	db.updateRefreshTime(objId, sliceInfo.WopInfo.Slice, chunkId, time.Now())
	return db.lvl.Put(makeKey(objId, sliceInfo.WopInfo.Slice, chunkId, sliceDBDiscoverRoot), blob, nil)
}

// deleteNode deletes all information/keys associated with a node.
func (db *sliceDB) DeleteNode(id blizstorage.ChunkIdentity) error {
	deleter := db.lvl.NewIterator(util.BytesPrefix(makeKey("", 0, id, "")), nil)
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
func (db *sliceDB) ensureExpirer() {
	db.runner.Do(func() { go db.expirer() })
}

// expirer should be started in a go routine, and is responsible for looping ad
// infinitum and dropping stale data from the database.
func (db *sliceDB) expirer() {
	tick := time.NewTicker(sliceDBCleanupCycle)
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
func (db *sliceDB) expireNodes() error {
	threshold := time.Now().Add(-sliceDBNodeExpiration)

	// Find discovered nodes that are older than the allowance
	it := db.lvl.NewIterator(nil, nil)
	defer it.Release()

	for it.Next() {
		// Skip the item if not a discovery node
		objId, id, sliceId, field := splitKey(it.Key())
		if field != sliceDBDiscoverRoot {
			continue
		}
		if sliceId < 0 {
			continue
		}
		// Skip the node if not expired yet (and not self)
		if seen := db.refreshTime(objId, uint32(sliceId), id); seen.After(threshold) {
			continue
		}
		// Otherwise delete all associated information
		db.DeleteNode(id)
	}
	return nil
}

// bondTime retrieves the time of the last successful pong from remote node.
func (db *sliceDB) refreshTime(objId string, sliceId uint32, chunkId blizstorage.ChunkIdentity) time.Time {
	return time.Unix(db.fetchInt64(makeKey(objId, sliceId, chunkId, sliceDBDiscoverRefresh)), 0)
}

// hasBond reports whether the given node is considered bonded.
func (db *sliceDB) hasRefresh(objId string, sliceId uint32, chunkId blizstorage.ChunkIdentity) bool {
	return time.Since(db.refreshTime(objId, sliceId, chunkId)) < sliceDBNodeExpiration
}

// updateBondTime updates the last pong time of a node.
func (db *sliceDB) updateRefreshTime(objId string, sliceId uint32, chunkId blizstorage.ChunkIdentity, instance time.Time) error {
	return db.storeInt64(makeKey(objId, sliceId, chunkId, sliceDBDiscoverRefresh), instance.Unix())
}

// querySeeds retrieves random nodes to be used as potential seed nodes
// for bootstrapping.
func (db *sliceDB) querySeeds(objId string, n int, maxAge time.Duration) []*SliceDbInfo {
	var (
		now   = time.Now()
		nodes = make([]*SliceDbInfo, 0, n)
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
		sliceId := mrand.Intn(30000)
		it.Seek(makeKey(objId, uint32(sliceId), id, sliceDBDiscoverRoot))

		n := nextNode(it)
		if n == nil {
			id[0] = 0
			continue seek // iterator exhausted
		}

		chunkId := blizstorage.Int2ChunkIdentity(n.WopInfo.Chunk)
		if now.Sub(db.refreshTime(objId, n.WopInfo.Slice, chunkId)) > maxAge {
			continue seek
		}
		for i := range nodes {
			if nodes[i].WopInfo.Chunk == n.WopInfo.Chunk && nodes[i].WopInfo.Slice == n.WopInfo.Slice {
				continue seek // duplicate
			}
		}
		nodes = append(nodes, n)
	}
	return nodes
}

// querySeeds retrieves random nodes to be used as potential seed nodes
// for bootstrapping.
func (db *sliceDB) AllSlices(objId string, total int, maxAge time.Duration) []*SliceDbInfo {
	var (
		cnt   = 0
		now   = time.Now()
		nodes = make([]*SliceDbInfo, 0)
		it    = db.lvl.NewIterator(nil, nil)
	)
	defer it.Release()

	it.First()
seek:
	for {
		// Seek to a random entry. The first byte is incremented by a
		// random amount each time in order to increase the likelihood
		// of hitting all existing nodes in very small databases.
		n := nextNode(it)
		if n == nil {
			break seek // iterator exhausted
		}

		chunkId := blizstorage.Int2ChunkIdentity(n.WopInfo.Chunk)

		if maxAge > 0 && now.Sub(db.refreshTime(objId, n.WopInfo.Slice, chunkId)) > maxAge {
			continue seek
		}
		for i := range nodes {
			if nodes[i].WopInfo.Chunk == n.WopInfo.Chunk && nodes[i].WopInfo.Slice == n.WopInfo.Slice {
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
func nextNode(it iterator.Iterator) *SliceDbInfo {
	for end := false; !end; end = !it.Next() {
		_, id, _, field := splitKey(it.Key())
		if field != sliceDBDiscoverRoot {
			continue
		}
		var slice SliceDbInfo
		if err := rlp.DecodeBytes(it.Value(), &slice); err != nil {
			log.Warn("Failed to decode node RLP", "id", id, "err", err)
			continue
		}
		return &slice
	}
	return nil
}

// close flushes and closes the database files.
func (db *sliceDB) close() {
	close(db.quit)
	db.lvl.Close()
}

func (db *sliceDB) Close() {
	db.close()
}
