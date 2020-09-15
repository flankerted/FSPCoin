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
package types

import (
	"encoding/binary"
	"fmt"
	"io"
	"math/big"
	"sort"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/contatract/go-contatract/baseparam"
	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/common/hexutil"
	"github.com/contatract/go-contatract/crypto/sha3"
	"github.com/contatract/go-contatract/rlp"
)

var (
	EmptyRootHash  = DeriveSha(Transactions{})
	EmptyUncleHash = CalcUncleHash(nil)

	EmptyElector     = make([]common.AddressNode, 0)
	EmptyElectorHash = DeriveSha(NextEleSealersList(EmptyElector))

	EmptyValidHeight     = make([]*VerifiedValid, 0)
	EmptyValidHeightHash = DeriveSha(VerifiedValids(EmptyValidHeight))
)

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
	ParentHash      common.Hash    `json:"parentHash"       gencodec:"required"`
	UncleHash       common.Hash    `json:"sha3Uncles"       gencodec:"required"`
	Coinbase        common.Address `json:"miner"            gencodec:"required"`
	Root            common.Hash    `json:"stateRoot"        gencodec:"required"`
	EleSealersHash  common.Hash    `json:"eleSealersRoot"   gencodec:"required"`
	PoliceHash      common.Hash    `json:"policeRoot"       gencodec:"required"`
	ValidHeightHash common.Hash    `json:"ValidHeightRoot"  gencodec:"required"`
	TxHash          common.Hash    `json:"transactionsRoot" gencodec:"required"`
	ReceiptHash     common.Hash    `json:"receiptsRoot"     gencodec:"required"`
	Bloom           Bloom          `json:"logsBloom"        gencodec:"required"`
	Difficulty      *big.Int       `json:"difficulty"       gencodec:"required"`
	Number          *big.Int       `json:"number"           gencodec:"required"`
	GasLimit        uint64         `json:"gasLimit"         gencodec:"required"`
	GasUsed         uint64         `json:"gasUsed"          gencodec:"required"`
	Time            *big.Int       `json:"timestamp"        gencodec:"required"`
	Extra           []byte         `json:"extraData"        gencodec:"required"`
	MixDigest       common.Hash    `json:"mixHash"          gencodec:"required"`
	Nonce           BlockNonce     `json:"nonce"            gencodec:"required"`
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
	return rlpHash(h)
}

// HashNoNonce returns the hash which is used as input for the proof-of-work search.
func (h *Header) HashNoNonce() common.Hash {
	return rlpHash([]interface{}{
		h.ParentHash,
		h.UncleHash,
		h.Coinbase,
		h.Root,
		h.TxHash,
		h.EleSealersHash,
		h.PoliceHash,
		h.ValidHeightHash,
		h.ReceiptHash,
		h.Bloom,
		h.Difficulty,
		h.Number,
		h.GasLimit,
		h.GasUsed,
		h.Time,
		h.Extra,
	})
}

func (h *Header) IsElectionBlock() bool { return h.Number.Int64()%baseparam.ElectionBlockCount == 0 }

// Size returns the approximate memory used by all internal contents. It is used
// to approximate and limit the memory consumption of various caches.
func (h *Header) Size() common.StorageSize {
	return common.StorageSize(unsafe.Sizeof(*h)) + common.StorageSize(len(h.Extra)+(h.Difficulty.BitLen()+h.Number.BitLen()+h.Time.BitLen())/8)
}

func rlpHash(x interface{}) (h common.Hash) {
	hw := sha3.NewKeccak256()
	// JiangHan: 意思是将x首先进行rlp编码，然后将编码输入 hw 进行散列，用hw 的散列输出当成结果
	rlp.Encode(hw, x)
	hw.Sum(h[:0])
	return h
}

// VerifiedValid is a simple data container for the verified block header by over half of police
type VerifiedValid struct {
	ShardingID      uint16         // The elephant sharding ID of the block
	BlockHeight     uint64         // The elephant block height
	LampBase        *common.Hash   // The lampBase hash which the elephant block depends on
	Header          *common.Hash   // The header hash of the elephant block
	PreviousHeaders []*common.Hash // Hashes of the verified headers from last to this one (index 0 represents the hash in last election block)

	Signs []string // The signatures of police officers who verified the block

	// caches
	size atomic.Value
}

