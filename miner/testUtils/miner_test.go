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
	"github.com/bouk/monkey"
	"github.com/contatract/go-contatract/consensus/ethash"
	"github.com/prashantv/gostub"
	"reflect"
	"testing"
	"time"
	//. "github.com/smartystreets/goconvey/convey"

	"github.com/contatract/go-contatract/miner"
)

func CheckFatalWithInfo(t *testing.T, actual bool, expected bool, info string) {
	if actual != expected {
		t.Fatalf("---Fatal: %s", info)
	}
}

func CheckErrorWithInfo(t *testing.T, actual bool, expected bool, info string) {
	if actual != expected {
		t.Errorf("---Error: %s", info)
	}
}

///////////////////////////////////////////////////////////////////////
///////////////////////////Integration Test////////////////////////////
///////////////////////////////////////////////////////////////////////

var (
	minerBlocksCnt    = uint64(50000)
	lampMinerNodesCnt = 1 // 4
	ifUseRandomDelay  = false
	recoverTimes      = 3000
	diabledNodes      = []int{} // lampMinerNodesCnt - 1}
)

func init() {
	InitGlobal(lampMinerNodesCnt, ifUseRandomDelay, recoverTimes, diabledNodes)
}

func initMinerTest() error {
	LoggerTest = CreateLogger("lampMiner.log")
	for i := range LampMinersGlobal {
		logger := CreateLogger(fmt.Sprintf("lampMiner%d.log", i))

		nodeName := fmt.Sprintf("lampMiner%d", i)
		bftNodeBackend, err := NewBackendForLampMinerTest(nodeName, i, logger)
		if err != nil {
			return err
		}

		bftNodeBackend.TestBlocksCntTotal = minerBlocksCnt
	}

	allLampNode := ""
	for _, n := range LampNodeAddressGlobal {
		allLampNode += "[" + n.String() + "] "
	}
	LogInfo("---Init lamp miner test successfully", "lamp miner nodes", allLampNode)

	return nil
}

func StartNodeMinerLoop(nodeIdx int) {
	LogInfo("---Start mining blocks", "node", LampNodeBackGlobal[nodeIdx].NodeName)
	go LampMinersGlobal[nodeIdx].Worker().Update()
	go LampMinersGlobal[nodeIdx].Start(LampNodeAddressGlobal[nodeIdx])
	go LampNodeBackGlobal[nodeIdx].StartBlockEventLoop()
	go LampNodeBackGlobal[nodeIdx].StartBlockSyncLoop()
}

func TestNodeMiner(t *testing.T) {
	LampNodeBackGlobal = make([]*BackendForMinerTest, lampMinerNodesCnt)

	stubsBlockSealingTime := gostub.Stub(&ethash.SealingTime, 200)
	defer stubsBlockSealingTime.Reset()

	// Patch miner_elephant.Worker
	var s *miner.Worker
	guardUpdate := monkey.PatchInstanceMethod(reflect.TypeOf(s), "Update",
		func(self *miner.Worker) {
			return
		})
	defer guardUpdate.Unpatch()

	// Init test case
	err := initMinerTest()
	if err != nil {
		t.Fatal(err)
	}

	// Re-patch Update()
	guardUpdate = monkey.PatchInstanceMethod(reflect.TypeOf(s), "Update",
		func(self *miner.Worker) {
			LampNodeBackGlobal[LampNodeMapAddressIndex[self.GetEtherbase()]].StartMinerUpdateLoop(self)
		})

	for i := 0; i < len(LampMinersGlobal); i++ {
		go StartNodeMinerLoop(i)
	}
	go StartBlockContinuousCheckLoop()

	for {
		if LampMinerTestStopped {
			time.Sleep(time.Second)
			break
		} else {
			time.Sleep(time.Second)
		}
	}

	if len(LampNodeBackGlobal) > 0 {
		lastBlockHash := LampNodeBackGlobal[0].GetCurrentBlock().Hash()
		blockCnt := LampNodeBackGlobal[0].blockChain.CurrentBlock().NumberU64() + 1
		fmt.Println(fmt.Sprintf("---There are %d blocks sealed in %s", blockCnt-1, LampNodeBackGlobal[0].NodeName))

		//txCnt := uint64(0)
		//for i := uint64(1); i < blockCnt-1; i++ {
		//	for _, tx := range LampNodeBackGlobal[0].blockChain.GetBlockByNumber(i).Transactions() {
		//		txCnt++
		//		LogInfo(fmt.Sprintf("---The hash of the transaction in block %d", i), "tx", tx.Hash().String())
		//	}
		//}
		//CheckErrorWithInfo(t, txCnt == minerBlocksCnt-1, true,
		//	fmt.Sprintf("the count of transactions(%d) is not equal with the expected(%d)", txCnt, minerBlocksCnt-1))

		for _, nodeBackend := range LampNodeBackGlobal {
			CheckFatalWithInfo(t, blockCnt <= nodeBackend.blockChain.CurrentBlock().NumberU64()+1, true,
				fmt.Sprintf("the count of blocks is %d in %s",
					nodeBackend.blockChain.CurrentBlock().NumberU64()+1, nodeBackend.NodeName))
			//So(blockCnt == len(nodeBackend.Blocks), ShouldBeTrue)
			CheckFatalWithInfo(t, lastBlockHash.Equal(nodeBackend.GetBlockByNumber(uint64(blockCnt-1)).Hash()), true,
				fmt.Sprintf("the count of blocks is %d in %s",
					nodeBackend.blockChain.CurrentBlock().NumberU64()+1, nodeBackend.NodeName))
			//So(lastBlockHash.Equal(nodeBackend.GetCurrentBlock().Hash()), ShouldBeTrue)

			fmt.Println("---The newest block checking passed in node", nodeBackend.NodeName)
		}
	}
}

///////////////////////////////////////////////////////////////////////
///////////////////////////Integration Test////////////////////////////
///////////////////////////////////////////////////////////////////////
