package nodeHbft

import (
	"errors"
	"fmt"
	"time"

	"github.com/contatract/go-contatract/accounts"
	"github.com/contatract/go-contatract/bft/consensus/hbft"
	"github.com/contatract/go-contatract/common"
	types "github.com/contatract/go-contatract/core/types_elephant"
	core "github.com/contatract/go-contatract/core_elephant"
	"github.com/contatract/go-contatract/crypto"
	"github.com/contatract/go-contatract/event"
)

const (
	BlockSealingBeatConst = time.Millisecond * 50

	BlockMinPeriodHbftConst     = time.Second * 2
	BlockMaxPeriodHbftConst     = time.Second * 16
	BlockMaxStartDelayHbftConst = (BlockMaxPeriodHbftConst - BlockMinPeriodHbftConst) / 2

	NodeServerMinCountHbft = 4

	expPeriodBlockSealingCnt = 10

	failureLimit = 20 // The maximum times of failure due to own network to start future block sync
)

var (
	BlockSealingBeat = BlockSealingBeatConst

	BlockMinPeriodHbft     = BlockMinPeriodHbftConst
	BlockMaxPeriodHbft     = BlockMaxPeriodHbftConst
	BlockMaxStartDelayHbft = BlockMaxStartDelayHbftConst
	//BlockAvgPeriodHbft     = BlockAvgPeriodHbftConst
)

type Elephant interface {
	Etherbase() (eb common.Address, err error)
	AccountManager() *accounts.Manager
	SendBFTMsg(msg hbft.MsgHbftConsensus, msgType uint)
	GetCurRealSealers() ([]common.Address, bool)
	GetRealSealersByNum(number uint64) ([]common.Address, bool)
	Get2fRealSealersCnt() int
	GetBlockByNumber(number uint64) *types.Block
	GetCurrentBlock() *types.Block
	AddBFTFutureBlock(b *types.Block)
	BFTFutureBlocks() []*types.Block
	BFTOldestFutureBlock() *types.Block
	ClearOutDatedBFTFutureBlock(height uint64)
	ClearAllBFTFutureBlock()
	ClearWorkerFutureParent()
	NextIsNewSealersFirstBlock(block *types.Block) bool
	ValidateNewBftBlocks(blocks []*types.Block) error
	SendHBFTCurrentBlock(currentBlock *types.Block)
	SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription
}

func CalculateLastTime(parentBlock *types.Block, expTimePeriod int64) int64 {
	lastTime := int64(parentBlock.HBFTStageChain()[0].ReqTimeStamp) + expTimePeriod
	return lastTime
}

func (n *Node) InitNodeTable() {
	lastTime := int64(0)
	if n.EleBackend.GetCurrentBlock().NumberU64() != 0 {
		lastTime = CalculateLastTime(n.EleBackend.GetCurrentBlock(), n.expBlockPeriod)
	}

	lastSealer := n.EleBackend.GetCurrentBlock().Coinbase()
	sealers, _ := n.EleBackend.GetCurRealSealers()
	n.UpdateNodeTable(lastSealer, lastTime, sealers)
}

func (n *Node) UpdateNodeTable(lastSealer common.Address, lastTime int64, sealers []common.Address) {
	n.NodeTable = make(map[common.Address]int64)
	num := 0
	for i, sealer := range sealers {
		if sealer.Equal(lastSealer) {
			num = i + 1
		}
	}
	for cnt := 0; cnt < len(sealers); cnt++ {
		if num == len(sealers) {
			num = 0
		}
		if cnt == 0 {
			n.SetNodeTable(sealers[num], lastTime+int64(BlockMinPeriodHbft.Nanoseconds()/1e6))
		} else {
			n.SetNodeTable(sealers[num], lastTime+int64(BlockMinPeriodHbft.Nanoseconds()/1e6)+
				int64(cnt)*int64((BlockMinPeriodHbft+BlockMaxPeriodHbft).Nanoseconds()/1e6))
		}
		num++
	}
}

func (n *Node) SetNodeTable(address common.Address, timeStart int64) {
	n.nodeTableMu.Lock()
	defer n.nodeTableMu.Unlock()

	n.NodeTable[address] = timeStart
}

func (n *Node) SetPassphrase(passphrase string) {
	n.passphrase = passphrase
}

func (n *Node) IsBusy() bool {
	return n.isBusy
}

func (n *Node) SetBusy(flag bool) {
	n.isBusy = flag
}

