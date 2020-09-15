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

package testUtils

import (
	"fmt"
	"math/big"
	"math/rand"
	"sync"
	"time"

	"github.com/contatract/go-contatract/blizparam"
	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/common/hexutil"
	"github.com/contatract/go-contatract/core/types"
	core "github.com/contatract/go-contatract/core_eth"
	"github.com/contatract/go-contatract/event"
	"github.com/contatract/go-contatract/internal/ethapi"
	"github.com/contatract/go-contatract/miner"
)

const TimeUnit = 10 // 30

var (
	LampMinersGlobal        []*miner.Miner         // Miners of testing nodes for normal case
	LampNodeBackGlobal      []*BackendForMinerTest // Backends of testing nodes for normal case
	LampNodeAddressGlobal   []common.Address       // Addresses of testing nodes for normal case
	LampNodeMapAddressIndex map[common.Address]int
	LampNodeBlockHashMap    map[common.Hash]*types.Block
	LampNodeBlockNumMap     map[uint64]*types.Block
	LampNodeTimeErrMap      map[common.Address]int64
	LampDisabledNodes       []int
	LampNodeGlobalMu        sync.Mutex

	LampTxFromAddr common.Address
	LampTxToAddr   common.Address

	// New block net transmitting event for test
	chainNetCh   chan core.ChainEvent // New block net transmitting event in elephant
	chainNetFeed event.Feed           // New block net transmitting event in elephant
)

func InitGlobal(NodesCnt int, randomDelay bool, recoverTimes int, disabledNodes []int) {
	LampMinerTestStopped = false
	testTimes = 0
	useRandomDelay = randomDelay
	recoverDalayTimes = recoverTimes

	LampNodeAddressGlobal = make([]common.Address, 0)
	LampNodeMapAddressIndex = make(map[common.Address]int)
	LampMinersGlobal = make([]*miner.Miner, NodesCnt)
	LampNodeBlockHashMap = make(map[common.Hash]*types.Block)
	LampNodeBlockNumMap = make(map[uint64]*types.Block)
	LampNodeTimeErrMap = make(map[common.Address]int64)
	LampDisabledNodes = disabledNodes

	chainNetCh = make(chan core.ChainEvent, 32)
}

func InsertGlobalBlock(block *types.Block) {
	LampNodeGlobalMu.Lock()
	defer LampNodeGlobalMu.Unlock()

	if blockHave, ok := LampNodeBlockHashMap[block.Hash()]; !ok {
		if existBlock, exist := LampNodeBlockNumMap[block.NumberU64()]; exist {
			existBlockHash := existBlock.Hash()
			if !existBlockHash.Equal(block.Hash()) {
				fmt.Println(fmt.Sprintf("\r---Inserted different global blocks in number: %d", block.NumberU64()))
				LogCrit(fmt.Sprintf("\r---Inserted different global blocks in number: %d, we have: %s, another: %s",
					block.NumberU64(), existBlockHash.TerminalString(), block.Hash().TerminalString()))
			}
		}
		LampNodeBlockHashMap[block.Hash()] = block
		LampNodeBlockNumMap[block.NumberU64()] = block

		if block.NumberU64()%1000 == 0 {
			fmt.Println(fmt.Sprintf("\r---Inserted global block number: %d", block.NumberU64()))
		}
		//else {
		//	if block.NumberU64()%10 == 0 {
		//		num := block.NumberU64() % 1000
		//		h := strings.Repeat("-", int(num/10)) + strings.Repeat(" ", 99-int(num/10))
		//		fmt.Printf("\r[%s]", h)
		//		os.Stdout.Sync()
		//	}
		//}

		// Add random transaction
		//if block.NumberU64() > 0 && len(LampNodeBackGlobal) > 0 &&
		//	block.NumberU64() < LampNodeBackGlobal[0].TestBlocksCntTotal {
		//	_, tx := GetRandomTransaction(block.NumberU64() - 1)
		//	for _, back := range LampNodeBackGlobal {
		//		//back.AddTx(tx)
		//		if err := back.TxPool().AddRemote(tx); err != nil {
		//			fmt.Println(fmt.Sprintf("---Add transaction failed, node: %s, %s, tx: %s",
		//				back.NodeName, err.Error(), tx.Hash().TerminalString()))
		//			LogWarn(fmt.Sprintf("---Add transaction failed, node: %s, %s, tx: %s",
		//				back.NodeName, err.Error(), tx.Hash().TerminalString()))
		//		} else {
		//			LogInfo(fmt.Sprintf("---Add transaction, node: %s, tx: %s", back.NodeName, tx.Hash().TerminalString()))
		//		}
		//	}
		//}
	} else {
		fmt.Println("===========", blockHave.Hash().TerminalString())
		// TODO
		//blockHashWithBft := blockHave.HashWithBft()
		//if !blockHashWithBft.Equal(block.HashWithBft()) && block.HBFTStageChain()[1].ViewID > blockHave.HBFTStageChain()[1].ViewID {
		//	LampNodeBlockHashMap[block.Hash()] = block
		//	LampNodeBlockNumMap[block.NumberU64()] = block
		//}
	}
}

