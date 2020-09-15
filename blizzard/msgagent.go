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
	"sync"
	"sync/atomic"
	"time"

	//"github.com/contatract/go-contatract/blizzard/storage"
	//"github.com/contatract/go-contatract/elephant/exporter"
	"github.com/contatract/go-contatract/p2p"

	"github.com/contatract/go-contatract/blizcore"
)

// 监听来自其他模块的调用呼叫

var (
	//queryMap 	map[int64] chan *exporter.FarmerRsp
	queryMap map[uint64]*chan interface{} = make(map[uint64]*chan interface{})
	queryTs  map[uint64]int64             = make(map[uint64]int64)
	queryId  uint64

	lock sync.RWMutex
)

func (pm *ProtocolManager) QueryData(partner *partner, Ch *chan interface{}, query blizcore.DataQuery) {
	return
	atomic.AddUint64(&queryId, 1)
	queryMap[queryId] = Ch
	queryTs[queryId] = time.Now().Unix()

	query.SetQuerId(queryId)

	if msgData, ok := query.(*blizcore.GetWriteOpMsgData); ok {
		partner.SendGetWriteOpMsg(msgData)
		pm.Log().Info("QueryData", "msgData", msgData)
	} else if msgData, ok := query.(*blizcore.GetSliceInfoMsgData); ok {
		partner.SendGetSliceInfoMsg(msgData)
		pm.Log().Info("QueryData", "msgData", msgData)
	} else if msgData, ok := query.(*blizcore.GetSegmentDataMsgData); ok {
		partner.SendGetSegmentDataMsg(msgData)
		pm.Log().Info("QueryData", "msgData", msgData)
	}
}

func (pm *ProtocolManager) doQueryMsgRsp(msg *p2p.Msg) {

	switch {
	case msg.Code == WriteOpMsg:
		// 解码消息，获取 queryId 并从 queryMap[queryId] 发回消息
		// 这里我们不收线，刷新 ts 后，后面还可以等待新消息
		var msgdata blizcore.WriteOpMsgData
		if err := msg.Decode(&msgdata); err == nil {
			lock.RLock()
			(*queryMap[msgdata.QueryId]) <- &msgdata
			queryTs[msgdata.QueryId] = time.Now().Unix()
			lock.RUnlock()
		}

	case msg.Code == SliceInfoMsg:
		// 解码消息，获取 queryId 并从 queryMap[queryId] 发回消息
		// 这里我们不收线，刷新 ts 后，后面还可以等待新消息
		var msgdata blizcore.SliceInfoMsgData
		if err := msg.Decode(&msgdata); err == nil {
			lock.RLock()
			(*queryMap[msgdata.QueryId]) <- &msgdata
			queryTs[msgdata.QueryId] = time.Now().Unix()
			lock.RUnlock()
		}

	case msg.Code == SegmentDataMsg:
		// 解码消息，获取 queryId 并从 queryMap[queryId] 发回消息
		// 这里我们不收线，刷新 ts 后，后面还可以等待新消息
		var msgdata blizcore.SegmentDataMsgData
		if err := msg.Decode(&msgdata); err == nil {
			lock.RLock()
			(*queryMap[msgdata.QueryId]) <- &msgdata
			queryTs[msgdata.QueryId] = time.Now().Unix()
			lock.RUnlock()
		}

	}
}

func (pm *ProtocolManager) agentLoop() {

	queryMap = make(map[uint64]*chan interface{}, 0)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 维护一下 queryMap ，对请求超时的予以清除
			now := time.Now().Unix()
			for k, ts := range queryTs {
				if now-ts > 20000 {
					delete(queryMap, k)
					delete(queryTs, k)
				}
			}

		case <-pm.quitSync:
			return
		}
	}
}
