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

package types_elephant

import (
	"container/heap"
	"errors"
	"fmt"
	"io"
	"math/big"
	"sync/atomic"
	"unsafe"

	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/common/hexutil"
	"github.com/contatract/go-contatract/crypto"
	"github.com/contatract/go-contatract/ethdb"
	"github.com/contatract/go-contatract/rlp"
	"github.com/contatract/go-contatract/trie"
)

//go:generate gencodec -type txdata -field-override txdataMarshaling -out gen_tx_json.go

var (
	ErrInvalidSig = errors.New("invalid transaction v, r, s values")
	errNoSigner   = errors.New("missing signing methods")
)

// deriveSigner makes a *best* guess about which signer to use.
func deriveSigner(V *big.Int) Signer {
	if V.Sign() != 0 && isProtectedV(V) {
		return NewEIP155Signer(deriveChainId(V))
	} else {
		return HomesteadSigner{}
	}
}

type Transaction struct {
	data txdata
	// caches
	hash atomic.Value
	size atomic.Value
	from atomic.Value
}

type txdata struct {
	AccountNonce uint64          `json:"nonce"    gencodec:"required"`
	Price        *big.Int        `json:"gasPrice" gencodec:"required"`
	GasLimit     uint64          `json:"gas"      gencodec:"required"`
	Recipient    *common.Address `json:"to"       rlp:"nil"` // nil means contract creation
	Amount       *big.Int        `json:"value"    gencodec:"required"`
	Payload      []byte          `json:"input"    gencodec:"required"`

	// Signature values
	V *big.Int `json:"v" gencodec:"required"`
	R *big.Int `json:"r" gencodec:"required"`
	S *big.Int `json:"s" gencodec:"required"`

	// This is only used when marshaling to JSON.
	Hash *common.Hash `json:"hash" rlp:"-"`

	ActType uint8  `json:"actType"`
	ActData []byte `json:"actData"`
}

type txdataMarshaling struct {
	AccountNonce hexutil.Uint64
	Price        *hexutil.Big
	GasLimit     hexutil.Uint64
	Amount       *hexutil.Big
	Payload      hexutil.Bytes
	V            *hexutil.Big
	R            *hexutil.Big
	S            *hexutil.Big
	ActType      uint8
	ActData      []byte
}

func NewTransaction(nonce uint64, to common.Address, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte, actType uint8, actData []byte) *Transaction {
	return newTransaction(nonce, &to, amount, gasLimit, gasPrice, data, actType, actData)
}

func NewContractCreation(nonce uint64, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte) *Transaction {
	return newTransaction(nonce, nil, amount, gasLimit, gasPrice, data, 0, nil)
}

func newTransaction(nonce uint64, to *common.Address, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte, actType uint8, actData []byte) *Transaction {
	if len(data) > 0 {
		data = common.CopyBytes(data)
	}
	d := txdata{
		AccountNonce: nonce,
		Recipient:    to,
		Payload:      data,
		Amount:       new(big.Int),
		GasLimit:     gasLimit,
		Price:        new(big.Int),
		V:            new(big.Int),
		R:            new(big.Int),
		S:            new(big.Int),
		ActType:      actType,
		ActData:      actData,
	}
	if amount != nil {
		d.Amount.Set(amount)
	}
	if gasPrice != nil {
		d.Price.Set(gasPrice)
	}
	return &Transaction{data: d}
}

// ChainId returns which chain id this transaction was signed for (if at all)
func (tx *Transaction) ChainId() *big.Int {
	return deriveChainId(tx.data.V)
}

// Protected returns whether the transaction is protected from replay protection.
func (tx *Transaction) Protected() bool {
	return isProtectedV(tx.data.V)
}

func isProtectedV(V *big.Int) bool {
	if V.BitLen() <= 8 {
		v := V.Uint64()
		return v != 27 && v != 28
	}
	// anything not 27 or 28 are considered unprotected
	return true
}

// EncodeRLP implements rlp.Encoder
func (tx *Transaction) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, &tx.data)
}

// DecodeRLP implements rlp.Decoder
func (tx *Transaction) DecodeRLP(s *rlp.Stream) error {
	_, size, _ := s.Kind()
	err := s.Decode(&tx.data)
	if err == nil {
		tx.size.Store(common.StorageSize(rlp.ListSize(size)))
	}

	return err
}

