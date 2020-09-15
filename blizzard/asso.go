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
	//"encoding/hex"
	"errors"
	"time"

	//"math/big"
	"sort"
	"sync"

	//"github.com/contatract/go-contatract/log"
	//"github.com/contatract/go-contatract/p2p"

	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/log"
	discover "github.com/contatract/go-contatract/p2p/discover"

	//"github.com/contatract/go-contatract/node"
	storage "github.com/contatract/go-contatract/blizzard/storage"
	"github.com/contatract/go-contatract/blizzard/storagecore"

	//"github.com/contatract/go-contatract/ethdb"
	//"github.com/contatract/go-contatract/rpc"
	//"github.com/contatract/go-contatract/sharding"

	"github.com/contatract/go-contatract/common/queue"
	//"github.com/contatract/go-contatract/elephant/exporter"

	"github.com/contatract/go-contatract/blizcore"
	"github.com/contatract/go-contatract/blizparam"
)

const (
	StandardWopSyncQueueSize   = 32   // see blizparam\paramter.go
	MaxWopSyncQueueSize        = 1024 // 同步wop队列的最大长度（不允许超过，达到就阻塞进一步的写操作）
	StandardWopCacheTimeSecond = 30   // 最近wop驻留内存cache的标秤时间（秒），锁定或者写操作不频繁时可以长时间驻留
	syncGoMaxCnt               = 5    //  Synchrodata goroutine maximum count
)

const (
	fromCache uint8 = iota
	fromFile
)

const (
	fromCacheTimeout uint32 = 5
	fromFileTimeout  uint32 = 30
)

type partnerNodeIdSortList struct {
	nodes []*discover.NodeID
}

func (s partnerNodeIdSortList) Len() int      { return len(s.nodes) }
func (s partnerNodeIdSortList) Swap(i, j int) { s.nodes[i], s.nodes[j] = s.nodes[j], s.nodes[i] }
func (s partnerNodeIdSortList) Less(i, j int) bool {
	if compare(s.nodes[i], s.nodes[j]) >= 0 {
		return false
	}
	return true
}

func compare(si, sj *discover.NodeID) int {
	var k int
	var cnt int

	if len(*si) > len(*sj) {
		cnt = len(*sj)
	} else {
		cnt = len(*si)
	}

	for k = 0; k < cnt; k++ {
		da := (*si)[k]
		db := (*sj)[k]
		if da > db {
			return 1
		} else if da < db {
			return -1
		}
	}
	return 0
}

type SliceWopProgList []*blizcore.SliceWopProg

func (s SliceWopProgList) Len() int { return len(s) }
func (s SliceWopProgList) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s SliceWopProgList) Less(i, j int) bool {
	if s[i].Slice > s[j].Slice {
		return false
	}
	return true
}

type checkHeaderHash struct {
	nodeId     discover.NodeID
	opId       uint64
	headerHash common.Hash
}

type partnerObj struct {
	partner *partner
	slice   uint32
	wopId   uint64
}

type partnerProg struct {
	partner *partner
	wopId   uint64
	from    uint8
	time    uint32
}

type ChunkPartnerAsso struct {
	bliz    *Bliz
	chunkId storage.ChunkIdentity

	partners  map[discover.NodeID]*partner
	refreshTs map[discover.NodeID]int64 // 更新时间戳

	quitChan chan struct{}

	queryBegTs int64

	status   int
	onlineTs int64

	lock sync.RWMutex

	dailstep int

	// data synchronize & data transfer

	// Wop 的同步队列池
	WopSyncQueueLock sync.RWMutex
	WopSyncQueue     queue.SliceFifo

	// 本地该chunk下所有 slice 的进度记录
	WopProgs SliceWopProgList

	syncCheckHeaderHash   map[uint32][]*checkHeaderHash
	syncCheckHeaderHashMu sync.RWMutex

	syncingSliceLock sync.RWMutex
	syncingSlice     map[uint32]*partnerProg
}

func (asso *ChunkPartnerAsso) startPartnerQuery() {
	asso.status = AssoStatusQuerying
	asso.queryBegTs = time.Now().Unix()
	asso.bliz.QueryChunkFarmer(asso.chunkId)
}

func (asso *ChunkPartnerAsso) IsAbandom() bool {
	return (asso.status == AssoStatusAbandom)
}

