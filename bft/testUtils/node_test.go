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
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/contatract/go-contatract/bft/consensus/hbft"
	"github.com/contatract/go-contatract/bft/nodeHbft"
	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/consensus/elephanthash"
	types "github.com/contatract/go-contatract/core/types_elephant"
	core "github.com/contatract/go-contatract/core_elephant"
	"github.com/contatract/go-contatract/core_elephant/state"
	"github.com/contatract/go-contatract/eth"
	sealer "github.com/contatract/go-contatract/miner_elephant"
	"github.com/contatract/go-contatract/rpc"

	"github.com/bouk/monkey"
	"github.com/prashantv/gostub"
	//. "github.com/smartystreets/goconvey/convey"
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
	bftNormalCaseCnt  = uint64(50000)
	bftStateDBCaseCnt = uint64(50000)
	bftNodesCnt       = 4
	ifUseRandomDelay  = true
	recoverTimes      = 3000
	diabledNodes      = []int{bftNodesCnt - 1}
)

func init() {
	InitGlobal(bftNodesCnt, ifUseRandomDelay, recoverTimes, diabledNodes)
}

func initNormalCase() error {
	LoggerTest = CreateLogger("normalCase.log")
	for i := range BftNodesGlobal {
		logger := CreateLogger(fmt.Sprintf("normalCase%d.log", i))

		var lamp *eth.Ethereum
		nodeName := fmt.Sprintf("BFTTestNode%d", i)
		bftNodeBackend, err := NewBackendForTestWithoutStateDB(nodeName, i, lamp, logger)
		if err != nil {
			return err
		}

		bftNodeBackend.TestBlocksCntTotal = bftNormalCaseCnt

		// Add time error
		if i < 2 {
			rand.Seed(time.Now().UnixNano())
			random := rand.Intn(1000)
			BftNodeTimeErrMap[bftNodeBackend.EtherBase] = int64(i*1000 + random)
		} else {
			BftNodeTimeErrMap[bftNodeBackend.EtherBase] = 0
		}

		LogInfo(fmt.Sprintf("---Init testing node, %s: %s, time error: %v", BftNodeBackNoStateGlobal[i].NodeName,
			BftNodeBackNoStateGlobal[i].EtherBase.String(), time.Duration(BftNodeTimeErrMap[bftNodeBackend.EtherBase])*time.Millisecond))
	}

	allBftNode := ""
	for _, n := range BftNodeEtherBasesGlobal {
		allBftNode += "[" + n.String() + "] "
	}
	LogInfo("---Init normal case testing successfully", "BFT nodes", allBftNode)

	return nil
}

func StartNormalCaseLoop(nodeIdx int) {
	LogInfo("---Start sealing blocks", "node", BftNodeBackNoStateGlobal[nodeIdx].NodeName)
	go BftSealersGlobal[nodeIdx].Worker().Update()
	go BftSealersGlobal[nodeIdx].Start(BftNodeEtherBasesGlobal[nodeIdx])
	go BftNodeBackNoStateGlobal[nodeIdx].StartBlockEventLoop()
	go BftNodeBackNoStateGlobal[nodeIdx].StartBlockSyncLoop()
	go BftNodeBackNoStateGlobal[nodeIdx].StartBFTMsgEventLoop()
}

