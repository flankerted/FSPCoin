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

package elephant

import (
	"fmt"
	"io"
	"math/big"

	"github.com/contatract/go-contatract/common"
	types "github.com/contatract/go-contatract/core/types_elephant"
	"github.com/contatract/go-contatract/core_elephant"
	"github.com/contatract/go-contatract/event"
	"github.com/contatract/go-contatract/rlp"
)

// Constants to match up protocol versions and messages
const (
	elephantv1 = 1
	elephantv2 = 2
	elephantv3 = 3
)

// Official short name of the protocol used during capability negotiation.
var ProtocolName = "elephant"

// Supported versions of the lamp protocol (first is primary).
var ProtocolVersions = []uint{elephantv1, elephantv2}

// Added by jianghan for version retrive
var ProtocolVersionMin = elephantv1
var ProtocolVersionMax = elephantv3

// Number of implemented message corresponding to different protocol versions.
var ProtocolLengths = []uint64{13, 0x1d + 1}

const ProtocolMaxMsgSize = 10 * 1024 * 1024 // Maximum cap on the size of a protocol message

// elephant protocol message codes
// JiangHan：（重要）elephant 核心应用层协议（非底层p2p协议），阅读交易，合约等协议到这里看
const (
	// Protocol messages belonging to elephantv1
	// v1 的Lengths 定义了到了8 ，这里实现了8个 --jianghan
	StatusMsg                = 0x00
	NewBlockHashesMsg        = 0x01
	TxMsg                    = 0x02
	GetBlockHeadersMsg       = 0x03
	BlockHeadersMsg          = 0x04
	BlockHeadersForPoliceMsg = 0x05
	GetBlockBodiesMsg        = 0x06
	BlockBodiesMsg           = 0x07
	NewBlockMsg              = 0x08
	GetFarmersMsg            = 0x09
	FarmerMsg                = 0x0a
	GetBlizObjectMetaMsg     = 0x0b
	BlizObjectMetaMsg        = 0x0c

	// Protocol messages belonging to elephantv2
	// v2 的Length定义到了17，实际上就是8-17这9个，这里只实现了尾部4个，还有5个扩展空间 -- jianghan
	GetNodeDataMsg = 0x0d
	NodeDataMsg    = 0x0e
	GetReceiptsMsg = 0x0f
	ReceiptsMsg    = 0x10

	GetDataHashMsg = 0x11 // 获取检验数据
	DataHashMsg    = 0x12 // 检验数据
	// GetWriteDataMsg = 0x13 // 获取写数据
	// WriteDataMsg    = 0x14 // 写数据
	GetReadDataMsg = 0x15 // 获取读数据
	ReadDataMsg    = 0x16 // 读数据

	BFTQueryMsg = 0x17
	// PbftQueryMsg    = 0x18
	ShardingTxMsg     = 0x18
	VerifiedHeaderMsg = 0x19

	// BlizReHandshakeMsg = 0x11 // Bliz重新handshake

	EthHeightMsg = 0x1a
	EleHeightMsg = 0x1b
)

type errCode int

const (
	ErrMsgTooLarge = iota
	ErrDecode
	ErrInvalidMsgCode
	ErrProtocolVersionMismatch
	ErrNetworkIdMismatch
	ErrGenesisBlockMismatch
	ErrNoStatusMsg
	ErrExtraStatusMsg
	ErrSuspendedPeer
	ErrSend
)

func (e errCode) String() string {
	return errorToString[int(e)]
}

// XXX change once legacy code is out
var errorToString = map[int]string{
	ErrMsgTooLarge:             "Message too long",
	ErrDecode:                  "Invalid message",
	ErrInvalidMsgCode:          "Invalid message code",
	ErrProtocolVersionMismatch: "Protocol version mismatch",
	ErrNetworkIdMismatch:       "NetworkId mismatch",
	ErrGenesisBlockMismatch:    "Genesis block mismatch",
	ErrNoStatusMsg:             "No status message",
	ErrExtraStatusMsg:          "Extra status message",
	ErrSuspendedPeer:           "Suspended peer",
	ErrSend:                    "Send",
}