// asso 状态维护等
func (asso *ChunkPartnerAsso) run() {
	asso.Log().Trace("run", "chunkId", storage.ChunkIdentity2Int(asso.chunkId))
	if asso.status != AssoStatusAbandom {
		asso.lock.RLock()
		cnt := len(asso.partners)
		asso.lock.RUnlock()
		if cnt <= 0 {
			// asso.partners 为空，还不知道同伴信息
			asso.startPartnerQuery()
		}
	}

	switch asso.status {

	case AssoStatusQuerying:
		// 记录一下 query 开始的时间
		if time.Now().Unix()-asso.queryBegTs > AssoQueryTimeout {
			// query 超过3分钟了

			// 让 bliz 将chunk对应分片的矿工缓存做一下超时处理
			// 如果是它让这个过程超时的，则就将缓存置空，重新获取
			asso.bliz.DoChunkShardingNodesTimeout(asso.chunkId)

			// 重新开启一轮
			asso.startPartnerQuery()
		}

	case AssoStatusNormal:
		asso.doNormal()

	case AssoStatusOnline:
		asso.doOnline()

	case AssoStatusExpired:
		asso.startPartnerQuery()

		//asso.status = AssoStatusQuerying
		//asso.bliz.queryShardingNodeChan <- &asso.chunkId

	case AssoStatusAbandom:
	}

	// 检查 farmer 列表，看是否存在超时很久的人选：
	//
	// 每隔一段时间，我们都要通过 Query 去chunk分片矿工处获取该chunk的
	// farmer 列表，获取到之后都会调用 RegisterPartner ，如果已经存在
	// 则刷新 partner 的refreshTs记录，如果某节点跟其他节点相比，
	// 很久都没有被刷新了，说明他可能已经被淘汰出这个chunk的farmer名单

	ourID := discover.PubkeyID(&asso.bliz.nodeserver.PrivateKey.PublicKey)
	maxTs := int64(0)
	for _, ts := range asso.refreshTs {
		if ts > maxTs {
			maxTs = ts
		}
	}

	for id, ts := range asso.refreshTs {
		if maxTs-ts > 3600 {
			// 该 partner 已经过时，它跟其他队友相差了3600秒以上
			if compare(&id, &ourID) == 0 {
				// 过时的是自己，自己已经被该chunk淘汰
				// 退出该chunkasso
				// asso.status = AssoStatusAbandom // qiwy: TODO, for test
				//asso.bliz.chunkdb.DeleteNode()
				break
			}

			// 将淘汰的partner从内存中清除
			// 而该chunk相关的db记录则在此
			// 之前已经在 connLoop 中更新
			// asso.UnRegisterPartner(id) // qiwy: TODO, for test
		}
	}
}

func (asso *ChunkPartnerAsso) getLinkedPartnerCount() int {
	var cnt = 0
	asso.lock.RLock()
	asso.Log().Info("", "partners len", len(asso.partners))
	for nodeId, partner := range asso.partners {
		if partner != nil {
			asso.Log().Info("", "nodeId", nodeId)
			cnt++
		}
	}
	asso.lock.RUnlock()
	return cnt
}

func (asso *ChunkPartnerAsso) doOnline() {
	if time.Now().Unix()-asso.onlineTs > 1200 {
		// 每20分钟设置重新查询一下该chunk的farmer名单
		asso.status = AssoStatusExpired
	}

	if asso.getLinkedPartnerCount() < 4 {
		asso.status = AssoStatusNormal
		asso.onlineTs = time.Now().Unix()
		return
	}
}

