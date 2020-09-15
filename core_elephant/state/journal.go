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

package state

import (
	"math/big"

	"github.com/contatract/go-contatract/common"
)

type journalEntry interface {
	undo(*StateDB)
}

type journal []journalEntry

type (
	// Changes to the account trie.
	createObjectChange struct {
		account *common.Address
	}
	resetObjectChange struct {
		prev *stateObject
	}
	suicideChange struct {
		account     *common.Address
		prev        bool // whether account had already suicided
		prevbalance *big.Int
	}

	// Changes to individual accounts.
	balanceChange struct {
		account *common.Address
		prev    *big.Int
	}
	nonceChange struct {
		account *common.Address
		prev    uint64
	}
	storageChange struct {
		account       *common.Address
		key, prevalue common.Hash
	}
	codeChange struct {
		account            *common.Address
		prevcode, prevhash []byte
	}

	// Changes to other state values.
	refundChange struct {
		prev uint64
	}
	addLogChange struct {
		txhash common.Hash
	}
	addPreimageChange struct {
		hash common.Hash
	}
	touchChange struct {
		account   *common.Address
		prev      bool
		prevDirty bool
	}

	// Changes to the trie.
	createChunkChange struct {
		chunkId *uint64
	}
	resetChunkChange struct {
		prev *stateChunk
	}

	createSliceChange struct {
		account *common.Address
	}
	resetSliceChange struct {
		prev *stateSlice
	}

	// createClaimChange struct {
	// 	account *common.Address
	// }
	// resetClaimChange struct {
	// 	prev *stateClaim
	// }

	signCsAddrChange struct {
		account *common.Address
		prev    []common.Address
	}

	serverCliAddrChange struct {
		account *common.Address
		prev    []common.CSAuthData
	}
)

func (ch createObjectChange) undo(s *StateDB) {
	delete(s.stateObjects, *ch.account)
	delete(s.stateObjectsDirty, *ch.account)
}

func (ch resetObjectChange) undo(s *StateDB) {
	s.setStateObject(ch.prev)
}

func (ch suicideChange) undo(s *StateDB) {
	obj := s.getStateObject(*ch.account)
	if obj != nil {
		obj.suicided = ch.prev
		obj.setBalance(ch.prevbalance)
	}
}

var ripemd = common.HexToAddress("0000000000000000000000000000000000000003")

func (ch touchChange) undo(s *StateDB) {
	if !ch.prev && *ch.account != ripemd {
		s.getStateObject(*ch.account).touched = ch.prev
		if !ch.prevDirty {
			delete(s.stateObjectsDirty, *ch.account)
		}
	}
}

func (ch balanceChange) undo(s *StateDB) {
	s.getStateObject(*ch.account).setBalance(ch.prev)
}

func (ch signCsAddrChange) undo(s *StateDB) {
	s.getStateObject(*ch.account).setSignCsAddr(ch.prev)
}

func (ch serverCliAddrChange) undo(s *StateDB) {
	s.getStateObject(*ch.account).setServerCliAddr(ch.prev)
}

func (ch nonceChange) undo(s *StateDB) {
	s.getStateObject(*ch.account).setNonce(ch.prev)
}

func (ch codeChange) undo(s *StateDB) {
	s.getStateObject(*ch.account).setCode(common.BytesToHash(ch.prevhash), ch.prevcode)
}

func (ch storageChange) undo(s *StateDB) {
	s.getStateObject(*ch.account).setState(ch.key, ch.prevalue)
}

func (ch refundChange) undo(s *StateDB) {
	s.refund = ch.prev
}

func (ch addLogChange) undo(s *StateDB) {
	logs := s.logs[ch.txhash]
	if len(logs) == 1 {
		delete(s.logs, ch.txhash)
	} else {
		s.logs[ch.txhash] = logs[:len(logs)-1]
	}
	s.logSize--
}

func (ch addPreimageChange) undo(s *StateDB) {
	delete(s.preimages, ch.hash)
}

func (ch createChunkChange) undo(s *StateDB) {
	delete(s.stateChunks, *ch.chunkId)
	delete(s.stateChunksDirty, *ch.chunkId)
}

func (ch resetChunkChange) undo(s *StateDB) {
	s.setStateChunk(ch.prev)
}

func (ch createSliceChange) undo(s *StateDB) {
	delete(s.stateSlices, *ch.account)
	delete(s.stateSlicesDirty, *ch.account)
}

func (ch resetSliceChange) undo(s *StateDB) {
	s.setStateSlice(ch.prev)
}

// func (ch createClaimChange) undo(s *StateDB) {
// 	delete(s.stateClaims, *ch.account)
// 	delete(s.stateClaimsDirty, *ch.account)
// }

// func (ch resetClaimChange) undo(s *StateDB) {
// 	s.setStateClaim(ch.prev)
// }
