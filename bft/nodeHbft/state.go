package nodeHbft

import (
	"errors"
	"fmt"
	"sync"

	"github.com/contatract/go-contatract/bft/consensus/hbft"
	"github.com/contatract/go-contatract/common"
	types "github.com/contatract/go-contatract/core/types_elephant"
	"github.com/contatract/go-contatract/log"
)

const (
	SameViewPrimaryErr    = "BFT message from the same view"
	InvalidViewOrSeqIDErr = "the view ID or sequence ID of BFT message is not valid"
	InvalidParentHashErr  = "the hash of the parent completed stage is not valid"
	InvalidSignsCntErr    = "the signatures of the completed stage is not sufficient"
	NoSpecificBlockErr    = "the block in the stage is not verified or received"
	InvalidNewBlockErr    = "new block that the BFT message contained is not valid"
	InvalidCompletedErr   = "the completed stage is not valid"
	ShouldNotHandleErr    = "the message we received should not be handled due to the stage of the node"
)

type State struct {
	NodeCoinBase common.Address

	ViewPrimary  string
	ViewID       uint64
	SequenceID   uint64
	CurrentStage hbft.Stage
	MsgHash      common.Hash
	Blocks       []*types.Block
	SignedBlocks []*types.Block
	completed    *sync.Map //map[common.Hash]*types.HBFTStageCompleted
	MsgLogs      *hbft.MsgLogs

	Cnt2fRealSealers int

	logger log.Logger

	mu          sync.Mutex
	completedMu sync.RWMutex
}

func CreateState(coinbase common.Address, viewPrimary string, viewID uint64, sequenceID uint64,
	Cnt2fRealSealers int, logger log.Logger) *State {
	return &State{
		NodeCoinBase: coinbase,
		ViewPrimary:  viewPrimary,
		ViewID:       viewID,
		SequenceID:   sequenceID,
		CurrentStage: hbft.Idle,
		Blocks:       make([]*types.Block, 0),
		SignedBlocks: make([]*types.Block, 0),
		completed:    new(sync.Map), //make(map[common.Hash]*types.HBFTStageCompleted),
		MsgLogs: &hbft.MsgLogs{
			PreConfirmMsgs: new(sync.Map),
			ConfirmMsgs:    new(sync.Map),
		},
		Cnt2fRealSealers: Cnt2fRealSealers,
		logger:           logger,
	}
}

func (state *State) UpdateState(coinbase common.Address, viewPrimary string, viewID uint64, sequenceID uint64, currentBlockNum uint64) {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.NodeCoinBase = coinbase
	state.ViewPrimary = viewPrimary
	state.ViewID = viewID
	state.SequenceID = sequenceID
	state.ViewPrimary = viewPrimary

	state.CurrentStage = hbft.Idle
	state.MsgLogs.ClearOutdated(currentBlockNum)
}

func GetCompletedLength(syncMap *sync.Map) (length int) {
	length = 0
	syncMap.Range(func(k, v interface{}) bool {
		length++
		return true
	})

	return
}

func (state *State) ShowCompletedStagesDebug(info string) {
	state.completedMu.RLock()
	defer state.completedMu.RUnlock()

	state.completed.Range(func(k, v interface{}) bool {
		state.HBFTDebugInfo(fmt.Sprintf("%s completed: %s, blockHash: %s", info, k.(common.Hash).TerminalString(),
			v.(*types.HBFTStageCompleted).BlockHash.TerminalString()))
		state.HBFTDebugInfo(fmt.Sprintf("%s parent Completed: %s", info, v.(*types.HBFTStageCompleted).ParentStage.TerminalString()))
		return true
	})
}

func (state *State) GetCompletedStage(hash common.Hash) (*types.HBFTStageCompleted, bool) {
	state.completedMu.RLock()
	defer state.completedMu.RUnlock()

	v, ok := state.completed.Load(hash)
	if ok {
		completed := v.(*types.HBFTStageCompleted)
		return completed, ok
	} else {
		return nil, ok
	}
}

func (state *State) SliceCompletedStages() []*types.HBFTStageCompleted {
	state.completedMu.RLock()
	defer state.completedMu.RUnlock()

	sliceCompleted := make([]*types.HBFTStageCompleted, 0)
	state.completed.Range(func(k, v interface{}) bool {
		sliceCompleted = append(sliceCompleted, v.(*types.HBFTStageCompleted))
		return true
	})

	return sliceCompleted
}

