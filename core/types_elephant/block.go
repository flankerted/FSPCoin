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

// Package types contains data types related to Ethereum consensus.
package types_elephant

import (
	"encoding/binary"
	"fmt"
	"io"
	"math/big"
	"reflect"
	"sort"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/common/hexutil"
	"github.com/contatract/go-contatract/crypto/sha3"
	"github.com/contatract/go-contatract/rlp"
)

var (
	EmptyRootHash    = DeriveSha(Transactions{})
	EmptyInputTxHash = DeriveSha(Transactions{})
	EmptyUncleHash   = CalcUncleHash(nil)

	EmptyHbftHash     = CalcBftHash(HBFTStageCompletedChain{})
	EmptyHbftStage    = &HBFTStageCompleted{}
	EmptyLampBaseHash = CalcLampBaseHash(common.Hash{})

	EmptyHeader = &Header{
		Number:         new(big.Int).SetUint64(0),
		Time:           new(big.Int).SetUint64(0),
		ParentHash:     EmptyRootHash,
		Extra:          []byte{},
		GasLimit:       0,
		GasUsed:        0,
		Difficulty:     new(big.Int).SetUint64(0),
		MixDigest:      EmptyRootHash,
		Coinbase:       common.Address{},
		Root:           EmptyRootHash,
		LampBaseHash:   EmptyLampBaseHash,
		LampBaseNumber: new(big.Int).SetUint64(0),
	}

	EmptyBlockHash = NewBlockWithHeader(EmptyHeader).Hash()
)

type HBFTStageCompleted struct {
	ViewID       uint64      `json:"viewID"`
	SequenceID   uint64      `json:"sequenceID"`
	BlockHash    common.Hash `json:"blockHash"`
	BlockNum     uint64      `json:"blockNum"`
	MsgType      uint        `json:"msgHBFTType"`
	MsgHash      common.Hash `json:"msgHash"`
	ValidSigns   [][]byte    `json:"validSigns"`
	ParentStage  common.Hash `json:"parentStage"`
	Timestamp    uint64      `json:"timestamp"`
	ReqTimeStamp uint64      `json:"reqTimeStamp"`
}

func (stage *HBFTStageCompleted) Hash() common.Hash {
	return rlpHash([]interface{}{
		stage.ViewID,
		stage.SequenceID,
		stage.BlockHash,
		stage.BlockNum,
		stage.MsgType,
		stage.MsgHash,
		stage.ValidSigns,
		stage.ParentStage,
		stage.Timestamp,
		stage.ReqTimeStamp,
	})
}

func (stage *HBFTStageCompleted) Size() common.StorageSize {
	size := common.StorageSize(reflect.TypeOf(stage.ViewID).Size()) +
		common.StorageSize(reflect.TypeOf(stage.SequenceID).Size()) +
		common.StorageSize(common.HashLength) +
		common.StorageSize(reflect.TypeOf(stage.BlockNum).Size()) +
		common.StorageSize(reflect.TypeOf(stage.MsgType).Size()) +
		common.StorageSize(common.HashLength) + // MsgHash
		common.StorageSize(common.HashLength) + // ParentStage
		common.StorageSize(reflect.TypeOf(stage.Timestamp).Size()) +
		common.StorageSize(reflect.TypeOf(stage.ReqTimeStamp).Size())

	for i := range stage.ValidSigns {
		size += common.StorageSize(len(stage.ValidSigns[i]))
	}

	return common.StorageSize(size)
}

func (stage *HBFTStageCompleted) Fake(viewID, SequenceID uint64) []*HBFTStageCompleted {
	completed1 := &HBFTStageCompleted{
		ViewID:      viewID,
		SequenceID:  SequenceID,
		BlockHash:   EmptyRootHash,
		BlockNum:    uint64(0),
		MsgType:     uint(0),
		MsgHash:     EmptyRootHash,
		ValidSigns:  nil,
		ParentStage: EmptyRootHash,
	}
	completed2 := &HBFTStageCompleted{
		ViewID:      viewID,
		SequenceID:  SequenceID,
		BlockHash:   EmptyRootHash,
		BlockNum:    uint64(0),
		MsgType:     uint(1),
		MsgHash:     EmptyRootHash,
		ValidSigns:  nil,
		ParentStage: EmptyRootHash,
	}

	completed := make([]*HBFTStageCompleted, 0)
	completed = append(completed, completed1)
	completed = append(completed, completed2)

	return completed
}

