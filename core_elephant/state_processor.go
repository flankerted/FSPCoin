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

package core_elephant

import (
	"errors"

	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/consensus"
	"github.com/contatract/go-contatract/log"

	//"github.com/contatract/go-contatract/consensus/misc"
	types "github.com/contatract/go-contatract/core/types_elephant"
	"github.com/contatract/go-contatract/core_elephant/state"

	//"github.com/contatract/go-contatract/core/vm"
	//"github.com/contatract/go-contatract/crypto"

	vm "github.com/contatract/go-contatract/core/vm_elephant"
	"github.com/contatract/go-contatract/crypto"
	"github.com/contatract/go-contatract/params"
)

// StateProcessor is a basic Processor, which takes care of transitioning
// state from one point to another.
//
// StateProcessor implements Processor.
type StateProcessor struct {
	config *params.ElephantChainConfig // Chain configuration options
	bc     *BlockChain                 // Canonical block chain
	engine consensus.EngineCtt         // Consensus engine used for block rewards
}

// NewStateProcessor initialises a new StateProcessor.
func NewStateProcessor(config *params.ElephantChainConfig, bc *BlockChain, engine consensus.EngineCtt) *StateProcessor {
	return &StateProcessor{
		config: config,
		bc:     bc,
		engine: engine,
	}
}

// JiangHan: (重点：交易执行点二）这里是执行插入的新 block 的主入口
// 一般新区块来自其他矿工announce宣布，这里承认并执行其中的交易，以同步本地stateDB,receiptDB
// 调用点在 \core\blockchain.go 模块的 insertChain() 方法中
// Process processes the state changes according to the Ethereum rules by running
// the transaction messages using the statedb and applying any rewards to both
// the processor (coinbase) and any included uncles.
//
// Process returns the receipts and logs accumulated during the process and
// returns the amount of gas that was used in the process. If any of the
// transactions failed to execute due to insufficient gas it will return an error.
func (p *StateProcessor) Process(block *types.Block, statedb *state.StateDB, lampHeight uint64, eBase *common.Address) (types.Receipts, uint64, error) {
	var (
		receipts types.Receipts
		usedGas  = new(uint64)
		header   = block.Header()
		gp       = new(GasPool).AddGas(block.GasLimit())
	)

	// Iterate over and process the individual transactions
	for i, tx := range block.Transactions() {
		// fmt.Println("tx: ", tx)
		statedb.Prepare(tx.Hash(), block.Hash(), i)
		// JiangHan：执行区块中所有交易！！（以更新本地statedb, receiptdb）
		receipt, _, err := ApplyTransaction(p.config, p.bc, eBase, gp, statedb, header, tx, usedGas, true, lampHeight)
		if err != nil {
			return nil, 0, err
		}
		receipts = append(receipts, receipt)
	}
	// Finalize the block, applying any consensus engine specific extras (e.g. block rewards)
	p.engine.Finalize(p.bc, header, statedb, block.Transactions(), block.InputTxs(), block.Uncles(), receipts, eBase)

	for _, txBatch := range block.InputTxs() {
		toShardingID := txBatch.ToShardingID()
		height := statedb.GetShardingHeight(toShardingID)
		if txBatch.BlockHeight <= height {
			log.Error("Process", "current height", txBatch.BlockHeight, "already height", height)
			return nil, 0, errors.New("check input sharding transactions block height failed")
		}
		statedb.UpdateStoreShardingHeight(toShardingID, txBatch.BlockHeight)

		nonce := statedb.GetBatchNonce(toShardingID)
		if txBatch.BatchNonce != nonce+1 {
			log.Error("Process", "current batch nonce", txBatch.BatchNonce, "already batch nonce", nonce)
			return nil, 0, errors.New("check input sharding transactions batch nonce failed")
		}
		statedb.UpdateStoreBatchNonce(toShardingID, txBatch.BatchNonce)

		ethHeight := txBatch.LampNum
		for _, tx := range txBatch.Txs {
			if success := ApplyInputTxs(statedb, tx, ethHeight, eBase); !success {
				return nil, 0, errors.New("apply input sharding transactions failed")
			}
		}
	}

	return receipts, *usedGas, nil
}

// JiangHan: 执行交易（智能合约）
// ApplyTransaction attempts to apply a transaction to the given state database
// and uses the input parameters for its environment. It returns the receipt
// for the transaction, gas used and an error if the transaction failed,
// indicating the block was invalid.
func ApplyTransaction(config *params.ElephantChainConfig, bc *BlockChain, author *common.Address, gp *GasPool, statedb *state.StateDB,
	header *types.Header, tx *types.Transaction, usedGas *uint64, update bool, lampHeight uint64) (*types.Receipt, uint64, error) {
	msg, err := tx.AsMessage(types.MakeSigner(config, header.Number))
	if err != nil {
		return nil, 0, err
	}

	// Create a new context to be used in the EVM environment
	context := NewEVMContext(msg, header, bc, nil, update)
	// Create a new environment which holds all relevant information
	// about the transaction and calling mechanisms.

	vmenv := vm.NewEVM(context, statedb, config, vm.Config{}, lampHeight, author)
	// Apply the transaction to the current state (included in the env)
	// JiangHan：调用执行交易（合约）
	_, gas, failed, err := ApplyMessage(vmenv, msg, gp, update)
	if err != nil {
		return nil, 0, err
	}

	// Update the state with pending changes
	var root []byte
	// 其他分叉
	root = statedb.IntermediateRoot(false).Bytes()
	*usedGas += gas

	// Create a new receipt for the transaction, storing the intermediate root and gas used by the tx
	// based on the eip phase, we're passing wether the root touch-delete accounts.
	receipt := types.NewReceipt(root, failed, *usedGas)
	// fmt.Println("root", common.BytesToHash(root).Hex(), "failed", failed, "usedGas", *usedGas)
	statedb.Print()

	receipt.TxHash = tx.Hash()
	receipt.GasUsed = gas
	// if the transaction created a contract, store the creation address in the receipt.
	if msg.To() == nil {
		receipt.ContractAddress = crypto.CreateAddress(vmenv.Context.Origin, tx.Nonce())
	}
	return receipt, gas, err
}

func ApplyInputTxs(statedb *state.StateDB, tx *types.Transaction, ethHeight uint64, eBase *common.Address) bool {
	if !tx.IsInputTx(ethHeight, eBase) {
		return false
	}
	statedb.AddBalance(*tx.To(), tx.Value())
	return true
}