func (asso *ChunkPartnerAsso) doNormal() {
	asso.Log().Trace("doNormal", "getLinkedPartnerCount()", asso.getLinkedPartnerCount())
	if asso.getLinkedPartnerCount() >= 4 {
		asso.status = AssoStatusOnline
		asso.onlineTs = time.Now().Unix()
		return
	}

	// 对 partners 根据它们的 id 进行一个排序
	slist := &partnerNodeIdSortList{nodes: make([]*discover.NodeID, 0)}
	asso.lock.RLock()
	for nodeId, _ := range asso.partners {
		//b, _ := hex.DecodeString(partner.id)
		nId := nodeId
		slist.nodes = append(slist.nodes, &nId)
	}
	asso.lock.RUnlock()
	if len(slist.nodes) < ChunkFarmerMin {
		// farmer 数量不足，需要重新query
		asso.status = AssoStatusExpired
		return
	}

	sort.Sort(slist)

	// 排序完毕，首先找到自己
	var idx = 0
	for _, nodeId := range slist.nodes {
		ourID := discover.PubkeyID(&asso.bliz.nodeserver.PrivateKey.PublicKey)
		if compare(nodeId, &ourID) == 0 {
			// 找到了自己的index
			// 看看前后两个节点是否我们已经连接
			var cnt = 0

			// 从 nodes 节点列表中寻找自己相邻的4个节点
			// 10..01x10..01
			// 上图中x表示自己，自己相邻的两个节点一定要dail
			// 然后就是拨号dailstep表示的远端节点，dailstep
			// 根据超时不断向远端推进

			// 找左边
			cnt = 0
			for n := idx - 1; n >= idx-asso.dailstep; n-- {
				if n < 0 {
					break
				}
				if n == idx-1 || n == idx-asso.dailstep {
					if asso.GetPartner(*slist.nodes[n]) != nil || asso.bliz.InDailing(slist.nodes[n]) {
						cnt++
					}
				}
			}
			if cnt < 2 {
				for n := idx - 1; n >= idx-asso.dailstep; n-- {
					if n < 0 {
						break
					}
					if n == idx-1 || n == idx-asso.dailstep {
						if asso.GetPartner(*slist.nodes[n]) == nil && !asso.bliz.InDailing(slist.nodes[n]) {
							asso.bliz.DailFarmer(asso.chunkId, CBModePartner, slist.nodes[n])
							cnt++
							if cnt >= 2 {
								break
							}
						}
					}
				}
			} else {
				// 不管左边的 cnt 是不是已经满足了数量条件，我们都要确保 idx-1 这个元素
				n := idx - 1
				if n >= 0 && asso.GetPartner(*slist.nodes[n]) == nil && !asso.bliz.InDailing(slist.nodes[n]) {
					asso.bliz.DailFarmer(asso.chunkId, CBModePartner, slist.nodes[n])
				}
			}

			// 找右边
			cnt = 0
			for n := idx + 1; n <= idx+asso.dailstep; n++ {
				if n >= len(slist.nodes) {
					break
				}
				if asso.GetPartner(*slist.nodes[n]) != nil || asso.bliz.InDailing(slist.nodes[n]) {
					cnt++
				}
			}
			if cnt < 2 {
				for n := idx + 1; n <= idx+asso.dailstep; n++ {
					if n >= len(slist.nodes) {
						break
					}
					if asso.GetPartner(*slist.nodes[n]) == nil && !asso.bliz.InDailing(slist.nodes[n]) {
						asso.bliz.DailFarmer(asso.chunkId, CBModePartner, slist.nodes[n])
						cnt++
						if cnt >= 2 {
							break
						}
					}
				}
			} else {
				// 不管右边的 cnt 是不是已经满足了数量条件，我们都要确保 idx+1 这个元素
				n := idx + 1
				if n >= 0 && asso.GetPartner(*slist.nodes[n]) == nil && !asso.bliz.InDailing(slist.nodes[n]) {
					asso.bliz.DailFarmer(asso.chunkId, CBModePartner, slist.nodes[n])
				}
			}

			break
		}
		idx++
	}
	if idx >= len(slist.nodes) {
		// 自己不在名单中？？
		// 该asso可以设置为废弃
		// asso.status = AssoStatusAbandom // qiwy: TODO, for test
		return
	}
}

func (asso *ChunkPartnerAsso) LoginPartner(partner *partner) (bool, error) {
	asso.lock.Lock()
	if _, ok := asso.partners[partner.ID()]; ok {
		//存在,表示本地已经确认过该partner在这个chunk的提供者名单中
		asso.partners[partner.ID()] = partner
		asso.lock.Unlock()
		partner.Link2Chunk(asso.chunkId, asso)
		return true, nil
	}
	asso.lock.Unlock()

	return false, blizcore.ErrPartnerNotChunkFarmer
}

func (asso *ChunkPartnerAsso) LogoutPartner(partner *partner) {
	partner.UnLink2Chunk(asso.chunkId)
	asso.lock.Lock()
	if _, ok := asso.partners[partner.ID()]; ok {
		//存在,表示本地已经确认过该partner在这个chunk的提供者名单中
		asso.partners[partner.ID()] = nil
	}
	asso.lock.Unlock()
}

func (asso *ChunkPartnerAsso) AllowPartnerLogin(partner *partner) bool {
	asso.lock.RLock()
	defer asso.lock.RUnlock()

	if _, ok := asso.partners[partner.ID()]; ok {
		//存在,表示本地已经确认过该partner在这个chunk的提供者名单中
		return true
	}
	return false
}

func (asso *ChunkPartnerAsso) GetPartner(nodeId discover.NodeID) *partner {
	asso.lock.RLock()
	defer asso.lock.RUnlock()

	return asso.partners[nodeId]
}

// func (asso *ChunkPartnerAsso) GetPartners() map[discover.NodeID]*partner {
// 	return asso.partners
// }

// 将 partner 的 nodeID 注册进来
func (asso *ChunkPartnerAsso) RegisterPartner(nodeId discover.NodeID, ts int64) bool {
	asso.lock.Lock()
	if _, ok := asso.partners[nodeId]; !ok {
		//不存在,表示本地还没确认过该partner在这个chunk的提供者名单中
		asso.partners[nodeId] = nil
		asso.lock.Unlock()
		asso.refreshTs[nodeId] = ts
		return true
	}
	asso.lock.Unlock()

	//存在,表示本地已经确认过该partner在这个chunk的提供者名单中

	// 每隔一段时间，我们都要通过 Query 去chunk分片矿工处获取该chunk的
	// farmer 列表，获取到之后都会调用 RegisterPartner ，如果已经存在
	// 则刷新 partner 的refreshTs记录，如果某节点跟其他节点相比，
	// 很久都没有被刷新了，说明他可能已经被淘汰出这个chunk的farmer名单
	asso.refreshTs[nodeId] = ts
	return true
}