func (stage *HBFTStageCompleted) String() string {
	return fmt.Sprintf("[viewID: %d, seqID: %d, block: %s, msgType: %d\nhash: %s, parentStage: %s]",
		stage.ViewID, stage.SequenceID, stage.BlockHash.TerminalString(), stage.MsgType, stage.Hash().String(), stage.ParentStage.String())
}

// HBFTStageCompletedChain is a HBFTStageCompleted slice type for basic sorting.
type HBFTStageCompletedChain []*HBFTStageCompleted

// Len returns the length of s.
func (s HBFTStageCompletedChain) Len() int { return len(s) }

// Swap swaps the i'th and the j'th element in s.
func (s HBFTStageCompletedChain) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// GetRlp implements Rlpable and returns the i'th element of s in rlp.
func (s HBFTStageCompletedChain) GetRlp(i int) []byte {
	enc, _ := rlp.EncodeToBytes(s[i])
	return enc
}

// A BlockNonce is a 64-bit hash which proves (combined with the
// mix-hash) that a sufficient amount of computation has been carried
// out on a block.
type BlockNonce [8]byte

// EncodeNonce converts the given integer to a block nonce.
func EncodeNonce(i uint64) BlockNonce {
	var n BlockNonce
	binary.BigEndian.PutUint64(n[:], i)
	return n
}

// Uint64 returns the integer value of a block nonce.
func (n BlockNonce) Uint64() uint64 {
	return binary.BigEndian.Uint64(n[:])
}

// MarshalText encodes n as a hex string with 0x prefix.
func (n BlockNonce) MarshalText() ([]byte, error) {
	return hexutil.Bytes(n[:]).MarshalText()
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (n *BlockNonce) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedText("BlockNonce", input, n[:])
}

//go:generate gencodec -type Header -field-override headerMarshaling -out gen_header_json.go

// Header represents a block header in the Ethereum blockchain.
type Header struct {
	ParentHash common.Hash `json:"parentHash"       gencodec:"required"`
	//UncleHash       common.Hash    `json:"sha3Uncles"       gencodec:"required"`
	Coinbase       common.Address `json:"miner"            gencodec:"required"`
	Root           common.Hash    `json:"stateRoot"        gencodec:"required"`
	TxHash         common.Hash    `json:"transactionsRoot" gencodec:"required"`
	InputTxHash    common.Hash    `json:"inputTxsRoot"     gencodec:"required"`
	OutputTxHash   common.Hash    `json:"outputTxsRoot"    gencodec:"required"`
	ReceiptHash    common.Hash    `json:"receiptsRoot"     gencodec:"required"`
	Difficulty     *big.Int       `json:"difficulty"       gencodec:"required"`
	Number         *big.Int       `json:"number"           gencodec:"required"`
	GasLimit       uint64         `json:"gasLimit"         gencodec:"required"`
	GasUsed        uint64         `json:"gasUsed"          gencodec:"required"`
	Time           *big.Int       `json:"timestamp"        gencodec:"required"`
	Extra          []byte         `json:"extraData"        gencodec:"required"`
	MixDigest      common.Hash    `json:"mixHash"          gencodec:"required"`
	LampBaseHash   common.Hash    `json:"lampBaseHash"     gencodec:"required"`
	LampBaseNumber *big.Int       `json:"lampBaseNumber"   gencodec:"required"`
	BftHash        common.Hash    `json:"bftRoot"          gencodec:"required"`
}

// field type overrides for gencodec
type headerMarshaling struct {
	Difficulty *hexutil.Big
	Number     *hexutil.Big
	GasLimit   hexutil.Uint64
	GasUsed    hexutil.Uint64
	Time       *hexutil.Big
	Extra      hexutil.Bytes
	Hash       common.Hash `json:"hash"` // adds call to Hash() in MarshalJSON
}

// Hash returns the block hash of the header, which is simply the keccak256 hash of its
// RLP encoding.
func (h *Header) Hash() common.Hash {
	return rlpHash([]interface{}{h.HashNoOutTxRoot(), h.OutputTxHash})
}