// Case 1: normal case indicates that everything works without state database
func TestNormalCase(t *testing.T) {
	BftNodeBackNoStateGlobal = make([]*BackendForTestWithoutStateDB, bftNodesCnt)

	stubsIsConsenusBft := gostub.Stub(&common.IsConsensusBft, true)
	defer stubsIsConsenusBft.Reset()
	stubsBlockSealingBeat := gostub.Stub(&nodeHbft.BlockSealingBeat, time.Millisecond/20*TimeUnit)
	defer stubsBlockSealingBeat.Reset()
	stubsBlockMinPeriodHbft := gostub.Stub(&nodeHbft.BlockMinPeriodHbft, time.Millisecond*2*TimeUnit) // 2 * timeUnit
	defer stubsBlockMinPeriodHbft.Reset()
	stubsBlockMaxPeriodHbft := gostub.Stub(&nodeHbft.BlockMaxPeriodHbft, time.Millisecond*12*TimeUnit) // 8 * timeUnit
	defer stubsBlockMaxPeriodHbft.Reset()
	stubsBlockMaxStartDelayHbft := gostub.Stub(&nodeHbft.BlockMaxStartDelayHbft,
		(nodeHbft.BlockMaxPeriodHbft-nodeHbft.BlockMinPeriodHbft)/2)
	defer stubsBlockMaxStartDelayHbft.Reset()
	stubsHBFTDebugInfoOn1 := gostub.Stub(&hbft.HBFTDebugInfoOn, true)
	defer stubsHBFTDebugInfoOn1.Reset()
	stubsHBFTDebugInfoOn2 := gostub.Stub(&sealer.HBFTDebugInfoOn, true)
	defer stubsHBFTDebugInfoOn2.Reset()

	// Patch nodeHbft.Node
	var n *nodeHbft.Node
	guardHbftSign := monkey.PatchInstanceMethod(reflect.TypeOf(n), "HbftSign",
		func(_ *nodeHbft.Node, address common.Address, _ string, _ []byte) ([]byte, error) {
			time.Sleep(time.Millisecond * 2 * TimeUnit)
			return []byte(address.String()), nil
		})
	defer guardHbftSign.Unpatch()
	guardHbftEcRecover := monkey.PatchInstanceMethod(reflect.TypeOf(n), "HbftEcRecover",
		func(_ *nodeHbft.Node, _, sig []byte) (common.Address, error) {
			return common.HexToAddress(string(sig)), nil
		})
	defer guardHbftEcRecover.Unpatch()

	// Patch core.BlockChain
	var blockChain *core.BlockChain
	guardGetBlock := monkey.PatchInstanceMethod(reflect.TypeOf(blockChain), "GetBlock",
		func(bc *core.BlockChain, hash common.Hash, number uint64) *types.Block {
			blockByNum := BftNodeBackNoStateGlobal[BftNodeMapAddressIndex[bc.GetEtherBase()]].GetBlockByNumber(number)
			if blockByNum != nil {
				if hash.Equal(blockByNum.Hash()) {
					return blockByNum
				}
			}
			return nil
		})
	defer guardGetBlock.Unpatch()
	guardGetBlockByNumber := monkey.PatchInstanceMethod(reflect.TypeOf(blockChain), "GetBlockByNumber",
		func(bc *core.BlockChain, number uint64) *types.Block {
			return BftNodeBackNoStateGlobal[BftNodeMapAddressIndex[bc.GetEtherBase()]].GetBlockByNumber(number)
		})
	defer guardGetBlockByNumber.Unpatch()
	guardGetBlocksFromHash := monkey.PatchInstanceMethod(reflect.TypeOf(blockChain), "GetBlocksFromHash",
		func(bc *core.BlockChain, hash common.Hash, n int) (blocks []*types.Block) {
			targetBlock := BftNodeBackNoStateGlobal[BftNodeMapAddressIndex[bc.GetEtherBase()]].GetBlockByHash(hash)
			if targetBlock == nil {
				return
			}
			number := targetBlock.NumberU64()
			for i := 0; i < n; i++ {
				block := bc.GetBlock(hash, number)
				if block == nil {
					break
				}
				blocks = append(blocks, block)
				hash = block.ParentHash()
				number--
			}
			return
		})
	defer guardGetBlocksFromHash.Unpatch()
	guardCurrentBlock := monkey.PatchInstanceMethod(reflect.TypeOf(blockChain), "CurrentBlock",
		func(bc *core.BlockChain) *types.Block {
			return BftNodeBackNoStateGlobal[BftNodeMapAddressIndex[bc.GetEtherBase()]].GetCurrentBlock()
		})
	defer guardCurrentBlock.Unpatch()
	guardBFTNewestFutureBlock := monkey.PatchInstanceMethod(reflect.TypeOf(blockChain), "BFTNewestFutureBlock",
		func(bc *core.BlockChain) *types.Block {
			return BftNodeBackNoStateGlobal[BftNodeMapAddressIndex[bc.GetEtherBase()]].BFTNewestFutureBlock()
		})
	defer guardBFTNewestFutureBlock.Unpatch()
	guardBFTOldestFutureBlock := monkey.PatchInstanceMethod(reflect.TypeOf(blockChain), "BFTOldestFutureBlock",
		func(bc *core.BlockChain) *types.Block {
			return BftNodeBackNoStateGlobal[BftNodeMapAddressIndex[bc.GetEtherBase()]].BFTOldestFutureBlock()
		})
	defer guardBFTOldestFutureBlock.Unpatch()
	guardBFTFutureBlocks := monkey.PatchInstanceMethod(reflect.TypeOf(blockChain), "BFTFutureBlocks",
		func(bc *core.BlockChain) []*types.Block {
			return BftNodeBackNoStateGlobal[BftNodeMapAddressIndex[bc.GetEtherBase()]].BFTFutureBlocks()
		})
	defer guardBFTFutureBlocks.Unpatch()
	guardClearBFTFutureBlock := monkey.PatchInstanceMethod(reflect.TypeOf(blockChain), "ClearBFTFutureBlock",
		func(bc *core.BlockChain, hash common.Hash) {
			BftNodeBackNoStateGlobal[BftNodeMapAddressIndex[bc.GetEtherBase()]].ClearBFTFutureBlock(hash)
		})
	defer guardClearBFTFutureBlock.Unpatch()
	guardStateAt := monkey.PatchInstanceMethod(reflect.TypeOf(blockChain), "StateAt",
		func(bc *core.BlockChain, _ common.Hash) (*state.StateDB, error) {
			return nil, nil
		})
	defer guardStateAt.Unpatch()

	// Patch lamp
	var lamp *eth.Ethereum
	guardCurrEleSealersBlockNum := monkey.PatchInstanceMethod(reflect.TypeOf(lamp), "CurrEleSealersBlockNum",
		func(_ *eth.Ethereum) rpc.BlockNumber {
			return rpc.BlockNumber(0)
		})
	defer guardCurrEleSealersBlockNum.Unpatch()
	guardCurrEleSealersBlockHash := monkey.PatchInstanceMethod(reflect.TypeOf(lamp), "CurrEleSealersBlockHash",
		func(_ *eth.Ethereum) common.Hash {
			return common.Hash{}
		})
	defer guardCurrEleSealersBlockHash.Unpatch()

	// Patch miner_elephant.Worker
	var s *sealer.Worker
	guardCheckIntermediateRoot := monkey.PatchInstanceMethod(reflect.TypeOf(s), "CheckIntermediateRoot",
		func(_ *sealer.Worker, _ *types.Block) error {
			return nil
		})
	defer guardCheckIntermediateRoot.Unpatch()
	guardCommitTransactions := monkey.PatchInstanceMethod(reflect.TypeOf(s), "CommitTransactions",
		func(_ *sealer.Worker, _ bool, _ *types.Block, _ *sealer.Work) error {
			return nil
		})
	defer guardCommitTransactions.Unpatch()
	guardConnectBlock := monkey.PatchInstanceMethod(reflect.TypeOf(s), "ConnectBlock",
		func(self *sealer.Worker, block *types.Block, _ *sealer.Work, isFutureBlock bool, mustCommitNewWork *bool) bool {
			BftNodeBackNoStateGlobal[BftNodeMapAddressIndex[self.GetEtherbase()]].ConnectBlock(self, block, isFutureBlock, mustCommitNewWork)
			return true
		})
	defer guardConnectBlock.Unpatch()
	guardUpdate := monkey.PatchInstanceMethod(reflect.TypeOf(s), "Update",
		func(self *sealer.Worker) {
			return
		})
	defer guardUpdate.Unpatch()

	err := initNormalCase()
	if err != nil {
		t.Fatal(err)
	}

	// Re-patch Update()
	guardUpdate = monkey.PatchInstanceMethod(reflect.TypeOf(s), "Update",
		func(self *sealer.Worker) {
			BftNodeBackNoStateGlobal[BftNodeMapAddressIndex[self.GetEtherbase()]].StartSealerUpdateLoop(self)
		})

	// Patch hbft.GetTimeNowMs to simulate time error
	guardGetTimeNowMs := monkey.Patch(hbft.GetTimeNowMs, func(address common.Address) int64 {
		return time.Now().UnixNano()/1e6 + BftNodeTimeErrMap[address]*TimeUnit/1000
	})
	defer guardGetTimeNowMs.Unpatch()

	for i := 0; i < len(BftNodesGlobal); i++ {
		go StartNormalCaseLoop(i)
	}
	go StartBlockContinuousCheckLoop()

	for {
		if BftTestCaseStopped {
			time.Sleep(time.Second)
			break
		} else {
			time.Sleep(time.Second)
		}
	}

	//Convey("HBFT integration test case 1: TestNormalCase", t, func() {
	//
	//})

	// HBFT integration test case 1: TestNormalCase
	if len(BftNodeBackNoStateGlobal) > 0 {
		lastBlockHash := BftNodeBackNoStateGlobal[0].GetCurrentBlock().Hash()
		blockCnt := len(BftNodeBackNoStateGlobal[0].Blocks)
		fmt.Println(fmt.Sprintf("---There are %d blocks sealed in %s", blockCnt-1, BftNodeBackNoStateGlobal[0].NodeName))

		for _, nodeBackend := range BftNodeBackNoStateGlobal {
			CheckFatalWithInfo(t, blockCnt <= len(nodeBackend.Blocks), true,
				fmt.Sprintf("the count of blocks is %d in %s",
					len(nodeBackend.Blocks), nodeBackend.NodeName))
			//So(blockCnt == len(nodeBackend.Blocks), ShouldBeTrue)
			CheckFatalWithInfo(t, lastBlockHash.Equal(nodeBackend.GetBlockByNumber(uint64(blockCnt-1)).Hash()), true,
				fmt.Sprintf("the count of blocks is %d in %s",
					len(nodeBackend.Blocks), nodeBackend.NodeName))
			//So(lastBlockHash.Equal(nodeBackend.GetCurrentBlock().Hash()), ShouldBeTrue)

			fmt.Println("---The newest block checking passed in node", nodeBackend.NodeName)
		}
	}
}

