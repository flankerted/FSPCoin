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

package elephant

import (
	"time"

	"github.com/contatract/go-contatract/blizcore"
	"github.com/contatract/go-contatract/blizzard/storage"
	"github.com/contatract/go-contatract/elephant/exporter"
	"github.com/contatract/go-contatract/event"
	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/p2p"
)

// 监听来自其他模块的调用呼叫
var (
	//queryMap 	map[int64] chan *exporter.FarmerRsp
	queryMap map[uint64]interface{}
	queryTs  map[uint64]int64
	queryId  uint64
)

func (pm *ProtocolManager) exporterLoop() {

	queryMap = make(map[uint64]interface{}, 0)
	queryTs = make(map[uint64]int64)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 维护一下 queryMap, 对于超时的消息予以清除
			now := time.Now().Unix()
			for k, ts := range queryTs {
				if now-ts > 20000 {
					delete(queryMap, k)
					delete(queryTs, k)
				}
			}

		case query := <-pm.ethHeightCh:
			peers := pm.peers.Peers()
			for _, p := range peers {
				p.SendEthHeight(&query.EthHeight)
			}
		case <-pm.ethHeightSub.Err():
			return

		case query := <-pm.eleHeightCh:
			peers := pm.peers.Peers()
			for _, p := range peers {
				p.SendEleHeight(&query.EleHeight)
			}
		case <-pm.eleHeightSub.Err():
			return

		case query := <-pm.claimFileCh:
			pm.elephant.storageManager.CheckClaimFile(query)

		case <-pm.claimFileSub.Err():
			return

		case <-pm.claimFileEndCh:

		case <-pm.claimFileSub.Err():
			return

		case <-pm.quitSync:
			return
		}
	}
}

func (pm *ProtocolManager) doFarmerMsg(farmerInfo *storage.FarmerData) {
	if rspFeed, ok := queryMap[farmerInfo.QueryId]; ok {
		req := &exporter.FarmerRsp{Farmers: &farmerInfo.Nodes}
		rspFeed.(*event.Feed).Send(req)
	}
}

func (pm *ProtocolManager) doObjectMetaMsg(data *storage.ObjectMetaData) {
	// log.Info("doObjectMetaMsg", "data.Data.QueryId", data.Data.QueryId, "queryMap len", len(queryMap))
	if rspChan, ok := queryMap[data.Data.QueryId]; ok {
		log.Info("")
		slices := make([]blizcore.SliceInfo, len(data.Slices))
		log.Info("doObjectMetaMsg", "data.Slices len", len(data.Slices))
		for i, sc := range data.Slices {
			slices[i].ChunkId = sc.ChunkId
			slices[i].SliceId = sc.SliceId
			// log.Info("doObjectMetaMsg", "i", i, "slices[i]", slices[i])
		}
		meta := blizcore.BlizObjectMeta{
			Owner:      data.Data.Address,
			SliceInfos: slices,
		}
		req := &exporter.ObjMetaRsp{Meta: meta}
		rspChan.(*event.Feed).Send(req)
	}
}

// SendNodeDataRLP sends a batch of arbitrary internal data, corresponding to the
// hashes requested.
func (p *peer) SendBlizObjectMetaQuery(query *storage.GetObjectMetaData) error {
	return p2p.Send(p.rw, GetBlizObjectMetaMsg, query)
	//return  nil
}

// func (p *peer) SendPbftRsp(query *exporter.ObjPbftToElephantData) error {
// 	return p2p.Send(p.rw, PbftQueryMsg, query)
// }

func (p *peer) SendEthHeight(data *uint64) error {
	return p2p.Send(p.rw, EthHeightMsg, data)
}

func (p *peer) SendEleHeight(data *uint64) error {
	return p2p.Send(p.rw, EleHeightMsg, data)
}
