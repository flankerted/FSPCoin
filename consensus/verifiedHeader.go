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

package consensus

import (
	"sync/atomic"

	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/crypto/sha3"
	"github.com/contatract/go-contatract/rlp"
)

// PoliceVerifiedHeader is a result that contains the header verified by the police representing it's transactions are valid
type PoliceVerifiedHeader struct {
	ShardingID uint16
	BlockNum   uint64
	LampBase   *common.Hash
	Header     *common.Hash
	Previous   []*common.Hash

	Sign string

	// caches
	hash atomic.Value
}

func (h *PoliceVerifiedHeader) Hash() common.Hash {
	if hash := h.hash.Load(); hash != nil {
		return hash.(common.Hash)
	}
	v := rlpHash([]interface{}{
		h.ShardingID,
		h.BlockNum,
		h.LampBase,
		h.Header,
		h.Previous,
	})
	h.hash.Store(v)
	return v
}

func rlpHash(x interface{}) (h common.Hash) {
	hw := sha3.NewKeccak256()
	// JiangHan: 意思是将x首先进行rlp编码，然后将编码输入 hw 进行散列，用hw 的散列输出当成结果
	rlp.Encode(hw, x)
	hw.Sum(h[:0])
	return h
}
