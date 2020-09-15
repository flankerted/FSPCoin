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

package blizcs

import (
	//"errors"
	//"fmt"
	//"net"
	//"strconv"
	//"time"
	//"sync"
	"errors"
	"fmt"
	"math/big"

	//"github.com/contatract/go-contatract/log"

	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/elephant/exporter"
	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/p2p"
	"github.com/contatract/go-contatract/rlp"

	//storage"github.com/contatract/go-contatract/blizzard/storage"
	//"github.com/contatract/go-contatract/ethdb"
	"github.com/contatract/go-contatract/blizcore"
	"github.com/contatract/go-contatract/blizzard/storage"
	"github.com/contatract/go-contatract/core_elephant/state"
)

// 操作产生的segment更新的内存记录
// 就是将 protocol.WriteOperReqData中的
// 非Datas部分去除，其他部分作为一个记录
// 存放于 SliceHeader 中
type SliceModifyRec struct {
	WopId uint64
	Ts    uint64      // TimeStamp
	Hash  common.Hash // 产生该 Rec 的 Wop Request 消息中的数据hash
	Infos []blizcore.WriteDataInfoS

	// Signature values( for Id & Hash & Infos)
	V *big.Int //`json:"v" gencodec:"required"`
	R *big.Int //`json:"r" gencodec:"required"`
	S *big.Int //`json:"s" gencodec:"required"`
}

// 对比内容的不同分成两级
// 0. 获取所有slice header，
// 1. 对比 Slice Header 中的最近更新的 wop流水号，由 tenant 签名
// 2. 如果发现wop流水号比本地的新，则获取该slice的所有segment更新流水号
// 3. 逐一对比segment的流水号，发现更新的就下载对应segment
//

// 元数据逻辑设计
// 客户端保存每个 chunk:slice 分片的最近更新信息，保存最近更新的 wopId
// 如果没有保存，则需要对该 chunk:slice 的所有 farmer 进行轮询比对，选出
// 最大的 wopId ，才能进行进一步的写入操作

// 如果客户端保存了 Chunk:slice 的元数据信息，但进行写入操作时，farmer发现
// 客户端上传的 WopId < 自己保存的最近 WopId，则可以拒绝写入，并反馈错误
// 这时客户端需要对该chunk:slice的所有 farmer 进行轮询比对，确定下一步的
// WopId

// 每个 slice 的元数据都封装在 sliceheader 中。
// 它的内容是整体更新的，所以能够对不同矿工存储的元数据内容进行hash比对
// tenant 每次连接2个以上的famrer, 对比它们的 sliceheader,取wop流水号
// 较大的那个