// MarshalJSON encodes the web3 RPC transaction format.
func (tx *Transaction) MarshalJSON() ([]byte, error) {
	hash := tx.Hash()
	data := tx.data
	data.Hash = &hash
	return data.MarshalJSON()
}

// UnmarshalJSON decodes the web3 RPC transaction format.
func (tx *Transaction) UnmarshalJSON(input []byte) error {
	var dec txdata
	if err := dec.UnmarshalJSON(input); err != nil {
		return err
	}
	var V byte
	if isProtectedV(dec.V) {
		chainID := deriveChainId(dec.V).Uint64()
		V = byte(dec.V.Uint64() - 35 - 2*chainID)
	} else {
		V = byte(dec.V.Uint64() - 27)
	}
	if !crypto.ValidateSignatureValues(V, dec.R, dec.S, false) {
		return ErrInvalidSig
	}
	*tx = Transaction{data: dec}
	return nil
}

func (tx *Transaction) Data() []byte            { return common.CopyBytes(tx.data.Payload) }
func (tx *Transaction) Gas() uint64             { return tx.data.GasLimit }
func (tx *Transaction) GasPrice() *big.Int      { return new(big.Int).Set(tx.data.Price) }
func (tx *Transaction) Value() *big.Int         { return new(big.Int).Set(tx.data.Amount) }
func (tx *Transaction) Nonce() uint64           { return tx.data.AccountNonce }
func (tx *Transaction) CheckNonce() bool        { return true }
func (tx *Transaction) ActType() uint8          { return tx.data.ActType }
func (tx *Transaction) ActData() []byte         { return common.CopyBytes(tx.data.ActData) }
func (tx *Transaction) IsFreeAct(h uint64) bool { return common.IsFreeAct(h, tx.data.ActType) }

func (tx *Transaction) From() *common.Address {
	var signer EIP155Signer
	from, err := signer.Sender(tx)
	if err != nil {
		return nil
	}
	return &from
}

// To returns the recipient address of the transaction.
// It returns nil if the transaction is a contract creation.
func (tx *Transaction) To() *common.Address {
	if tx.data.Recipient == nil {
		return nil
	}
	to := *tx.data.Recipient
	return &to
}

// Hash hashes the RLP encoding of tx.
// It uniquely identifies the transaction.
func (tx *Transaction) Hash() common.Hash {
	if hash := tx.hash.Load(); hash != nil {
		return hash.(common.Hash)
	}
	v := rlpHash(tx)
	tx.hash.Store(v)
	return v
}

// Size returns the true RLP encoded storage size of the transaction, either by
// encoding and returning it, or returning a previsouly cached value.
func (tx *Transaction) Size() common.StorageSize {
	if size := tx.size.Load(); size != nil {
		return size.(common.StorageSize)
	}
	c := writeCounter(0)
	rlp.Encode(&c, &tx.data)
	tx.size.Store(common.StorageSize(c))
	return common.StorageSize(c)
}

// AsMessage returns the transaction as a core.Message.
//
// AsMessage requires a signer to derive the sender.
//
// XXX Rename message to something less arbitrary?
func (tx *Transaction) AsMessage(s Signer) (Message, error) {
	msg := Message{
		nonce:      tx.data.AccountNonce,
		gasLimit:   tx.data.GasLimit,
		gasPrice:   new(big.Int).Set(tx.data.Price),
		to:         tx.data.Recipient,
		amount:     tx.data.Amount,
		data:       tx.data.Payload,
		checkNonce: true,
		actType:    tx.data.ActType,
		actData:    tx.data.ActData,
	}

	var err error
	msg.from, err = Sender(s, tx)
	return msg, err
}

// WithSignature returns a new transaction with the given signature.
// This signature needs to be formatted as described in the yellow paper (v+27).
func (tx *Transaction) WithSignature(signer Signer, sig []byte) (*Transaction, error) {
	r, s, v, err := signer.SignatureValues(tx, sig)
	if err != nil {
		return nil, err
	}
	cpy := &Transaction{data: tx.data}
	cpy.data.R, cpy.data.S, cpy.data.V = r, s, v
	return cpy, nil
}