func (n *Node) SendHBFTMsg(msg hbft.MsgHbftConsensus, msgType hbft.TypeHBFTMsg) {
	go n.EleBackend.SendBFTMsg(msg, uint(msgType))
}

func (n *Node) ValidateNewBlocks(HbftMsg hbft.MsgHbftConsensus, addFutureParent bool) error {
	blocks := make([]*types.Block, 0)
	completedStages := HbftMsg.GetCompleted()
	for i := 0; i < len(completedStages); i++ {
		if signedBlock, ok := n.CurrentState.GetSignedBlock(completedStages[i].BlockHash); ok {
			blocks = append(blocks, signedBlock)
		}
	}
	if len(blocks) == 0 && addFutureParent {
		blocks = append(blocks, HbftMsg.(*hbft.PreConfirmMsg).FutureParent)
		n.Info("ValidateNewBlocks add a missing future parent block", "hash", HbftMsg.(*hbft.PreConfirmMsg).FutureParent.Hash())
	}
	if _, ok := HbftMsg.(*hbft.PreConfirmMsg); ok {
		blocks = append(blocks, HbftMsg.GetNewBlock())
	} else if _, ok := HbftMsg.(*hbft.OldestFutureMsg); ok && len(blocks) == 0 {
		blocks = append(blocks, HbftMsg.GetNewBlock())
	}

	return n.EleBackend.ValidateNewBftBlocks(blocks)
}

func (n *Node) GetCurrentSealer(sealers []common.Address, selfWork bool) common.Address {
	if len(n.NodeTable) < NodeServerMinCountHbft {
		n.InitNodeTable()
	}

	now := hbft.GetTimeNowMs(n.CoinBase)
	minTimeDiff := int64(0)
	sealerAddr := common.Address{}
	first := true

	n.nodeTableMu.Lock()
	defer n.nodeTableMu.Unlock()
	for sealer, startTime := range n.NodeTable {
		if first && now-startTime >= 0 {
			minTimeDiff = getTimeDiff(now, startTime, len(sealers))
			sealerAddr = sealer
			first = false
		} else {
			if now-startTime >= 0 && getTimeDiff(now, startTime, len(sealers)) < minTimeDiff {
				minTimeDiff = getTimeDiff(now, startTime, len(sealers))
				sealerAddr = sealer
			}
		}
	}

	// If the node self is current sealer, the time of starting sealing should before the time which pass BlockMaxStartDelayHbft,
	// and if node self is not current sealer, the time of sealing should between starting time and the time which pass BlockMaxPeriodHbft
	if selfWork && minTimeDiff > int64(BlockMaxStartDelayHbft.Nanoseconds()/1e6) {
		return common.Address{}
	} else if !selfWork && minTimeDiff > int64(BlockMaxPeriodHbft.Nanoseconds()/1e6) {
		return common.Address{}
	}

	return sealerAddr
}

// The unit of now and startTime is ms
func getTimeDiff(now, startTime int64, cntSealers int) int64 {
	return (now - startTime) % (int64(cntSealers) * int64((BlockMinPeriodHbft+BlockMaxPeriodHbft).Nanoseconds()/1e6))
}

func (n *Node) resolveMsg(msg interface{}) {
	//n.Log.Info("==================================resolveMsg")
	switch msg.(type) {
	case *hbft.RequestMsg:
		n.MsgMutex.RLock()
		defer n.MsgMutex.RUnlock()
		err := n.resolveRequestMsg(msg.(*hbft.RequestMsg))
		if err != nil {
			n.Warn("resolveRequestMsg failed", "err", err, "seq", msg.(*hbft.RequestMsg).SequenceID,
				"view", msg.(*hbft.RequestMsg).ViewID, "prim", msg.(*hbft.RequestMsg).ViewPrimary)
		}

	case *hbft.PreConfirmMsg:
		n.MsgMutex.Lock()
		defer n.MsgMutex.Unlock()
		err := n.resolvePreConfirmMsg(msg.(*hbft.PreConfirmMsg))
		if err != nil {
			n.Warn("resolvePreConfirmMsg failed", "err", err, "seq", msg.(*hbft.PreConfirmMsg).SequenceID,
				"view", msg.(*hbft.PreConfirmMsg).ViewID, "prim", msg.(*hbft.PreConfirmMsg).ViewPrimary)
		}

	case *hbft.ConfirmMsg:
		n.MsgMutex.Lock()
		defer n.MsgMutex.Unlock()
		err := n.resolveConfirmMsg(msg.(*hbft.ConfirmMsg))
		if err != nil {
			n.Warn("resolveConfirmMsg failed", "err", err, "seq", msg.(*hbft.ConfirmMsg).SequenceID,
				"view", msg.(*hbft.ConfirmMsg).ViewID, "prim", msg.(*hbft.ConfirmMsg).ViewPrimary)
		}

	case *hbft.OldestFutureMsg:
		n.MsgMutex.RLock()
		defer n.MsgMutex.RUnlock()
		err := n.resolveOldestFutureMsg(msg.(*hbft.OldestFutureMsg))
		if err != nil {
			n.Warn("resolveOldestFutureMsg failed", "err", err, "seq", msg.(*hbft.OldestFutureMsg).SequenceID,
				"view", msg.(*hbft.OldestFutureMsg).ViewID, "prim", msg.(*hbft.OldestFutureMsg).ViewPrimary)
		}

	default:
		n.Error("resolve failed, unknown msg type")
	}
}

