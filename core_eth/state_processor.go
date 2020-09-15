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

package core_eth

import (
	"encoding/json"
	"errors"
	"math/big"

	"github.com/contatract/go-contatract/baseparam"
	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/consensus"
	"github.com/contatract/go-contatract/core/types"
	"github.com/contatract/go-contatract/core/vm"
	"github.com/contatract/go-contatract/core_eth/state"
	"github.com/contatract/go-contatract/crypto"
	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/params"
)

// StateProcessor is a basic Processor, which takes care of transitioning
// state from one point to another.
//
// StateProcessor implements Processor.
type StateProcessor struct {
	config *params.ChainConfig // Chain configuration options
	bc     *BlockChain         // Canonical block chain
	engine consensus.Engine    // Consensus engine used for block rewards
}

// NewStateProcessor initialises a new StateProcessor.
func NewStateProcessor(config *params.ChainConfig, bc *BlockChain, engine consensus.Engine) *StateProcessor {
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
func (p *StateProcessor) Process(block *types.Block, statedb *state.StateDB, cfg vm.Config) (types.Receipts, []*types.Log, uint64, error) {
	var (
		receipts types.Receipts
		usedGas  = new(uint64)
		header   = block.Header()
		allLogs  []*types.Log
		gp       = new(GasPool).AddGas(block.GasLimit())
	)

	// Iterate over and process the individual transactions
	for i, tx := range block.Transactions() {
		statedb.Prepare(tx.Hash(), block.Hash(), i)
		// JiangHan：执行区块中所有交易！！（以更新本地statedb, receiptdb）
		receipt, _, err := ApplyTransaction(p.config, p.bc, nil, gp, statedb, header, tx, usedGas, cfg)
		if err != nil {
			return nil, nil, 0, err
		}
		receipts = append(receipts, receipt)
		allLogs = append(allLogs, receipt.Logs...)
	}

	// Finalize the block, applying any consensus engine specific extras (e.g. block rewards)
	p.engine.Finalize(p.bc, header, statedb, block.Transactions(), block.Uncles(), receipts)

	return receipts, allLogs, *usedGas, nil
}

// JiangHan: 执行交易（智能合约）
// ApplyTransaction attempts to apply a transaction to the given state database
// and uses the input parameters for its environment. It returns the receipt
// for the transaction, gas used and an error if the transaction failed,
// indicating the block was invalid.
func ApplyTransaction(config *params.ChainConfig, bc *BlockChain, author *common.Address, gp *GasPool, statedb *state.StateDB,
	header *types.Header, tx *types.Transaction, usedGas *uint64, cfg vm.Config) (*types.Receipt, uint64, error) {
	//return nil, 0, nil

	msg, err := tx.AsMessage(types.MakeSigner(config, header.Number))
	if err != nil {
		return nil, 0, err
	}
	// Create a new context to be used in the EVM environment
	context := NewEVMContext(msg, header, bc, author)
	// Create a new environment which holds all relevant information
	// about the transaction and calling mechanisms.
	vmenv := vm.NewEVM(context, statedb, config, cfg)
	// Apply the transaction to the current state (included in the env)
	// JiangHan：调用执行交易（合约）
	_, gas, failed, err := ApplyMessage(vmenv, msg, gp)
	if err != nil {
		return nil, 0, err
	}

	// Add election to election pool
	actType := msg.ActType()
	actData := msg.ActData()
	if actType == baseparam.TypeSealersElection || actType == baseparam.TypePoliceElection {
		from := msg.From()

		var data common.BlockIDNode
		if err := json.Unmarshal(actData, &data); err != nil {
			log.Error("Unmarshal", "err", err)
			return nil, 0, err
		}
		blockNum := new(big.Int).SetUint64(data.BlockID)

		electionType := baseparam.TypeSealersElection
		if actType == baseparam.TypePoliceElection {
			electionType = baseparam.TypePoliceElection
		}
		elect := types.AddressElection{Address: &from, ForBlockNum: blockNum, ElectionType: electionType, NodeStr: data.NodeStr}
		if bc.ElectionPool() != nil {
			electionNeedBalance := getElectionReqBalance()
			leftBalance := statedb.GetBalance(*(elect.Address))
			if common.GetConsensusBftTest() || leftBalance.Cmp(electionNeedBalance) >= 0 { // Check balance
				if err := bc.ElectionPool().Add(&elect, header.Number.Uint64()); err != nil {
					log.Error(err.Error())
					return nil, 0, err
				}
			} else {
				log.Info("not enough balance to elect: %s, discard the election", elect.Address.String())
			}
		} else {
			log.Error("the election pool is nil when ApplyTransaction()")
			return nil, 0, errors.New("the election pool is nil when ApplyTransaction()")
		}
	}

	// Update the state with pending changes
	var root []byte
	if config.IsByzantium(header.Number) {
		// JiangHan：拜占庭分叉
		statedb.Finalise(true)
	} else {
		// 其他分叉
		root = statedb.IntermediateRoot(config.IsEIP158(header.Number)).Bytes()
	}
	*usedGas += gas

	// Create a new receipt for the transaction, storing the intermediate root and gas used by the tx
	// based on the eip phase, we're passing wether the root touch-delete accounts.
	receipt := types.NewReceipt(root, failed, *usedGas)
	receipt.TxHash = tx.Hash()
	receipt.GasUsed = gas
	// if the transaction created a contract, store the creation address in the receipt.
	if msg.To() == nil {
		receipt.ContractAddress = crypto.CreateAddress(vmenv.Context.Origin, tx.Nonce())
	}
	// Set the receipt logs and create a bloom for filtering
	receipt.Logs = statedb.GetLogs(tx.Hash())
	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})

	return receipt, gas, err
}
