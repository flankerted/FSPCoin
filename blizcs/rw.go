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
	"crypto/ecdsa"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/contatract/go-contatract/blizcore"
	"github.com/contatract/go-contatract/blizzard/storage"
	"github.com/contatract/go-contatract/blizzard/storagecore"
	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/p2p/discover"
)

var (
	WriteCheckCount = 1
	ReadCheckCount  = WriteCheckCount
)

type WriteVerifyHash struct {
	HeaderHash common.Hash
	Count      uint8
}

type WriteSliceRet struct {
	ChunkId uint64
	SliceId uint32
	NodeIds []discover.NodeID
	Err     error
}

type ReadSliceRet struct {
	ChunkId uint64
	SliceId uint32
	data    []byte
	NodeId  discover.NodeID
	Err     error
}

func init() {
	// if runtime.GOOS != "windows" {
	// 	SetRWCheckCount(2)
	// }
}

func (clnt *BlizClntObject) WriteMeta(chunkId uint64, sliceId uint32, address common.Address) {
	clnt.farmerlock.RLock()
	defer clnt.farmerlock.RUnlock()
	farmers, ok := clnt.chunkFarmers[storage.Int2ChunkIdentity(chunkId)]
	if !ok {
		return
	}
	req := &blizcore.GetSliceMetaFromFarmerReqData{Address: address}
	for _, farmer := range farmers {
		ret := farmer.SendGetSliceMetaFromFarmerReq(req)
		if ret == nil {
			break
		}
	}
}

