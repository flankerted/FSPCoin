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

package blizcore

import (
	"fmt"
	"strings"

	"math/big"

	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/p2p/discover"

	//"github.com/contatract/go-contatract/blizzard/storage"
	//"github.com/contatract/go-contatract/p2p/discover"

	"github.com/contatract/go-contatract/blizzard/storagecore"
)

const (
	ReqData = 0
	ReqHash = 1
)

// WOP 定义
type WriteDataInfoS struct {
	Offset uint32
	Length uint32
	Hash   common.Hash
}

type WopInfo struct {
	WopId uint64
	WopTs uint64

	Chunk uint64 // 该 Wop 涉及的Chunk:Slice
	Slice uint32

	Infos *[]*WriteDataInfoS // 一组写入数据的相关Info

	Type       uint8 // 0: write data, 1: checked header hash
	HeaderHash common.Hash

	// Signature values( for Id & Hash(Datas) & Infos & Chunk & Slice)
	V *big.Int //`json:"v" gencodec:"required"`
	R *big.Int //`json:"r" gencodec:"required"`
	S *big.Int //`json:"s" gencodec:"required"`

	AuthAddress   common.Address
	AuthDeadlineC uint64
	RC            *big.Int // comming from client
	SC            *big.Int // comming from client
	VC            *big.Int // comming from client
}

type WopData struct {
	WopId uint64
}

type WriteOperReqData struct {
	Info   *WopInfo
	Datas  [][]byte // 一组写入数据（跟Infos一一对应）
	ObjUId string
	Sign   []byte
}

type FarmerSliceOpReq struct {
	ObjUId     string
	SliceInfos []SliceInfo
}

type FarmerSliceOp struct {
	ChunkId uint64
	SliceId uint32
	OpId    uint64
}

type FarmerSliceOpRsp struct {
	ObjUId         string
	FarmerSliceOps []FarmerSliceOp
	NodeId         discover.NodeID
}

type GetSliceMetaFromFarmerReqData struct {
	Address common.Address
}

type GetBuffFromSharerCSData struct {
	Sharer    string
	ObjId     uint64
	Offset    uint64
	Len       uint64
	SigPeerId string
}

type BuffFromSharerCSData struct {
	ID   string
	Buff []byte
}

// 计算 Datas 的hash值
func (wop *WriteOperReqData) VerifySeal(tenant common.Address) error {
	sender, err := wop.Sender()
	if err != nil {
		log.Error("VerifySeal", "err", err)
		return err
	}

	if sender.Equal(tenant) == false {
		log.Error("VerifySeal", "tenant", tenant, "sender", sender)
		return ErrWopInvalidPermission
	}

	return nil
}

func (wop *WriteOperReqData) AuthVerifySeal(tenant common.Address) error {
	sender, err := wop.AuthSender()
	if err != nil {
		log.Error("Auth verify seal", "err", err)
		return err
	}

	if sender.Equal(tenant) == false {
		log.Error("Auth verify seal", "tenant", tenant, "sender", sender)
		return ErrWopInvalidPermission
	}

	return nil
}

type WriteOperRspData struct {
	Chunk      uint64
	Slice      uint32
	Id         uint64 // WopId
	Status     uint32 // return status: success or fail
	ErrBytes   []byte
	Type       uint8       // 0: write data, 1: checked data hash, stored at local
	HeaderHash common.Hash // for signing
	ObjUId     string
	Infos      *[]*WriteDataInfoS
}

type ReadOperReqData struct {
	Id     uint64
	Chunk  uint64
	Slice  uint32
	Offset uint32
	Length uint32
	Type   uint8 // 0: data, 1: hash
	NodeId discover.NodeID
	ObjId  string
}

type ReadOperRspData struct {
	ReadOperReqData
	ErrBytes []byte
	Data     []byte
}

type SliceWopProg struct {
	Slice uint32
	WopId uint64
}

type HeartBeatMsgData struct {
	Ts      uint64
	ChunkId uint64
	Progs   []*SliceWopProg

	// Signature values( for Id & Hash(Datas) & Infos & Chunk & Slice)
	Signature []byte
}