type txPoolInterface interface {
	// AddRemotes should add the given transactions to the pool.
	AddRemotes([]*types.Transaction, uint64) []error

	// Pending should return pending transactions.
	// The slice should be modifiable by the caller.
	Pending() (map[common.Address]types.Transactions, error)

	// SubscribeTxPreEvent should return an event subscription of
	// TxPreEvent and send events to the given channel.
	SubscribeTxPreEvent(chan<- core_elephant.TxPreEvent) event.Subscription

	AddTxBatch(*types.ShardingTxBatch, common.Address) error
}

// statusData is the network packet for the status message.
type statusData struct {
	ProtocolVersion uint32
	NetworkId       uint64
	TD              *big.Int
	HeightEle       *big.Int
	LampHeight      *big.Int
	LampBaseHeight  *big.Int
	LampBaseHash    common.Hash
	LampTD          *big.Int
	CurrentBlock    common.Hash
	GenesisBlock    common.Hash
	RealShard       uint16
}

// newBlockHashesData is the network packet for the block announcements.
type newBlockHashesData []struct {
	Hash        common.Hash // Hash of one particular block being announced
	HashWithBft common.Hash // HashWithBft of one particular block being announced
	Number      uint64      // Number of one particular block being announced
}

// getBlockHeadersData represents a block header query.
type getBlockHeadersData struct {
	Origin    hashOrNumber // Block from which to retrieve headers
	Amount    uint64       // Maximum number of headers to retrieve
	Skip      uint64       // Blocks to skip between consecutive headers
	Reverse   bool         // Query direction (false = rising towards latest, true = falling towards genesis)
	ForPolice bool         // Query from the police
}

// hashOrNumber is a combined field for specifying an origin block.
type hashOrNumber struct {
	Hash   common.Hash // Block hash from which to retrieve headers (excludes Number)
	Number uint64      // Block hash from which to retrieve headers (excludes Hash)
}

// EncodeRLP is a specialized encoder for hashOrNumber to encode only one of the
// two contained union fields.
func (hn *hashOrNumber) EncodeRLP(w io.Writer) error {
	if hn.Hash == (common.Hash{}) {
		return rlp.Encode(w, hn.Number)
	}
	if hn.Number != 0 {
		return fmt.Errorf("both origin hash (%x) and number (%d) provided", hn.Hash, hn.Number)
	}
	return rlp.Encode(w, hn.Hash)
}

// DecodeRLP is a specialized decoder for hashOrNumber to decode the contents
// into either a block hash or a block number.
func (hn *hashOrNumber) DecodeRLP(s *rlp.Stream) error {
	_, size, _ := s.Kind()
	origin, err := s.Raw()
	if err == nil {
		switch {
		case size == 32:
			err = rlp.DecodeBytes(origin, &hn.Hash)
		case size <= 8:
			err = rlp.DecodeBytes(origin, &hn.Number)
		default:
			err = fmt.Errorf("invalid input size %d for origin", size)
		}
	}
	return err
}

// newBlockData is the network packet for the block propagation message.
type newBlockData struct {
	Block    *types.Block
	TD       *big.Int
	LampHash common.Hash
	LampTD   *big.Int
}

// blockBody represents the data content of a single block.
type blockBody struct {
	Transactions []*types.Transaction // Transactions contained within a block
	InputTxs     []*types.ShardingTxBatch
	Uncles       []*types.Header // Uncles contained within a block
	HbftStages   []*types.HBFTStageCompleted
}

// blockBodiesData is the network packet for block content distribution.
type blockBodiesData []*blockBody

type GetLastestEleData struct {
	LampHeight uint64
	EleHeight  uint64
}

type LastestEleData struct {
	EleHeight uint64
	EleHead   common.Hash
}