// Cost returns amount + gasprice * gaslimit.
func (tx *Transaction) Cost() *big.Int {
	total := new(big.Int).Mul(tx.data.Price, new(big.Int).SetUint64(tx.data.GasLimit))
	total.Add(total, tx.data.Amount)
	return total
}

func (tx *Transaction) RawSignatureValues() (*big.Int, *big.Int, *big.Int) {
	return tx.data.V, tx.data.R, tx.data.S
}

func (tx *Transaction) String() string {
	var from, to string
	if tx.data.V != nil {
		// make a best guess about the signer and use that to derive
		// the sender.
		signer := deriveSigner(tx.data.V)
		if f, err := Sender(signer, tx); err != nil { // derive but don't cache
			from = "[invalid sender: invalid sig]"
		} else {
			from = fmt.Sprintf("%x", f[:])
		}
	} else {
		from = "[invalid sender: nil V field]"
	}

	if tx.data.Recipient == nil {
		to = "[contract creation]"
	} else {
		to = fmt.Sprintf("%x", tx.data.Recipient[:])
	}
	enc, _ := rlp.EncodeToBytes(&tx.data)
	hexLen := len(enc)
	if hexLen > 10 {
		hexLen = 10
	}
	actDataLen := len(tx.data.ActData)
	if actDataLen > 10 {
		actDataLen = 10
	}
	return fmt.Sprintf(`
	TX(%x)
	Contract: %v
	From:     %s
	To:       %s
	Nonce:    %v
	GasPrice: %#x
	GasLimit  %#x
	Value:    %#x
	Data:     0x%x
	V:        %#x
	R:        %#x
	S:        %#x
	Hex[:10]: %x
	ActType:%v
	ActData[:10]:%v
`,
		tx.Hash(),
		tx.data.Recipient == nil,
		from,
		to,
		tx.data.AccountNonce,
		tx.data.Price,
		tx.data.GasLimit,
		tx.data.Amount,
		tx.data.Payload,
		tx.data.V,
		tx.data.R,
		tx.data.S,
		enc[:hexLen],
		tx.data.ActType,
		tx.data.ActData[:actDataLen],
	)
}

// Transactions is a Transaction slice type for basic sorting.
type Transactions []*Transaction

// Len returns the length of s.
func (s Transactions) Len() int { return len(s) }

// Swap swaps the i'th and the j'th element in s.
func (s Transactions) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// GetRlp implements Rlpable and returns the i'th element of s in rlp.
func (s Transactions) GetRlp(i int) []byte {
	enc, _ := rlp.EncodeToBytes(s[i])
	return enc
}

// TxDifference returns a new set t which is the difference between a to b.
func TxDifference(a, b Transactions) (keep Transactions) {
	keep = make(Transactions, 0, len(a))

	// 将 b 集合中的 tx 都加入到一个 remove 组中，但只加入key值，却不用分配空间（用了空结构体struct{}{})
	remove := make(map[common.Hash]struct{})
	for _, tx := range b {
		remove[tx.Hash()] = struct{}{}
	}

	// 遍历 a 集合中的 tx 的 hash 值，看看是否出现在 remove 的键值对象中
	// 如果不在(!ok表示没有找到），就加入keep保留组
	for _, tx := range a {
		if _, ok := remove[tx.Hash()]; !ok {
			keep = append(keep, tx)
		}
	}

	// 最后返回剔除了a和 b 中重合的Tx的组
	return keep
}

// TxByNonce implements the sort interface to allow sorting a list of transactions
// by their nonces. This is usually only useful for sorting transactions from a
// single account, otherwise a nonce comparison doesn't make much sense.
type TxByNonce Transactions

func (s TxByNonce) Len() int           { return len(s) }
func (s TxByNonce) Less(i, j int) bool { return s[i].data.AccountNonce < s[j].data.AccountNonce }
func (s TxByNonce) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// TxByPrice implements both the sort and the heap interface, making it useful
// for all at once sorting as well as individually adding and removing elements.
type TxByPrice Transactions