// 将 partner 的 nodeID 从注册map中消除
func (asso *ChunkPartnerAsso) UnRegisterPartner(nodeId discover.NodeID) bool {
	asso.lock.Lock()
	if _, ok := asso.partners[nodeId]; ok {
		//存在,表示本地已经确认过该partner在这个chunk的提供者名单中
		delete(asso.partners, nodeId)
		asso.lock.Unlock()
		delete(asso.refreshTs, nodeId)
		return true
	}
	asso.lock.Unlock()
	return true
}

func (asso *ChunkPartnerAsso) Start() {
	asso.dailstep = 2
	if asso.status != AssoStatusOnline && asso.status != AssoStatusQuerying {
		asso.startPartnerQuery()
	}
	asso.syncLoop() // qiwy: TODO, 新增chunkId也需要调用
}

// ----------------------- Asso Partner Data(Wop) Synchronize And Data Transfer Implementation --------------------------
type WopSyncData struct {
	wop  *blizcore.WriteOperReqData
	ts   uint64
	lock bool
}

func (asso *ChunkPartnerAsso) PrepareLocalStorage() bool {
	// TODO: 加载 chunk 中的所有 SliceHeader 的WopId 到 asso.WopProgs 中
	// 需要排除掉无人使用的 Slice
	chunkId := storage.ChunkIdentity2Int(asso.chunkId)
	asso.WopProgs = asso.bliz.GetStorageManager().ReadChunkSliceOps(chunkId)
	return true
}

func (asso *ChunkPartnerAsso) CanAddNewWopSyncCache() bool {
	// qiwy: todo
	return true
	// return asso.WopSyncQueue.Size() < MaxWopSyncQueueSize
}

// Add new Wop into sync Queue (wait for broadcast)
func (asso *ChunkPartnerAsso) AddNewWopSyncCache(wop *blizcore.WriteOperReqData) {
	asso.WopSyncQueueLock.Lock()
	defer asso.WopSyncQueueLock.Unlock()

	syncData := &WopSyncData{
		wop:  wop,
		ts:   uint64(time.Now().Unix()),
		lock: false,
	}
	asso.Log().Info("", "Infos len", len(*(wop.Info.Infos)))
	asso.WopSyncQueue.Put(syncData)
	asso.Log().Info("Put", "WopSyncQueue Size", asso.WopSyncQueue.Size())
	asso.UpdateWopProg(wop.Info.Slice, wop.Info.WopId)
}

func (asso *ChunkPartnerAsso) UpdateWopProg(sliceId uint32, wopId uint64) {
	asso.Log().Info("UpdateWopProg", "sliceId", sliceId, "wopId", wopId, "WopProgs len", len(asso.WopProgs))
	bFound := false
	for _, prog := range asso.WopProgs {
		if prog.Slice == sliceId {
			prog.WopId = wopId
			bFound = true
		}
		asso.Log().Trace("UpdateWopProg", "prog", prog)
	}
	if !bFound {
		asso.WopProgs = append(asso.WopProgs, &blizcore.SliceWopProg{Slice: sliceId, WopId: wopId})
	}
}

func (asso *ChunkPartnerAsso) GetWopProg(sliceId uint32) uint64 {
	for _, prog := range asso.WopProgs {
		if prog.Slice == sliceId {
			return prog.WopId
		}
	}
	return 0
}

func (asso *ChunkPartnerAsso) SweepSyncQueue() {
	var k int
	cnt := asso.WopSyncQueue.Size() - StandardWopSyncQueueSize
	now := uint64(time.Now().Unix())

	for k = 0; k < cnt; k++ {
		syncData_ := asso.WopSyncQueue.Peek()
		if syncData_ != nil {
			if syncData, ok := syncData_.(*WopSyncData); ok {
				// 同步 WopId
				//wop.Info.WopId
				if now-syncData.ts > StandardWopCacheTimeSecond && syncData.lock == false {
					asso.WopSyncQueue.Get()
				} else {
					break
				}
			}
		}
	}
}