func (h *Header) HashNoOutTxRoot() common.Hash {
	return rlpHash([]interface{}{
		h.ParentHash,
		h.Coinbase,
		h.Root,
		h.TxHash,
		h.InputTxHash,
		h.ReceiptHash,
		h.Difficulty,
		h.Number,
		h.GasLimit,
		h.GasUsed,
		h.Time,
		h.Extra,
		h.MixDigest,
		h.LampBaseHash,
		h.LampBaseNumber,
		//h.BftHash,
	})
}

func (h *Header) HashWithBft() common.Hash {
	return rlpHash([]interface{}{
		h.ParentHash,
		h.Coinbase,
		h.Root,
		h.TxHash,
		h.InputTxHash,
		h.OutputTxHash,
		h.ReceiptHash,
		h.Difficulty,
		h.Number,
		h.GasLimit,
		h.GasUsed,
		h.Time,
		h.Extra,
		h.MixDigest,
		h.LampBaseHash,
		h.LampBaseNumber,
		h.BftHash,
	})
}

// Size returns the approximate memory used by all internal contents. It is used
// to approximate and limit the memory consumption of various caches.
func (h *Header) Size() common.StorageSize {
	return common.StorageSize(unsafe.Sizeof(*h)) + common.StorageSize(len(h.Extra)+(h.Difficulty.BitLen()+h.Number.BitLen()+h.Time.BitLen()+h.LampBaseNumber.BitLen())/8)
}

func (h *Header) LampNumberU64() uint64 { return h.LampBaseNumber.Uint64() }

func rlpHash(x interface{}) (h common.Hash) {
	hw := sha3.NewKeccak256()
	// JiangHan: 意思是将x首先进行rlp编码，然后将编码输入 hw 进行散列，用hw 的散列输出当成结果
	rlp.Encode(hw, x)
	hw.Sum(h[:0])
	return h
}

// Body is a simple (mutable, non-safe) data container for storing and moving
// a block's data contents (transactions and uncles) together.
type Body struct {
	Transactions []*Transaction
	InputTxs     []*ShardingTxBatch
	Uncles       []*Header
	HbftStages   []*HBFTStageCompleted
}

// Block represents an entire block in the Ethereum blockchain.
type Block struct {
	header       *Header
	uncles       []*Header
	transactions Transactions
	inputTxs     []*ShardingTxBatch

	// caches
	hash atomic.Value
	size atomic.Value

	// These fields are used by package eth to track
	// inter-peer block relay.
	ReceivedAt   time.Time
	ReceivedFrom interface{}

	// HBFT consensus information
	hbftStages HBFTStageCompletedChain
}

// [deprecated by eth/63]
// StorageBlock defines the RLP encoding of a Block stored in the
// state database. The StorageBlock encoding contains fields that
// would otherwise need to be recomputed.
type StorageBlock Block

// "external" block encoding. used for eth protocol, etc.
type extblock struct {
	Header     *Header
	Txs        []*Transaction
	InputTxs   []*ShardingTxBatch
	Uncles     []*Header
	HbftStages []*HBFTStageCompleted
}

// [deprecated by eth/63]
// "storage" block encoding. used for database.
type storageblock struct {
	Header     *Header
	Txs        []*Transaction
	InputTxs   []*ShardingTxBatch
	Uncles     []*Header
	HbftStages []*HBFTStageCompleted
}

func GetOutTxsByShardingID(ethHeight uint64, outPutTxs Transactions) map[uint16]Transactions {
	txsByShardID := make(map[uint16]Transactions)
	for _, tx := range outPutTxs {
		shardID := common.GetSharding(*tx.To(), ethHeight)
		txsByShardID[shardID] = append(txsByShardID[shardID], tx)
	}

	return txsByShardID
}

func DeriveOutTxsRoot(ethHeight uint64, outPutTxs Transactions) common.Hash {
	txsByShardID := GetOutTxsByShardingID(ethHeight, outPutTxs)
	txsHashesByShardID := make(map[uint16]common.Hash)
	for id, txs := range txsByShardID {
		txsHashesByShardID[id] = DeriveSha(txs)
	}
	return DeriveHashForOutTxs(txsHashesByShardID)
}

