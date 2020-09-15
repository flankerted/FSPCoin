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

// Package eleapi implements the general Elephant API functions.
package elephantapi

import (
	"context"
	"fmt"
	"math/big"
	"sync"

	"github.com/contatract/go-contatract/accounts"
	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/common/hexutil"
	"github.com/contatract/go-contatract/core/types_elephant"
	core "github.com/contatract/go-contatract/core_elephant"
	"github.com/contatract/go-contatract/core_elephant/state"
	"github.com/contatract/go-contatract/ethdb"
	"github.com/contatract/go-contatract/params"
	"github.com/contatract/go-contatract/rpc"
)

type AddrLocker struct {
	mu    sync.Mutex
	locks map[common.Address]*sync.Mutex
}

// lock returns the lock of the given address.
func (l *AddrLocker) lock(address common.Address) *sync.Mutex {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.locks == nil {
		l.locks = make(map[common.Address]*sync.Mutex)
	}
	if _, ok := l.locks[address]; !ok {
		l.locks[address] = new(sync.Mutex)
	}
	return l.locks[address]
}

// LockAddr locks an account's mutex. This is used to prevent another tx getting the
// same nonce until the lock is released. The mutex prevents the (an identical nonce) from
// being read again during the time that the first transaction is being signed.
func (l *AddrLocker) LockAddr(address common.Address) {
	l.lock(address).Lock()
}

// UnlockAddr unlocks the mutex of the given account.
func (l *AddrLocker) UnlockAddr(address common.Address) {
	l.lock(address).Unlock()
}

type Backend interface {
	ChainDb() ethdb.Database
	AccountManager() *accounts.Manager
	ChainConfig() *params.ElephantChainConfig
	SuggestPrice(ctx context.Context) (*big.Int, error)
	GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error)

	// BlockChain API
	CurrentBlock() *types_elephant.Block
	HeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types_elephant.Header, error)
	BlockByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types_elephant.Block, error)
	GetBlock(ctx context.Context, blockHash common.Hash) (*types_elephant.Block, error)
	GetTd(blockHash common.Hash) *big.Int

	SendTx(ctx context.Context, signedTx *types_elephant.Transaction) error
	StateAndHeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*state.StateDB, *types_elephant.Header, error)

	GetPoolTransaction(txHash common.Hash) *types_elephant.Transaction
	Stats() (pending int, queued int)
	TxPoolContent() (map[common.Address]types_elephant.Transactions, map[common.Address]types_elephant.Transactions)
	TxPoolInputContent() map[common.Hash]*types_elephant.ShardingTxBatch

	GetEBase() *common.Address
}

// PublicTransactionPoolAPI exposes methods for the RPC interface
type PublicTransactionPoolAPI struct {
	b         Backend
	nonceLock *AddrLocker
}

// NewPublicTransactionPoolAPI creates a new RPC service with methods specific for the transaction pool.
func NewPublicTransactionPoolAPI(b Backend, nonceLock *AddrLocker) *PublicTransactionPoolAPI {
	return &PublicTransactionPoolAPI{b, nonceLock}
}

// PublicTxPoolAPI offers and API for the transaction pool. It only operates on data that is non confidential.
type PublicTxPoolAPI struct {
	b Backend
}

// NewPublicTxPoolAPI creates a new tx pool service that gives information about the transaction pool.
func NewPublicTxPoolAPI(b Backend) *PublicTxPoolAPI {
	return &PublicTxPoolAPI{b}
}

// Content returns the transactions contained within the transaction pool.
func (s *PublicTxPoolAPI) Content() map[string]map[string]map[string]*RPCTransaction {
	content := map[string]map[string]map[string]*RPCTransaction{
		"pending": make(map[string]map[string]*RPCTransaction),
		"queued":  make(map[string]map[string]*RPCTransaction),
	}
	pending, queue := s.b.TxPoolContent()

	// Flatten the pending transactions
	for account, txs := range pending {
		dump := make(map[string]*RPCTransaction)
		for _, tx := range txs {
			dump[fmt.Sprintf("%d", tx.Nonce())] = newRPCPendingTransaction(tx)
		}
		content["pending"][account.Hex()] = dump
	}
	// Flatten the queued transactions
	for account, txs := range queue {
		dump := make(map[string]*RPCTransaction)
		for _, tx := range txs {
			dump[fmt.Sprintf("%d", tx.Nonce())] = newRPCPendingTransaction(tx)
		}
		content["queued"][account.Hex()] = dump
	}
	return content
}

// Status returns the number of pending and queued transaction in the pool.
func (s *PublicTxPoolAPI) Status() map[string]hexutil.Uint {
	pending, queue := s.b.Stats()
	return map[string]hexutil.Uint{
		"pending": hexutil.Uint(pending),
		"queued":  hexutil.Uint(queue),
	}
}