func (asso *ChunkPartnerAsso) syncLoop() {
	asso.Log().Trace("syncLoop")
	heartbeatticker := time.NewTicker(5 * time.Second)
	defer heartbeatticker.Stop()

	syncDuration := 2 * time.Second
	syncticker := time.NewTicker(syncDuration)
	defer syncticker.Stop()

	for {
		select {
		case <-asso.bliz.shutdownChan:
			return

		case <-heartbeatticker.C:
			// 心跳时间到，需要向其他 farmer partner 发送心跳信息，
			// 告知对方我们的数据进度版本
			asso.Log().Trace("heartbeatticker.C")
			msg := &blizcore.HeartBeatMsgData{
				ChunkId: storage.ChunkIdentity2Int(asso.chunkId),
				Progs:   asso.WopProgs,
				Ts:      uint64(time.Now().Unix()),
			}
			// 本地签名
			err := msg.SignHeartbeat(asso.bliz.nodeserver.PrivateKey)
			if err != nil {
				asso.Log().Error("SignHeartbeat", "err", err, "PrivateKey", asso.bliz.nodeserver.PrivateKey)
				break
			}
			// 标记接收
			asso.bliz.MarkHeartBeat(msg)
			asso.Log().Trace("", "ChunkId", msg.ChunkId)
			for _, prog := range msg.Progs {
				if prog.WopId != 0 {
					asso.Log().Trace("", "prog", prog)
				}
			}
			// 广播给所有连接的 partner
			asso.lock.RLock()
			asso.Log().Info("", "partner len", len(asso.partners))
			for nodeId, partner := range asso.partners {
				asso.Log().Info("", "nodeId", nodeId)
				if partner != nil {
					asso.Log().Trace("partners")
					err := partner.SendHeartBeatMsg(msg)
					if err != nil {
						asso.Log().Error("SendHeartBeatMsg", "err", err)
					}
				}
			}
			asso.lock.RUnlock()

			// 判断队列中的数量是否超标,清除掉超标部分
			// 如果被锁定则先不清除

		case <-syncticker.C:
			asso.Log().Info("")
			objs := make([]*partnerObj, 0)
			// 同步时间到，对比 asso 中所有 partner 的进度，挑选同步对象
			asso.lock.RLock()
			for nodeId, part := range asso.partners {
				if part == nil {
					asso.Log().Info("part nil", "nodeId", nodeId)
					continue
				}
				asso.Log().Info("", "nodeId", nodeId)
				// 这里我们还没有考虑有关 WopId 回绕的问题，这个问题需要再研究一下如何解决
				progs, ok := part.chunkWopProgs[asso.chunkId.String()]
				if !ok {
					continue
				}
				asso.Log().Trace("range", "progs len", len(progs))
				// 对方料子
				for i, progr := range progs {
					asso.Log().Trace("range", "i", i, "progr", progr)
					var progL *blizcore.SliceWopProg
					// 本地料子
					for j, prog2 := range asso.WopProgs {
						asso.Log().Trace("range", "j", j, "prog2", prog2)
						if prog2.Slice == progr.Slice {
							progL = prog2
							break
						}
					}
					if progL == nil {
						// 本地还没有数据，建立一个
						progL = &blizcore.SliceWopProg{Slice: progr.Slice, WopId: 0}
						asso.WopProgs = append(asso.WopProgs, progL)
						// 排序
						sort.Sort(asso.WopProgs)
					}
					if progr.WopId != progL.WopId {
						asso.Log().Info("Sync loop", "slice", progL.Slice, "progr.WopId", progr.WopId, "progL.WopId", progL.WopId)
					}
					if progr.WopId <= progL.WopId {
						continue
					}

					var find bool
					obj := &partnerObj{partner: part, slice: progr.Slice, wopId: progr.WopId}
					for i, v := range objs {
						if v.slice != obj.slice {
							continue
						}
						find = true
						if v.wopId < obj.wopId {
							objs[i] = obj
						}
						break
					}
					if !find {
						objs = append(objs, obj)
					}
				}
			}
			asso.lock.RUnlock()
			asso.syncSliceData(objs)

		case <-asso.quitChan:
			return
		}
	}
}

func (asso *ChunkPartnerAsso) syncSliceData(objs []*partnerObj) {
	now := uint32(time.Now().Unix())
	asso.syncingSliceLock.Lock()
	defer asso.syncingSliceLock.Unlock()

	for i, v := range asso.syncingSlice {
		if v.from == fromCache {
			if v.time+fromCacheTimeout < now {
				v.from = fromFile
				v.time = now
				go asso.syncFromFile(v.partner, i)
			}
		} else {
			if v.time+fromFileTimeout < now {
				delete(asso.syncingSlice, i)
			}
		}
	}

	for _, obj := range objs {
		if len(asso.syncingSlice) >= syncGoMaxCnt {
			break
		}
		var find bool
		for i, _ := range asso.syncingSlice {
			if obj.slice == i {
				find = true
				break
			}
		}
		if find {
			continue
		}
		asso.syncingSlice[obj.slice] = &partnerProg{
			partner: obj.partner,
			wopId:   obj.wopId,
			from:    fromCache,
			time:    uint32(time.Now().Unix()),
		}
		go asso.syncFromCache(obj)
	}
	for i, v := range asso.syncingSlice {
		asso.Log().Info("Syncing slice list", "slice", i, "wopid", v.wopId, "from", v.from)
	}
}

func (asso *ChunkPartnerAsso) syncFromCache(obj *partnerObj) {
	slice := obj.slice
	remoteWop := obj.wopId
	localWop := asso.getLocalWop(slice)
	if localWop >= remoteWop {
		return
	}
	chunk := storage.ChunkIdentity2Int(asso.chunkId)
	cnt := uint32(remoteWop - localWop)
	asso.GetWopData2(obj.partner, chunk, slice, localWop+1, cnt)
}