func (v *VerifiedValid) String() string {
	return fmt.Sprintf(`[ ShardingID: %x, BlockHeight: %d ]`, v.ShardingID, v.BlockHeight)
}

func (v *VerifiedValid) Size() common.StorageSize {
	if size := v.size.Load(); size != nil {
		return size.(common.StorageSize)
	}
	c := writeCounter(0)
	rlp.Encode(&c, v)
	v.size.Store(common.StorageSize(c))
	return common.StorageSize(c)
}

type VerifiedValids []*VerifiedValid

func (s VerifiedValids) Len() int { return len(s) }

func (s VerifiedValids) GetRlp(i int) []byte {
	enc, _ := rlp.EncodeToBytes(s[i])
	return enc
}

// Body is a simple (mutable, non-safe) data container for storing and moving
// a block's data contents (transactions and uncles) together.
type Body struct {
	Transactions   []*Transaction
	NextEleSealers []common.AddressNode
	NextPolice     []common.AddressNode
	ValidHeights   []*VerifiedValid
	Uncles         []*Header
}

// Block represents an entire block in the Ethereum blockchain.
type Block struct {
	header       *Header
	uncles       []*Header
	transactions Transactions

	// Elections result
	nextEleSealers []common.AddressNode
	nextPolice     []common.AddressNode

	// validHeights is the newest block height verified by the police in different shards
	validHeights []*VerifiedValid

	// caches
	hash atomic.Value
	size atomic.Value

	// Td is used by package core to store the total difficulty
	// of the chain up to and including the block.
	td *big.Int

	// These fields are used by package eth to track
	// inter-peer block relay.
	ReceivedAt   time.Time
	ReceivedFrom interface{}
}

// DeprecatedTd is an old relic for extracting the TD of a block. It is in the
// code solely to facilitate upgrading the database from the old format to the
// new, after which it should be deleted. Do not use!
func (b *Block) DeprecatedTd() *big.Int {
	return b.td
}

// [deprecated by eth/63]
// StorageBlock defines the RLP encoding of a Block stored in the
// state database. The StorageBlock encoding contains fields that
// would otherwise need to be recomputed.
type StorageBlock Block

// "external" block encoding. used for eth protocol, etc.
type extblock struct {
	Header         *Header
	Txs            []*Transaction
	NextEleSealers []common.AddressNode
	NextPolice     []common.AddressNode
	ValidHeights   []*VerifiedValid
	Uncles         []*Header
}

// [deprecated by eth/63]
// "storage" block encoding. used for database.
type storageblock struct {
	Header         *Header
	Txs            []*Transaction
	NextEleSealers []common.AddressNode
	NextPolice     []common.AddressNode
	ValidHeights   []*VerifiedValid
	Uncles         []*Header
	TD             *big.Int
}

// NewBlock creates a new block. The input data is copied,
// changes to header and to the field values will not affect the
// block.
//
// The values of TxHash, UncleHash, ReceiptHash and Bloom in header
// are ignored and set to values derived from the given txs, uncles
// and receipts.
func NewBlock(header *Header, txs []*Transaction, uncles []*Header, receipts []*Receipt) *Block {
	b := &Block{header: CopyHeader(header), td: new(big.Int)}

	// TODO: panic if len(txs) != len(receipts)
	if len(txs) == 0 {
		b.header.TxHash = EmptyRootHash
	} else {
		b.header.TxHash = DeriveSha(Transactions(txs))
		b.transactions = make(Transactions, len(txs))
		copy(b.transactions, txs)
	}

	if len(receipts) == 0 {
		b.header.ReceiptHash = EmptyRootHash
	} else {
		b.header.ReceiptHash = DeriveSha(Receipts(receipts))
		b.header.Bloom = CreateBloom(receipts)
	}

	if len(uncles) == 0 {
		b.header.UncleHash = EmptyUncleHash
	} else {
		b.header.UncleHash = CalcUncleHash(uncles)
		b.uncles = make([]*Header, len(uncles))
		for i := range uncles {
			b.uncles[i] = CopyHeader(uncles[i])
		}
	}

	b.nextEleSealers = EmptyElector
	b.header.EleSealersHash = EmptyElectorHash
	b.nextPolice = EmptyElector
	b.header.PoliceHash = EmptyElectorHash
	b.validHeights = EmptyValidHeight
	b.header.ValidHeightHash = EmptyValidHeightHash

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
	return &cpy
}