func (clnt *BlizClntObject) WriteSlices(data []byte) (int, error) {
	// 往 offset 处写入
	wlen := uint32(len(data))
	if wlen == 0 {
		return 0, blizcore.ErrParameter
	}

	// 计算 offset 所在的 chunk
	sliceBeginIdx := uint32(clnt.offset / storage.SliceSize)
	sliceBeginOffset := uint32(clnt.offset % storage.SliceSize)
	sliceEndIdx := uint32((clnt.offset + uint64(wlen)) / storage.SliceSize)
	if uint64(sliceEndIdx)*storage.SliceSize == clnt.offset+uint64(wlen) {
		sliceEndIdx--
	}
	if sliceEndIdx >= uint32(len(clnt.meta.SliceInfos)) {
		clnt.Log().Error("WriteSlices", "sliceEndIdx", sliceEndIdx, "SliceInfos len", len(clnt.meta.SliceInfos))
		// 超出范围
		return 0, blizcore.ErrObjectSpaceInsufficient
	}

	mapWriteData := make(map[common.Hash][][]byte)
	totalWriteLen := uint32(0)
	sliceOffset := sliceBeginOffset
	sliceWriteLen := wlen

	seconds := wlen / (1024 * 2)
	if seconds < 60 {
		seconds = 60
	}
	duration := time.Second * time.Duration(seconds)

	writeSlicePeers := make(map[string][]discover.NodeID)
	curId := sliceBeginIdx
	curOffset := sliceOffset
	for {
		from := totalWriteLen
		if sliceWriteLen > storage.SliceSize-curOffset {
			sliceWriteLen = storage.SliceSize - curOffset
		}
		to := totalWriteLen + sliceWriteLen
		curData := data[from:to]
		clnt.Log().Info("", "curid", curId, "curoffset", curOffset)
		t := time.Now()
		writeRet := clnt.WriteOneSlice(curId, curOffset, curData, &mapWriteData)
		log.Info("Write one slice", "curdatalen", len(curData), "elapse", common.PrettyDuration(time.Since(t)))
		if writeRet.Err != nil {
			return 0, writeRet.Err
		}
		sliceInfo := blizcore.SliceInfo{ChunkId: writeRet.ChunkId, SliceId: writeRet.SliceId}
		clnt.Log().Info("Write slice peers", "sliceinfo", sliceInfo)
		writeSlicePeers[sliceInfo.String()] = writeRet.NodeIds

		totalWriteLen += sliceWriteLen
		if totalWriteLen == wlen {
			break
		}
		curId++
		curOffset = 0
		sliceWriteLen = wlen - totalWriteLen
	}

	sliceWriteCh := make(chan *blizcore.WriteOperRspData)
	sliceWriteSub := clnt.scope.Track(clnt.sliceWriteFeed.Subscribe(sliceWriteCh))
	defer sliceWriteSub.Unsubscribe()

	writeVerifyHashs := make(map[string]*WriteVerifyHash)
	totalCount := sliceEndIdx - sliceBeginIdx + 1
	succCount := uint32(0)
	for {
		select {
		case <-time.After(duration):
			clnt.Log().Info("")
			return 0, blizcore.ErrTimeout

		case data := <-sliceWriteCh:
			clnt.Log().Info("")
			if len(data.ErrBytes) != 0 {
				return 0, errors.New(string(data.ErrBytes))
			}

			sliceInfo := blizcore.SliceInfo{ChunkId: data.Chunk, SliceId: data.Slice}
			v, ok := writeVerifyHashs[sliceInfo.String()]
			if !ok {
				writeVerifyHashs[sliceInfo.String()] = &WriteVerifyHash{HeaderHash: data.HeaderHash, Count: 1}
				v, _ = writeVerifyHashs[sliceInfo.String()]
			} else {
				if v.HeaderHash != data.HeaderHash {
					return 0, blizcore.ErrHash
				}
				v.Count++
			}
			// 校验不通过
			if v.Count < uint8(WriteCheckCount) {
				break
			}

			info := &blizcore.WopInfo{WopId: data.Id, Chunk: data.Chunk, Slice: data.Slice, Type: 1, HeaderHash: data.HeaderHash, Infos: data.Infos}
			shs := &storagecore.SliceHeaderSign{HeaderHash: data.HeaderHash}
			sig, err := storagecore.SignSliceHeader(shs, clnt.blizcs.basePrivateKey)
			if err != nil {
				sig = nil
			}
			// dHash := (*data.Infos)[0].Hash
			var datas [][]byte // The server gets the data from cache by hash
			// if v, ok := mapWriteData[dHash]; ok {
			// 	datas = v
			// }
			req := &blizcore.WriteOperReqData{Info: info, ObjUId: clnt.ID(), Datas: datas, Sign: sig}
			err = clnt.authSignWop(req, clnt.blizcs.basePrivateKey)
			if err != nil {
				return 0, err
			}

			nodeIds, ok := writeSlicePeers[sliceInfo.String()]
			if !ok {
				break
			}
			chunkId := storage.Int2ChunkIdentity(data.Chunk)
			for _, nodeId := range nodeIds {
				farmer := clnt.getFarmer(&chunkId, &nodeId)
				if farmer == nil {
					return 0, blizcore.ErrFarmer
				}
				req.Datas = nil // Cached the data to avoid network transmission back and forth
				ret := farmer.SendWop(req)
				if ret != nil {
					return 0, blizcore.ErrNetwork
				}
			}
			succCount++
			if succCount == totalCount {
				return int(wlen), nil
			}

		case err := <-sliceWriteSub.Err():
			return 0, err

		case <-clnt.shutdownChan:
			return 0, blizcore.ErrShutdown
		}
	}
}

func (clnt *BlizClntObject) GetFarmerSliceMap(info string) (map[discover.NodeID]*blizcore.WopData, bool) {
	infos, ok := clnt.GetFarmerSliceMap2(info)
	if ok {
		return infos, ok
	}

	clnt.farmerSliceMapsLock.RLock()
	defer clnt.farmerSliceMapsLock.RUnlock()

	// 新分配的对象空间没有nodeId
	parts := strings.Split(info, ":")
	if len(parts) < 2 {
		return infos, ok
	}
	chunkId, err := strconv.Atoi(parts[0])
	if err != nil {
		return infos, ok
	}
	for i, v := range clnt.farmerSliceMaps {
		parts2 := strings.Split(i, ":")
		if len(parts2) < 2 {
			continue
		}
		id, err := strconv.Atoi(parts2[0])
		if err != nil {
			continue
		}
		if id != chunkId {
			continue
		}
		infos = make(map[discover.NodeID]*blizcore.WopData)
		for k, _ := range v {
			infos[k] = &blizcore.WopData{WopId: 0}
		}
		return infos, true
	}
	return infos, ok
}

func (clnt *BlizClntObject) getMapWopData(info string) (map[discover.NodeID]*blizcore.WopData, bool) {
	clnt.farmerSliceMapsLock.RLock()
	defer clnt.farmerSliceMapsLock.RUnlock()

	infos, ok := clnt.farmerSliceMaps[info]
	if !ok {
		return nil, false
	}
	ret := make(map[discover.NodeID]*blizcore.WopData)
	for i, v := range infos {
		ret[i] = v
	}
	return ret, true
}