func (state *State) GetNewestStateCompletedStage() *types.HBFTStageCompleted {
	state.completedMu.RLock()
	defer state.completedMu.RUnlock()

	curStateStage := new(types.HBFTStageCompleted)
	curStateStage.MsgHash = common.Hash{}
	viewID, seqID, msgType, blockNum := uint64(0), uint64(0), uint(0), uint64(0)
	num := 0
	if state.completed != nil {
		state.completed.Range(func(k, v interface{}) bool {
			completed := v.(*types.HBFTStageCompleted)
			if num == 0 {
				*curStateStage, viewID, seqID, msgType, blockNum = *completed, completed.ViewID, completed.SequenceID, completed.MsgType, completed.BlockNum
			} else if completed.SequenceID > seqID || (completed.SequenceID == seqID && completed.ViewID > viewID) ||
				(completed.SequenceID == seqID && completed.ViewID == viewID && completed.MsgType > msgType) ||
				(completed.SequenceID == seqID && completed.ViewID == viewID && completed.MsgType == msgType && completed.BlockNum > blockNum) {
				*curStateStage, viewID, seqID, msgType, blockNum = *completed, completed.ViewID, completed.SequenceID, completed.MsgType, completed.BlockNum
			}
			num++
			return true
		})
	}

	return curStateStage
}

func (state *State) GetNewestCompletedStage(currentBlock *types.Block) *types.HBFTStageCompleted {
	HBFTStageChain := currentBlock.HBFTStageChain()
	if HBFTStageChain == nil || len(HBFTStageChain) == 0 {
		return state.GetNewestStateCompletedStage()
	}

	blockStage := HBFTStageChain[len(HBFTStageChain)-1]
	stateStage := state.GetNewestStateCompletedStage()
	if stateStage.SequenceID > blockStage.SequenceID ||
		(stateStage.SequenceID == blockStage.SequenceID && stateStage.ViewID > blockStage.ViewID) ||
		(stateStage.SequenceID == blockStage.SequenceID && stateStage.ViewID == blockStage.ViewID && stateStage.MsgType > blockStage.MsgType) {
		return stateStage
	} else {
		return blockStage
	}
}

func (state *State) CheckView(msg hbft.MsgHbftConsensus, currentSeq, currentView uint64) error {
	switch msg.(type) {
	case *hbft.PreConfirmMsg:
		//m := msg.(*hbft.PreConfirmMsg)
		if (state.CurrentStage == hbft.PreConfirm || state.CurrentStage == hbft.Confirm) && state.ViewID == msg.GetViewID() &&
			state.SequenceID == msg.GetSequenceID() && state.ViewPrimary == msg.GetViewPrimary() && msg.GetNewBlock().NumberU64() != 1 {
			return errors.New(fmt.Sprintf("CheckView failed in pre-confirm msg, %s, stateSeq: %d, stateView: %d, statePrim: %s",
				SameViewPrimaryErr, state.SequenceID, state.ViewID, state.ViewPrimary))
		} else if currentSeq != msg.GetSequenceID() || currentView != msg.GetViewID() {
			return errors.New(fmt.Sprintf("CheckView failed in pre-confirm msg, %s, currentSeq: %d, currentView: %d, statePrim: %s",
				InvalidViewOrSeqIDErr, currentSeq, currentView, state.ViewPrimary))
		}

	case *hbft.ConfirmMsg:
		//m := msg.(*hbft.ConfirmMsg)
		if state.CurrentStage != hbft.Idle &&
			(state.ViewID != msg.GetViewID() || state.SequenceID != msg.GetSequenceID() || state.ViewPrimary != msg.GetViewPrimary()) {
			return errors.New(fmt.Sprintf("CheckView failed in confirm msg, %s, stateSeq: %d, stateView: %d, statePrim: %s",
				InvalidViewOrSeqIDErr, msg.GetSequenceID(), msg.GetViewID(), state.ViewPrimary))
		}

	case *hbft.OldestFutureMsg:
		//m := msg.(*hbft.OldestFutureMsg)
		if currentSeq != msg.GetSequenceID() || currentView != msg.GetViewID() {
			return errors.New(fmt.Sprintf("CheckView failed in oldest-future msg, %s, currentSeq: %d, currentView: %d, statePrim: %s",
				InvalidViewOrSeqIDErr, currentSeq, currentView, state.ViewPrimary))
		}
	}

	return nil
}