func (n *Node) BlockSealedCompleted(blockHash common.Hash) bool {
	sealers, _ := n.EleBackend.GetCurRealSealers()
	count := len(sealers)
	if count < NodeServerMinCountHbft {
		n.Error("insufficient node server, this should not occur", "count", count)
		return false
	}

	times := uint32((BlockMaxPeriodHbft).Nanoseconds() / BlockSealingBeat.Nanoseconds())
	ticker := time.NewTicker(BlockSealingBeat)
	defer ticker.Stop()
	var current uint32 = 0

loop:
	for {
		select {
		case <-ticker.C:
			if n.sealed(blockHash) {
				return true
			}

			if current >= times {
				break loop
			}
			sealerAddr := n.GetCurrentSealer(sealers, false)
			//if common.EmptyAddress(sealerAddr) {
			//	current = times - 1
			//}
			if !sealerAddr.Equal(n.CoinBase) {
				break loop
			}

			current++
		}
	}

	n.Sealed.Store(blockHash, false)

	//n.Log.Info("==================================BlockSealedCompleted")

	n.ReplyBufMutex.Lock()
	*n.ReplyBuffer = make([]*hbft.ReplyMsg, 0)
	n.ReplyBufMutex.Unlock()

	n.Warn(fmt.Sprintf("Block sealing timed out, seq: %d, view: %d, prim: %s",
		n.CurrentState.SequenceID, n.CurrentState.ViewID, n.CurrentState.ViewPrimary))
	return false
}

func (n *Node) sealed(blockHash common.Hash) bool {
	ret, ok := n.Sealed.Load(blockHash)
	if !ok || (ok && !ret.(bool)) {
		return false
	} else {
		return true
	}
}

func (n *Node) HbftProcess(msg interface{}) {
	// First handle reply message
	if replyMsg, ok := msg.(*hbft.ReplyMsg); ok {
		sigAddr, err := n.HbftEcRecover(replyMsg.MsgHash.Bytes(), replyMsg.Signature)
		if err != nil {
			n.Error("HbftProcess failed", "err", err.Error())
			return
		}

		sealers, _ := n.EleBackend.GetCurRealSealers()
		for _, addr := range sealers {
			if sigAddr.Equal(addr) {
				curSealer := n.GetCurrentSealer(sealers, false)
				if curSealer.Equal(common.HexToAddress(replyMsg.GetViewPrimary())) &&
					n.GetCurrentViewID(n.EleBackend.GetCurrentBlock()) == replyMsg.GetViewID() {
					n.AddReplyMsg(replyMsg)
					break
				} else {
					n.HBFTDebugInfo(fmt.Sprintf("Discard expired HBFT reply message, sigAddr: %s, prim: %s",
						sigAddr.String(), replyMsg.ViewPrimary))
				}
			}
		}
		return
	}

	n.resolveMsg(msg)
}