// NewBlock creates a new block. The input data is copied,
// changes to header and to the field values will not affect the
// block.
//
// The values of TxHash, UncleHash, ReceiptHash and Bloom in header
// are ignored and set to values derived from the given txs, uncles
// and receipts.
func NewBlock(header *Header, txs []*Transaction, inputTxs []*ShardingTxBatch, uncles []*Header, receipts []*Receipt,
	hbftStages []*HBFTStageCompleted, eBase *common.Address) *Block {
	b := &Block{header: CopyHeader(header)}

	// TODO: panic if len(txs) != len(receipts)
	if len(txs) == 0 {
		b.header.TxHash = EmptyRootHash
	} else {
		b.header.TxHash = DeriveSha(Transactions(txs))
		b.transactions = make(Transactions, len(txs))
		copy(b.transactions, txs)
	}

	lampBaseNum := header.LampBaseNumber
	outputTxs := make(Transactions, 0)
	if lampBaseNum != nil {
		outputTxs = getOutputTxs(txs, lampBaseNum.Uint64(), eBase)
	}
	if len(outputTxs) == 0 {
		b.header.OutputTxHash = EmptyRootHash
	} else {
		b.header.OutputTxHash = DeriveOutTxsRoot(header.LampBaseNumber.Uint64(), outputTxs)
	}

	if len(inputTxs) == 0 {
		b.header.InputTxHash = EmptyInputTxHash
	} else {
		b.header.InputTxHash = DeriveSha(ShardingTxBatches(inputTxs))
		b.inputTxs = make([]*ShardingTxBatch, len(inputTxs))
		copy(b.inputTxs, inputTxs)
	}

	if len(receipts) == 0 {
		b.header.ReceiptHash = EmptyRootHash
	} else {
		b.header.ReceiptHash = DeriveSha(Receipts(receipts))
		//b.header.Bloom = CreateBloom(receipts)
	}

	//if len(uncles) == 0 {
	//	b.header.UncleHash = EmptyUncleHash
	//} else {
	//	b.header.UncleHash = CalcUncleHash(uncles)
	//	b.uncles = make([]*Header, len(uncles))
	//	for i := range uncles {
	//		b.uncles[i] = CopyHeader(uncles[i])
	//	}
	//}

	if len(hbftStages) == 0 {
		b.header.BftHash = EmptyHbftHash
	} else {
		b.header.BftHash = DeriveSha(HBFTStageCompletedChain(hbftStages))
		b.hbftStages = make([]*HBFTStageCompleted, len(hbftStages))
		copy(b.hbftStages, hbftStages)
	}

	return b
}

// NewBlockWithHeader creates a block with the given header data. The
// header data is copied, changes to header and to the field values
// will not affect the block.
func NewBlockWithHeader(header *Header) *Block {
	return &Block{header: CopyHeader(header)}
}

// CopyHeader creates a deep copy of a block header to prevent side effects from
// modifying a header variable.
func CopyHeader(h *Header) *Header {
	cpy := *h
	if cpy.Time = new(big.Int); h.Time != nil {
		cpy.Time.Set(h.Time)
	}
	if cpy.Difficulty = new(big.Int); h.Difficulty != nil {
		cpy.Difficulty.Set(h.Difficulty)
	}
	if cpy.Number = new(big.Int); h.Number != nil {
		cpy.Number.Set(h.Number)
	}
	if len(h.Extra) > 0 {
		cpy.Extra = make([]byte, len(h.Extra))
		copy(cpy.Extra, h.Extra)
	}
	if cpy.LampBaseNumber = new(big.Int); h.LampBaseNumber != nil {
		cpy.LampBaseNumber.Set(h.LampBaseNumber)
	}
	return &cpy
}

// DecodeRLP decodes the Ethereum
func (b *Block) DecodeRLP(s *rlp.Stream) error {
	var eb extblock
	_, size, _ := s.Kind()
	if err := s.Decode(&eb); err != nil {
		return err
	}
	b.header, b.uncles, b.transactions, b.inputTxs, b.hbftStages = eb.Header, eb.Uncles, eb.Txs, eb.InputTxs, eb.HbftStages
	b.size.Store(common.StorageSize(rlp.ListSize(size)))
	return nil
}

// EncodeRLP serializes b into the Ethereum RLP block format.
func (b *Block) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, extblock{
		Header:     b.header,
		Txs:        b.transactions,
		InputTxs:   b.inputTxs,
		Uncles:     b.uncles,
		HbftStages: b.hbftStages,
	})
}