func (s TxByPrice) Len() int           { return len(s) }
func (s TxByPrice) Less(i, j int) bool { return s[i].data.Price.Cmp(s[j].data.Price) > 0 }
func (s TxByPrice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (s *TxByPrice) Push(x interface{}) {
	*s = append(*s, x.(*Transaction))
}

func (s *TxByPrice) Pop() interface{} {
	old := *s
	n := len(old)
	x := old[n-1]
	*s = old[0 : n-1]
	return x
}

// TransactionsByPriceAndNonce represents a set of transactions that can return
// transactions in a profit-maximizing sorted order, while supporting removing
// entire batches of transactions for non-executable accounts.
type TransactionsByPriceAndNonce struct {
	txs    map[common.Address]Transactions // Per account nonce-sorted list of transactions
	heads  TxByPrice                       // Next transaction for each unique account (price heap)
	signer Signer                          // Signer for the set of transactions
}

// NewTransactionsByPriceAndNonce creates a transaction set that can retrieve
// price sorted transactions in a nonce-honouring way.
//
// Note, the input map is reowned so the caller should not interact any more with
// if after providing it to the constructor.
func NewTransactionsByPriceAndNonce(signer Signer, txs map[common.Address]Transactions) *TransactionsByPriceAndNonce {
	// Initialize a price based heap with the head transactions
	heads := make(TxByPrice, 0, len(txs))
	for _, accTxs := range txs {
		heads = append(heads, accTxs[0])
		// Ensure the sender address is from the signer
		acc, _ := Sender(signer, accTxs[0])
		txs[acc] = accTxs[1:]
	}
	heap.Init(&heads)

	// Assemble and return the transaction set
	return &TransactionsByPriceAndNonce{
		txs:    txs,
		heads:  heads,
		signer: signer,
	}
}

// Peek returns the next transaction by price.
func (t *TransactionsByPriceAndNonce) Peek() *Transaction {
	if len(t.heads) == 0 {
		return nil
	}
	return t.heads[0]
}

// Shift replaces the current best head with the next one from the same account.
func (t *TransactionsByPriceAndNonce) Shift() {
	acc, _ := Sender(t.signer, t.heads[0])
	if txs, ok := t.txs[acc]; ok && len(txs) > 0 {
		t.heads[0], t.txs[acc] = txs[0], txs[1:]
		heap.Fix(&t.heads, 0)
	} else {
		heap.Pop(&t.heads)
	}
}

// GetTransactions returns the total transactions
func (t *TransactionsByPriceAndNonce) GetTransactions() map[common.Address]Transactions {
	return t.txs
}

// Pop removes the best transaction, *not* replacing it with the next one from
// the same account. This should be used when a transaction cannot be executed
// and hence all subsequent ones should be discarded from the same account.
func (t *TransactionsByPriceAndNonce) Pop() {
	heap.Pop(&t.heads)
}

// ShardingTxBatch are all mined sharding transactions that sent from one sharding to another
type ShardingTxBatch struct {
	Txs Transactions // all sorted sharding transactions sent to the same shard

	OutTxsRoot    *common.Hash       // the root hash of all output transactions in it's block
	OutTxsProof   *ethdb.MemDatabase // the proof to verify the existence of the output sharding transactions in all output transactions
	BlockHash     *common.Hash       // the root hash of the block that contains the sharding transactions
	BlockHashPath *common.Hash       // the root hash of the block excluding the root hash of all output transactions
	BlockHeight   uint64             // the height of the block

	LampBase   *common.Hash // the lampBase hash of the block
	LampNum    uint64       // the height of the lampBase
	ShardingID uint16       // the sharding ID of the mined block

	BatchNonce uint64 // the nonce of the sharding transaction batch from this one shard to another

	// caches
	hash atomic.Value
	size atomic.Value
}

func NewMinedShardingTxBatch(txs Transactions, outTxsRoot, block *common.Hash, height uint64) *ShardingTxBatch {
	return &ShardingTxBatch{
		Txs:         txs,
		OutTxsRoot:  outTxsRoot,
		BlockHash:   block,
		BlockHeight: height,
	}
}

func (shardTxs *ShardingTxBatch) SetProof(proof *ethdb.MemDatabase) {
	if proof != nil {
		shardTxs.OutTxsProof = proof
	}
}

func (shardTxs *ShardingTxBatch) SetBlockHashPath(path *common.Hash) {
	if path != nil {
		shardTxs.BlockHashPath = path
	}
}

func (shardTxs *ShardingTxBatch) SetShardingInfo(lampBase *common.Hash, lampHeight uint64, shardingID uint16) {
	if lampBase != nil {
		shardTxs.LampBase = lampBase
		shardTxs.LampNum = lampHeight
		shardTxs.ShardingID = shardingID
	}
}

func (shardTxs *ShardingTxBatch) SetBatchNonce(batchNonce uint64) {
	shardTxs.BatchNonce = batchNonce
}

func (shardTxs *ShardingTxBatch) Height() uint64 {
	return shardTxs.BlockHeight
}

func (shardTxs *ShardingTxBatch) Hash() common.Hash {
	if hash := shardTxs.hash.Load(); hash != nil {
		return hash.(common.Hash)
	}
	hashTxs := DeriveSha(shardTxs.Txs)
	v := rlpHash([]interface{}{
		hashTxs,
		shardTxs.OutTxsRoot,
		shardTxs.OutTxsProof,
		shardTxs.BlockHash,
		shardTxs.BlockHashPath,
		shardTxs.LampBase,
		shardTxs.ShardingID,
		shardTxs.BatchNonce,
	})
	shardTxs.hash.Store(v)
	return v
}

func (shardTxs *ShardingTxBatch) GetBlockHeight() uint64 {
	return shardTxs.BlockHeight
}

// GetBlockHash returns the hash that the block in which these sharding transactions are
func (shardTxs *ShardingTxBatch) GetBlockHash() common.Hash {
	if shardTxs.BlockHash != nil {
		return *shardTxs.BlockHash
	}
	return common.Hash{}
}

func (shardTxs *ShardingTxBatch) OutTxRoot() common.Hash {
	if shardTxs.OutTxsRoot != nil {
		return *shardTxs.OutTxsRoot
	} else {
		return common.Hash{}
	}
}

func (shardTxs *ShardingTxBatch) TxsHash() common.Hash {
	return DeriveSha(shardTxs.Txs)
}

func (shardTxs *ShardingTxBatch) IsValid() bool {
	if len(shardTxs.Txs) == 0 || shardTxs.OutTxsProof == nil || shardTxs.OutTxsRoot == nil || shardTxs.LampBase == nil {
		return false
	}

	// Check all output transactions path
	val, err, _ := trie.VerifyProof(*shardTxs.OutTxsRoot, shardTxs.TxsHash().Bytes(), shardTxs.OutTxsProof)
	if err != nil {
		return false
	}

	// Check Sharding ID
	shardingID := new(uint16)
	errDecode := rlp.DecodeBytes(val, shardingID)
	if errDecode != nil {
		return false
	}
	for _, tx := range shardTxs.Txs {
		if *shardingID != common.GetSharding(*tx.To(), shardTxs.LampNum) {
			return false
		}
	}

	// Check block hash path
	if !shardTxs.BlockHash.Equal(rlpHash([]interface{}{shardTxs.BlockHashPath, shardTxs.OutTxsRoot})) {
		return false
	}

	return true
}

func (shardTxs *ShardingTxBatch) Size() common.StorageSize {
	if size := shardTxs.size.Load(); size != nil {
		return size.(common.StorageSize)
	}
	size := common.StorageSize(unsafe.Sizeof(*shardTxs.OutTxsRoot)) +
		common.StorageSize(unsafe.Sizeof(*shardTxs.OutTxsProof)) +
		common.StorageSize(unsafe.Sizeof(*shardTxs.BlockHash)) +
		common.StorageSize(unsafe.Sizeof(*shardTxs.BlockHashPath)) +
		common.StorageSize(unsafe.Sizeof(*shardTxs.LampBase)) +
		common.StorageSize(2)
	for _, tx := range shardTxs.Txs {
		size += tx.Size()
	}
	shardTxs.size.Store(common.StorageSize(size))
	return common.StorageSize(size)
}

func (shardTxs *ShardingTxBatch) FromShardingID() uint16 {
	return shardTxs.ShardingID
}

func (shardTxs *ShardingTxBatch) ToShardingID() uint16 {
	if len(shardTxs.Txs) == 0 {
		return 0
	}
	to := shardTxs.Txs[0].To()
	if to == nil {
		return 0
	}
	return common.GetSharding(*to, shardTxs.LampNum)
}

// ShardingTxBatches is a sharding transaction slice type for basic sorting.
type ShardingTxBatches []*ShardingTxBatch

// Len returns the length of s.
func (s ShardingTxBatches) Len() int { return len(s) }

// Swap swaps the i'th and the j'th element in s.
func (s ShardingTxBatches) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// GetRlp implements Rlpable and returns the i'th element of s in rlp.
func (s ShardingTxBatches) GetRlp(i int) []byte {
	enc, _ := rlp.EncodeToBytes(s[i])
	return enc
}

// func (s ShardingTxBatches) Less(i, j int) bool {
// 	iPrice := big.NewInt(0)
// 	for _, tx := range s[i].Txs {
// 		iPrice = iPrice.Add(iPrice, tx.data.Price)
// 	}

// 	jPrice := big.NewInt(0)
// 	for _, tx := range s[j].Txs {
// 		jPrice = jPrice.Add(jPrice, tx.data.Price)
// 	}
// 	return iPrice.Cmp(jPrice) > 0
// }

func (s ShardingTxBatches) Less(i, j int) bool { return s[i].BatchNonce < s[j].BatchNonce }

func (s *ShardingTxBatches) Push(x *ShardingTxBatch) {
	if s == nil {
		s = &ShardingTxBatches{x}
		return
	}
	*s = append(*s, x)
}

func (s *ShardingTxBatches) Pop() *ShardingTxBatch {
	if s == nil {
		return nil
	}
	old := *s
	if len(old) == 0 {
		return nil
	}
	n := len(old)
	x := old[n-1]
	*s = old[0 : n-1]
	return x
}

// Message is a fully derived transaction and implements core.Message
//
// NOTE: In a future PR this will be removed.
type Message struct {
	to         *common.Address
	from       common.Address
	nonce      uint64
	amount     *big.Int
	gasLimit   uint64
	gasPrice   *big.Int
	data       []byte
	checkNonce bool
	actType    uint8
	actData    []byte
}

func NewMessage(from common.Address, to *common.Address, nonce uint64, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte, checkNonce bool) Message {
	return Message{
		from:       from,
		to:         to,
		nonce:      nonce,
		amount:     amount,
		gasLimit:   gasLimit,
		gasPrice:   gasPrice,
		data:       data,
		checkNonce: checkNonce,
	}
}

func (m Message) From() common.Address { return m.from }
func (m Message) To() *common.Address  { return m.to }
func (m Message) GasPrice() *big.Int   { return m.gasPrice }
func (m Message) Value() *big.Int      { return m.amount }
func (m Message) Gas() uint64          { return m.gasLimit }
func (m Message) Nonce() uint64        { return m.nonce }
func (m Message) Data() []byte         { return m.data }
func (m Message) CheckNonce() bool     { return m.checkNonce }
func (m Message) ActType() uint8       { return m.actType }
func (m Message) ActData() []byte      { return m.actData }

const (
	LocalTxs   = common.LocalTxs
	InputTxs   = common.InputTxs
	OutputTxs  = common.OutputTxs
	MaxTxs     = common.MaxTxs
	InvalidTxs = common.InvalidTxs
)

func (tx *Transaction) IsLocalTx(lampHeight uint64, eBase *common.Address) bool {
	return common.GetTxsType(eBase, tx.From(), tx.To(), lampHeight) == LocalTxs
}

func (tx *Transaction) IsInputTx(lampHeight uint64, eBase *common.Address) bool {
	return common.GetTxsType(eBase, tx.From(), tx.To(), lampHeight) == InputTxs
}

func (tx *Transaction) IsOutputTx(lampHeight uint64, eBase *common.Address) bool {
	return common.GetTxsType(eBase, tx.From(), tx.To(), lampHeight) == OutputTxs
}

func (tx *Transaction) IsLocalOrOutputTx(lampHeight uint64, eBase *common.Address) bool {
	txType := common.GetTxsType(eBase, tx.From(), tx.To(), lampHeight)
	return txType == LocalTxs || txType == OutputTxs
}
