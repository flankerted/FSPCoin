package state

import (
	"encoding/json"
	"fmt"

	"github.com/contatract/go-contatract/blizparam"
	"github.com/contatract/go-contatract/blizzard/storage"
	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/elephant/exporter"
	"github.com/contatract/go-contatract/event"
	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/p2p/discover"
)

type stateChunk struct {
	claimersDB  ClaimersDB
	verifyersDB VerifyersDB
	rentDB      RentDB // max: storage.SliceCount
	db          *StateDB
	deleted     bool
	chunkId     uint64
	//claimCount map[common.Address]int
	CanUse   bool // 可以使用
	CanClaim bool // 可以申领
	trie     Trie
}

type ClaimerDB struct {
	Address  common.Address
	RootHash common.Hash
	Node     *discover.Node
}

type ClaimersDB []ClaimerDB

type VerifyerDB struct {
	Address  common.Address
	RootHash common.Hash
}

type VerifyersDB []VerifyerDB

type RentDB []common.Address

// newObject creates a state object.
func newStateChunk(db *StateDB, chunkId uint64, claimersDB ClaimersDB, onDirty func(chunkId uint64)) *stateChunk {
	// fmt.Println("chunkId: ", chunkId)
	// for i, claimerDB := range claimersDB {
	// 	log.Info("New state chunk", "i", i, "addr", claimerDB.Address.ShortString())
	// }
	chunk := &stateChunk{
		claimersDB: claimersDB,
		db:         db,
		chunkId:    chunkId,
		//claimCount: make(map[common.Address]int),
		rentDB: make(RentDB, storage.SliceCount),
	}
	chunk.updateClaimCount2()
	return chunk
}

func (c *stateChunk) updateClaimCount(oldMax int, feeds []*event.Feed) {
	claimCount := make(map[common.Hash]int)
	cls := c.claimersDB
	for _, v := range cls {
		hash := v.RootHash
		if claimCount[hash] == 0 {
			claimCount[hash] = 1
		} else {
			claimCount[hash] += 1
		}
	}

	newMax, relateHash := c.getClaimMaxCount()
	copyCnt := blizparam.GetMinCopyCount()
	if newMax >= copyCnt {
		c.CanUse = true
	}

	// 删除不一致数据
	if oldMax < copyCnt && newMax >= copyCnt {
		var cls ClaimersDB
		for _, v := range c.claimersDB {
			if v.RootHash == relateHash {
				cls = append(cls, v)
			}
		}
		if len(c.claimersDB) != len(cls) {
			c.claimersDB = cls
			c.db.updateStateChunkClaim(c)
		}
	}

	if c.CanUse {
		// 通知建立asso
		count := len(c.claimersDB)
		nodes := make([]discover.Node, 0, count)
		for i := 0; i < count; i++ {
			nodes = append(nodes, *c.claimersDB[i].Node)
		}
		chundID := c.chunkId
		// go func() {
		req := &exporter.ObjChunkPartners{ChunkId: chundID, Partners: nodes}
		feeds[CChunkPartners].Send(req)
		// }()
	}
}

func (c *stateChunk) updateClaimCount2() {
	claimCount := make(map[common.Hash]int)
	cls := c.claimersDB
	for _, v := range cls {
		hash := v.RootHash
		if claimCount[hash] == 0 {
			claimCount[hash] = 1
		} else {
			claimCount[hash] += 1
		}
	}

	newMax, _ := c.getClaimMaxCount()
	copyCnt := blizparam.GetMinCopyCount()
	if newMax >= copyCnt {
		c.CanUse = true
	}
}

func (c *stateChunk) getClaimMaxCount() (int, common.Hash) {
	var maxCount int
	var hash common.Hash
	hashCount := make(map[common.Hash]int)
	for _, v := range c.claimersDB {
		h := v.RootHash
		if _, ok := hashCount[h]; !ok {
			hashCount[h] = 1
		} else {
			hashCount[h] += 1
		}
		if maxCount < hashCount[h] {
			maxCount = hashCount[h]
			hash = h
		}
	}
	// stateDebug("Get claim max count", "count", maxCount)
	return maxCount, hash
}

func (c *stateChunk) hasClaimerDB(address common.Address) bool {
	return c.IndexClaimerDB(address) != -1
}

func (c *stateChunk) IndexClaimerDB(address common.Address) int {
	for i, claimerDB := range c.claimersDB {
		if claimerDB.Address == address {
			return i
		}
	}
	return -1
}

func (c *stateChunk) IndexVerifyerDB(address common.Address) int {
	for i, verifyerDB := range c.verifyersDB {
		if verifyerDB.Address == address {
			return i
		}
	}
	return -1
}

func FromClaimersDB(cs ClaimersDB) []byte {
	ret, err := json.Marshal(cs)
	if err != nil {
		log.Error("from claimers db", "err", err)
		return nil
	}
	//fmt.Printf("from claimers db %+v\n", ret)
	return ret
}

func ToClaimersDB(req []byte) ClaimersDB {
	data := make(ClaimersDB, 0)
	if err := json.Unmarshal(req, &data); err != nil {
		log.Error("to claimers db", "err", err)
		return data
	}
	//for i, v := range data {
	//	fmt.Printf("to claimers db [%v]: %+v\n", i, v)
	//}
	return data
}

func FromVerifyersDB(vs VerifyersDB) []byte {
	ret, err := json.Marshal(vs)
	if err != nil {
		log.Error("from verifyers db", "err", err)
		return nil
	}
	//fmt.Printf("from verifyers db %+v\n", ret)
	return ret
}

func ToVerifyersDB(req []byte) VerifyersDB {
	data := make(VerifyersDB, 0)
	if err := json.Unmarshal(req, &data); err != nil {
		log.Error("to verifyers db", "err", err)
		return data
	}
	//for i, v := range data {
	//	fmt.Printf("to verifyers db [%v]: %+v\n", i, v)
	//}
	return data
}

func (self *stateChunk) deepCopy(db *StateDB, onDirty func(chunkId uint64)) *stateChunk {
	stateChunk := newStateChunk(db, self.chunkId, self.claimersDB, onDirty)
	if self.trie != nil {
		stateChunk.trie = db.db.CopyTrie(self.trie)
	}
	return stateChunk
}

// CommitTrie the storage trie of the object to dwb.
// This updates the trie root.
func (self *stateChunk) CommitTrie(db Database) error {
	var err error
	if self.trie != nil {
		_, err = self.trie.Commit(nil)
	}
	return err
}

func (s *stateChunk) Log() {
	fmt.Println("claimers: ")
	for i, c := range s.claimersDB {
		fmt.Println("i: ", i, ", shortaddr", c.Address.ShortString(), "shortroothash", c.RootHash.ShortString())
	}

	fmt.Println("verifyers: ")
	for i, v := range s.verifyersDB {
		fmt.Println("i: ", i, ", shortaddr", v.Address.ShortString(), "shortroothash", v.RootHash.ShortString())
	}

	fmt.Println("chunk: ", s.chunkId, ", canuse: ", s.CanUse, "canclaim: ", s.CanClaim)
}
