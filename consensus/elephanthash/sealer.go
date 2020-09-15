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

package elephanthash

import (
	"math/rand"
	"sync"
	"time"

	"github.com/contatract/go-contatract/bft/consensus/hbft"
	"github.com/contatract/go-contatract/bft/nodeHbft"
	"github.com/contatract/go-contatract/common"
	types "github.com/contatract/go-contatract/core/types_elephant"
	"github.com/contatract/go-contatract/log"
)

type foundResult struct {
	Block  *types.Block
	Stages []*types.HBFTStageCompleted
}

// Seal implements consensus.Engine, attempting to find a nonce that satisfies
// the block's difficulty requirements.
func (elephanthash *Elephanthash) Seal(block *types.Block, stop <-chan struct{}) (*types.Block,
	[]*types.HBFTStageCompleted, error) {
	// Create a runner and the multiple search threads it directs
	abort := make(chan struct{})
	found := make(chan foundResult)

	//elephanthash.lock.Lock()
	//if elephanthash.rand == nil {
	//	seed, err := crand.Int(crand.Reader, big.NewInt(math.MaxInt64))
	//	if err != nil {
	//		elephanthash.lock.Unlock()
	//		return nil, nil, err
	//	}
	//	elephanthash.rand = rand.New(rand.NewSource(seed.Int64()))
	//}
	//elephanthash.lock.Unlock()

	//var parentTime uint64
	//parent := chain.GetHeader(block.Header().ParentHash, block.NumberU64()-1)
	//if parent != nil {
	//	parentTime = parent.Time.Uint64()
	//}

	var pend sync.WaitGroup
	pend.Add(1)
	go func() {
		defer pend.Done()
		elephanthash.mine(block, abort, found)
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

// mine is the actual proof-of-work miner that searches for a nonce starting from
// seed that results in correct final block difficulty.
func (elephanthash *Elephanthash) mine(block *types.Block, abort chan struct{}, found chan foundResult) {
	// Extract some data from the header
	header := block.Header()
	logger := log.New("elephant BFT")
	log.Trace("【*】elephant sealing process start")

	ticker := time.NewTicker(nodeHbft.BlockSealingBeat)
	defer ticker.Stop()
	defer elephanthash.hbftNode.SetBusy(false)

seal:
	for {
		select {
		case <-abort:
			// BFT sealing terminated, update stats and abort
			close(found)
			return

		case <-ticker.C:
			elephanthash.sealerLock.Lock()
			var (
				ret        bool
				hbftStages []*types.HBFTStageCompleted
			)
			if !common.GetConsensusBft() {
				seconds := (rand.Int() % 3) + 3
				time.Sleep(time.Second * time.Duration(seconds))
				ret = true
				hbftStages = elephanthash.hbftNode.GetFakeStagesForTest(uint64(block.NumberU64()))
			} else if elephanthash.hbftNode != nil {
				// First check if the node should re-commit new sealing work for the BFT future block
				if future := elephanthash.hbftNode.EleBackend.BFTOldestFutureBlock(); future != nil {
					if sealer, work := elephanthash.hbftNode.ItsTimeToDoHbftWork(block, future); work {
						header.BftHash = types.EmptyHbftHash
						found <- foundResult{block.WithSeal(header), elephanthash.hbftNode.CurrentState.BuildCompletedStageChain()}
						elephanthash.sealerLock.Unlock()
						break seal
					} else if !work && common.EmptyAddress(sealer) {
						elephanthash.sealerLock.Unlock()
						continue
					}
				}

				ret = elephanthash.hbftNode.StartHbftWork(block)
				if ret {
					ticker.Stop()
					ret = elephanthash.hbftNode.BlockSealedCompleted(block.Hash())
				} else {
					elephanthash.sealerLock.Unlock()
					continue
				}
				if ret {
					hbftStages, ret = elephanthash.hbftNode.GetSealedBlockStages()
					if !ret {
						header.BftHash = common.Hash{} // represents that HBFT confirm stage failed
						found <- foundResult{block.WithSeal(header), elephanthash.hbftNode.CurrentState.BuildCompletedStageChain()}
						log.Warn("GetSealedBlockStages failed, can not find completed chain of two stages")
						elephanthash.sealerLock.Unlock()
						break seal
					}
				} else {
					if elephanthash.hbftNode.GetCurrentStateStage() == hbft.PreConfirm {
						header.BftHash = types.EmptyHbftHash // represents that HBFT pre-confirm stage failed
					} else if elephanthash.hbftNode.GetCurrentStateStage() == hbft.Idle {
						header.BftHash = types.EmptyBlockHash // represents that HBFT request stage failed
					} else {
						header.BftHash = common.Hash{} // represents that HBFT confirm stage failed
					}
					found <- foundResult{block.WithSeal(header), elephanthash.hbftNode.CurrentState.BuildCompletedStageChain()}
					elephanthash.sealerLock.Unlock()
					break seal
				}
			}

			if ret {
				block.SetBftStageComplete(hbftStages)
				header.BftHash = types.CalcBftHash(hbftStages)
				stageChain := make([]*types.HBFTStageCompleted, 0)
				if common.GetConsensusBft() {
					stageChain = elephanthash.hbftNode.CurrentState.BuildCompletedStageChain()
				}

				// Seal and return a block (if still needed)
				select {
				case found <- foundResult{block.WithSeal(header), stageChain}:
					logger.Trace("【*】Elephant block sealed and reported")
					//case <-abort:
					//	logger.Trace("Elephant block sealing abort")
				}
				elephanthash.sealerLock.Unlock()
				break seal
			}
		}
	}

	time.Sleep(nodeHbft.BlockMinPeriodHbft)
	// Datasets are unmapped in a finalizer. Ensure that the dataset stays live
	// during sealing so it's not unmapped while being read.
	// runtime.KeepAlive(dataset)
}
