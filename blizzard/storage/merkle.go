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
	"crypto/sha256"

	"github.com/contatract/go-contatract/common"
)

const (
	relateHashCount = 30 //chunkSize对应的兄弟hash个数
)

type Merkle struct {
	hashs []common.Hash
}

func CalculateContentHash(content []byte) common.Hash {
	hash := sha256.Sum256(content)
	return hash
}

func CalculateParentHash(hash1 common.Hash, hash2 common.Hash) common.Hash {
	data := append(hash1.Bytes(), hash2.Bytes()...)
	hash := sha256.Sum256(data)
	return hash
}

func NewMerkle(hashs []common.Hash) *Merkle {
	return &Merkle{
		hashs: hashs,
	}
}

func GetBrotherHashs(hashs []common.Hash, pos int) []common.Hash {
	depth := 0
	count := len(hashs)
	tmpCount := count
	for {
		if tmpCount == 1 {
			break
		}
		depth++
		tmpCount /= 2
	}

	hashRet := make([]common.Hash, depth)
	if depth == 0 {
		return hashRet
	}
	if pos == 0 || pos > count {
		return hashRet
	}

	posLeft := true
	if pos%2 == 0 {
		posLeft = false
	}

	parentHash := make([]common.Hash, len(hashs))
	for i := 0; i < len(hashs); i++ {
		parentHash[i] = hashs[i]
	}

	for i := 0; i < depth; i++ {
		if posLeft {
			hashRet[i] = parentHash[pos]
		} else {
			hashRet[i] = parentHash[pos-1]
		}
		pos /= 2

		count /= 2
		for j := 0; j < count; j++ {
			left := parentHash[j*2]
			right := parentHash[j*2+1]
			parentHash[j] = CalculateParentHash(left, right)
		}
	}
	return hashRet
}
