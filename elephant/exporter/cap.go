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

package exporter

import (
	"math/big"
	"strconv"

	"github.com/contatract/go-contatract/blizcore"
	"github.com/contatract/go-contatract/blizzard/storage"
	"github.com/contatract/go-contatract/event"
	"github.com/contatract/go-contatract/p2p"
	"github.com/contatract/go-contatract/p2p/discover"

	"github.com/contatract/go-contatract/blizzard/storagecore"
	"github.com/contatract/go-contatract/common"
)

const (
	TypeElephantSign uint8 = iota
	TypeElephantCSGetFlow
	TypeCsToElephantServeCost
	TypeCsToElephantAuthAllowFlow
	TypeCsToElephantAddUsedFlow
	TypeElephantToCsDeleteFlow
)

type FarmerRsp struct {
	Farmers *[]discover.Node
}

type ObjFarmerQuery struct {
	Peer    *p2p.Peer
	ChunkId storage.ChunkIdentity
	RspFeed *event.Feed
}

type ObjMetaRsp struct {
	//Farmers	*[]discover.Node
	Meta blizcore.BlizObjectMeta
}

type ObjMetaQuery struct {
	Peer    *p2p.Peer
	Base    common.Address
	ObjId   uint64
	RspFeed *event.Feed
}

type ObjChunkPartners struct {
	ChunkId  uint64
	Partners []discover.Node
}

type ObjClaimChunkQuery struct {
	Password string
	Address  common.Address
	Node     *discover.Node
}

type ObjGetObjectQuery struct {
	Base     common.Address
	Password string
	Ty       uint8
}

type RetObjectInfo struct {
	Sharing bool `json:"Sharing,omitempty"`

	ObjId string `json:"ObjId,omitempty"`
	Size  string `json:"Size,omitempty"`

	Owner          string `json:"Owner,omitempty"`
	Offset         string `json:"Offset,omitempty"`
	Length         string `json:"Length,omitempty"`
	StartTime      string `json:"StartTime,omitempty"`
	EndTime        string `json:"StopTime,omitempty"`
	FileName       string `json:"FileName,omitempty"`
	SharerCSNodeID string `json:"SharerCSNodeID,omitempty"`
	Key            string `json:"Key,omitempty"`
}

type SelfObjectInfo struct {
	ObjId uint64
	Size  uint64
}

func (self *SelfObjectInfo) NewRetObjectInfo() *RetObjectInfo {
	return &RetObjectInfo{
		ObjId: strconv.Itoa(int(self.ObjId)),
		Size:  strconv.Itoa(int(self.Size) / storagecore.MSize),
	}
}

type SelfObjectsInfo []*SelfObjectInfo

type ShareObjectInfo struct {
	Owner          common.Address
	ObjId          string
	Offset         string
	Length         string
	StartTime      string
	EndTime        string
	FileName       string
	SharerCSNodeID string
	Key            string
}

func (share *ShareObjectInfo) NewRetObjectInfo() *RetObjectInfo {
	return &RetObjectInfo{
		Sharing:        true,
		ObjId:          share.ObjId,
		Owner:          share.Owner.Hex(),
		Offset:         share.Offset,
		Length:         share.Length,
		StartTime:      share.StartTime,
		EndTime:        share.EndTime,
		FileName:       share.FileName,
		SharerCSNodeID: share.SharerCSNodeID,
		Key:            share.Key,
	}
}

type ShareObjectsInfo []*ShareObjectInfo

// ObjectsInfo includes SelfObjectInfo and ShareObjectInfo
type ObjectsInfo []interface{}

type ObjElephantToBlizcsSliceInfo struct {
	Addr   common.Address
	ObjId  uint64
	Slices []storagecore.Slice
	CSAddr common.Address
}

type ObjWalletSign struct {
	PeerID string
	Hash   common.Hash
}

type ObjWalletSignRsp struct {
	PeerID  string
	R, S, V *big.Int
}

type ObjWriteFtp2CS struct {
	ObjId  uint64
	Offset uint64
	Buff   []byte
	PeerId string
}

type ObjWriteFtp2CSRsp struct {
	ObjId  uint64
	Size   uint64
	RetVal []byte
	PeerId string
}

type ObjReadFtp2CS struct {
	ObjId          uint64
	Offset         uint64
	Len            uint64
	PeerId         string
	SharerCSNodeID string
	Sharer         string
}

type ObjReadFtp2CSRsp struct {
	ObjId   uint64
	Ret     []byte
	Buff    []byte
	RawSize uint32
	PeerId  string
}

type ObjAuthFtp2CS struct {
	CsAddr       common.Address
	UserAddr     common.Address
	AuthDeadline uint64
	R, S, V      *big.Int
	Peer         string
}

type ObjGetCliAddr struct {
	Address common.Address
}

type ObjGetCliAddrRsp struct {
	Addresses []common.CSAuthData
}

type ObjGetElephantData struct {
	Address common.Address
	Ty      uint8
	Params  []interface{}
}

type ObjGetElephantDataRsp struct {
	Ty        uint8
	RspParams []interface{}
}

type ObjElephantToCsData struct {
	Ty     uint8
	Params []interface{}
}

type ObjEthHeight struct {
	EthHeight uint64
}

type ObjEleHeight struct {
	EleHeight uint64
}

type ObjWriteSliceHeader struct {
	Addr  common.Address
	Slice *storagecore.Slice
	OpId  uint64
}

type ObjBuffFromSharerCS struct {
	Buff []byte
}

type ObjFarmerAward struct {
	EleHeight uint64
	BlockHash common.Hash
}
