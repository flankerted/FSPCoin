// Copyright 2014 The go-contatract Authors
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

package types_elephant

import (
	"bytes"

	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/rlp"
	"github.com/contatract/go-contatract/trie"
)

type DerivableList interface {
	Len() int
	GetRlp(i int) []byte
}

func DeriveSha(list DerivableList) common.Hash {
	keybuf := new(bytes.Buffer)
	trie := new(trie.Trie)
	for i := 0; i < list.Len(); i++ {
		keybuf.Reset()
		rlp.Encode(keybuf, uint(i))
		trie.Update(keybuf.Bytes(), list.GetRlp(i))
	}
	return trie.Hash()
}

//func DeriveTrie(list DerivableList) *trie.Trie {
//	keybuf := new(bytes.Buffer)
//	trie := new(trie.Trie)
//	for i := 0; i < list.Len(); i++ {
//		keybuf.Reset()
//		rlp.Encode(keybuf, uint(i))
//		trie.Update(keybuf.Bytes(), list.GetRlp(i))
//	}
//	return trie
//}

func DeriveTrieForOutTxs(outTxsByShardingID map[uint16]common.Hash) *trie.Trie {
	valbuf := new(bytes.Buffer)
	trie := new(trie.Trie)
	for id, hash := range outTxsByShardingID {
		valbuf.Reset()
		rlp.Encode(valbuf, uint(id))
		trie.Update(hash.Bytes(), valbuf.Bytes())
	}
	return trie
}

func DeriveHashForOutTxs(outTxsByShardingID map[uint16]common.Hash) common.Hash {
	valbuf := new(bytes.Buffer)
	trie := new(trie.Trie)
	for id, hash := range outTxsByShardingID {
		valbuf.Reset()
		rlp.Encode(valbuf, uint(id))
		trie.Update(hash.Bytes(), valbuf.Bytes())
	}
	return trie.Hash()
}