func RandomDelayMsForHBFT(nodeIndex int, info string, returnFalse bool) bool {
	if testTimes >= recoverDalayTimes {
		if testTimes <= recoverDalayTimes+5 {
			LogInfo("---Recover net delay")
		}

		LampNodeGlobalMu.Lock()
		if len(LampNodeBlockNumMap)%3000 == 0 {
			testTimes = 0
			for i := 0; i < 5; i++ {
				LogInfo("---Restart net delay")
			}
		}
		LampNodeGlobalMu.Unlock()

		return true
	}

	if !useRandomDelay {
		return true
	}

	rand.Seed(time.Now().UnixNano())
	random := rand.Intn(100)
	//random = 50
	if random < 60 {
		time.Sleep(time.Millisecond / 10 * TimeUnit)
	} else if random < 75 {
		time.Sleep(time.Millisecond * 1 * TimeUnit)
	} else if random < 86 {
		time.Sleep(time.Millisecond * 3 * TimeUnit)
	} else if random < 90 {
		time.Sleep(time.Millisecond * 5 * TimeUnit)
	} else if random < 94 {
		time.Sleep(time.Millisecond * 7 * TimeUnit)
	} else if random < 96 {
		time.Sleep(time.Millisecond * 11 * TimeUnit)
	} else if random < 98 {
		time.Sleep(time.Millisecond * 20 * TimeUnit)
	} else if random < 100 && returnFalse {
		LampMinersGlobal[nodeIndex].Info("RandomDelayMsForHBFT() return false",
			"node", LampNodeAddressGlobal[nodeIndex].String(), "info", info)
		return false
	}

	return true
}

func DisableNode(nodeIndex int) bool {
	for _, idx := range LampDisabledNodes {
		if idx == nodeIndex {
			return true
		}
	}
	return false
}

func GetRandomTransaction(nonce uint64) (uint64, *types.Transaction) {
	rand.Seed(time.Now().UnixNano())
	randVal := rand.Intn(len(LampNodeAddressGlobal))

	fromAddr := LampTxFromAddr //LampNodeAddressGlobal[0]
	var args ethapi.SendTxArgs
	args.From = fromAddr
	toAddr := LampTxToAddr
	args.To = &toAddr
	args.Value = (*hexutil.Big)(big.NewInt(int64(randVal)))
	args.Nonce = (*hexutil.Uint64)(&nonce)
	gasPrice := big.NewInt(3)
	args.GasPrice = (*hexutil.Big)(gasPrice)
	//fmt.Printf("from: %s, to: %s, value: %s\n", from, to, value.String())
	args.ActType = blizparam.TypeActNone
	args.ActData = nil

	errArgs := args.WalletSetDefaults()
	if errArgs != nil {
		LogError("WalletSetDefaults", "err", errArgs)
		return nonce, nil
	}
	tx := args.WalletToTransaction()

	return nonce + 1, tx
}

func StartBlockContinuousCheckLoop() {
	currentCnt := 0
	timedOut := int64(0)
	for {
		if LampMinerTestStopped {
			return
		}

		if len(LampNodeBlockNumMap) > currentCnt {
			currentCnt = len(LampNodeBlockNumMap)
			timedOut = 0
		}

		if timedOut >= 60 && timedOut%60 == 0 {
			fmt.Println(fmt.Sprintf("---No new global block inserted in %s, warning",
				common.PrettyDuration(time.Duration(timedOut)*time.Second)))
		}

		if timedOut >= 300 {
			fmt.Println(fmt.Sprintf("---No new global block inserted in %s, test terminated",
				common.PrettyDuration(time.Duration(timedOut)*time.Second)))
			LogCrit(fmt.Sprintf("---No new global block inserted in %s, test terminated",
				common.PrettyDuration(time.Duration(timedOut)*time.Second)))
		}

		timedOut++
		time.Sleep(time.Second)
	}
}