// query 类消息
type DataQuery interface {
	SetQuerId(queryId uint64)
}

type GetWriteOpMsgData struct {
	Chunk     uint64
	Slice     uint32
	FromWopId uint64
	Count     uint32

	QueryId uint64
}

type GetWriteDataMsgData struct {
	Chunk uint64
	Slice uint32
}

type GetHeaderHashMsgData struct {
	Chunk uint64
	Slice uint32
}

func (query *GetWriteOpMsgData) SetQuerId(queryId uint64) {
	query.QueryId = queryId
}

// 基本跟 WriteOperReqData 一样
type WriteOpMsgData struct {
	QueryId uint64

	Info  *WopInfo
	Datas [][]byte // 一组写入数据（跟Infos一一对应）
	Sign  []byte
}

type WriteDataMsgData struct {
	Rsp    *GetWriteDataMsgData
	Header *storagecore.SliceHeader
	Data   []byte
}

type HeaderHashMsgData struct {
	Rsp        *GetHeaderHashMsgData
	HeaderHash common.Hash
	OpId       uint64
}

type GetSliceInfoMsgData struct {
	Chunk uint64
	Slice uint32

	QueryId uint64
}

func (query *GetSliceInfoMsgData) SetQuerId(queryId uint64) {
	query.QueryId = queryId
}

type SliceInfoMsgData struct {
	Chunk uint64
	Slice uint32

	QueryId uint64

	WopProg      uint64   // 当前slice的 wopId 进度
	SegmentHashs [][]byte // 当前slice中所有 segment 的 hash 值列表

	Signature []byte // tenant 为 WopProg + Hash(SegmentHashs) 的签名，可以由上面两个值来计算校验
}

// 校验 Signature 和 WopProg + Hash(SegmentHashs) 是否对的上
func (msgdata *SliceInfoMsgData) Verify() error {
	// 1. 基本检查
	if len(msgdata.SegmentHashs) < storagecore.SegmentCount {
		return ErrInvalidMsgParameter
	}
	// 2.
	// TODO: 校验签名，待实现

	return nil
}

type GetSegmentDataMsgData struct {
	Chunk   uint64
	Slice   uint32
	Segment uint32 // 获取的起始 Segment index
	Count   uint32 // 获取 Segment 数量

	QueryId uint64
}

func (query *GetSegmentDataMsgData) SetQuerId(queryId uint64) {
	query.QueryId = queryId
}

type SegmentDataMsgData struct {
	Chunk   uint64
	Slice   uint32
	Segment uint32 // 返回的起始 Segment Index
	Count   uint32 // 获取 Segment 数量

	QueryId uint64

	data []byte
}

// implement the client part of Blizzard Service
type SliceInfo struct {
	ChunkId uint64
	SliceId uint32
}

func (si *SliceInfo) String() string {
	return fmt.Sprintf("%d:%d", si.ChunkId, si.SliceId)
}

func (si *SliceInfo) ChunkString() string {
	return fmt.Sprintf("%d", si.ChunkId)
}

// BlizObjectMeta 包含在该对象对应的 chunk:slice 世界状态
// 指向的存储区域，从该存储区域的一个slice(1M)获取到所有的
// 对象信息
type BlizObjectMeta struct {
	Name       string         // 名称
	Length     uint64         // 长度
	Permission uint64         // （rw）权限
	Owner      common.Address // 对象属主（tenant)

	// 根据 length 可以计算出当前占用了多少个 slice
	SliceInfos []SliceInfo // 内容所在的所有slice信息
}

func (a *BlizObjectMeta) Equal(b *BlizObjectMeta) bool {
	if strings.Compare(a.Name, b.Name) != 0 {
		return false
	}

	if len(a.SliceInfos) != len(b.SliceInfos) ||
		a.Permission != b.Permission ||
		a.Owner.Equal(b.Owner) == false ||
		a.Length != b.Length {
		return false
	}

	var i int
	for i = 0; i < len(a.SliceInfos); i++ {
		if a.SliceInfos[i].ChunkId != b.SliceInfos[i].ChunkId ||
			a.SliceInfos[i].SliceId != b.SliceInfos[i].SliceId {
			return false
		}
	}
	return true
}