func initStateDBCase() error {
	LoggerTest = CreateLogger("stateDBCase.log")

	for i := range BftNodesGlobal {
		logger := CreateLogger(fmt.Sprintf("stateDBCase%d.log", i))

		var lamp *eth.Ethereum
		nodeName := fmt.Sprintf("BFTTestNode%d", i)
		bftNodeBackend, err := NewBackendForTestWithStateDB(nodeName, i, lamp, logger)
		if err != nil {
			return err
		}

		bftNodeBackend.TestBlocksCntTotal = bftStateDBCaseCnt

		// Add time error
		if i < 2 {
			rand.Seed(time.Now().UnixNano())
			BftNodeTimeErrMap[bftNodeBackend.EtherBase] = int64(i*1000 + rand.Intn(1000))
		} else {
			BftNodeTimeErrMap[bftNodeBackend.EtherBase] = 0
		}

		// set tx from and to
		if i == 0 {
			BftTxFromAddr = bftNodeBackend.EtherBase
		} else if i == 1 {
			BftTxToAddr = bftNodeBackend.EtherBase
		}

		LogInfo(fmt.Sprintf("---Init testing node, %s: %s, time error: %v", BftNodeBackWithStateGlobal[i].NodeName,
			BftNodeBackWithStateGlobal[i].EtherBase.String(), time.Duration(BftNodeTimeErrMap[bftNodeBackend.EtherBase])*time.Millisecond))
	}

	allBftNode := ""
	for _, n := range BftNodeEtherBasesGlobal {
		allBftNode += "[" + n.String() + "] "
	}
	LogInfo("---Init normal case testing successfully", "BFT nodes", allBftNode)

	return nil
}

