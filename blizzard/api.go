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
	"github.com/contatract/go-contatract/blizzard/storage"
	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/sharding"
)

// PublicBlizzardAPI provides an API to access Contatract full node-related
// information.
type PublicBlizzardAPI struct {
	bliz *Bliz
}

// NewPublicElephantAPI creates a new Contatract protocol API for full nodes.
func NewPublicBlizzardAPI(b *Bliz) *PublicBlizzardAPI {
	return &PublicBlizzardAPI{b}
}

// Etherbase is the address that mining rewards will be send to
func (api *PublicBlizzardAPI) Test() {
	log.Info("It works")
	return
}

func (api *PublicBlizzardAPI) GetShardingMiner() {
	var address common.Address
	shardingIdx := sharding.ShardingAddressMagic(sharding.ShardingCount, address.String())
	if api.bliz.cachedShardingNodesMap[shardingIdx] == nil {
		api.bliz.GetShardingMiner(shardingIdx, address)
	}
}

func (api *PublicBlizzardAPI) DeleteNode(cId uint64) {
	chunkId := storage.Int2ChunkIdentity(cId)
	api.bliz.chunkdb.DeleteNode(chunkId)
}
