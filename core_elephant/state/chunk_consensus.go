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

// Package state provides a caching layer atop the Ethereum state trie.
package state

import (
	"github.com/contatract/go-contatract/blizparam"
	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/p2p/discover"
)

// a.	在本批次开放的65536范围内，从0号Chunk往后进行新chunk的认领开拓，每个区块周期提供16个Chunk进行认领，处于本分片的 Farmer 根据公式  Fn(Fn(BlockHash,FarmerSelfID), ChunkID) 的方式逐一判断自己在这16个Chunk的认购权。每区块执行完毕，判断这16个Chunk的认购程度，如果认购副本数符合提供服务的要求（>=8）,即开放该块到Online模式。如果本批次16个Chunk认购程度整体偏低，则进度不推进，下个区块继续开放认购。
// b.	一个补强遍历层，补强遍历用来从Chunk0开始检查是否有某个Chunk副本数没有达到16（16作为标秤副本额度），如果没有继续开放认购，一次也提供16个来进行，并在世界状态中记录当前进度，以方便在下个区块确定新的Chunk组
// c.	以上的a,b两个层级可以满足空间开拓的基本需求，这里还需要增加一个层级，它也是遍历，它遍历范围不局限于当前的65536批次，它放宽到整个分片的全局Chunk列表，也就是0~ CurrentChunkCeiling这个范围（CurrentChunkCeiling是指的当前分片的最大ChunkID; 如果地址空间有分段，这个值就会有几个不同的范围，这里泛指整个分片当前的所有Chunk集合），这个层级遍历所有Chunk, 检查offline状态的Chunk（副本数<8）提供给Farmer认领，并在世界状态中记录当前进度，以方便在下个区块确定新的Chunk组

// 每个区块周期提供16个Chunk进行认领
const (
	claimChunkOpenMaxCount = 16
	claimChunkFindMaxCount = 65536
)

type chunkClaim struct {
	chunkId uint64
	nodes   []*discover.Node
}

type chunkList struct {
	ccs []*chunkClaim
}

func (cl chunkList) Len() int      { return len(cl.ccs) }
func (cl chunkList) Swap(i, j int) { cl.ccs[i], cl.ccs[j] = cl.ccs[j], cl.ccs[i] }
func (cl chunkList) Less(i, j int) bool {
	if cl.ccs[i].chunkId < cl.ccs[j].chunkId {
		return true
	}
	return false
}

// clist is the list in which the claimed copy count is less than the standard copy count
func getChunkIdA(chunkStartId uint64, clist chunkList, blockNumber uint64, node *discover.Node) uint64 {
	log.Info("get chunk id a", "chunkstartid", chunkStartId, "chunklistlen", clist.Len(), "blocknumber", blockNumber)
	from := chunkStartId
	if from%claimChunkOpenMaxCount != 1 {
		from = from/claimChunkOpenMaxCount*claimChunkOpenMaxCount + 1
	}
	to := from + claimChunkOpenMaxCount
	cl := getChunkList(clist, from, to)
	id := uint64(0)

	id += (blockNumber % claimChunkOpenMaxCount)

	for _, v := range node.ID {
		id += uint64(v)
	}
	chunkId := from + (id % claimChunkOpenMaxCount)
	chunkId = 1
	log.Info("rule", "chunk", chunkId)

	copyCnt := blizparam.GetMaxCopyCount()
	// 从规则a找(寻找此chunkId)
	for _, v := range cl.ccs {
		if v.chunkId != chunkId {
			continue
		}
		// 超上限
		if len(v.nodes) >= copyCnt {
			break
		}
		bFound := false
		for _, n := range v.nodes {
			if n != nil && n.Equal(node) {
				bFound = true
				break
			}
		}
		// 已经存在
		if bFound {
			break
		}
		log.Info("rule a1", "chunk", chunkId)
		return chunkId
	}
	// 从规则a找(寻找其他chunkId)
	for _, v := range cl.ccs {
		if v.chunkId < from || v.chunkId >= to {
			continue
		}
		// 超上限
		if len(v.nodes) >= copyCnt {
			continue
		}
		bFound := false
		for _, n := range v.nodes {
			if n != nil && n.Equal(node) {
				bFound = true
				break
			}
		}
		// 已经存在
		if bFound {
			continue
		}
		log.Info("rule a2", "chunk", v.chunkId)
		return v.chunkId
	}

	// 从规则b找
	if from > claimChunkFindMaxCount {
		from -= claimChunkFindMaxCount
	} else {
		from = 0
	}
	chunkId = getChunkIdB(clist, from, to)
	if chunkId != 0 {
		log.Info("rule b", "chunk", chunkId)
		return chunkId
	}

	// 从规则c找
	chunkId = getChunkIdC(clist)
	log.Info("rule c", "chunk", chunkId)
	return chunkId
}

func getChunkIdB(clist chunkList, from uint64, to uint64) uint64 {
	cl := getChunkList(clist, from, to)
	for _, v := range cl.ccs {
		if len(v.nodes) < blizparam.GetStandardCopyCount() {
			return v.chunkId
		}
	}
	return 0
}

func getChunkIdC(cl chunkList) uint64 {
	len := len(cl.ccs)
	if len == 0 {
		return 0
	}
	from := cl.ccs[0].chunkId
	to := cl.ccs[len-1].chunkId
	return getChunkIdB(cl, from, to)
}

func getChunkList(clist chunkList, from uint64, to uint64) chunkList {
	ccs := make([]*chunkClaim, 0, to-from)
	for _, v := range clist.ccs {
		chunkId := v.chunkId
		if chunkId < from || chunkId >= to {
			continue
		}
		ccs = append(ccs, v)
	}
	cl := chunkList{
		ccs: ccs,
	}
	return cl
}
