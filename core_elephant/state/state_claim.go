package state

import (
	"runtime/debug"

	"github.com/contatract/go-contatract/common"
)

type ObjClaim struct {
	ChunkId    uint64
	ClaimCount uint8
}

type ClaimInfo struct {
	ClaimId uint64      // 开放claim start chunk id
	Claims  []*ObjClaim // 历史claim chunk id list
}

type stateClaim struct {
	address   common.Address
	db        *StateDB
	claimInfo *ClaimInfo
	trie      Trie
	onDirty   func(addr common.Address)
}

// newClaim creates a state claim.
func newClaim(address common.Address, db *StateDB, claimInfo *ClaimInfo, onDirty func(addr common.Address)) *stateClaim {
	return &stateClaim{
		address:   address,
		db:        db,
		claimInfo: claimInfo,
		onDirty:   onDirty,
	}
}

func (self *stateClaim) deepCopy(db *StateDB, onDirty func(addr common.Address)) *stateClaim {
	debug.PrintStack()
	stateClaim := newClaim(self.address, db, self.claimInfo, onDirty)
	if self.trie != nil {
		stateClaim.trie = db.db.CopyTrie(self.trie)
	}
	return stateClaim
}