func (asso *ChunkPartnerAsso) getLocalWop(slice uint32) uint64 {
	for _, prog := range asso.WopProgs {
		if prog.Slice == slice {
			return prog.WopId
		}
	}
	return 0
}

func (asso *ChunkPartnerAsso) GetWopData(partner *partner, chunk uint64, slice uint32, fromWopId uint64, count uint32) {

	// 从己方的最后一个被确认的 wopId 开始请求对方的所有 wopIds
	// 走 GetWriteOpMsg 消息
	var recvCh chan interface{}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	msgData := &blizcore.GetWriteOpMsgData{
		Chunk:     chunk,
		Slice:     slice,
		FromWopId: fromWopId,
		Count:     count,
	}
	asso.Log().Info("GetWopData", "msgData", msgData)
	asso.bliz.protocolManager.QueryData(partner, &recvCh, msgData)

	var desProg *blizcore.SliceWopProg
	asso.Log().Info("GetWopData", "WopProgs len", len(asso.WopProgs))
	for _, progl := range asso.WopProgs {
		asso.Log().Info("GetWopData", "progl", progl)
		if progl.Slice == slice {
			desProg = progl
		}
	}

	if desProg == nil {
		return
	}

	lastReceive := time.Now().Unix()

_work:
	// 一个获取，多个获得
	for {
		select {
		case data := <-recvCh:
			lastReceive = time.Now().Unix()
			if msgdata, ok := data.(*blizcore.WriteOpMsgData); ok {
				asso.Log().Info("GetWopData", "msgdata", msgdata)
				// 消息回来了,当收齐了所有（Count个） Wop 后，完成当前任务
				// 推进本地的 WopProgs[slice] 进度

				// 验证消息
				if msgdata.Info.Chunk == chunk ||
					msgdata.Info.Slice == slice ||
					msgdata.Info.WopId == desProg.WopId+1 {

					// 消息符合要求

				}
			}
		case <-ticker.C:
			// 每2秒钟检查一下消息是否回来
			now := time.Now().Unix()
			asso.Log().Info("GetWopData", "now", now, "lastReceive", lastReceive)
			if now-lastReceive > 5 {
				// 放弃本次行动
				break _work
			}
		}
	}
}

func (asso *ChunkPartnerAsso) GetWopData2(partner *partner, chunk uint64, slice uint32, fromWopId uint64, count uint32) {
	msgData := &blizcore.GetWriteOpMsgData{
		Chunk:     chunk,
		Slice:     slice,
		FromWopId: fromWopId,
		Count:     count,
	}
	asso.Log().Info("GetWopData2", "msgData", msgData)
	err := partner.SendGetWriteOpMsg(msgData)
	if err != nil {
		asso.Log().Error("GetWopData2", "err", err)
		//return
	}
}

func (asso *ChunkPartnerAsso) Synchronize(partner *partner, chunk uint64, slice uint32) {
	return
	var recvCh chan interface{}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// [开始整体同步]

	// 1. 首先向对方请求Slice它当前的所有Segment的hash列表和跟WopId绑定的根hash（被tenant签名）
	// 	  走 GetSliceInfoMsg 消息
	msgData := &blizcore.GetSliceInfoMsgData{
		Chunk: chunk,
		Slice: slice,
	}
	asso.Log().Info("Synchronize", "msgData", msgData)
	asso.bliz.protocolManager.QueryData(partner, &recvCh, msgData)

	lastReceive := time.Now().Unix()

	var sliceInfoMsgData *blizcore.SliceInfoMsgData

_step1:
	// 一个获取，多个获得
	for {
		select {
		case data := <-recvCh:
			lastReceive = time.Now().Unix()
			if sliceInfoMsgData, ok := data.(*blizcore.SliceInfoMsgData); ok {
				asso.Log().Info("Synchronize", "sliceInfoMsgData", sliceInfoMsgData)
				// 消息回来了,当收齐了所有（Count个） Wop 后，完成当前任务
				// 推进本地的 WopProgs[slice] 进度

				// 验证消息
				if sliceInfoMsgData.Chunk == chunk ||
					sliceInfoMsgData.Slice == slice {
					// 消息符合要求,跳出当前循环
					break _step1
				}
			}
		case <-ticker.C:
			// 每2秒钟检查一下消息是否回来
			now := time.Now().Unix()
			asso.Log().Info("Synchronize", "now", now, "lastReceive", lastReceive)
			if now-lastReceive > 8 {
				// 放弃本次行动,返回
				goto _exit
			}
		}
	}

	// 2. 核实数据准确后，使用 sliceInfoMsgData 开始逐一对比本地segment hash，不一致则启动下载，并将我们的同步目标 WopId 记录下来
	// 	  走 GetSegmentDataMsg 消息
	if err := sliceInfoMsgData.Verify(); err != nil {
		asso.Log().Error("Synchronize", "err", err)
		// 签名校验失败,放弃本次行动,返回
		goto _exit
	}
	//for sliceInfoMsgData.SegmentHashs

	// 3. 下载完成，在 asso.syncing 期间，对方可能会向我们传递新的 Wop 包，我们对比该 Wop 和我们当前正在同步的 WopId 进度
	//    如果该 Wop 更加新，于是我们直接 Apply 它们到我们的数据上，以防止我们同步过程中已经错过相应的数据变化
	// 	  所以当我们下载完毕时，我们需要根据下载过程中，接收并应用的最新 WopId 向对方请求（或者等待对方传递）对应该 WopId
	// 	  的用户对 slice hash 的签名消息，可以仍旧走 GetSliceInfoMsg 消息
	asso.bliz.protocolManager.QueryData(partner, &recvCh, msgData)

	lastReceive = time.Now().Unix()

_step3:
	// 一个获取，多个获得
	for {
		select {
		case data := <-recvCh:
			lastReceive = time.Now().Unix()
			if sliceInfoMsgData, ok := data.(*blizcore.SliceInfoMsgData); ok {
				asso.Log().Info("Synchronize _step3", "sliceInfoMsgData", sliceInfoMsgData)
				// 消息回来了,当收齐了所有（Count个） Wop 后，完成当前任务
				// 推进本地的 WopProgs[slice] 进度

				// 验证消息
				if sliceInfoMsgData.Chunk == chunk ||
					sliceInfoMsgData.Slice == slice {
					// 消息符合要求,跳出当前循环
					break _step3
				}
			}
		case <-ticker.C:
			// 每2秒钟检查一下消息是否回来
			now := time.Now().Unix()
			asso.Log().Info("Synchronize _step3", "now", now, "lastReceive", lastReceive)
			if now-lastReceive > 8 {
				// 放弃本次行动,返回
				goto _exit
			}
		}
	}

_exit:
}

