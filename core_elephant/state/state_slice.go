package state

import (
	"fmt"

	"github.com/contatract/go-contatract/common"
)

type ObjSlice struct {
	ChunkId    uint64
	SliceId    uint32
	ObjId      uint64
	Permission uint32
}

type stateSlice struct {
	address common.Address
	slices  []*ObjSlice
	db      *StateDB
	dbErr   error
	trie    Trie
	onDirty func(addr common.Address)
}

// newSlice creates a state slice.
func newSlice(db *StateDB, address common.Address, slices []*ObjSlice, onDirty func(addr common.Address)) *stateSlice {
	return &stateSlice{
		db:      db,
		address: address,
		slices:  slices,
		onDirty: onDirty,
	}
}

func (self *stateSlice) deepCopy(db *StateDB, onDirty func(addr common.Address)) *stateSlice {
	stateSlice := newSlice(db, self.address, self.slices, onDirty)
	if self.trie != nil {
		stateSlice.trie = db.db.CopyTrie(self.trie)
	}
	return stateSlice
}

func (self *stateSlice) print() {
	fmt.Println("address", self.address.Hex())
	for i, s := range self.slices {
		fmt.Println("slice", i, s.ChunkId, s.SliceId, s.ObjId, s.Permission)
	}
}

// CommitTrie the storage trie of the object to dwb.
// This updates the trie root.
func (self *stateSlice) CommitTrie(db Database) error {
	if self.dbErr != nil {
		return self.dbErr
	}
	_, err := self.trie.Commit(nil)
	return err
}