func (pm *ProtocolManager) doWriteReq(p *Peer, compressBytes []byte) (*blizcore.WriteOperRspData, error) {
	uncompressBytes, err := common.UncompressBytes(compressBytes)
	if err != nil {
		return nil, err
	}
	var req blizcore.WriteOperReqData
	if err := rlp.DecodeBytes(uncompressBytes, &req); err != nil {
		return nil, fmt.Errorf("receive wop err: %v", err)
	}

	rsp := &blizcore.WriteOperRspData{Chunk: req.Info.Chunk, Slice: req.Info.Slice, Id: req.Info.WopId, Status: 1, Type: req.Info.Type, ObjUId: req.ObjUId, Infos: req.Info.Infos}
	bChecked := false
	if req.Info.Type == 1 {
		bChecked = true
	}

	if bChecked {
		// Get the data from cache to avoid network transmission back and forth
		infos := req.Info.Infos
		h := (*infos)[0].Hash
		data := pm.getCacheData(p, h)
		if data == nil {
			return rsp, err
		}
		req.Datas = data
		pm.deleteCacheData(p, h)
	}
	//bliz := blizzard.GetBliz()

	// 1.判断 Chunk:Slice 的合法性（是否属于本farmer管辖)
	chunkId := storage.Int2ChunkIdentity(req.Info.Chunk)
	if pm.blizcs.bliz.AmiLocalFarmer(chunkId) == false {
		log.Error("doWriteReq AmiLocalFarmer false", "chunkId", chunkId)
		return nil, blizcore.ErrLocalNotChunkFarmer
	}

	// 2.从本地对应 Chunk:Slice 中读取 SliceHeader, 并判断该 WopId 是否允许应用
	header := pm.blizcs.GetStorageManager().ReadChunkSliceHeader(req.Info.Chunk, req.Info.Slice)
	if header == nil {
		log.Error("doWriteReq ReadChunkSliceHeader nil")
		return rsp, blizcore.ErrSliceHeader
	}
	log.Trace("doWriteReq", "req.Info.WopId", req.Info.WopId, "header.OpId", header.OpId, "bChecked", bChecked)
	//if (!bChecked && req.Info.WopId != header.OpId+1) || (bChecked && req.Info.WopId != header.OpId) {
	if (!bChecked && req.Info.WopId < header.OpId+1) || (bChecked && req.Info.WopId < header.OpId) { // qiwy: TODO, for test
		log.Error("doWriteReq", "req.Info.WopId", req.Info.WopId, "header.OpId", header.OpId, "bChecked", bChecked)
		// 不合法，反馈回复
		//return rsp, blizcore.ErrWopIdNotValid
	}

	// 3.计算数据hash
	// 4.使用hash,infos,Id,Ts,Chunk:Slice等数据验证消息中的签名
	if err := req.AuthVerifySeal(header.OwnerId); err != nil {
		log.Error("Auth verify seal", "err", err)
		// return rsp, err
	}

	// 5.验证通过后，我们判断一下该 Wop 是否能够被直接应用，如果不是则需要缓存
	// if req.Info.WopId != header.OpId+1 {
	// 这里还有中间的一些 wop 没有收到，不能
	// 直接使用这个 wop 可以先缓存到一个“未使用池”(Waiting Pool)
	// CacheWop(req)

	// 我们这里先采用简单做法，直接反馈失败，不接纳它
	//return rsp, blizcore.ErrWopIdNotValid // qiwy: for test, TODO
	// }

	// 6.判断本地的同步缓存队列是否已经满员，这里只判断，到第7步添加
	if pm.blizcs.bliz.CanPutIntoSyncPool(&req) == false {
		log.Error("doWriteReq CanPutIntoSyncPool false", "chunkId", chunkId)
		return rsp, blizcore.ErrWouldBlock
	}

	if !bChecked {
		// 7.逐一写入 Datas 中的各个数据段落到本地存储 storage
		pointer := 0

		for _, info := range *req.Info.Infos {
			pm.Log().Info("doWriteReq", "Datas[pointer] len", len(req.Datas[pointer]))
			err := pm.blizcs.GetStorageManager().WriteChunkSlice(req.Info.Chunk, req.Info.Slice, info.Offset, req.Datas[pointer], req.Info.WopId, header.OwnerId)
			if err != nil {
				log.Error("WriteChunkSlice", "err", err)
			}
			pointer++

			// 此循环暂时只有1次
			// 暂时从磁盘读取(TODO))
			hash, err := pm.blizcs.GetStorageManager().GetHeaderHash(req.Info.Chunk, req.Info.Slice)
			if err != nil {
				return rsp, blizcore.ErrHash
			}
			rsp.HeaderHash = hash

			pm.putCacheData(p, req.Datas)
		}
	} else {
		pm.blizcs.GetStorageManager().SignChunkSlice(req.Info.Chunk, req.Info.Slice, req.Sign)
		// 7. 传递该 Wop 给其他 partner
		pm.Log().Info("", "Infos len", len(*(req.Info.Infos)))
		pm.blizcs.bliz.PutIntoSyncPool(&req)
	}
	return rsp, nil
}

func (pm *ProtocolManager) doWriteRes(p *Peer, msg *p2p.Msg) error {
	var req blizcore.WriteOperRspData
	err := msg.Decode(&req)
	if err != nil {
		log.Error("doReadReq", "err", err)
		return err
	}
	bChecked := false
	if req.Type == 1 {
		bChecked = true
	}
	if !bChecked {
		go func() {
			v, ok := pm.blizcs.objectMap.Load(req.ObjUId)
			if !ok {
				return
			}
			clnt := v.(*BlizClntObject)
			clnt.sliceWriteFeed.Send(&req)
		}()
	} else {
		// 在本地储存data hash
		// pm.blizcs.GetStorageManager().UpdateChunkSliceDataHash(req.Chunk, req.Slice, req.DataHash)
	}
	return nil
}