// [deprecated by eth/63]
func (b *StorageBlock) DecodeRLP(s *rlp.Stream) error {
	var sb storageblock
	if err := s.Decode(&sb); err != nil {
		return err
	}
	b.header, b.uncles, b.transactions, b.inputTxs, b.hbftStages = sb.Header, sb.Uncles, sb.Txs, sb.InputTxs, sb.HbftStages
	return nil
}

// TODO: copies

func (b *Block) Uncles() []*Header          { return b.uncles }
func (b *Block) Transactions() Transactions { return b.transactions }
func (b *Block) LocalTxs(eBase *common.Address) Transactions {
	return getLocalTxs(b.transactions, b.LampBaseNumberU64(), eBase)
}
func (b *Block) InputTxs() ShardingTxBatches { return b.inputTxs }
func (b *Block) OutputTxs(eBase *common.Address) Transactions {
	return getOutputTxs(b.transactions, b.LampBaseNumberU64(), eBase)
}
func (b *Block) HBFTStageChain() HBFTStageCompletedChain { return b.hbftStages }

func getLocalTxs(txs Transactions, ethHeight uint64, eBase *common.Address) Transactions {
	ret := make(Transactions, 0, len(txs))
	for _, tx := range txs {
		if !tx.IsLocalTx(ethHeight, eBase) {
			continue
		}
		ret = append(ret, tx)
	}
	return ret
}

func getOutputTxs(txs Transactions, ethHeight uint64, eBase *common.Address) Transactions {
	ret := make(Transactions, 0, len(txs))
	for _, tx := range txs {
		if !tx.IsOutputTx(ethHeight, eBase) {
			continue
		}
		ret = append(ret, tx)
	}
	return ret
}

func (b *Block) Transaction(hash common.Hash) *Transaction {
	for _, transaction := range b.transactions {
		if transaction.Hash() == hash {
			return transaction
		}
	}
	return nil
}

func (b *Block) Number() *big.Int     { return new(big.Int).Set(b.header.Number) }
func (b *Block) GasLimit() uint64     { return b.header.GasLimit }
func (b *Block) GasUsed() uint64      { return b.header.GasUsed }
func (b *Block) Difficulty() *big.Int { return new(big.Int).Set(b.header.Difficulty) }
func (b *Block) Time() *big.Int       { return new(big.Int).Set(b.header.Time) }

func (b *Block) NumberU64() uint64         { return b.header.Number.Uint64() }
func (b *Block) LampBaseNumber() *big.Int  { return b.header.LampBaseNumber }
func (b *Block) LampBaseNumberU64() uint64 { return b.header.LampBaseNumber.Uint64() }
func (b *Block) LampHash() common.Hash     { return b.header.LampBaseHash }
func (b *Block) MixDigest() common.Hash    { return b.header.MixDigest }

//func (b *Block) Bloom() Bloom             { return b.header.Bloom }
func (b *Block) Coinbase() common.Address { return b.header.Coinbase }
func (b *Block) Root() common.Hash        { return b.header.Root }
func (b *Block) ParentHash() common.Hash  { return b.header.ParentHash }
func (b *Block) TxHash() common.Hash      { return b.header.TxHash }
func (b *Block) OutTxHash() common.Hash   { return b.header.OutputTxHash }
func (b *Block) ReceiptHash() common.Hash { return b.header.ReceiptHash }
func (b *Block) Extra() []byte            { return common.CopyBytes(b.header.Extra) }
func (b *Block) CurrentShard() uint16     { return common.GetSharding(b.Coinbase(), b.LampBaseNumberU64()) }

func (b *Block) Header() *Header { return CopyHeader(b.header) }

// Body returns the non-header content of the block.
func (b *Block) Body() *Body {
	return &Body{b.transactions, b.inputTxs, b.uncles, b.hbftStages}
}

func (b *Block) HashNoOutTxRoot() common.Hash {
	return b.header.HashNoOutTxRoot()
}

func (b *Block) HashWithBft() common.Hash {
	return b.header.HashWithBft()
}

// Size returns the true RLP encoded storage size of the block, either by encoding
// and returning it, or returning a previsouly cached value.
func (b *Block) Size() common.StorageSize {
	if size := b.size.Load(); size != nil {
		return size.(common.StorageSize)
	}
	c := writeCounter(0)
	rlp.Encode(&c, b)
	b.size.Store(common.StorageSize(c))
	return common.StorageSize(c)
}