func StartStateDBCaseLoop(nodeIdx int) {
	LogInfo("---Start sealing blocks", "node", BftNodeBackWithStateGlobal[nodeIdx].NodeName)
	go BftSealersGlobal[nodeIdx].Worker().Update()
	go BftSealersGlobal[nodeIdx].Start(BftNodeEtherBasesGlobal[nodeIdx])
	go BftNodeBackWithStateGlobal[nodeIdx].StartBlockEventLoop()
	go BftNodeBackWithStateGlobal[nodeIdx].StartBlockSyncLoop()
	go BftNodeBackWithStateGlobal[nodeIdx].StartBFTMsgEventLoop()
}

func ItsTimeToDoHbftWork(n *nodeHbft.Node, block, future *types.Block) (common.Address, bool) {
	if n.IsBusy() {
		return common.Address{}, false
	}

	sealers, _ := n.EleBackend.GetCurRealSealers()
	count := len(sealers)
	if count < nodeHbft.NodeServerMinCountHbft {
		n.Log.Info("insufficient node server", "count", count)
		time.Sleep(nodeHbft.BlockMinPeriodHbft)
		return common.Address{}, false
	}

	sealerAddr := n.GetCurrentSealer(sealers, true)
	if common.EmptyAddress(sealerAddr) {
		return common.Address{}, false
	}

	coinbase := block.Header().Coinbase
	if !sealerAddr.Equal(coinbase) {
		n.Log.Trace("waiting to be a sealer", "now sealer address", sealerAddr.String(), "my address", coinbase.String())
		return common.Address{}, false
	}

	if future != nil {
		oldestFutureHash := future.Hash()
		futures := n.EleBackend.BFTFutureBlocks()
		if (len(futures) > 1 &&
			oldestFutureHash.Equal(futures[1].ParentHash())) || (!oldestFutureHash.Equal(block.ParentHash())) {
			return sealerAddr, true
		} else {
			// Check unfinished block which has two completed stages but in timed out confirm stage
			if len(futures) == 1 {
				hbftStages, ret := n.GetSealedBlockStages()
				if ret && oldestFutureHash.Equal(hbftStages[0].BlockHash) && oldestFutureHash.Equal(hbftStages[1].BlockHash) {
					n.Log.Info("There is an unfinished block which has two completed stages but in timed out confirm stage to handle first",
						"hash", oldestFutureHash)
					return sealerAddr, true
				}
			}
			return sealerAddr, false
		}
	}

	testTimes++
	return sealerAddr, true
}