// DecodeRLP decodes the Ethereum
func (b *Block) DecodeRLP(s *rlp.Stream) error {
	var eb extblock
	_, size, _ := s.Kind()
	if err := s.Decode(&eb); err != nil {
		return err
	}
	b.header, b.uncles, b.transactions, b.nextEleSealers, b.nextPolice, b.validHeights = eb.Header, eb.Uncles, eb.Txs, eb.NextEleSealers, eb.NextPolice, eb.ValidHeights
	b.size.Store(common.StorageSize(rlp.ListSize(size)))
	return nil
}

// EncodeRLP serializes b into the Ethereum RLP block format.
func (b *Block) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, extblock{
		Header:         b.header,
		Txs:            b.transactions,
		NextEleSealers: b.nextEleSealers,
		NextPolice:     b.nextPolice,
		ValidHeights:   b.validHeights,
		Uncles:         b.uncles,
	})
}

// [deprecated by eth/63]
func (b *StorageBlock) DecodeRLP(s *rlp.Stream) error {
	var sb storageblock
	if err := s.Decode(&sb); err != nil {
		return err
	}
	b.header, b.uncles, b.transactions, b.nextEleSealers, b.nextPolice, b.validHeights, b.td = sb.Header, sb.Uncles, sb.Txs, sb.NextEleSealers, sb.NextPolice, sb.ValidHeights, sb.TD
	return nil
}

// TODO: copies

func (b *Block) Uncles() []*Header          { return b.uncles }
func (b *Block) Transactions() Transactions { return b.transactions }

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

func (b *Block) NumberU64() uint64                 { return b.header.Number.Uint64() }
func (b *Block) MixDigest() common.Hash            { return b.header.MixDigest }
func (b *Block) Nonce() uint64                     { return binary.BigEndian.Uint64(b.header.Nonce[:]) }
func (b *Block) Bloom() Bloom                      { return b.header.Bloom }
func (b *Block) Coinbase() common.Address          { return b.header.Coinbase }
func (b *Block) Root() common.Hash                 { return b.header.Root }
func (b *Block) ParentHash() common.Hash           { return b.header.ParentHash }
func (b *Block) TxHash() common.Hash               { return b.header.TxHash }
func (b *Block) EleSealersHash() common.Hash       { return b.header.EleSealersHash }
func (b *Block) PoliceHash() common.Hash           { return b.header.PoliceHash }
func (b *Block) ValidHeightHash() common.Hash      { return b.header.ValidHeightHash }
func (b *Block) ReceiptHash() common.Hash          { return b.header.ReceiptHash }
func (b *Block) UncleHash() common.Hash            { return b.header.UncleHash }
func (b *Block) Extra() []byte                     { return common.CopyBytes(b.header.Extra) }
func (b *Block) IsElectionBlock() bool             { return b.NumberU64()%baseparam.ElectionBlockCount == 0 }
func (b *Block) NextSealers() []common.AddressNode { return b.nextEleSealers }
func (b *Block) NextPolice() []common.AddressNode  { return b.nextPolice }

func (b *Block) Header() *Header { return CopyHeader(b.header) }

// Body returns the non-header content of the block.
func (b *Block) Body() *Body {
	return &Body{b.transactions, b.nextEleSealers, b.nextPolice, b.validHeights, b.uncles}
}