func (state *State) CheckCurrentState(msg interface{}, currentBlock, BFTFuture *types.Block, cnt2f int, addFutureParent bool) error {
	switch msg.(type) {
	case *hbft.PreConfirmMsg:
		preConfirmMsg := msg.(*hbft.PreConfirmMsg)
		err := state.CheckCompletedStage(preConfirmMsg.CurState.Completed, currentBlock, cnt2f)
		if err != nil {
			return err
		}

		newestBlockInCompletedStage := state.GetNewestStateCompletedStage().BlockHash
		if state.CurrentStage == hbft.Confirm && !newestBlockInCompletedStage.Equal(preConfirmMsg.NewBlock.ParentHash()) {
			//return state.checkIfClearFutureBlocks(newestBlockInCompletedStage), errors.New(fmt.Sprintf(
			return errors.New(fmt.Sprintf(
				"CheckCurrentState failed, %s case 1, whose parent is supposed to be %s instead of %s",
				InvalidNewBlockErr, newestBlockInCompletedStage.TerminalString(), preConfirmMsg.NewBlock.ParentHash().TerminalString()))
		} else if BFTFuture != nil {
			oldestFutureBlockInConfirmHeight := BFTFuture.NumberU64()
			if preConfirmMsg.NewBlock.NumberU64() == oldestFutureBlockInConfirmHeight+1 &&
				!newestBlockInCompletedStage.Equal(preConfirmMsg.NewBlock.ParentHash()) && !addFutureParent {
				//return state.checkIfClearFutureBlocks(newestBlockInCompletedStage), errors.New(fmt.Sprintf(
				return errors.New(fmt.Sprintf(
					"CheckCurrentState failed, %s case 2, whose parent is supposed to be %s instead of %s",
					InvalidNewBlockErr, newestBlockInCompletedStage.TerminalString(), preConfirmMsg.NewBlock.ParentHash().TerminalString()))
			} else if preConfirmMsg.NewBlock.NumberU64() == oldestFutureBlockInConfirmHeight &&
				!newestBlockInCompletedStage.Equal(preConfirmMsg.NewBlock.Hash()) {
				//return state.checkIfClearFutureBlocks(newestBlockInCompletedStage), errors.New(fmt.Sprintf(
				return errors.New(fmt.Sprintf(
					"CheckCurrentState failed, %s case 3, whose parent is supposed to be %s instead of %s",
					InvalidNewBlockErr, newestBlockInCompletedStage.TerminalString(), preConfirmMsg.NewBlock.ParentHash().TerminalString()))
			}
		} else if state.CurrentStage != hbft.Confirm && BFTFuture == nil {
			currentBlockHash := currentBlock.Hash()
			if addFutureParent && currentBlockHash.Equal(preConfirmMsg.FutureParent.ParentHash()) {
				return nil
			}
			if !currentBlockHash.Equal(preConfirmMsg.NewBlock.ParentHash()) {
				return errors.New(fmt.Sprintf(
					"CheckCurrentState failed, %s case 4, whose parent is supposed to be %s instead of %s",
					InvalidNewBlockErr, currentBlockHash.TerminalString(), preConfirmMsg.NewBlock.ParentHash().TerminalString()))
			}
		}

	case *hbft.ConfirmMsg: // BFTFuture == nil
		confirmMsg := msg.(*hbft.ConfirmMsg)
		err := state.CheckCompletedStage(confirmMsg.CurState.Completed, currentBlock, cnt2f)
		if err != nil {
			return err
		}

		if len(state.SignedBlocks) == 0 {
			return errors.New(fmt.Sprintf("CheckCurrentState failed, %s", NoSpecificBlockErr))
		}
		if len(confirmMsg.CurState.Completed) == 0 {
			return errors.New(fmt.Sprintf("CheckCurrentState failed, %s", InvalidCompletedErr))
		}
		newestBlockInPreConfirm := state.SignedBlocks[len(state.SignedBlocks)-1]
		newestBlockHashInMsg := confirmMsg.CurState.Completed[len(confirmMsg.CurState.Completed)-1].BlockHash
		if state.CurrentStage == hbft.PreConfirm && !newestBlockHashInMsg.Equal(newestBlockInPreConfirm.Hash()) {
			return errors.New(fmt.Sprintf("CheckCurrentState failed, %s", InvalidNewBlockErr))
		} else if state.CurrentStage == hbft.Confirm {
			if currentBlock.Number().Uint64() >= newestBlockInPreConfirm.Number().Uint64() {
				state.Reset(currentBlock.Number().Uint64(), true)
			}
			return errors.New(fmt.Sprintf("CheckCurrentState failed, %s", ShouldNotHandleErr))
		}

	}

	return nil
}