func (pm *ProtocolManager) doReadReq(p *Peer, compressBytes []byte) (*blizcore.ReadOperRspData, error) {
	// （上层APP）需要同步到整个 slice 的 segment 写入进度，
	// 这样才知道主要跟哪个 farmer 通讯.
	// 我们这里只负责提供本地数据给到对方
	uncompressBytes, err := common.UncompressBytes(compressBytes)
	if err != nil {
		return nil, err
	}
	var req blizcore.ReadOperReqData
	if err := rlp.DecodeBytes(uncompressBytes, &req); err != nil {
		return nil, fmt.Errorf("do read req err: %v", err)
	}
	rsp := &blizcore.ReadOperRspData{ReadOperReqData: req}

	// 1.判断 Chunk:Slice 的合法性（是否属于本farmer管辖)
	chunkId := storage.Int2ChunkIdentity(req.Chunk)
	if pm.blizcs.bliz.AmiLocalFarmer(chunkId) == false {
		log.Error("doReadReq AmiLocalFarmer false", "chunkId", chunkId)
		return rsp, blizcore.ErrLocalNotChunkFarmer
	}

	// 2.从本地对应 Chunk:Slice 中读取 SliceHeader, 并判断该 WopId 是否允许应用
	header := pm.blizcs.GetStorageManager().ReadChunkSliceHeader(req.Chunk, req.Slice)
	if req.Id != header.OpId {
		log.Error("doReadReq", "req.Id", req.Id, "header.OpId", header.OpId)
		// 不合法，反馈回复
		return rsp, blizcore.ErrWopIdNotValid
	}

	var data []byte
	if req.Type == blizcore.ReqHash {
		data, err = pm.blizcs.GetStorageManager().ReadChunkSliceSegmentHash(req.Chunk, req.Slice, req.Offset, req.Length)
	} else {
		data, err = pm.blizcs.GetStorageManager().ReadChunkSliceSegmentData(req.Chunk, req.Slice, req.Offset, req.Length)
	}
	pm.Log().Info("doReadReq", "data len", len(data), "err", err, "req.Type", req.Type)
	if err != nil {
		return rsp, err
	}
	rsp.Data = data
	return rsp, nil
}

func (pm *ProtocolManager) doReadRes(p *Peer, msg *p2p.Msg) error {
	var compressBytes []byte
	if err := msg.Decode(&compressBytes); err != nil {
		log.Error("Do read res", "err", err)
		return err
	}
	uncompressBytes, err := common.UncompressBytes(compressBytes)
	if err != nil {
		return err
	}
	var req blizcore.ReadOperRspData
	if err := rlp.DecodeBytes(uncompressBytes, &req); err != nil {
		return fmt.Errorf("Do read res, err: %v", err)
	}

	go func() {
		v, ok := pm.blizcs.objectMap.Load(req.ReadOperReqData.ObjId)
		if !ok {
			return
		}
		clnt := v.(*BlizClntObject)
		clnt.sliceReadFeed.Send(&req)
	}()
	if len(req.ErrBytes) != 0 {
		err := errors.New(string(req.ErrBytes))
		log.Error("Do read res", "err", err)
		return err
	}
	return nil
}

func (pm *ProtocolManager) doFarmerSliceOpReq(p *Peer, req *blizcore.FarmerSliceOpReq) error {
	farmerSliceOps := pm.blizcs.GetFarmersSliceOps(req.SliceInfos)
	rsp := &blizcore.FarmerSliceOpRsp{ObjUId: req.ObjUId, FarmerSliceOps: farmerSliceOps, NodeId: p.ID()}
	return p.SendFarmerSliceOpRsp(rsp)
}

func (pm *ProtocolManager) doFarmerSliceOpRsp(p *Peer, msg *p2p.Msg) error {
	var req blizcore.FarmerSliceOpRsp
	if err := msg.Decode(&req); err != nil {
		log.Error("doFarmerSliceOpRsp", "err", err)
		return err
	}
	pm.blizcs.GetFarmersSliceOpsRsp(req.ObjUId, req.FarmerSliceOps, p.ID())
	return nil
}

func (pm *ProtocolManager) doSliceMetaFromFarmerRsp(p *Peer, req *blizcore.GetSliceMetaFromFarmerReqData) error {
	pm.blizcs.GetStorageManager().DoSliceMeta(req.Address)
	return nil
}

func (pm *ProtocolManager) doGetBuffFromSharerCS(p *Peer, req *blizcore.GetBuffFromSharerCSData) error {
	pm.Log().Info("Do get buff from sharer cs", "req", req)
	clnt, err := pm.blizcs.GetObject(req.ObjId, common.HexToAddress(req.Sharer))
	if err != nil {
		pm.Log().Error("clnt nil")
		return err
	}
	clnt.SetSignPeer(req.SigPeerId)
	clnt.DoSeek(req.Offset)
	buff := make([]byte, req.Len)
	err = clnt.Read(buff)
	len := len(buff)
	if len > 5 {
		len = 5
	}
	pm.Log().Info("Do get buff from sharer cs", "buff[:len]", string(buff[:len]))
	if err != nil {
		return err
	}

	rsp := &blizcore.BuffFromSharerCSData{
		Buff: buff,
	}
	p.SendBuffFromSharerCS(rsp)

	return nil
}

func (pm *ProtocolManager) doBuffFromSharerCS(p *Peer, msg *p2p.Msg) error {
	var req blizcore.BuffFromSharerCSData
	if err := msg.Decode(&req); err != nil {
		log.Error("Do buff from sharer cs", "err", err)
		return err
	}

	rsp := &exporter.ObjBuffFromSharerCS{
		Buff: req.Buff,
	}
	feeds := pm.blizcs.bliz.EleCross.BlockChain().GetFeeds()
	feeds[state.CBuffFromSharerCS].Send(rsp)
	return nil
}
