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

package ftransfer

import (
	"crypto/ecdsa"
	"math/big"
	"sync/atomic"

	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/crypto"
	"github.com/contatract/go-contatract/crypto/sha3"
	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/rlp"
)

const (
	TransferCSPercentExit = TransferCSPercent(-1)
	TransferPercentExit   = TransferPercent(-1)
)

type GetHashSign struct {
	Hash common.Hash
}

type HashSign struct {
	R, S, V *big.Int
}

type SignAuthorizeData struct {
	CsAddr       common.Address
	UserAddr     common.Address
	AuthDeadline uint64 // 授权最后期限
	R, S, V      *big.Int
	Peer         string
}

type WriteDataMsgData struct {
	ObjId  uint64 // 暂时作为Key
	Offset uint64
	Buff   []byte
}

type WriteDataRet struct {
	ObjId  uint64 // 暂时作为Key
	Size   uint64
	RetVal []byte
	SeqID  uint32
}

type WriteFileRet struct {
	FileName string
	Ret      bool
	ErrInfo  string
}

type ReadFileRet struct {
	FileName string
	Ret      bool
	ErrInfo  string
}

type WriteFileCSRate struct {
	FileName string
	CSRate   uint32
	ErrInfo  string
	SeqID    uint32
}

type TransferRet struct {
	FileName string
	Ret      bool
	ErrInfo  string
}

type ReadDataMsgData struct {
	ObjId          uint64 // 暂时作为Key
	Offset         uint64
	Len            uint64
	pubKey, sig    []byte
	SharerCSNodeID string
	Sharer         string
}

type DataReadRet struct {
	ObjId   uint64 // 暂时作为Key
	Ret     []byte // nil: success
	Buff    []byte
	RawSize uint32
}

type GetDataSignConfirmMsgData struct {
	ObjId    uint64 // 暂时作为Key
	DataHash common.Address
}

type DataSignConfirmMsgData struct {
	ObjId   uint64 // 暂时作为Key
	R, S, V *big.Int
}

type DataSendRet struct {
	Ret bool
	Err string
}

type FTFileMsgData struct {
	Buff    []byte
	RawSize uint32
	SeqID   uint32
}

type FTFileData struct {
	Buff    []byte
	RawSize uint32
}

func logChan(flag uint8) {
	return
	var str string
	switch flag {
	case 1:
		str = "before"
	case 2:
		str = "after"
	default:
		str = "recover"
	}
	log.Info(str)
}

func (ret *DataSendRet) SafeSendChTo(ch chan DataSendRet) (closed bool) {
	defer func() {
		if recover() != nil {
			closed = true
			logChan(3)
		}
	}()
	logChan(1)
	ch <- *ret
	logChan(2)
	return false
}

type TransferPercent int
type TransferCSPercent int

func (rate *TransferPercent) SafeSendChTo(ch chan TransferPercent) (closed bool) {
	defer func() {
		if recover() != nil {
			closed = true
			logChan(3)
		}
	}()
	logChan(1)
	ch <- *rate
	logChan(2)
	return false
}

func (rate *TransferCSPercent) SafeSendChTo(ch chan TransferCSPercent) (closed bool) {
	defer func() {
		if recover() != nil {
			closed = true
			logChan(3)
		}
	}()
	logChan(1)
	ch <- *rate
	logChan(2)
	return false
}

type BaseAddressMsgData struct {
	Address common.Address
	Peer    string
}

type AuthAllowFlowMsgData struct {
	Signer common.Address
	Flow   uint64
	Sign   []byte
}

type FileTransferReqData struct {
	Name      string
	ObjId     uint64
	Offset    uint64
	Size      uint64
	PubKey    []byte
	Signature []byte

	TransferCSSize       uint64
	interactWithCSfailed bool
	SharerCSNodeID       string
	Sharer               string
	AckID                uint32
}

func (h *FileTransferReqData) SetTransferCSSize(size uint64) {
	h.TransferCSSize = size
}

func (h *FileTransferReqData) SetRetWithCS(flag bool) {
	h.interactWithCSfailed = flag
}

func (h *FileTransferReqData) SetAckID(ackID uint32) {
	log.Info("Set ack id", "ackid", ackID)
	if atomic.LoadUint32(&h.AckID) < ackID {
		atomic.StoreUint32(&h.AckID, ackID)
	}
}

func (h *FileTransferReqData) ComputeHash() []byte {
	return rlpHash([]interface{}{h.Name, h.ObjId, h.Size, h.PubKey}).Bytes()
}

// zeroKey zeroes a private key in memory.
func zeroKey(k *ecdsa.PrivateKey) {
	b := k.D.Bits()
	for i := range b {
		b[i] = 0
	}
}

func rlpHash(x interface{}) (h common.Hash) {
	hw := sha3.NewKeccak256()
	rlp.Encode(hw, x)
	hw.Sum(h[:0])
	return h
}

func (h *FileTransferReqData) VerifyHash() bool {
	hash := h.ComputeHash()
	return crypto.VerifySignature(h.PubKey, hash, h.Signature)
}

func (h *FileTransferReqData) Clear() {
	h.Name = ""
	h.ObjId = 0
	h.Offset = 0
	h.Size = 0
	h.PubKey = nil
	h.Signature = make([]byte, 0)

	h.interactWithCSfailed = false
	h.TransferCSSize = 0
}

type InteractiveInfo struct {
	typeInfo int
	Contents []interface{}
}