func (clnt *BlizClntObject) GetFarmerSliceMap2(info string) (map[discover.NodeID]*blizcore.WopData, bool) {
	infos, ok := clnt.getMapWopData(info)
	if ok {
		return infos, ok
	}

	times := 0
	ticker := time.NewTicker(time.Second * 1 / 4)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			times++
			clnt.Log().Info("", "farmerSliceMaps len", len(clnt.farmerSliceMaps))
			infos, ok := clnt.getMapWopData(info)
			if ok || times >= 10 {
				return infos, ok
			}
		}
	}
}

func (clnt *BlizClntObject) WriteOneSlice(idx uint32, sliceOffset uint32, data []byte, mapWriteData *map[common.Hash][][]byte) *WriteSliceRet {
	k := idx
	sliceInfo := clnt.meta.SliceInfos[k]
	clnt.Log().Info("Write one slice", "sliceinfo", sliceInfo)
	writeRet := &WriteSliceRet{ChunkId: sliceInfo.ChunkId, SliceId: sliceInfo.SliceId}
	wlen := uint32(len(data))
	var nodeIds []discover.NodeID
	if wlen == 0 || sliceOffset+wlen > storage.SliceSize {
		writeRet.Err = blizcore.ErrParameter
		return writeRet
	}

	var wopInfos []*blizcore.WopInfo
	if _, ok := clnt.localSliceMap[sliceInfo.String()]; ok {
		var dis []*blizcore.WriteDataInfoS
		dis = append(dis, &blizcore.WriteDataInfoS{Offset: sliceOffset, Length: wlen})
		info := &blizcore.WopInfo{
			WopId: clnt.localSliceMap[sliceInfo.String()].WopId,
			WopTs: uint64(time.Now().Unix()),
			Chunk: sliceInfo.ChunkId,
			Slice: sliceInfo.SliceId,
			Infos: &dis,
		}
		wopInfos = append(wopInfos, info)
	} else {
		var dis []*blizcore.WriteDataInfoS
		dis = append(dis, &blizcore.WriteDataInfoS{Offset: sliceOffset, Length: wlen})
		info := &blizcore.WopInfo{
			WopId: 0, // 0 后面需要从 farmer 处获取并定义
			WopTs: uint64(time.Now().Unix()),
			Chunk: sliceInfo.ChunkId,
			Slice: sliceInfo.SliceId,
			Infos: &dis,
		}
		wopInfos = append(wopInfos, info)
	}

	infos, ok := clnt.GetFarmerSliceMap(sliceInfo.String())
	if !ok {
		clnt.Log().Info("GetFarmerSliceMap not", "sliceInfo", sliceInfo)
		writeRet.Err = blizcore.ErrFarmerSlice
		return writeRet
	}

	clnt.Log().Info("Write one slice", "wopInfocount", len(wopInfos))
	// 其实循环只有1次
	for _, info := range wopInfos {
		chunkId := storage.Int2ChunkIdentity(info.Chunk)
		// blizCSDebug2("range wopInfos", "info", info)
		if info.WopId > 0 {
			// 本地有该 slice 的写入进度信息
			var datas [][]byte
			dHash := blizcore.DataHash(data)
			(*info.Infos)[0].Hash = dHash
			info.WopId = info.WopId + 1 // 流水号+1
			datas = append(datas, data)
			req := &blizcore.WriteOperReqData{Info: info, Datas: datas, ObjUId: clnt.ID()}
			// clnt.Log().Info("Write one slice", "privatekey", clnt.blizcs.basePrivateKey)
			err := clnt.authSignWop(req, clnt.blizcs.basePrivateKey)
			if err != nil {
				clnt.Log().Error("authSignWop", "err", err)
				writeRet.Err = err
				return writeRet
			}
			sender, err := req.Sender()
			clnt.Log().Info("Write one slice", "sender", sender, "err", err)
			(*mapWriteData)[dHash] = datas
			clnt.Log().Info("", "infos len", len(infos))
			for nodeId, si := range infos {
				if si == nil {
					continue
				}
				clnt.Log().Info("", "siwopid", si.WopId, "infowopid", info.WopId)
				if si.WopId+1 != info.WopId {
					clnt.Log().Error("")
					continue
				}
				farmer := clnt.getFarmer(&chunkId, &nodeId)
				clnt.Log().Info("", "farmer", farmer)
				if farmer == nil {
					continue
				}
				ret := farmer.SendWop(req)
				clnt.Log().Info("", "ret", ret)
				if ret != nil {
					continue
				}
				nodeIds = append(nodeIds, nodeId)
				if len(nodeIds) >= WriteCheckCount {
					break
				}
			}
		} else {
			// 本地没有该 slice 的写入进度信息
			maxWopId := uint64(0)
			repeatCnt := uint8(0)
			for _, si := range infos {
				if si == nil {
					continue
				}
				clnt.Log().Info("", "si.WopId", si.WopId, "maxWopId", maxWopId)
				if si.WopId > maxWopId {
					maxWopId = si.WopId
					repeatCnt = 1
				} else if si.WopId == maxWopId {
					repeatCnt++
				}
			}

			if repeatCnt >= uint8(WriteCheckCount) {
				var datas [][]byte
				dHash := blizcore.DataHash(data)
				(*info.Infos)[0].Hash = dHash
				info.WopId = maxWopId + 1 // 流水号+1
				datas = append(datas, data)
				req := &blizcore.WriteOperReqData{Info: info, Datas: datas, ObjUId: clnt.ID()}
				err := clnt.authSignWop(req, clnt.blizcs.basePrivateKey)
				if err != nil {
					clnt.Log().Error("authSignWop", "err", err)
					writeRet.Err = err
					return writeRet
				}
				(*mapWriteData)[dHash] = datas
				for nodeId, si := range infos {
					if si == nil {
						clnt.Log().Info("si nil")
						continue
					}
					clnt.Log().Info("", "si.WopId", si.WopId, "maxWopId", maxWopId)
					if si.WopId != maxWopId {
						continue
					}
					farmer := clnt.getFarmer(&chunkId, &nodeId)
					if farmer == nil {
						clnt.Log().Info("farmer nil")
						continue
					}
					ret := farmer.SendWop(req)
					if ret != nil {
						clnt.Log().Info("SendWop", "ret", ret)
						continue
					}
					nodeIds = append(nodeIds, nodeId)
					if len(nodeIds) == WriteCheckCount {
						break
					}
				}
			}
		}
		clnt.Log().Info("Write one slice", "nodecount", len(nodeIds))
		if len(nodeIds) < WriteCheckCount {
			// 看来能够写入的 farmer 个数不足
			// 需要重新连其他Farmer了
			// 考虑看看：是否需要使用 go routine
			go clnt.trickConnFarmers(&chunkId, 2)
			// 返回需要阻塞的错误
			writeRet.Err = blizcore.ErrWouldBlock
			return writeRet
		}

		// 写入成功，更新一下 slice 的 wopId 进度
		err := clnt.SaveSliceInfo(info)
		clnt.Log().Info("Write one slice", "err", err)

		// 更新本地
		clnt.localSliceMap[sliceInfo.String()] = info
		// 更新FarmerSliceOps
		for _, nodeId := range nodeIds {
			op := blizcore.FarmerSliceOp{ChunkId: info.Chunk, SliceId: info.Slice, OpId: info.WopId}
			ops := []blizcore.FarmerSliceOp{op}
			clnt.UpdateFarmersSliceOps(ops, nodeId)
		}
	}

	writeRet.NodeIds = nodeIds
	return writeRet
}