func (b *Block) HashNoNonce() common.Hash {
	return b.header.HashNoNonce()
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

// WithSeal returns a new block with the data from b but the header replaced with
// the sealed one.
func (b *Block) WithSeal(header *Header) *Block {
	cpy := *header

	return &Block{
		header:         &cpy,
		transactions:   b.transactions,
		nextEleSealers: b.nextEleSealers,
		nextPolice:     b.nextPolice,
		validHeights:   b.validHeights,
		uncles:         b.uncles,
	}
}

// WithBody returns a new block with the given transaction and uncle contents.
func (b *Block) WithBody(transactions []*Transaction, uncles []*Header, nextEleSealers, nextPolice []common.AddressNode,
	validHeights []*VerifiedValid) *Block {
	block := &Block{
		header:         CopyHeader(b.header),
		transactions:   make([]*Transaction, len(transactions)),
		nextEleSealers: make([]common.AddressNode, 0),
		nextPolice:     make([]common.AddressNode, 0),
		validHeights:   make([]*VerifiedValid, 0),
		uncles:         make([]*Header, len(uncles)),
	}
	copy(block.transactions, transactions)
	for i := range uncles {
		block.uncles[i] = CopyHeader(uncles[i])
	}
	for i := range nextEleSealers {
		block.nextEleSealers = append(block.nextEleSealers, nextEleSealers[i])
	}
	for i := range nextPolice {
		block.nextPolice = append(block.nextPolice, nextPolice[i])
	}
	for i := range validHeights {
		block.validHeights = append(block.validHeights, validHeights[i])
	}
	return block
}

func (b *Block) SetNextEleElectors(newElectors []common.AddressNode, electionType uint8) {
	if electionType == baseparam.TypeSealersElection {
		b.nextEleSealers = newElectors
		b.header.EleSealersHash = DeriveSha(NextEleSealersList(newElectors))
	} else {
		b.nextPolice = newElectors
		b.header.PoliceHash = DeriveSha(NextPoliceList(newElectors))
	}
}

func (b *Block) SetValidHeaders(values []*VerifiedValid) {
	b.validHeights = values
	b.header.ValidHeightHash = DeriveSha(VerifiedValids(values))
}

func (b *Block) GetValidHeaders() map[uint16]*VerifiedValid {
	validHeights := make(map[uint16]*VerifiedValid)
	for _, verified := range b.validHeights {
		validHeights[verified.ShardingID] = verified
	}
	return validHeights
}

func (b *Block) GetValidHeadersSlice() []*VerifiedValid {
	return b.validHeights
}

func (b *Block) ShowValidHeaders() []uint64 {
	heights := make([]uint64, 0)
	validHeights := b.GetValidHeaders()
	for id := uint16(0); id < uint16(len(validHeights)); id++ {
		heights = append(heights, validHeights[id].BlockHeight)
	}
	return heights
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
NextEleSealers:
%v
NextPolice:
%v
ValidHeights:
%v
Uncles:
%v
}
`, b.Number(), b.Size(), b.header.HashNoNonce(), b.header, b.transactions, b.nextEleSealers, b.nextPolice, b.validHeights, b.uncles)
	return str
}

func (h *Header) String() string {
	return fmt.Sprintf(`Header(%x):
[
	ParentHash:	    %x
	UncleHash:	    %x
	Coinbase:	    %x
	Root:		    %x
	TxSha		    %x
	EleSealersSha   %x
	PoliceSha       %x
	ValidHeightSha  %x
	ReceiptSha:	    %x
	Bloom:		    %x
	Difficulty:	    %v
	Number:		    %v
	GasLimit:	    %v
	GasUsed:	    %v
	Time:		    %v
	Extra:		    %s
	MixDigest:      %x
	Nonce:		    %x
]`, h.Hash(), h.ParentHash, h.UncleHash, h.Coinbase, h.Root, h.TxHash, h.EleSealersHash, h.PoliceHash, h.ValidHeightHash, h.ReceiptHash, h.Bloom, h.Difficulty, h.Number, h.GasLimit, h.GasUsed, h.Time, h.Extra, h.MixDigest, h.Nonce)
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

type NextEleSealersList []common.AddressNode

func (s NextEleSealersList) Len() int { return len(s) }

func (s NextEleSealersList) GetRlp(i int) []byte {
	enc, _ := rlp.EncodeToBytes(s[i])
	return enc
}

type NextPoliceList []common.AddressNode

func (s NextPoliceList) Len() int { return len(s) }

func (s NextPoliceList) GetRlp(i int) []byte {
	enc, _ := rlp.EncodeToBytes(s[i])
	return enc
}