// Case 2: stateDB case indicates that everything works with state database
func TestStateDBCase(t *testing.T) {
	BftNodeBackWithStateGlobal = make([]*BackendForTestWithStateDB, bftNodesCnt)

	stubsIsConsenusBft := gostub.Stub(&common.IsConsensusBft, true)
	defer stubsIsConsenusBft.Reset()
	stubsBlockSealingBeat := gostub.Stub(&nodeHbft.BlockSealingBeat, time.Millisecond/20*TimeUnit)
	defer stubsBlockSealingBeat.Reset()
	stubsBlockMinPeriodHbft := gostub.Stub(&nodeHbft.BlockMinPeriodHbft, time.Millisecond*2*TimeUnit) // 2 * timeUnit
	defer stubsBlockMinPeriodHbft.Reset()
	stubsBlockMaxPeriodHbft := gostub.Stub(&nodeHbft.BlockMaxPeriodHbft, time.Millisecond*12*TimeUnit) // 8 * timeUnit
	defer stubsBlockMaxPeriodHbft.Reset()
	stubsBlockMaxStartDelayHbft := gostub.Stub(&nodeHbft.BlockMaxStartDelayHbft,
		(nodeHbft.BlockMaxPeriodHbft-nodeHbft.BlockMinPeriodHbft)/2)
	defer stubsBlockMaxStartDelayHbft.Reset()
	stubsHBFTDebugInfoOn1 := gostub.Stub(&hbft.HBFTDebugInfoOn, true)
	defer stubsHBFTDebugInfoOn1.Reset()
	stubsHBFTDebugInfoOn2 := gostub.Stub(&sealer.HBFTDebugInfoOn, true)
	defer stubsHBFTDebugInfoOn2.Reset()

	// Patch nodeHbft.Node
	var n *nodeHbft.Node
	guardHbftSign := monkey.PatchInstanceMethod(reflect.TypeOf(n), "HbftSign",
		func(_ *nodeHbft.Node, address common.Address, _ string, _ []byte) ([]byte, error) {
			time.Sleep(time.Millisecond * 2 * TimeUnit)
			return []byte(address.String()), nil
		})
	defer guardHbftSign.Unpatch()
	guardHbftEcRecover := monkey.PatchInstanceMethod(reflect.TypeOf(n), "HbftEcRecover",
		func(_ *nodeHbft.Node, _, sig []byte) (common.Address, error) {
			return common.HexToAddress(string(sig)), nil
		})
	defer guardHbftEcRecover.Unpatch()
	guardItsTimeToDoHbftWork := monkey.PatchInstanceMethod(reflect.TypeOf(n), "ItsTimeToDoHbftWork",
		func(n *nodeHbft.Node, block, future *types.Block) (common.Address, bool) {
			return ItsTimeToDoHbftWork(n, block, future)
		})
	defer guardItsTimeToDoHbftWork.Unpatch()

	// Patch core.BlockChain
	//var blockChain *core.BlockChain

	// Patch lamp
	var lamp *eth.Ethereum
	guardCurrEleSealersBlockNum := monkey.PatchInstanceMethod(reflect.TypeOf(lamp), "CurrEleSealersBlockNum",
		func(_ *eth.Ethereum) rpc.BlockNumber {
			return rpc.BlockNumber(0)
		})
	defer guardCurrEleSealersBlockNum.Unpatch()
	guardCurrEleSealersBlockHash := monkey.PatchInstanceMethod(reflect.TypeOf(lamp), "CurrEleSealersBlockHash",
		func(_ *eth.Ethereum) common.Hash {
			return common.Hash{}
		})
	defer guardCurrEleSealersBlockHash.Unpatch()
	guardCurrBlockNum := monkey.PatchInstanceMethod(reflect.TypeOf(lamp), "CurrBlockNum",
		func(_ *eth.Ethereum) rpc.BlockNumber {
			return rpc.BlockNumber(0)
		})
	defer guardCurrBlockNum.Unpatch()

	// Patch miner_elephant.Worker
	var s *sealer.Worker
	guardUpdate := monkey.PatchInstanceMethod(reflect.TypeOf(s), "Update",
		func(self *sealer.Worker) {
			return
		})
	defer guardUpdate.Unpatch()

	// Patch elephanthash
	var c *elephanthash.Elephanthash
	guardEcRecover := monkey.Patch(elephanthash.EcRecover, func(data, sig []byte) (common.Address, error) {
		return common.HexToAddress(string(sig)), nil
	})
	defer guardEcRecover.Unpatch()
	guardVerifyHeaderLampBase := monkey.PatchInstanceMethod(reflect.TypeOf(c), "VerifyHeaderLampBase",
		func(_ *elephanthash.Elephanthash, _ *types.Header) error {
			return nil
		})
	defer guardVerifyHeaderLampBase.Unpatch()

	// Patch types_elephant signer
	var fs types.FrontierSigner
	guardSender := monkey.PatchInstanceMethod(reflect.TypeOf(fs), "Sender",
		func(_ types.FrontierSigner, _ *types.Transaction) (common.Address, error) {
			return BftTxFromAddr, nil
		})
	defer guardSender.Unpatch()
	var sEIP types.EIP155Signer
	guardSenderEIP := monkey.PatchInstanceMethod(reflect.TypeOf(sEIP), "Sender",
		func(_ types.EIP155Signer, _ *types.Transaction) (common.Address, error) {
			return BftTxFromAddr, nil
		})
	defer guardSenderEIP.Unpatch()

	// Init test case
	err := initStateDBCase()
	if err != nil {
		t.Fatal(err)
	}

	// Re-patch Update()
	guardUpdate = monkey.PatchInstanceMethod(reflect.TypeOf(s), "Update",
		func(self *sealer.Worker) {
			BftNodeBackWithStateGlobal[BftNodeMapAddressIndex[self.GetEtherbase()]].StartSealerUpdateLoop(self)
		})

	// Patch hbft.GetTimeNowMs to simulate time error
	guardGetTimeNowMs := monkey.Patch(hbft.GetTimeNowMs, func(address common.Address) int64 {
		return time.Now().UnixNano()/1e6 + BftNodeTimeErrMap[address]*TimeUnit/1000
	})
	defer guardGetTimeNowMs.Unpatch()

	for i := 0; i < len(BftNodesGlobal); i++ {
		go StartStateDBCaseLoop(i)
	}
	go StartBlockContinuousCheckLoop()

	for {
		if BftTestCaseStopped {
			time.Sleep(time.Second)
			for i := 0; i < len(BftNodesGlobal); i++ {
				go BftNodeBackWithStateGlobal[i].QuitBlockBroadcastLoop()
			}
			break
		} else {
			time.Sleep(time.Second)
		}
	}

	// HBFT integration test case 2: TestStateDBCase
	if len(BftNodeBackWithStateGlobal) > 0 {
		lastBlockHash := BftNodeBackWithStateGlobal[0].GetCurrentBlock().Hash()
		blockCnt := BftNodeBackWithStateGlobal[0].blockChain.CurrentBlock().NumberU64() + 1
		fmt.Println(fmt.Sprintf("---There are %d blocks sealed in %s", blockCnt-1, BftNodeBackWithStateGlobal[0].NodeName))

		txCnt := uint64(0)
		for i := uint64(1); i < blockCnt-1; i++ {
			for _, tx := range BftNodeBackWithStateGlobal[0].blockChain.GetBlockByNumber(i).Transactions() {
				txCnt++
				LogInfo(fmt.Sprintf("---The hash of the transaction in block %d", i), "tx", tx.Hash().String())
			}
		}
		CheckErrorWithInfo(t, txCnt == bftStateDBCaseCnt-1, true,
			fmt.Sprintf("the count of transactions(%d) is not equal with the expected(%d)", txCnt, bftStateDBCaseCnt-1))

		for _, nodeBackend := range BftNodeBackWithStateGlobal {
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