func (clnt *BlizClntObject) ReadSlices(buffer []byte) error {
	// 读取逻辑设计：
	// 如果本地 slicedb 中存有当前 offset 所在的 slice 的写入流水号，且自己是
	// object 的 owner, 则对比当前连接的几个 farmer ，看看谁的 slice 写入流水
	// 号跟自己是匹配的，就选择谁进行数据读取
	// 如果发现当前连接的 farmer 都不合适，则尝试重新连接其他farmer, retry次数
	// 超过之后，选择流水号相等的最高的 farmer 进行数据读取（下载）

	rlen := uint32(len(buffer))
	if rlen == 0 {
		return blizcore.ErrParameter
	}

	// 计算 offset 所在的 chunk
	sliceBeginIdx := uint32(clnt.offset / storage.SliceSize)
	sliceBeginOffset := uint32(clnt.offset % storage.SliceSize)
	sliceEndIdx := uint32((clnt.offset + uint64(rlen)) / storage.SliceSize)
	if uint64(sliceEndIdx)*storage.SliceSize == clnt.offset+uint64(rlen) {
		sliceEndIdx--
	}
	if sliceEndIdx >= uint32(len(clnt.meta.SliceInfos)) {
		// 超出范围
		return blizcore.ErrOutOfRange
	}

	totalReadLen := uint32(0)
	sliceOffset := sliceBeginOffset
	sliceReadLen := rlen

	seconds := rlen / (1024 * 16)
	if seconds < 60 {
		seconds = 60
	}
	duration := time.Second * time.Duration(seconds)

	readSlicePeers := make(map[string]discover.NodeID)
	totalCount := uint32(0)
	curId := sliceBeginIdx
	curOffset := sliceOffset
	for {
		curLen := sliceReadLen
		if curLen > storage.SliceSize-curOffset {
			curLen = storage.SliceSize - curOffset
		}
		clnt.Log().Info("", "curId", curId, "curOffset", curOffset, "curLen", curLen)
		totalCount++

		readRet := clnt.ReadOneSlice(curId, curOffset, curLen)
		if readRet.Err != nil {
			return readRet.Err
		}
		sliceInfo := blizcore.SliceInfo{ChunkId: readRet.ChunkId, SliceId: readRet.SliceId}
		readSlicePeers[sliceInfo.String()] = readRet.NodeId

		totalReadLen += curLen
		if totalReadLen == rlen {
			break
		}
		curId++
		curOffset = 0
		sliceReadLen = rlen - totalReadLen
	}

	sliceReadCh := make(chan *blizcore.ReadOperRspData)
	sliceReadSub := clnt.scope.Track(clnt.sliceReadFeed.Subscribe(sliceReadCh))
	defer sliceReadSub.Unsubscribe()

	readVerifyHashs := make(map[string]map[string]uint8)
	readVerifyHashSucc := make(map[string]struct{})
	buffers := make(map[string][]byte)
	for {
		select {
		case <-time.After(duration):
			return blizcore.ErrTimeout

		case data := <-sliceReadCh:
			if len(data.ErrBytes) != 0 {
				return errors.New(string(data.ErrBytes))
			}
			sliceInfo := blizcore.SliceInfo{ChunkId: data.ReadOperReqData.Chunk, SliceId: data.ReadOperReqData.Slice}
			// check hash
			if data.Type == blizcore.ReqHash {
				if _, ok := readVerifyHashSucc[sliceInfo.String()]; ok {
					break
				}
				dataHash := string(data.Data)
				v, ok := readVerifyHashs[sliceInfo.String()]
				if !ok {
					hashCount := make(map[string]uint8)
					hashCount[dataHash] = 1
					readVerifyHashs[sliceInfo.String()] = hashCount
					v = readVerifyHashs[sliceInfo.String()]
				} else {
					if _, ok2 := v[dataHash]; !ok2 {
						v[dataHash] = 1
					} else {
						v[dataHash] += 1
					}
				}
				if v[dataHash] < uint8(ReadCheckCount) {
					break
				}
				// First arrive ReadCheckCount
				// Clear invalid data in readVerifyHashs[sliceInfo.String()]
				t := make(map[string]uint8)
				t[dataHash] = v[dataHash]
				readVerifyHashs[sliceInfo.String()] = t

				readVerifyHashSucc[sliceInfo.String()] = struct{}{}

				clnt.Log().Info("ReadSlices", "sliceInfo", sliceInfo)
				nodeId, ok := readSlicePeers[sliceInfo.String()]
				if !ok {
					clnt.Log().Error("readSlicePeers not")
					return blizcore.ErrFarmer
				}
				cId := storage.Int2ChunkIdentity(data.ReadOperReqData.Chunk)
				farmer := clnt.getFarmer(&cId, &nodeId)
				if farmer == nil {
					clnt.Log().Error("farmer nil", "cId", cId, "nodeId", nodeId)
					return blizcore.ErrFarmer
				}
				req := &data.ReadOperReqData
				req.Type = blizcore.ReqData
				err := farmer.SendRop(req)
				if err != nil {
					clnt.Log().Warn("SendRop", "err", err)
					return blizcore.ErrNetwork
				}
			} else if data.Type == blizcore.ReqData {
				if _, ok := readVerifyHashSucc[sliceInfo.String()]; !ok {
					break
				}

				buffers[sliceInfo.String()] = data.Data
				clnt.Log().Info("ReadSlices", "buffers len", len(buffers), "totalCount", totalCount)
				if uint32(len(buffers)) < totalCount {
					break
				}

				buff := make([]byte, 0)
				// Sort
				for i := sliceBeginIdx; i <= sliceEndIdx; i++ {
					sliceInfo := clnt.meta.SliceInfos[i]
					v, ok := buffers[sliceInfo.String()]
					if !ok {
						clnt.Log().Error("", "sliceBeginIdx", sliceBeginIdx, "sliceEndIdx", sliceEndIdx, "i", i, "sliceInfo", sliceInfo)
						return blizcore.ErrLackOfData
					}
					buff = append(buff, v...)
				}
				if len(buffer) < len(buff) {
					return blizcore.ErrCapacityNotEnough
				}
				copy(buffer, buff)
				return nil
			}

		case err := <-sliceReadSub.Err():
			return err

		case <-clnt.shutdownChan:
			return blizcore.ErrShutdown
		}
	}
}