// 先简单实现，暂时不考虑作弊
func (asso *ChunkPartnerAsso) syncFromFile(partner *partner, slice uint32) {
	msgData := &blizcore.GetWriteDataMsgData{
		Chunk: storage.ChunkIdentity2Int(asso.chunkId),
		Slice: slice,
	}
	asso.Log().Info("Sync from file", "msgdata", msgData)
	err := partner.SendGetWriteDataMsg(msgData)
	if err != nil {
		asso.Log().Error("Sync from file", "err", err)
		//return
	}
}

func (asso *ChunkPartnerAsso) GetAllWriteOpDataFromQueue(req *blizcore.GetWriteOpMsgData) ([]*blizcore.WriteOpMsgData, error) {
	from := req.FromWopId
	cnt := uint64(req.Count)
	ret := make([]*blizcore.WriteOpMsgData, 0, cnt)
	for i := uint64(0); i < cnt; i++ {
		data, err := asso.GetWriteOpDataFromQueue(req, from+i)
		if err != nil {
			return ret, nil // The previous data is valid
		}
		ret = append(ret, data)
	}
	return ret, nil
}

func (asso *ChunkPartnerAsso) GetWriteOpDataFromQueue(req *blizcore.GetWriteOpMsgData, wopId uint64) (*blizcore.WriteOpMsgData, error) {
	asso.WopSyncQueueLock.Lock()
	defer asso.WopSyncQueueLock.Unlock()

	resData := &blizcore.WriteOpMsgData{
		QueryId: req.QueryId,
	}

	size := uint32(asso.WopSyncQueue.Size())
	for i := uint32(0); i < size; i++ {
		data := asso.WopSyncQueue.GetData(uint32(i))
		if data == nil {
			break
		}
		syncData := data.(*WopSyncData)
		if syncData == nil {
			break
		}
		asso.Log().Info("GetWriteOpDataFromQueue", "Slice", syncData.wop.Info.Slice, "WopId", syncData.wop.Info.WopId)
		if syncData.wop.Info.WopId == wopId {
			resData.Info = syncData.wop.Info
			resData.Datas = syncData.wop.Datas
			resData.Sign = syncData.wop.Sign
			asso.Log().Info("", "Infos len", len(*(resData.Info.Infos)))
			return resData, nil
		}
	}

	return resData, errors.New("not find")
}

func (asso *ChunkPartnerAsso) GetWriteDataFromStorage(req *blizcore.GetWriteDataMsgData) (*storagecore.SliceHeader, []byte, error) {
	asso.WopSyncQueueLock.Lock()
	defer asso.WopSyncQueueLock.Unlock()

	chunkId := req.Chunk
	sliceId := req.Slice
	header := asso.bliz.GetStorageManager().ReadChunkSliceHeader(chunkId, sliceId)
	if header == nil {
		return nil, nil, errors.New("read slice header error")
	}
	data, err := asso.bliz.GetStorageManager().ReadRawData(chunkId, sliceId, 0, storage.SliceSize)
	if err != nil {
		return nil, nil, err
	}
	return header, data, nil
}

