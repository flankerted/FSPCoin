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

package elephant

import (
	"context"
	"math/big"

	"github.com/contatract/go-contatract/accounts"
	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/core/types_elephant"
	types "github.com/contatract/go-contatract/core/types_elephant"
	core "github.com/contatract/go-contatract/core_elephant"
	"github.com/contatract/go-contatract/core_elephant/state"
	downloader "github.com/contatract/go-contatract/elephant/downloader"
	"github.com/contatract/go-contatract/ethdb"
	"github.com/contatract/go-contatract/event"
	"github.com/contatract/go-contatract/params"
	"github.com/contatract/go-contatract/rpc"
)

// ElephantApiBackend implements ethapi.Backend for full nodes
type ElephantApiBackend struct {
	ele *Elephant
	//gpo *gasprice.Oracle
}

func (b *ElephantApiBackend) ChainConfig() *params.ElephantChainConfig {
	return b.ele.chainConfig
}

func (b *ElephantApiBackend) CurrentBlock() *types_elephant.Block {
	return b.ele.blockchain.CurrentBlock()
}

func (b *ElephantApiBackend) SetHead(number uint64) {
	b.ele.protocolManager.downloader.Cancel()
	b.ele.blockchain.SetHead(number)
}

func (b *ElephantApiBackend) HeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types_elephant.Header, error) {
	// Pending block is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		block := b.ele.miner.PendingBlock()
		return block.Header(), nil
	}
	// Otherwise resolve and return the block
	if blockNr == rpc.LatestBlockNumber {
		return b.ele.blockchain.CurrentBlock().Header(), nil
	}
	return b.ele.blockchain.GetHeaderByNumber(uint64(blockNr)), nil
}

func (b *ElephantApiBackend) BlockByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types_elephant.Block, error) {
	// Pending block is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		block := b.ele.miner.PendingBlock()
		return block, nil
	}
	// Otherwise resolve and return the block
	if blockNr == rpc.LatestBlockNumber {
		return b.ele.blockchain.CurrentBlock(), nil
	}
	return b.ele.blockchain.GetBlockByNumber(uint64(blockNr)), nil
}

func (b *ElephantApiBackend) StateAndHeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*state.StateDB, *types_elephant.Header, error) {
	// Pending state is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		block, state := b.ele.miner.Pending()
		return state, block.Header(), nil
	}
	// Otherwise resolve the block number and return its state
	header, err := b.HeaderByNumber(ctx, blockNr)
	if header == nil || err != nil {
		return nil, nil, err
	}
	stateDb, err := b.ele.BlockChain().StateAt(header.Root)
	return stateDb, header, err
}

func (b *ElephantApiBackend) GetBlock(ctx context.Context, blockHash common.Hash) (*types_elephant.Block, error) {
	return b.ele.blockchain.GetBlockByHash(blockHash), nil
}

func (b *ElephantApiBackend) GetReceipts(ctx context.Context, blockHash common.Hash) (types_elephant.Receipts, error) {
	return core.GetBlockReceipts(b.ele.chainDb, blockHash, core.GetBlockNumber(b.ele.chainDb, blockHash)), nil
}

func (b *ElephantApiBackend) GetTd(blockHash common.Hash) *big.Int {
	return b.ele.blockchain.GetTdByHash(blockHash)
}

//func (b *ElephantApiBackend) GetEVM(ctx context.Context, msg core.Message, state *state.StateDB, header *types_elephant.Header, vmCfg vm.Config) (*vm.EVM, func() error, error) {
//	state.SetBalance(msg.From(), math.MaxBig256)
//	vmError := func() error { return nil }
//
//	context := core.NewEVMContext(msg, header, b.ele.BlockChain(), nil)
//	return vm.NewEVM(context, state, b.ele.chainConfig, vmCfg), vmError, nil
//}

func (b *ElephantApiBackend) SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription {
	return b.ele.BlockChain().SubscribeChainEvent(ch)
}

func (b *ElephantApiBackend) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return b.ele.BlockChain().SubscribeChainHeadEvent(ch)
}

func (b *ElephantApiBackend) SubscribeChainSideEvent(ch chan<- core.ChainSideEvent) event.Subscription {
	return b.ele.BlockChain().SubscribeChainSideEvent(ch)
}

//func (b *ElephantApiBackend) SubscribeLogsEvent(ch chan<- []*types_elephant.Log) event.Subscription {
//	return b.ele.BlockChain().SubscribeLogsEvent(ch)
//}

func (b *ElephantApiBackend) SendTx(ctx context.Context, signedTx *types_elephant.Transaction) error {
	ethHeight := b.ele.GetEthHeight()
	return b.ele.txPool.AddLocal(signedTx, ethHeight)
}

func (b *ElephantApiBackend) GetPoolTransactions() (types_elephant.Transactions, error) {
	pending, err := b.ele.txPool.Pending()
	if err != nil {
		return nil, err
	}
	var txs types_elephant.Transactions
	for _, batch := range pending {
		txs = append(txs, batch...)
	}
	return txs, nil
}

func (b *ElephantApiBackend) GetPoolTransaction(hash common.Hash) *types_elephant.Transaction {
	return b.ele.txPool.Get(hash)
}

func (b *ElephantApiBackend) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
	return b.ele.txPool.State().GetNonce(addr), nil
}

func (b *ElephantApiBackend) Stats() (pending int, queued int) {
	return b.ele.txPool.Stats()
}

func (b *ElephantApiBackend) TxPoolContent() (map[common.Address]types_elephant.Transactions, map[common.Address]types_elephant.Transactions) {
	return b.ele.TxPool().Content()
}

func (b *ElephantApiBackend) GetEBase() *common.Address { return &b.ele.etherbase }

func (b *ElephantApiBackend) TxPoolInputContent() map[common.Hash]*types.ShardingTxBatch {
	return b.ele.TxPool().ContentInputTxs()
}

func (b *ElephantApiBackend) SubscribeTxPreEvent(ch chan<- core.TxPreEvent) event.Subscription {
	return b.ele.TxPool().SubscribeTxPreEvent(ch)
}

func (b *ElephantApiBackend) Downloader() *downloader.Downloader {
	return b.ele.Downloader()
}

func (b *ElephantApiBackend) ProtocolVersion() int {
	return b.ele.EthVersion()
}

func (b *ElephantApiBackend) SuggestPrice(ctx context.Context) (*big.Int, error) {
	return big.NewInt(1), nil
}

func (b *ElephantApiBackend) ChainDb() ethdb.Database {
	return b.ele.ChainDb()
}

func (b *ElephantApiBackend) EventMux() *event.TypeMux {
	return b.ele.EventMux()
}

func (b *ElephantApiBackend) AccountManager() *accounts.Manager {
	return b.ele.AccountManager()
}

//func (b *ElephantApiBackend) BloomStatus() (uint64, uint64) {
//	sections, _, _ := b.ele.bloomIndexer.Sections()
//	return params.BloomBitsBlocks, sections
//}
//
//func (b *ElephantApiBackend) ServiceFilter(ctx context.Context, session *bloombits.MatcherSession) {
//	for i := 0; i < bloomFilterThreads; i++ {
//		go session.Multiplex(bloomRetrievalBatch, bloomRetrievalWait, b.ele.bloomRequests)
//	}
//}