func (n *Node) AddReplyMsg(replyMsg *hbft.ReplyMsg) {
	//n.Log.Info("==================================AddReplyMsg")
	if n.CurrentState.CurrentStage == hbft.Idle {
		//Warn(hbft.ShouldNotHandleErr)
		return
	}

	if replyMsg.MsgType == hbft.MsgPreConfirm && replyMsg.MsgHash.Equal(n.CurrentState.MsgHash) {
		n.ReplyBufMutex.Lock()
		*n.ReplyBuffer = append(*n.ReplyBuffer, replyMsg)
		ReplyBufferLen := len(*n.ReplyBuffer)
		n.ReplyBufMutex.Unlock()

		var (
			confirmMsg *hbft.ConfirmMsg
			err        error
		)
		if ReplyBufferLen == n.EleBackend.Get2fRealSealersCnt()+1 {
			n.ReplyBufMutex.Lock()
			n.HBFTDebugInfo("2f+1 MsgPreConfirm")
			blockInReplyHash, blockInReplyNum, errBlock := n.getReplyBlockInfo(replyMsg)
			if errBlock != nil {
				n.Warn(errBlock.Error())
				n.ReplyBufMutex.Unlock()
				return
			}
			if _, ok := n.Sealed.Load(blockInReplyHash); ok {
				n.HBFTDebugInfo("timed out 2f+1 MsgPreConfirm")
				*n.ReplyBuffer = make([]*hbft.ReplyMsg, 0)
				n.ReplyBufMutex.Unlock()
				return
			}
			confirmMsg, err = n.GetConfirmMsg(replyMsg, blockInReplyHash, blockInReplyNum)
			if err != nil {
				n.Warn(err.Error())
				n.ReplyBufMutex.Unlock()
				return
			}

			// Change the stage to confirm.
			n.CurrentState.CurrentStage = hbft.Confirm
			n.CurrentState.MsgHash = confirmMsg.Hash()
			*n.ReplyBuffer = make([]*hbft.ReplyMsg, 0)
			//n.EleBackend.AddBFTFutureBlock(n.CurrentState.SignedBlocks[len(n.CurrentState.SignedBlocks)-1])
			//n.HBFTDebugInfo(fmt.Sprintf("AddBFTFutureBlock block: %s, number: %d, parent: %s",
			//	n.CurrentState.SignedBlocks[len(n.CurrentState.SignedBlocks)-1].Hash().TerminalString(),
			//	n.CurrentState.SignedBlocks[len(n.CurrentState.SignedBlocks)-1].NumberU64(),
			//	n.CurrentState.SignedBlocks[len(n.CurrentState.SignedBlocks)-1].ParentHash().TerminalString()))

			n.ReplyBufMutex.Unlock()

			n.LogStage(fmt.Sprintf("Pre-confirm, seq: %d, view: %d, prim: %s",
				confirmMsg.SequenceID, confirmMsg.ViewID, confirmMsg.ViewPrimary), true)
			n.LogStage(fmt.Sprintf("Confirm, seq: %d, view: %d, prim: %s",
				confirmMsg.SequenceID, confirmMsg.ViewID, confirmMsg.ViewPrimary), false)

			n.SendHBFTMsg(confirmMsg, hbft.MsgConfirm)
		}

		if confirmMsg != nil {
			replyMsg, err := n.GetReplyMsg(confirmMsg)
			if err != nil {
				n.Warn(err.Error())
			} else {
				n.AddReplyMsg(replyMsg)
			}
		}

	} else if replyMsg.MsgType == hbft.MsgConfirm && replyMsg.MsgHash.Equal(n.CurrentState.MsgHash) {
		n.ReplyBufMutex.Lock()
		*n.ReplyBuffer = append(*n.ReplyBuffer, replyMsg)
		n.confirmReplyCnt += 1
		n.ReplyBufMutex.Unlock()

		if n.confirmReplyCnt == n.EleBackend.Get2fRealSealersCnt()+1 {
			n.HBFTDebugInfo("2f+1 MsgConfirm")
			if len(n.CurrentState.SignedBlocks) <= 0 {
				n.HBFTDebugInfo("invalid 2f+1 MsgConfirm, the signed block has been cleared")
				n.confirmReplyCnt = 0
				return
			}
			newestBlock := n.CurrentState.SignedBlocks[len(n.CurrentState.SignedBlocks)-1]
			if !n.sealed(newestBlock.Hash()) {
				stageCompleted := &types.HBFTStageCompleted{
					ViewID:       n.CurrentState.ViewID, // TODO: need to be verified
					SequenceID:   n.CurrentState.SequenceID,
					BlockHash:    newestBlock.Hash(),
					BlockNum:     newestBlock.NumberU64(),
					MsgType:      uint(hbft.MsgConfirm),
					MsgHash:      n.CurrentState.MsgHash,
					ValidSigns:   make([][]byte, 0),
					ParentStage:  n.CurrentState.GetNewestStateCompletedStage().Hash(),
					Timestamp:    hbft.GetTimeStampForSet(n.CurrentState.GetNewestStateCompletedStage().Timestamp, n.CoinBase),
					ReqTimeStamp: replyMsg.ReqTimeStamp,
				}
				n.ReplyBufMutex.RLock()
				for _, reply := range *n.ReplyBuffer {
					if stageCompleted.MsgHash.Equal(reply.MsgHash) {
						stageCompleted.ValidSigns = append(stageCompleted.ValidSigns, reply.Signature)
					}
				}
				n.ReplyBufMutex.RUnlock()
				if len(stageCompleted.ValidSigns) <= n.EleBackend.Get2fRealSealersCnt() {
					n.confirmReplyCnt -= 1
					n.HBFTDebugInfo("invalid 2f+1 MsgConfirm")
					return
				}

				n.CurrentState.Reset(newestBlock.NumberU64()-1, false)
				n.CurrentState.SetStageCompleted(stageCompleted)

				n.Sealed.Store(newestBlock.Hash(), true)

				n.HBFTDebugInfo(fmt.Sprintf("SetStageCompleted stage: %s, blockHash: %s",
					stageCompleted.Hash().TerminalString(), stageCompleted.BlockHash.TerminalString()))
				n.HBFTDebugInfo(fmt.Sprintf("SetStageCompleted parent: %s", stageCompleted.ParentStage.TerminalString()))

				n.ReplyBufMutex.Lock()
				*n.ReplyBuffer = make([]*hbft.ReplyMsg, 0)
				n.ReplyBufMutex.Unlock()

				n.LogStage(fmt.Sprintf("Confirm, seq: %d, view: %d, prim: %s",
					replyMsg.SequenceID, replyMsg.ViewID, replyMsg.ViewPrimary), true)
			}
		}
	}
}

