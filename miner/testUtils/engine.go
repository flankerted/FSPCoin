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
	"sync"
	"time"

	"github.com/contatract/go-contatract/bft/consensus/hbft"
	"github.com/contatract/go-contatract/bft/nodeHbft"
	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/consensus"
	types "github.com/contatract/go-contatract/core/types_elephant"
	"github.com/contatract/go-contatract/core_elephant/state"
	"github.com/contatract/go-contatract/rpc"
)

type foundResult struct {
	Block  *types.Block
	Stages []*types.HBFTStageCompleted
}

type EngineCttForTest struct {
	NodeIndex  int
	hbftNode   *nodeHbft.Node
	sealerLock sync.Mutex
}

func CreateConsensusEngine(nodeIdx int, hbftNode *nodeHbft.Node) consensus.EngineCtt {
	return &EngineCttForTest{
		NodeIndex: nodeIdx,
		hbftNode:  hbftNode,
	}
}

func (engine *EngineCttForTest) Author(header *types.Header) (common.Address, error) {
	return header.Coinbase, nil
}

func (engine *EngineCttForTest) VerifyHeader(chain consensus.ChainReaderCtt, header *types.Header) error {
	return nil
}

func (engine *EngineCttForTest) VerifyBFT(chain consensus.ChainReaderCtt, block *types.Block) error {
	if block.HBFTStageChain() == nil {
		return fmt.Errorf("invalid BFT stage chain: stage is nil")
	}
	stages := block.HBFTStageChain()
	if len(stages) != 2 {
		return fmt.Errorf("invalid BFT stage chain: the count of stages is not valid")
	}
	if parent := chain.GetBlock(block.ParentHash(), block.NumberU64()-1); parent == nil {
		return fmt.Errorf("invalid BFT stage chain: can not find parent block, parentHash: %s, the hash of the block: %s",
			block.ParentHash().TerminalString(), block.Hash().TerminalString())
	} else if parent.NumberU64() > 0 {
		parentStages := parent.HBFTStageChain()
		if len(parentStages) != 2 {
			return fmt.Errorf("invalid BFT stage chain of parent block: the count of stages is not valid")
		}
		if !stages[0].BlockHash.Equal(block.Hash()) {
			return fmt.Errorf("invalid BFT pre-confirm stage's blockHash: %s, the hash of the block: %s",
				stages[0].BlockHash.TerminalString(), block.Hash().TerminalString())
		}
		//if !stages[0].ParentStage.Equal(parentStages[1].Hash()) {
		//	if parentStagesHash := parentStages[1].Hash(); !parentStagesHash.Equal(stages[0].Hash()) {
		//		return fmt.Errorf("invalid BFT stage chain: the parent stage mismatch, have %x, want %x", stages[0].ParentStage, parentStages[1].Hash())
		//	}
		//}
	}
	if !stages[1].ParentStage.Equal(stages[0].Hash()) {
		return fmt.Errorf("invalid BFT stage chain: the stages can not build a chain")
	}
	if stages[0].ReqTimeStamp <= block.Time().Uint64() {
		return fmt.Errorf("invalid BFT stage chain: the time stamp of request mesage is invalid")
	}
	if stages[1].Timestamp <= stages[0].Timestamp || stages[0].Timestamp <= stages[0].ReqTimeStamp {
		return fmt.Errorf("invalid BFT stage chain: the time stamp of stages are invalid")
	}

	sealers, _ := engine.hbftNode.EleBackend.GetRealSealersByNum(block.NumberU64() - 1)
	sealersMap := make(map[common.Address]common.Address)
	if sealers == nil {
		return fmt.Errorf("invalid BFT stage chain: can not get the sealers")
	}

	for i, stage := range stages {
		if len(stage.ValidSigns) <= engine.hbftNode.EleBackend.Get2fRealSealersCnt() {
			return fmt.Errorf("invalid BFT stage chain: the count of valid signatures can not be accepted in stage[%d], have %d, want %d",
				i, len(stage.ValidSigns), engine.hbftNode.EleBackend.Get2fRealSealersCnt()+1)
		}

		//msg := stage.MsgHash.Bytes()
		for _, sealer := range sealers {
			sealersMap[sealer] = sealer
		}
		for j, sign := range stage.ValidSigns {
			//addr, err := ecRecover(msg, sign)
			//if err != nil {
			//	return fmt.Errorf("invalid BFT stage chain: can not recover the address of the signature[%d]", j)
			//}
			addr := common.HexToAddress(string(sign))

			if _, ok := sealersMap[addr]; ok {
				delete(sealersMap, addr)
			} else {
				return fmt.Errorf("invalid BFT stage chain: the address %s recovered from the signature[%d] in stage[%d] is not valid",
					addr.String(), j, i)
			}
		}
	}

	return nil
}

func (engine *EngineCttForTest) VerifyHeaders(chain consensus.ChainReaderCtt, headers []*types.Header) (chan<- struct{}, <-chan error) {
	return nil, nil
}