func (state *State) CheckCompletedStage(completedStages []*types.HBFTStageCompleted, currentBlock *types.Block, cnt2f int) error {
	var parentHash common.Hash
	for i := 0; i < len(completedStages); i++ {
		if i == 0 {
			blockStage1 := new(types.HBFTStageCompleted)
			HBFTStageChain := currentBlock.HBFTStageChain()
			if HBFTStageChain != nil && len(HBFTStageChain) != 0 {
				blockStage1 = HBFTStageChain[len(HBFTStageChain)-1]
			}
			if blockStage1 == nil {
				return errors.New(fmt.Sprintf("CheckCompletedStage failed case 1, %s", InvalidParentHashErr))
			}

			blockStage1Hash := blockStage1.Hash()
			if currentBlock.NumberU64() > 0 && blockStage1.MsgType == uint(hbft.MsgPreConfirm) {
				// if the msgType of newest stage in the latest block is MsgPreConfirm, the stage may also belong to the next block,
				// or the new completed stage's parent stage is equal with the latest block's completed stage 0(stage replaced)
				if !blockStage1Hash.Equal(completedStages[i].Hash()) && // !blockStage1Hash.Equal(completedStages[i].ParentStage) &&
					!(blockStage1.ParentStage.Equal(completedStages[i].ParentStage) &&
						completedStages[i].ReqTimeStamp > blockStage1.ReqTimeStamp) {
					return errors.New(fmt.Sprintf("CheckCompletedStage failed case 2, %s", InvalidParentHashErr))
				}
			} else {
				if !blockStage1Hash.Equal(completedStages[i].ParentStage) &&
					!(blockStage1.MsgType == uint(hbft.MsgConfirm) && completedStages[i].BlockNum == blockStage1.BlockNum+1 &&
						completedStages[i].Timestamp > blockStage1.Timestamp) {
					return errors.New(fmt.Sprintf("CheckCompletedStage failed case 3, %s", InvalidParentHashErr))
				}
			}
		} else {
			if !parentHash.Equal(completedStages[i].ParentStage) {
				return errors.New(fmt.Sprintf("CheckCompletedStage failed case 4, %s", InvalidParentHashErr))
			}
		}
		parentHash = completedStages[i].Hash()

		if len(completedStages[i].ValidSigns) <= cnt2f {
			return errors.New(fmt.Sprintf("CheckCompletedStage failed, %s", InvalidSignsCntErr))
		}

		_, weHaveTheBlock := state.GetBlock(completedStages[i].BlockHash)
		if !weHaveTheBlock {
			return errors.New(fmt.Sprintf("CheckCompletedStage failed, %s, block: %s", NoSpecificBlockErr,
				completedStages[i].BlockHash.TerminalString()))
		}

	}

	return nil
}