func (n *Node) getReplyBlockInfo(replyMsg *hbft.ReplyMsg) (common.Hash, uint64, error) {
	var (
		blockInReplyHash common.Hash
		blockInReplyNum  uint64
	)
	if msg, ok := n.CurrentState.MsgLogs.PreConfirmMsgs.Load(replyMsg.MsgHash); ok {
		blockInReplyHash = msg.(*hbft.PreConfirmMsg).NewBlock.Hash()
		blockInReplyNum = msg.(*hbft.PreConfirmMsg).NewBlock.NumberU64()
		if _, exist := n.CurrentState.GetSignedBlock(blockInReplyHash); !exist {
			return common.Hash{}, 0, errors.New("getReplyBlockInfo failed, there is no specific block in current state")
		}
	} else {
		return common.Hash{}, 0, errors.New("getReplyBlockInfo failed, there is no specific pre-confirm message in current state")
	}

	return blockInReplyHash, blockInReplyNum, nil
}

func (n *Node) GetSealedBlockStages() ([]*types.HBFTStageCompleted, bool) {
	stages := make([]*types.HBFTStageCompleted, 0)
	newest := n.CurrentState.GetNewestStateCompletedStage()

	if stagePreConfirm, ok := n.CurrentState.GetCompletedStage(newest.ParentStage); ok {
		stages = append(stages, stagePreConfirm)
		stages = append(stages, newest)
		return stages, true
	}

	return nil, false
}

func (n *Node) GetCurrentStateStage() hbft.Stage {
	return n.CurrentState.CurrentStage
}

func (n *Node) HbftSign(address common.Address, passwd string, data []byte) ([]byte, error) {
	// Look up the wallet containing the requested signer
	account := accounts.Account{Address: address}
	wallet, err := n.AccMan.Find(account)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("HbftSign failed, %s", err.Error()))
	}
	// Assemble sign the data with the wallet
	signature, err := wallet.SignHashWithPassphrase(account, passwd, data)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("HbftSign failed, %s", err.Error()))
	}

	return signature, nil
}

func (n *Node) HbftEcRecover(data, sig []byte) (common.Address, error) {
	if len(sig) != 65 {
		return common.Address{}, fmt.Errorf("signature must be 65 bytes long")
	}
	signature := make([]byte, len(sig))
	copy(signature, sig)

	rpk, err := crypto.Ecrecover(data, signature)
	if err != nil {
		return common.Address{}, err
	}
	pubKey := crypto.ToECDSAPub(rpk)
	recoveredAddr := crypto.PubkeyToAddress(*pubKey)
	return recoveredAddr, nil
}

func (n *Node) GetFakeStagesForTest(viewID uint64) []*types.HBFTStageCompleted {
	fake := new(types.HBFTStageCompleted)
	return fake.Fake(viewID, 0)
}