type writeCounter common.StorageSize

func (c *writeCounter) Write(b []byte) (int, error) {
	*c += writeCounter(len(b))
	return len(b), nil
}

func CalcUncleHash(uncles []*Header) common.Hash {
	return rlpHash(uncles)
}

func CalcBftHash(hbftStages HBFTStageCompletedChain) common.Hash {
	return DeriveSha(hbftStages)
}

func CalcLampBaseHash(lampBaseHash common.Hash) common.Hash {
	return rlpHash(lampBaseHash)
}

// WithSeal returns a new block with the data from b but the header replaced with
// the sealed one.
func (b *Block) WithSeal(header *Header) *Block {
	cpy := *header

	return &Block{
		header:       &cpy,
		transactions: b.transactions,
		inputTxs:     b.inputTxs,
		uncles:       b.uncles,
		hbftStages:   b.hbftStages,
	}
}

// WithBody returns a new block with the given transaction and uncle contents.
func (b *Block) WithBody(transactions []*Transaction, inputTxs []*ShardingTxBatch, uncles []*Header, hbftStages []*HBFTStageCompleted) *Block {
	block := &Block{
		header:       CopyHeader(b.header),
		transactions: make([]*Transaction, len(transactions)),
		inputTxs:     make([]*ShardingTxBatch, len(inputTxs)),
		uncles:       make([]*Header, len(uncles)),
		hbftStages:   make([]*HBFTStageCompleted, len(hbftStages)),
	}
	copy(block.transactions, transactions)
	copy(block.inputTxs, inputTxs)
	for i := range uncles {
		block.uncles[i] = CopyHeader(uncles[i])
	}
	copy(block.hbftStages, hbftStages)
	return block
}

func (b *Block) SetBftStageComplete(hbftStages HBFTStageCompletedChain) {
	b.header.BftHash = DeriveSha(hbftStages)
	b.hbftStages = hbftStages
}

// Hash returns the keccak256 hash of b's header.
// The hash is computed on the first call and cached thereafter.
func (b *Block) Hash() common.Hash {
	if hash := b.hash.Load(); hash != nil {
		return hash.(common.Hash)
	}
	v := b.header.Hash()
	b.hash.Store(v)
	return v
}

func (b *Block) String() string {
	str := fmt.Sprintf(`Block(#%v): Size: %v {
MinerHash: %x
%v
Transactions:
%v
InputTxs:
%v
Uncles:
%v
HBFTStageChain:
%v
}
`, b.Number(), b.Size(), b.header.Hash(), b.header, b.transactions, b.inputTxs, b.uncles, b.hbftStages)
	return str
}

func (h *Header) String() string {
	return fmt.Sprintf(`Header(%x):
[
	ParentHash:	    %x
	Coinbase:	    %x
	Root:		    %x
	TxSha		    %x
	ReceiptSha:	    %x
	Difficulty:	    %v
	Number:		    %v
	GasLimit:	    %v
	GasUsed:	    %v
	Time:		    %v
	Extra:		    %s
	LampBaseHash:    %x
	LampBaseNumber:  %v
	BftHash:       %x
	MixDigest:      %x
]`, h.Hash(), h.ParentHash, h.Coinbase, h.Root, h.TxHash, h.ReceiptHash, h.Difficulty, h.Number, h.GasLimit, h.GasUsed, h.Time, h.Extra, h.LampBaseHash, h.LampBaseNumber, h.BftHash, h.MixDigest)
}

type Blocks []*Block

type BlockBy func(b1, b2 *Block) bool

func (self BlockBy) Sort(blocks Blocks) {
	bs := blockSorter{
		blocks: blocks,
		by:     self,
	}
	sort.Sort(bs)
}

type blockSorter struct {
	blocks Blocks
	by     func(b1, b2 *Block) bool
}

func (self blockSorter) Len() int { return len(self.blocks) }
func (self blockSorter) Swap(i, j int) {
	self.blocks[i], self.blocks[j] = self.blocks[j], self.blocks[i]
}
func (self blockSorter) Less(i, j int) bool { return self.by(self.blocks[i], self.blocks[j]) }

func Number(b1, b2 *Block) bool { return b1.header.Number.Cmp(b2.header.Number) < 0 }