func (state *State) StartConsensus(request *hbft.RequestMsg, currentBlock *types.Block) (*hbft.PreConfirmMsg, error) {
	curState := &hbft.StateMsg{
		ViewID:       request.ViewID,
		SequenceID:   request.SequenceID,
		CurrentStage: state.CurrentStage,
		Completed:    make([]*types.HBFTStageCompleted, 0),
	}

	parentOfNewBlock := request.NewBlock.ParentHash()
	futureParent := types.NewBlockWithHeader(types.EmptyHeader)
	if !parentOfNewBlock.Equal(currentBlock.Hash()) {
		currentBlockStages := currentBlock.HBFTStageChain()
		currBlockStage0Hash := types.EmptyHbftStage.Hash()
		currBlockStage1Hash := types.EmptyHbftStage.Hash()
		currBlockStage1Num := uint64(0)
		if currentBlockStages.Len() > 0 {
			currBlockStage0Hash = currentBlockStages[0].Hash()
			currBlockStage1Hash = currentBlockStages[currentBlockStages.Len()-1].Hash()
			currBlockStage1Num = currentBlockStages[currentBlockStages.Len()-1].BlockNum
		}
		if common.EmptyHash(currBlockStage1Hash) {
			state.logger.Error("the completed stage in current block is invalid, is it the genesis block?")
		}

		var completedCandidate *types.HBFTStageCompleted
		state.completedMu.RLock()
		state.completed.Range(func(k, v interface{}) bool {
			completed := v.(*types.HBFTStageCompleted)
			if (currBlockStage1Hash.Equal(completed.ParentStage) || currBlockStage1Hash.Equal(completed.Hash())) &&
				completed.BlockHash.Equal(parentOfNewBlock) {
				curState.Completed = append(curState.Completed, completed)
				return false
			} else if currBlockStage1Num == currentBlock.NumberU64()+1 && currBlockStage1Num == completed.BlockNum &&
				completed.BlockHash.Equal(parentOfNewBlock) {
				completedCandidate = completed
				return false
			} else if currBlockStage1Num == currentBlock.NumberU64() && currBlockStage1Num+1 == completed.BlockNum &&
				completed.BlockHash.Equal(parentOfNewBlock) {
				completedCandidate = completed
				return false
			}
			return true
		})
		state.completedMu.RUnlock()
		if len(curState.Completed) != 1 {
			if completedCandidate != nil && completedCandidate.ParentStage.Equal(currBlockStage0Hash) {
				curState.Completed = append(curState.Completed, completedCandidate)
				state.logger.Warn(fmt.Sprintf("StartConsensus with a newly replaced completed stage: %s, blockHash: %s, parent state: %s",
					completedCandidate.Hash().TerminalString(), completedCandidate.BlockHash.TerminalString(),
					completedCandidate.ParentStage.TerminalString()))
			}
			if len(curState.Completed) != 1 {
				return nil, errors.New(fmt.Sprintf(
					"can not build a correct chain for the completed stages in current state with new future block, "+
						"the hash of the cuurent block's completed stage 1 is %s, length of completed in new state is %d",
					currBlockStage1Hash.TerminalString(), len(curState.Completed)))
			}
		}

		exist := false
		if futureParent, exist = state.GetSignedBlock(parentOfNewBlock); !exist {
			return nil, errors.New(fmt.Sprintf("the parent future block in the completed stage is missing: %s",
				request.NewBlock.ParentHash().TerminalString()))
		}

		currBlockHash := currentBlock.Hash()
		if !currBlockHash.Equal(futureParent.ParentHash()) {
			return nil, errors.New(fmt.Sprintf(
				"the parent hash of the parent future block(%s) is not equal with the current block: %s, the parent of future: %s",
				futureParent.ParentHash().TerminalString(), currBlockHash.TerminalString(), parentOfNewBlock.TerminalString()))
		}
	}

	return &hbft.PreConfirmMsg{
		ViewPrimary:   request.ViewPrimary,
		ViewID:        request.ViewID,
		SequenceID:    request.SequenceID,
		NewBlock:      request.NewBlock,
		FutureParent:  futureParent,
		CurState:      curState,
		ReqTimeStamp:  request.Timestamp,
		OldestFutures: make([]*hbft.OldestFutureMsg, 0),
	}, nil
}

func (state *State) BuildCompletedStageChain() (stages []*types.HBFTStageCompleted) {
	state.completedMu.RLock()
	defer state.completedMu.RUnlock()

	state.completed.Range(func(k, v interface{}) bool {
		state.HBFTDebugInfo(fmt.Sprintf("BuildCompletedStageChain Completed state: %s, blockHash: %s",
			k.(common.Hash).TerminalString(), v.(*types.HBFTStageCompleted).BlockHash.TerminalString()))
		state.HBFTDebugInfo(fmt.Sprintf("BuildCompletedStageChain parent state: %s",
			v.(*types.HBFTStageCompleted).ParentStage.TerminalString()))
		return true
	})

	stages = []*types.HBFTStageCompleted{}
	state.completed.Range(func(k, v interface{}) bool {
		completed := v.(*types.HBFTStageCompleted)
		if chain, ok := obtainCompletedStageChain(completed, state.completed, 0); ok {
			chain = append(chain, completed)
			stages = append(stages, chain[:]...)
			return false
		}
		return true
	})

	state.HBFTDebugInfo(fmt.Sprintf("BuildCompletedStageChain length: %d", len(stages)))
	return stages
}

