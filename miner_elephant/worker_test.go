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

package miner_elephant

import "testing"

var (
	workerTest *Worker
)

func init() {
	//workerTest = &worker{
	//	config:         config,
	//	engine:         engine,
	//	elephant:       elephant,
	//	mux:            mux,
	//	txCh:           make(chan core.TxPreEvent, txChanSize),
	//	chainHeadCh:    make(chan core.ChainHeadEvent, chainHeadChanSize),
	//	chainSideCh:    make(chan core.ChainSideEvent, chainSideChanSize),
	//	chainDb:        elephant.ChainDb(),
	//	recv:           make(chan *Result, resultQueueSize),
	//	chain:          elephant.BlockChain(),
	//	proc:           elephant.BlockChain().Validator(),
	//	possibleUncles: make(map[common.Hash]*types.Block),
	//	coinbase:       coinbase,
	//	agents:         make(map[Agent]struct{}),
	//	unconfirmed:    newUnconfirmedBlocks(elephant.BlockChain(), miningLogAtDepth),
	//	lamp:           eth,
	//}
	//// Subscribe TxPreEvent for tx pool
	//worker.txSub = elephant.TxPool().SubscribeTxPreEvent(worker.txCh)
	//// Subscribe events for blockchain
	//worker.chainHeadSub = elephant.BlockChain().SubscribeChainHeadEvent(worker.chainHeadCh)
	//worker.chainSideSub = elephant.BlockChain().SubscribeChainSideEvent(worker.chainSideCh)
}

func TestFinalizeWork(t *testing.T) {

}