func (clnt *BlizClntObject) ReadOneSlice(idx uint32, sliceOffset uint32, length uint32) *ReadSliceRet {
	k := idx
	sliceInfo := clnt.meta.SliceInfos[k]
	clnt.Log().Info("ReadOneSlice", "k", k, "sliceInfo", sliceInfo)
	readRet := &ReadSliceRet{ChunkId: sliceInfo.ChunkId, SliceId: sliceInfo.SliceId}
	if length == 0 || sliceOffset+length > storage.SliceSize {
		readRet.Err = blizcore.ErrParameter
		return readRet
	}

	infos, ok := clnt.GetFarmerSliceMap(sliceInfo.String())
	if !ok {
		clnt.Log().Info("GetFarmerSliceMap not", "sliceInfo", sliceInfo)
		readRet.Err = blizcore.ErrFarmerSlice
		return readRet
	}
	maxWopId := uint64(0)
	var nodeIds []*discover.NodeID
	for nodeId, si := range infos {
		if si == nil {
			continue
		}
		n := nodeId
		if si.WopId > uint64(maxWopId) {
			maxWopId = si.WopId
			nodeIds = []*discover.NodeID{&n}
		} else if si.WopId == uint64(maxWopId) {
			nodeIds = append(nodeIds, &nodeId)
		}
	}
	if len(nodeIds) < ReadCheckCount {
		clnt.Log().Info("ReadOneSlice", "newest count", len(nodeIds))
		readRet.Err = blizcore.ErrNoEnoughFarmer
		return readRet
	}
	req := &blizcore.ReadOperReqData{
		Id:     maxWopId,
		Chunk:  sliceInfo.ChunkId,
		Slice:  sliceInfo.SliceId,
		Offset: sliceOffset,
		Length: length,
		Type:   blizcore.ReqHash,
		ObjId:  clnt.ID(),
	}
	succCount := 0
	for _, nodeId := range nodeIds {
		clnt.Log().Info("ReadOneSlice", "nodeId", nodeId)
		cId := storage.Int2ChunkIdentity(sliceInfo.ChunkId)
		farmer := clnt.getFarmer(&cId, nodeId)
		if farmer == nil {
			clnt.Log().Warn("farmer nil")
			continue
		}
		req.NodeId = *nodeId
		err := farmer.SendRop(req)
		if err != nil {
			clnt.Log().Warn("SendRop", "err", err)
			continue
		}
		succCount++
		if succCount == ReadCheckCount {
			readRet.NodeId = *nodeId
			break
		}
	}

	if succCount < ReadCheckCount {
		readRet.Err = blizcore.ErrNoEnoughFarmer
	}
	return readRet
}

// 授权签名
func (clnt *BlizClntObject) authSignWop(req *blizcore.WriteOperReqData, priKey *ecdsa.PrivateKey) error {
	cs := clnt.blizcs
	data := cs.GetUserAuthDataByPeer(clnt.GetSignPeer())
	if data == nil {
		return errors.New("no user auth")
	}
	req.Info.AuthAddress = data.CsAddr
	req.Info.AuthDeadlineC = data.Deadline
	req.Info.RC, req.Info.SC, req.Info.VC = data.R, data.S, data.V
	return blizcore.SignWop(req, priKey)
}