func obtainCompletedStageChain(completedStage *types.HBFTStageCompleted, completedMap *sync.Map,
	chainLen int) ([]*types.HBFTStageCompleted, bool) {
	if completed, ok := completedMap.Load(completedStage.ParentStage); ok {
		chainLen += 1
		if chain, ok := obtainCompletedStageChain(completed.(*types.HBFTStageCompleted), completedMap, chainLen); ok {
			chain = append(chain, completed.(*types.HBFTStageCompleted))
			return chain, true
		}
	} else {
		if chainLen == GetCompletedLength(completedMap)-1 {
			return make([]*types.HBFTStageCompleted, 0), true
		}
	}

	return []*types.HBFTStageCompleted{}, false
}

func (state *State) AppendBlock(b *types.Block) {
	state.mu.Lock()
	defer state.mu.Unlock()

	state.Blocks = append(state.Blocks, b)
}

func (state *State) GetBlock(hash common.Hash) (*types.Block, bool) {
	state.mu.Lock()
	defer state.mu.Unlock()

	for _, block := range state.Blocks {
		if hash.Equal(block.Hash()) {
			return block, true
		}
	}
	return nil, false
}

func (state *State) AppendSignedBlock(b *types.Block) {
	state.mu.Lock()
	defer state.mu.Unlock()

	state.SignedBlocks = append(state.SignedBlocks, b)
}

func (state *State) GetSignedBlock(hash common.Hash) (*types.Block, bool) {
	state.mu.Lock()
	defer state.mu.Unlock()

	for _, block := range state.SignedBlocks {
		if hash.Equal(block.Hash()) {
			return block, true
		}
	}
	return nil, false
}

func (state State) SetStageCompleted(stage *types.HBFTStageCompleted) {
	//state.completedMu.Lock()
	//defer state.completedMu.Unlock()

	state.completed.Store(stage.Hash(), stage)
}

func (state *State) ClearState(newHeadNum uint64, clearCompleted bool) {
	state.mu.Lock()
	defer state.mu.Unlock()

	state.CurrentStage = hbft.Idle
	state.MsgHash = common.Hash{}
	state.clearOutdatedSignedBlocks(newHeadNum)
	if clearCompleted {
		state.clearOutdatedCompleted(newHeadNum)
	}
}

func (state *State) Reset(newHeadNum uint64, clearCompleted bool) {
	state.mu.Lock()
	defer state.mu.Unlock()

	state.CurrentStage = hbft.Idle
	state.MsgHash = common.Hash{}
	state.Blocks = state.Blocks[len(state.Blocks)-1:]
	state.clearOutdatedSignedBlocks(newHeadNum)
	if clearCompleted {
		state.clearOutdatedCompleted(newHeadNum)
	}
}

func (state *State) ClearConnected(newHead *types.Header) {
	state.mu.Lock()
	defer state.mu.Unlock()

	state.clearOutdatedCompleted(newHead.Number.Uint64())

	if len(state.Blocks) > 0 {
		delHeight := uint64(0)
		for _, block := range state.Blocks {
			if block.NumberU64() < newHead.Number.Uint64() {
				delHeight += 1
			}
		}
		state.Blocks = state.Blocks[delHeight:]
	}
}

func (state *State) clearOutdatedCompleted(newHeadNum uint64) {
	state.completedMu.Lock()
	state.completed.Range(func(k, v interface{}) bool {
		c := v.(*types.HBFTStageCompleted)
		if c.BlockNum <= newHeadNum {
			state.completed.Delete(k.(common.Hash))
		}
		return true
	})
	state.completedMu.Unlock()

	state.HBFTDebugInfo(fmt.Sprintf("After clearOutdatedCompleted, the newest completed stage is: %s, the block hash in it is: %s",
		state.GetNewestStateCompletedStage().Hash().TerminalString(), state.GetNewestStateCompletedStage().BlockHash.TerminalString()))
}

func (state *State) clearOutdatedSignedBlocks(newHeadNum uint64) {
	signedBlocks := make([]*types.Block, 0)
	signedBlocks = append(signedBlocks, state.SignedBlocks...)
	state.SignedBlocks = make([]*types.Block, 0)
	for _, futureBlock := range signedBlocks {
		if futureBlock.NumberU64() > newHeadNum {
			state.SignedBlocks = append(state.SignedBlocks, futureBlock)
		}
	}
}

func (state *State) HBFTDebugInfo(msg string) {
	if hbft.HBFTDebugInfoOn {
		state.logger.Info(fmt.Sprintf("===HBFT Debug===%s", msg))
	}
}
