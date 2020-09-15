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

package blizzard

import (
	//"errors"
	//"fmt"
	//"net"
	//"strconv"
	//"time"
	//"sync"
	//"math/big"

	//"github.com/contatract/go-contatract/log"
	//"github.com/contatract/go-contatract/p2p"
	//"github.com/contatract/go-contatract/common"
	//storage"github.com/contatract/go-contatract/blizzard/storage"
	//"github.com/contatract/go-contatract/ethdb"
	"github.com/contatract/go-contatract/blizcore"
	"github.com/contatract/go-contatract/blizzard/storage"
	"github.com/contatract/go-contatract/log"
)

func (b *Bliz) CanPutIntoSyncPool(wop *blizcore.WriteOperReqData) bool {
	asso := b.chunkAssoMap[storage.Int2ChunkIdentity(wop.Info.Chunk)]
	if asso == nil {
		return false
	}

	return asso.CanAddNewWopSyncCache()
}

func (b *Bliz) PutIntoSyncPool(wop *blizcore.WriteOperReqData) error {

	//ChunkPartnerAsso
	asso := b.chunkAssoMap[storage.Int2ChunkIdentity(wop.Info.Chunk)]
	if asso == nil {
		log.Error("PutIntoSyncPool asso nil", "wop.Info.Chunk", wop.Info.Chunk)
		return blizcore.ErrLocalNotChunkFarmer
	}

	asso.AddNewWopSyncCache(wop)
	return nil
}