func (engine *EngineCttForTest) Prepare(chain consensus.ChainReaderCtt, header *types.Header) error {
	return nil
}

func (engine *EngineCttForTest) Finalize(chain consensus.ChainReaderCtt, header *types.Header, state *state.StateDB,
	txs []*types.Transaction, inputTxs []*types.ShardingTxBatch, uncles []*types.Header, receipts []*types.Receipt,
	eBase *common.Address) (*types.Block, error) {

	// Header seems complete, assemble into a block and return
	return types.NewBlock(header, nil, nil, nil, nil, nil, eBase), nil
}

func (engine *EngineCttForTest) Seal(block *types.Block, stop <-chan struct{}) (*types.Block,
	[]*types.HBFTStageCompleted, error) {
	// Create a runner and the multiple search threads it directs
	abort := make(chan struct{})
	found := make(chan foundResult)

	var pend sync.WaitGroup
	pend.Add(1)
	go func() {
		defer pend.Done()
		engine.mine(block, abort, found)
	}()

	// Wait until sealing is terminated or a nonce is found
	var result foundResult
	select {
	case <-stop:
		// Outside abort, stop all miner threads
		close(abort)
		<-found
	case result = <-found:
		// One of the threads found a block, abort all others
		close(abort)
	}

	// Wait for all miners to terminate and return the block
	pend.Wait()

	return result.Block, result.Stages, nil
}

func (engine *EngineCttForTest) mine(block *types.Block, abort chan struct{}, found chan foundResult) {
	ticker := time.NewTicker(nodeHbft.BlockSealingBeat)
	defer ticker.Stop()
	defer engine.hbftNode.SetBusy(false)

	header := block.Header()

seal:
	for {
		select {
		case <-abort:
			// BFT sealing terminated, update stats and abort
			close(found)
			return

		case <-ticker.C:
			engine.sealerLock.Lock()
			var (
				ret        bool
				hbftStages []*types.HBFTStageCompleted
			)

			// First check if the node should re-commit new sealing work for the BFT future block
			if future := engine.hbftNode.EleBackend.BFTOldestFutureBlock(); future != nil {
				if sealer, work := engine.hbftNode.ItsTimeToDoHbftWork(block, future); work {
					testTimes++
					header.BftHash = types.EmptyHbftHash
					found <- foundResult{block.WithSeal(header), engine.hbftNode.CurrentState.BuildCompletedStageChain()}
					engine.sealerLock.Unlock()
					break seal
				} else if !work && common.EmptyAddress(sealer) {
					engine.sealerLock.Unlock()
					continue
				}
			}

			ret = engine.hbftNode.StartHbftWork(block)
			if ret {
				ticker.Stop()
				testTimes++
				ret = engine.hbftNode.BlockSealedCompleted(block.Hash())
			} else {
				engine.sealerLock.Unlock()
				continue
			}
			if ret {
				hbftStages, ret = engine.hbftNode.GetSealedBlockStages()
				if !ret {
					header.BftHash = common.Hash{} // represents that HBFT confirm stage failed
					found <- foundResult{block.WithSeal(header), engine.hbftNode.CurrentState.BuildCompletedStageChain()}
					engine.hbftNode.Warn("GetSealedBlockStages failed, can not find completed chain of two stages")
					engine.sealerLock.Unlock()
					break seal
				}
			} else {
				if engine.hbftNode.GetCurrentStateStage() == hbft.PreConfirm {
					header.BftHash = types.EmptyHbftHash // represents that HBFT pre-confirm stage failed
				} else if engine.hbftNode.GetCurrentStateStage() == hbft.Idle {
					header.BftHash = types.EmptyBlockHash // represents that HBFT request stage failed
				} else {
					header.BftHash = common.Hash{} // represents that HBFT confirm stage failed
				}
				found <- foundResult{block.WithSeal(header), engine.hbftNode.CurrentState.BuildCompletedStageChain()}
				engine.sealerLock.Unlock()
				break seal
			}

			if ret {
				block.SetBftStageComplete(hbftStages)
				header.BftHash = types.CalcBftHash(hbftStages)
				stageChain := make([]*types.HBFTStageCompleted, 0)
				if common.GetConsensusBft() {
					stageChain = engine.hbftNode.CurrentState.BuildCompletedStageChain()
				}

				// Seal and return a block (if still needed)
				select {
				case found <- foundResult{block.WithSeal(header), stageChain}:
					//logger.Trace("【*】Elephant block sealed and reported")
				}
				engine.sealerLock.Unlock()
				break seal
			}
		}
	}

	time.Sleep(nodeHbft.BlockMinPeriodHbft)
}

func (engine *EngineCttForTest) CalcDifficulty(chain consensus.ChainReaderCtt, time uint64, parent *types.Header) *big.Int {
	return big.NewInt(400000)
}

func (engine *EngineCttForTest) APIs(chain consensus.ChainReaderCtt) []rpc.API {
	return nil
}