// Inspect retrieves the content of the transaction pool and flattens it into an
// easily inspectable list.
func (s *PublicTxPoolAPI) Inspect() map[string]map[string]map[string]string {
	content := map[string]map[string]map[string]string{
		"pending": make(map[string]map[string]string),
		"queued":  make(map[string]map[string]string),
	}
	pending, queue := s.b.TxPoolContent()

	// Define a formatter to flatten a transaction into a string
	var format = func(tx *types_elephant.Transaction) string {
		if to := tx.To(); to != nil {
			return fmt.Sprintf("%s: %v wei + %v gas × %v wei", tx.To().Hex(), tx.Value(), tx.Gas(), tx.GasPrice())
		}
		return fmt.Sprintf("contract creation: %v wei + %v gas × %v wei", tx.Value(), tx.Gas(), tx.GasPrice())
	}
	// Flatten the pending transactions
	for account, txs := range pending {
		dump := make(map[string]string)
		for _, tx := range txs {
			dump[fmt.Sprintf("%d", tx.Nonce())] = format(tx)
		}
		content["pending"][account.Hex()] = dump
	}
	// Flatten the queued transactions
	for account, txs := range queue {
		dump := make(map[string]string)
		for _, tx := range txs {
			dump[fmt.Sprintf("%d", tx.Nonce())] = format(tx)
		}
		content["queued"][account.Hex()] = dump
	}
	return content
}

type RPCShardingTxBatch struct {
	TxBatchHash common.Hash                `json:"txBatchHash"` // types_elephant.ShardingTxBatch hash
	Txs         map[string]*RPCTransaction `json:"txs"`
	OutTxsRoot  *common.Hash               `json:"outTxsRoot"`
	BlockHash   *common.Hash               `json:"blockHash"`
	BlockHeight uint64                     `json:"blockHeight"`
	LampBase    *common.Hash               `json:"lampBase"`
	LampNum     uint64                     `json:"lampNum"`
	ShardingID  uint16                     `json:"shardingID"`
}

// InputContent returns the input transactions contained within the transaction pool.
func (s *PublicTxPoolAPI) InputContent() map[uint16][]*RPCShardingTxBatch {
	rpcTxBatches := make(map[uint16][]*RPCShardingTxBatch)
	batches := s.b.TxPoolInputContent()
	for _, batch := range batches {
		rpcTxBatch := newRPCPendingInputTx(batch)
		rpcTxBatches[rpcTxBatch.ShardingID] = append(rpcTxBatches[rpcTxBatch.ShardingID], rpcTxBatch)
	}

	return rpcTxBatches
}

// DetailedInputTxs returns the detailed input transaction contained within the transaction pool by sharding id and block height.
func (s *PublicTxPoolAPI) DetailedInputTxs(shardingID uint16, blockHeight uint64) map[string]*RPCTransaction {
	batches := s.b.TxPoolInputContent()
	for _, batch := range batches {
		if batch.ShardingID != shardingID || batch.BlockHeight != blockHeight {
			continue
		}
		rpcTxs := make(map[string]*RPCTransaction)
		for _, tx := range batch.Txs {
			rpcTxs[fmt.Sprintf("%d", tx.Nonce())] = newRPCPendingTransaction(tx)
		}
		return rpcTxs
	}

	return nil
}

// newRPCPendingTransaction returns a pending transaction that will serialize to the RPC representation
func newRPCPendingTransaction(tx *types_elephant.Transaction) *RPCTransaction {
	return newRPCTransaction(tx, common.Hash{}, 0, 0)
}

// GetTransactionByHash returns the transaction for the given hash
func (s *PublicTransactionPoolAPI) GetTransactionByHash(hash common.Hash) *RPCTransaction {
	// Try to return an already finalized transaction
	if tx, blockHash, blockNumber, index := core.GetTransaction(s.b.ChainDb(), hash); tx != nil {
		return newRPCTransaction(tx, blockHash, blockNumber, index)
	}
	// No finalized transaction, try to retrieve it from the pool
	if tx := s.b.GetPoolTransaction(hash); tx != nil {
		return newRPCPendingTransaction(tx)
	}
	// Transaction unknown, return as such
	return nil
}

// newRPCPendingInputTx returns a pending input transaction that will serialize to the RPC representation
func newRPCPendingInputTx(batch *types_elephant.ShardingTxBatch) *RPCShardingTxBatch {
	rpcTxs := make(map[string]*RPCTransaction)
	for _, tx := range batch.Txs {
		rpcTxs[fmt.Sprintf("%d", tx.Nonce())] = newRPCPendingTransaction(tx)
	}
	return &RPCShardingTxBatch{
		TxBatchHash: batch.Hash(),
		Txs:         rpcTxs,
		OutTxsRoot:  batch.OutTxsRoot,
		BlockHash:   batch.BlockHash,
		BlockHeight: batch.BlockHeight,
		LampBase:    batch.LampBase,
		LampNum:     batch.LampNum,
		ShardingID:  batch.ShardingID,
	}
}

func GetAPIs(apiBackend Backend) []rpc.API {
	nonceLock := new(AddrLocker)
	return []rpc.API{
		// {
		// 	Namespace: "personal",
		// 	Version:   "1.0",
		// 	Service:   NewPrivateAccountAPI(apiBackend, nonceLock),
		// 	Public:    false,
		// }, {
		{
			Namespace: "elephant",
			Version:   "1.0",
			Service:   NewPublicBlockChainAPI(apiBackend),
			Public:    true,
		},
		{
			Namespace: "elephant",
			Version:   "1.0",
			Service:   NewPublicTransactionPoolAPI(apiBackend, nonceLock),
			Public:    true,
		},
		{
			Namespace: "eletxpool",
			Version:   "1.0",
			Service:   NewPublicTxPoolAPI(apiBackend),
			Public:    true,
		},
	}
}
