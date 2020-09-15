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

package storage

import (
	"github.com/contatract/go-contatract/blizzard/storagecore"
	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/p2p/discover"
)

type GetFarmerData struct {
	ChunkId uint64
	QueryId uint64
}

type FarmerData struct {
	QueryId  uint64
	ChunkId  uint64
	CanClaim bool
	CanUse   bool
	Nodes    []discover.Node
	//Addresss []common.Address
	//AddrIp []string
}

type GetObjectMetaData struct {
	Address  common.Address
	ObjectId uint64
	QueryId  uint64
}

type ObjectMetaData struct {
	Data   GetObjectMetaData
	Slices []*storagecore.Slice
}

type GetDataHashProto struct {
	ChunkId uint64
	SliceId uint32
	Off     uint32
	Size    uint32
}

type DataHashProto struct {
	ChunkId uint64
	SliceId uint32
	Off     uint32
	data    []byte
}

type GetWriteDataProto struct {
	ChunkId uint64
	SliceId uint32
	Off     uint32
	Size    uint32
}

type WriteDataProto struct {
	data []byte
}

type GetReadDataProto struct {
	ChunkId uint64
	SliceId uint32
	Off     uint32
	Size    uint32
}

type ReadDataProto struct {
	data []byte
}

type BlizReHandshakeProto struct {
	Reason uint8
}