func (asso *ChunkPartnerAsso) SynchronizeWrite(data *blizcore.WriteDataMsgData, sg *storage.Manager) error {
	header := data.Header
	sg.WriteSliceHeaderData(header)
	err := asso.bliz.GetStorageManager().WriteRawData(header.ChunkId, header.SliceId, 0, data.Data, header.OpId, header.OwnerId)
	if err != nil {
		asso.Log().Error("Synchronize write", "err", err)
		return err
	}
	asso.UpdateWopProg(header.SliceId, header.OpId)
	return nil
}

func (asso *ChunkPartnerAsso) UpdateCheckHeaderHash(sliceId uint32, headerHash common.Hash, nodeId discover.NodeID, opId uint64) error {
	asso.syncCheckHeaderHashMu.Lock()
	defer asso.syncCheckHeaderHashMu.Unlock()

	checkHash := &checkHeaderHash{
		nodeId:     nodeId,
		headerHash: headerHash,
		opId:       opId,
	}
	v, ok := asso.syncCheckHeaderHash[sliceId]
	if !ok {
		asso.syncCheckHeaderHash[sliceId] = []*checkHeaderHash{checkHash}
		return nil
	}
	for _, h := range v {
		if !nodeId.Equal(h.nodeId) {
			continue
		}
		// 更新
		if opId > h.opId {
			h.headerHash = headerHash
			h.opId = opId
		}
		return nil
	}
	// 新增加
	asso.syncCheckHeaderHash[sliceId] = append(v, checkHash)
	return nil
}

func (asso *ChunkPartnerAsso) CheckHeaderHashPass(sliceId uint32, nodeId discover.NodeID) bool {
	times := blizparam.GetSyncCheckTimes()
	if times == 1 {
		return true
	}

	asso.syncCheckHeaderHashMu.RLock()
	defer asso.syncCheckHeaderHashMu.RUnlock()

	v, ok := asso.syncCheckHeaderHash[sliceId]
	if !ok {
		return false
	}
	index := -1
	for i, h := range v {
		if h.nodeId == nodeId {
			index = i
			break
		}
	}
	if index == -1 {
		return false
	}

	var maxOpId uint64
	for _, h := range v {
		if h.opId > maxOpId {
			maxOpId = h.opId
		}
	}
	self := v[index]
	if self.opId != maxOpId {
		return false
	}

	var cnt int
	for _, h := range v {
		if h.opId != self.opId {
			continue
		}
		if !h.headerHash.Equal(self.headerHash) {
			continue
		}
		cnt++
	}
	if cnt < times {
		return false
	}

	return true
}

func (asso *ChunkPartnerAsso) SendSynchronizeMsg(sliceId uint32, sync discover.NodeID) {
	asso.Log().Info("SendSynchronizeMsg", "sliceId", sliceId)
	// asso.lock.RLock() // Having already used lock outside of this function
	for nodeId, part := range asso.partners {
		if part == nil {
			continue
		}
		asso.Log().Info("", "nodeId", nodeId, "part", part)
		progs, ok := part.chunkWopProgs[asso.chunkId.String()]
		if !ok {
			asso.Log().Info("", "chunkId", asso.chunkId)
			continue
		}
		for _, progr := range progs {
			if progr.Slice != sliceId {
				continue
			}
			ret := asso.IsCheckHeaderHash(sliceId, progr.WopId, part.ID())
			if ret {
				continue
			}
			ret = asso.CheckHeaderHashPass(sliceId, sync)
			if ret {
				break
			}
			// 请求partner HeaderHash
			req := &blizcore.GetHeaderHashMsgData{Chunk: storage.ChunkIdentity2Int(asso.chunkId), Slice: sliceId}
			err := part.SendGetHeaderHashMsg(req)
			if err != nil {
				asso.Log().Error("SendGetHeaderHashMsg", "err", err)
			}
		}
	}
	// asso.lock.RUnlock()
}

func (asso *ChunkPartnerAsso) IsCheckHeaderHash(sliceId uint32, opId uint64, nodeId discover.NodeID) bool {
	asso.syncCheckHeaderHashMu.RLock()
	defer asso.syncCheckHeaderHashMu.RUnlock()

	v, ok := asso.syncCheckHeaderHash[sliceId]
	if !ok {
		return false
	}
	for _, h := range v {
		if h.nodeId == nodeId && h.opId == opId {
			return true
		}
	}
	return false
}

func (asso *ChunkPartnerAsso) Log() log.Logger {
	return asso.bliz.Log()
}

func (asso *ChunkPartnerAsso) deleteSyncingSlice(partner *partner, slice uint32) {
	asso.syncingSliceLock.Lock()
	defer asso.syncingSliceLock.Unlock()

	for i, v := range asso.syncingSlice {
		if v.partner == partner && i == slice {
			delete(asso.syncingSlice, slice)
			break
		}
	}
	for i, v := range asso.syncingSlice {
		asso.Log().Info("Syncing slice list after deleting", "slice", i, "wopid", v.wopId, "from", v.from)
	}
}
