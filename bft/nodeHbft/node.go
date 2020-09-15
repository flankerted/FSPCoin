package nodeHbft

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/contatract/go-contatract/accounts"
	"github.com/contatract/go-contatract/bft/consensus/hbft"
	"github.com/contatract/go-contatract/blizparam"
	"github.com/contatract/go-contatract/common"
	types "github.com/contatract/go-contatract/core/types_elephant"
	core "github.com/contatract/go-contatract/core_elephant"
	"github.com/contatract/go-contatract/eth"
	"github.com/contatract/go-contatract/event"
	"github.com/contatract/go-contatract/log"
)

type Node struct {
	NodeTable    map[common.Address]int64 // stores the validators and the time that the node should start to seal a new block
	CurrentState *State
	CoinBase     common.Address

	ReplyBuffer     *[]*hbft.ReplyMsg // stores the replies of pre-confirm or confirm messages
	confirmReplyCnt int
	expBlockPeriod  int64 // the expected time period of sealing blocks (last 10 blocks)

	chainHeadCh  chan core.ChainHeadEvent
	chainHeadSub event.Subscription
	isBusy       bool

	oldestFutureMutex        sync.RWMutex
	oldestFutureMsgs         []*hbft.OldestFutureMsg
	oldestFutureFull         bool
	startFutureBlockSyncFlag bool

	myOldestFutureMsg   *hbft.OldestFutureMsg
	myOldestFutureMutex sync.RWMutex
	newCompletedChan    chan *types.HBFTStageCompleted

	MsgMutex      sync.RWMutex
	ReplyBufMutex sync.RWMutex
	nodeTableMu   sync.Mutex

	Sealed *sync.Map

	Lamp       *eth.Ethereum
	EleBackend Elephant
	AccMan     *accounts.Manager
	passphrase string

	Log log.Logger
}

func NewNode(lamp *eth.Ethereum, elephant Elephant, logger log.Logger) (*Node, error) {
	node := &Node{
		NodeTable:         make(map[common.Address]int64),
		ReplyBuffer:       new([]*hbft.ReplyMsg),
		expBlockPeriod:    BlockMaxPeriodHbft.Nanoseconds() / 1e6,
		chainHeadCh:       make(chan core.ChainHeadEvent, core.ChainHeadChanSize),
		isBusy:            false,
		oldestFutureMsgs:  make([]*hbft.OldestFutureMsg, 0),
		oldestFutureFull:  false,
		myOldestFutureMsg: nil,
		newCompletedChan:  make(chan *types.HBFTStageCompleted),
		Sealed:            new(sync.Map),
		Lamp:              lamp,
		AccMan:            elephant.AccountManager(),
		passphrase:        "",
		EleBackend:        elephant,
		Log:               logger,
	}

	if node.Log == nil {
		node.Log = log.New()
		node.Log.SetHandler(log.LvlFilterHandler(log.LvlInfo, log.StreamHandler(os.Stdout, log.TerminalFormat(false))))
	}

	coinBase, err := elephant.Etherbase()
	if err != nil {
		node.Warn("Etherbase have not be explicitly specified, the node can not start sealing")
	}
	node.CoinBase = coinBase

	return node, nil
}

// SubscribeChainHeadEvent Subscribes events from blockchain and start the loop
func (n *Node) SubscribeChainHeadEvent() {
	n.chainHeadSub = n.EleBackend.SubscribeChainHeadEvent(n.chainHeadCh)

	go n.loop()
	go n.genMyOldestFutureMsgLoop()
}

// loop is the hbft node's main event loop, waiting for and reacting to
// outside blockchain events.
func (n *Node) loop() {
	// Track the previous head headers for transaction reorgs
	head := n.EleBackend.GetCurrentBlock()

	// Keep waiting for and reacting to the various events
	for {
		select {
		// Handle ChainHeadEvent
		case ev := <-n.chainHeadCh:
			if ev.Block != nil && ev.Block.NumberU64() > head.NumberU64() {
				if ev.Block.NumberU64() > expPeriodBlockSealingCnt {
					n.expBlockPeriod = 0
					for i := uint64(ev.Block.NumberU64() - 1); i >= ev.Block.NumberU64()-expPeriodBlockSealingCnt; i-- {
						block := n.EleBackend.GetBlockByNumber(i)
						n.expBlockPeriod += int64(2 * (block.HBFTStageChain()[0].Timestamp - block.HBFTStageChain()[0].ReqTimeStamp))
					}
					n.expBlockPeriod = n.expBlockPeriod / expPeriodBlockSealingCnt
					if n.expBlockPeriod > BlockMaxPeriodHbft.Nanoseconds()/1e6 {
						n.expBlockPeriod = BlockMaxPeriodHbft.Nanoseconds() / 1e6
					}
				}

				lastTime := CalculateLastTime(ev.Block, n.expBlockPeriod)
				n.HBFTDebugInfo(fmt.Sprintf("================: ev.Block number: %d, hash: %s, lastTime: %d, expBlockPeriod: %d",
					ev.Block.NumberU64(), ev.Block.Hash().TerminalString(), lastTime, n.expBlockPeriod))
				n.reset(head.Header(), ev.Block.Header(), ev.LampHeight)
				head = ev.Block
				sealers, _ := n.EleBackend.GetCurRealSealers()
				n.UpdateNodeTable(head.Coinbase(), lastTime, sealers)
			}
		// Be unsubscribed due to system stopped
		case <-n.chainHeadSub.Err():
			n.Warn("ChainHeadSub is unsubscribed due to system stopped")
			close(n.newCompletedChan)
			n.newCompletedChan = nil
			return
		}
	}
}

func (n *Node) reset(oldHead, newHead *types.Header, lampHeight uint64) {
	n.MsgMutex.Lock()
	defer n.MsgMutex.Unlock()

	if n.CurrentState == nil {
		return
	}

	newestBlock := oldHead
	newestStage := n.CurrentState.GetNewestStateCompletedStage()
	if newestStage != nil {
		newestBlockHash := newestStage.BlockHash
		for _, block := range n.CurrentState.Blocks {
			if newestBlockHash.Equal(block.Hash()) {
				newestBlock = block.Header()
			}
		}
	}

	n.oldestFutureFull = false
	n.oldestFutureMutex.Lock()
	n.oldestFutureMsgs = make([]*hbft.OldestFutureMsg, 0)
	n.oldestFutureMutex.Unlock()

	if newHead.Number.Uint64() >= newestBlock.Number.Uint64() {
		newestHash := newestBlock.Hash()
		if n.CurrentState == nil || n.CurrentState.CurrentStage == hbft.Idle {

		} else if n.CurrentState.CurrentStage == hbft.Confirm && newestHash.Equal(newHead.Hash()) {
			n.LogStage(fmt.Sprintf("Confirm, seq: %d, view: %d, prim: %s",
				n.CurrentState.SequenceID, n.CurrentState.ViewID, n.CurrentState.ViewPrimary), true)
		} else {
			n.LogStageReset()
		}

		n.CurrentState.Reset(newHead.Number.Uint64(), true)
	} else {
		n.CurrentState.ClearConnected(newHead)
	}
}

func (n *Node) genMyOldestFutureMsgLoop() {
	var (
		completedStage    *types.HBFTStageCompleted
		oldestFutureBlock *types.Block
		viewID            uint64
		err               error
	)

	oldestFutureBlock = nil
	newMyOldestFutureMsg := func() {
		completedStage = new(types.HBFTStageCompleted)
		completedStage.BlockHash = common.Hash{}
		completedStage.ReqTimeStamp = 0
		if n.CurrentState != nil {
			newCompleted := n.CurrentState.GetNewestStateCompletedStage()
			oldestFutureBlock = n.EleBackend.BFTOldestFutureBlock()
			if oldestFutureBlock != nil && newCompleted.BlockHash.Equal(oldestFutureBlock.Hash()) {
				completedStage = newCompleted
			}
		}

		n.myOldestFutureMutex.Lock()
		n.myOldestFutureMsg, err = n.getMyOldestFutureMsg(completedStage, oldestFutureBlock, true)
		if err != nil {
			n.Warn("getMyOldestFutureMsg failed", "err", err)
			n.myOldestFutureMsg = nil
		}
		n.myOldestFutureMutex.Unlock()
	}

	newMyOldestFutureMsg()

	ticker := time.NewTicker(BlockSealingBeat)
	defer ticker.Stop()
	for {
		select {
		case completed, ok := <-n.newCompletedChan:
			if !ok {
				n.Warn("newCompletedChan is closed due to system stopped")
				return
			}
			if n.EleBackend.BFTOldestFutureBlock() != nil && completed.BlockHash.Equal(n.EleBackend.BFTOldestFutureBlock().Hash()) {
				completedStage = completed
			}
			if n.EleBackend.GetCurrentBlock().NumberU64() == 0 {
				go newMyOldestFutureMsg()
			}

		case <-ticker.C:
			newView := n.GetCurrentViewID(n.EleBackend.GetCurrentBlock())
			if viewID != newView {
				viewID = newView
				go newMyOldestFutureMsg()
			}
		}
	}
}

func (n *Node) StartHbftWork(block *types.Block) bool {
	if sealerAddr, work := n.ItsTimeToDoHbftWork(block, nil); work {
		n.Log.Info("【*】Start the process of sealing a new block", "number", block.NumberU64(),
			"hash", block.Hash(), "parent", block.ParentHash(), "root", block.Root())
		n.GetRequest(sealerAddr.String(), block)
		return true
	} else {
		return false
	}
}

func (n *Node) ItsTimeToDoHbftWork(block, future *types.Block) (common.Address, bool) {
	if n.IsBusy() {
		return common.Address{}, false
	}

	sealers, _ := n.EleBackend.GetCurRealSealers()
	count := len(sealers)
	if count < NodeServerMinCountHbft {
		n.Log.Info("insufficient node server", "count", count)
		time.Sleep(BlockMinPeriodHbft)
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
				n.HBFTDebugInfo(fmt.Sprintf("Check unfinished future block: %s", oldestFutureHash.TerminalString()))
				if ret && oldestFutureHash.Equal(hbftStages[0].BlockHash) && oldestFutureHash.Equal(hbftStages[1].BlockHash) {
					n.Log.Info("There is an unfinished block which has two completed stages but in timed out confirm stage to handle first",
						"hash", oldestFutureHash)
					return sealerAddr, true
				}
			}
			return sealerAddr, false
		}
	}

	return sealerAddr, true
}

func (n *Node) GetRequest(sealerAddr string, block *types.Block) {
	var msg hbft.RequestMsg
	currentBlock := n.EleBackend.GetCurrentBlock()

	msg.ViewID, msg.SequenceID = n.GetCurrentViewID(currentBlock), n.GetCurrentSequenceID(currentBlock)
	msg.ViewPrimary = sealerAddr
	msg.Timestamp = hbft.GetTimeStampForSet(block.Time().Uint64(), n.CoinBase)
	msg.NewBlock = block

	n.SetBusy(true)
	n.startFutureBlockSyncFlag = false
	n.clearOutDatedOldestFutureMsgs(msg.ViewID, msg.SequenceID)
	if currentBlock.NumberU64() > 0 && msg.ViewID%10 == 0 {
		n.HBFTDebugInfo(fmt.Sprintf("SendHBFTCurrentBlock, hash: %s, number: %d",
			currentBlock.Hash().TerminalString(), currentBlock.NumberU64()))
		n.EleBackend.SendHBFTCurrentBlock(currentBlock)
	}
	if currentBlock.NumberU64() > 0 &&
		msg.ViewID-currentBlock.HBFTStageChain()[1].ViewID > uint64(failureLimit) ||
		(currentBlock.NumberU64() == 0 && n.CurrentState != nil && n.CurrentState.GetNewestStateCompletedStage().BlockNum > 0) {
		n.EleBackend.ClearOutDatedBFTFutureBlock(currentBlock.NumberU64())
		oldestFutureBlock := n.EleBackend.BFTOldestFutureBlock()
		if oldestFutureBlock != nil {
			n.Warn("It's been a long time for sealing a new block since the last block, start future block synchronisation",
				"view", msg.ViewID)
			n.startFutureBlockSync(msg, currentBlock, oldestFutureBlock)
			n.startFutureBlockSyncFlag = true
		}
	}

	n.resolveMsg(&msg)
}

func (n *Node) GetCurrentSequenceID(currentBlock *types.Block) uint64 {
	if currentBlock.NumberU64() == 0 {
		return 0
	}

	if currentBlock.HBFTStageChain().Len() != 2 {
		n.Log.Crit("GetCurrentSequenceID failed, the HBFT stage chain's length of current block is invalid",
			"number", currentBlock.NumberU64(), "hash", currentBlock.Hash())
	}
	if n.EleBackend.NextIsNewSealersFirstBlock(currentBlock) {
		return currentBlock.HBFTStageChain()[1].SequenceID + 1
	} else {
		return currentBlock.HBFTStageChain()[1].SequenceID
	}
}

func (n *Node) GetCurrentViewID(currentBlock *types.Block) uint64 {
	if currentBlock.NumberU64() == 0 {
		return 0
	}

	if currentBlock.HBFTStageChain().Len() != 2 {
		n.Log.Crit("GetCurrentViewID failed, the HBFT stage chain's length of current block is invalid",
			"number", currentBlock.NumberU64(), "hash", currentBlock.Hash())
	}
	lastView := currentBlock.HBFTStageChain()[1].ViewID
	lastTime := CalculateLastTime(currentBlock, n.expBlockPeriod)
	viewStart := lastTime + int64(BlockMinPeriodHbft.Nanoseconds()/1e6/2)
	viewDuration := int64((BlockMinPeriodHbft + BlockMaxPeriodHbft).Nanoseconds() / 1e6)
	now := hbft.GetTimeNowMs(n.CoinBase)
	if n.EleBackend.NextIsNewSealersFirstBlock(currentBlock) {
		return uint64((now - viewStart) / viewDuration)
	} else {
		return uint64((now-viewStart)/viewDuration) + 1 + lastView
	}
}

func (n *Node) startFutureBlockSync(msg hbft.RequestMsg, currentBlock, oldestFutureBlock *types.Block) {
	newOldestFutureMsg, err := n.getMyOldestFutureMsg(n.CurrentState.GetNewestStateCompletedStage(),
		oldestFutureBlock, false)
	if err != nil {
		n.Error("start future block synchronisation failed", "err", err)
		return
	}

	currentBlockHash := currentBlock.Hash()
	if !currentBlockHash.Equal(oldestFutureBlock.ParentHash()) {
		n.Warn("start future block synchronisation failed", "err", "the oldest future block is invalid",
			"current block", currentBlockHash, "oldest block", oldestFutureBlock.Hash().TerminalString(),
			"parent of oldest block", oldestFutureBlock.ParentHash())
		return
	}
	if !newOldestFutureMsg.Completed.BlockHash.Equal(oldestFutureBlock.Hash()) {
		n.Warn("start future block synchronisation failed", "err", "the oldest future block is invalid",
			"the block in completed stage", newOldestFutureMsg.Completed.BlockHash,
			"oldest block", oldestFutureBlock.Hash())
		return
	}

	n.SendHBFTMsg(newOldestFutureMsg, hbft.MsgOldestFuture)

	go func() {
		var err error
		newOldestFutureMsg.Signature, err = n.getMsgSignature(newOldestFutureMsg.Hash())
		if err != nil {
			n.Warn("getMyOldestFutureMsg failed", "err", err)
			return
		}

		if full := n.appendValidOldestFutureMsgs(newOldestFutureMsg); full {
			n.oldestFutureFull = true
		}
	}()
}

func (n *Node) waitForFutureBlockSync(viewID uint64) {
	sealers, _ := n.EleBackend.GetCurRealSealers()
	times := uint32((BlockMaxPeriodHbft - BlockMinPeriodHbft).Nanoseconds() / BlockSealingBeat.Nanoseconds() / 2)
	ticker := time.NewTicker(BlockSealingBeat)
	defer ticker.Stop()
	var current uint32 = 0

loop:
	for {
		select {
		case <-ticker.C:
			if n.oldestFutureFull {
				n.Info("Future block synchronisation completed")
				return
			}
			if current >= times {
				break loop
			}
			sealerAddr := n.GetCurrentSealer(sealers, false)
			if !sealerAddr.Equal(n.CoinBase) {
				break loop
			}
			current++
		}
	}

	n.Warn(fmt.Sprintf("Future block synchronisation timed out, view: %d", viewID))
}

// HandleReq can be called when our node starting mining a block.
// Consensus start procedure for the Primary.
func (n *Node) HandleReq(reqMsg *hbft.RequestMsg) (*hbft.PreConfirmMsg, error) {
	n.HBFTDebugInfo(fmt.Sprintf("HandleRequestMsg, new block: %s, number: %d, parent: %s",
		reqMsg.NewBlock.Hash().TerminalString(), reqMsg.NewBlock.NumberU64(),
		reqMsg.NewBlock.ParentHash().TerminalString()))
	// Create a new state for the new consensus if the current state is nil.
	n.createStateForNewConsensus(reqMsg)
	n.CurrentState.AppendBlock(reqMsg.NewBlock)

	if hbft.HBFTDebugInfoOn {
		n.CurrentState.ShowCompletedStagesDebug("StartConsensus")
	}

	// Start the consensus process.
	preConfirmMsg, err := n.CurrentState.StartConsensus(reqMsg, n.EleBackend.GetCurrentBlock())
	if err != nil {
		n.CurrentState.CurrentStage = hbft.Idle
		n.Log.Info("【*】[STAGE-RESET] Current state reset")
		return nil, err
	} else {
		var pend sync.WaitGroup
		pend.Add(1)
		go func() {
			defer pend.Done()
			if n.startFutureBlockSyncFlag {
				n.waitForFutureBlockSync(reqMsg.ViewID)
			}
		}()

		n.CurrentState.AppendSignedBlock(reqMsg.NewBlock)
		signature, err := n.getMsgSignature(preConfirmMsg.Hash())
		if err != nil {
			return nil, err
		}
		preConfirmMsg.Signature = signature

		pend.Wait()

		if !types.EmptyBlockHash.Equal(preConfirmMsg.FutureParent.Hash()) {
			n.checkIfOldestFuturesValidForPreConfirm(preConfirmMsg)
		}

		n.CurrentState.ViewPrimary = preConfirmMsg.ViewPrimary
		n.CurrentState.MsgLogs.PreConfirmMsgs.Store(preConfirmMsg.Hash(), preConfirmMsg)

		return preConfirmMsg, nil
	}
}

// checkIfOldestFuturesValidForPreConfirm checks our oldest-future messages are valid for pre-confirm message,
// the pre-confirm message has same view ID with all these oldest-future messages is indispensable
func (n *Node) checkIfOldestFuturesValidForPreConfirm(preConfirmMsg *hbft.PreConfirmMsg) {
	if n.oldestFutureFull {
		n.oldestFutureMutex.Lock()
		defer n.oldestFutureMutex.Unlock()

		canReplace, hash, _ := n.checkIfOldestFuturesCanReplace(n.oldestFutureMsgs, preConfirmMsg.SequenceID, preConfirmMsg.ViewID)
		if canReplace && hash.Equal(preConfirmMsg.FutureParent.Hash()) {
			preConfirmMsg.OldestFutures = append(preConfirmMsg.OldestFutures, n.oldestFutureMsgs...)
			n.HBFTDebugInfo("preConfirmMsg.OldestFutures reached consensus, send these preConfirmMsg.OldestFutures")
			return
		}

		if !hash.Equal(preConfirmMsg.FutureParent.Hash()) {
			n.HBFTDebugInfo("preConfirmMsg.OldestFutures reached consensus, " +
				"but we do not have the specific future parent block in the pre-confirm message")
		}
		n.oldestFutureFull = false
		n.oldestFutureMsgs = make([]*hbft.OldestFutureMsg, 0)
	}
}

// checkIfOldestFuturesCanReplace checks when there ara different oldest future blocks in different 2f+1 nodes,
// if there is a oldest future block can replace others
func (n *Node) checkIfOldestFuturesCanReplace(oldestFutureMsgs []*hbft.OldestFutureMsg, currentSeqID, currentViewID uint64) (bool, common.Hash, bool) {
	differentFutureCntMap := make(map[common.Hash]int)
	differentFutureMsgMap := make(map[common.Hash]*hbft.OldestFutureMsg)
	differentFuturePrimMap := make(map[string]int)

	// Check view and signature
	cnt := 0
	for _, oldestFutureMsg := range oldestFutureMsgs {
		if oldestFutureMsg.SequenceID == currentSeqID &&
			oldestFutureMsg.ViewID == currentViewID {
			if err := n.checkMsgSignature(oldestFutureMsg); err != nil {
				n.HBFTDebugInfo(fmt.Sprintf("checkIfOldestFuturesCanReplace failed, check signature failed, %s", err.Error()))
				return false, common.Hash{}, false
			}
			if _, ok := differentFuturePrimMap[oldestFutureMsg.ViewPrimary]; ok {
				n.HBFTDebugInfo("checkIfOldestFuturesCanReplace failed, duplicated prim")
				return false, common.Hash{}, false
			} else {
				differentFuturePrimMap[oldestFutureMsg.ViewPrimary] = 1
			}

			if oldestFutureMsg.OldestFuture != nil {
				differentFutureMsgMap[oldestFutureMsg.OldestFuture.Hash()] = oldestFutureMsg
			} else {
				differentFutureMsgMap[common.Hash{}] = oldestFutureMsg
			}
			cnt++
		}
	}
	if cnt <= n.EleBackend.Get2fRealSealersCnt() {
		n.HBFTDebugInfo("checkIfOldestFuturesCanReplace failed, check view failed")
		return false, common.Hash{}, false
	}

	for _, oldestFutureMsg := range oldestFutureMsgs {
		if !oldestFutureMsg.NoAnyFutures {
			if _, ok := differentFutureCntMap[oldestFutureMsg.OldestFuture.Hash()]; !ok {
				differentFutureCntMap[oldestFutureMsg.OldestFuture.Hash()] = 1
			} else {
				differentFutureCntMap[oldestFutureMsg.OldestFuture.Hash()] += 1
			}
		} else {
			continue
		}
	}

	if len(differentFutureCntMap) == 1 {
		for hash, cnt := range differentFutureCntMap {
			if cnt > n.EleBackend.Get2fRealSealersCnt()/2 {
				return true, hash, true
			} else {
				return true, hash, false
			}
		}
	}

	oldestFutureBlockToReturn := common.Hash{}
	maxValue := uint64(0)
	for hash, cnt := range differentFutureCntMap {
		if cnt > n.EleBackend.Get2fRealSealersCnt()/2 {
			return true, hash, true
		} else {
			if differentFutureMsgMap[hash].Completed.ReqTimeStamp > maxValue && !common.EmptyHash(hash) {
				maxValue = differentFutureMsgMap[hash].Completed.ReqTimeStamp
				oldestFutureBlockToReturn = hash
			}
			//if differentFutureMsgMap[hash].Completed.BlockNum == 1 {
			//	if differentFutureMsgMap[hash].Completed.ReqTimeStamp > maxValue {
			//		maxValue = differentFutureMsgMap[hash].Completed.ReqTimeStamp
			//		oldestFutureBlockToReturn = hash
			//	}
			//} else {
			//	if differentFutureMsgMap[hash].Completed.ReqTimeStamp > maxValue {
			//		maxValue = differentFutureMsgMap[hash].Completed.ReqTimeStamp
			//		oldestFutureBlockToReturn = hash
			//	}
			//}
		}
	}
	if !common.EmptyHash(oldestFutureBlockToReturn) {
		return true, oldestFutureBlockToReturn, true
	}

	n.HBFTDebugInfo("checkIfOldestFuturesCanReplace failed")
	return false, common.Hash{}, false
}

// HandlePreConfirmMsg can be called when the node receives a pre-confirm message.
func (n *Node) HandlePreConfirmMsg(preConfirmMsg *hbft.PreConfirmMsg) (*hbft.ReplyMsg, error) {
	n.HBFTDebugInfo(fmt.Sprintf("HandlePreConfirmMsg, new block: %s, number: %d, parent: %s, count of oldest future: %d",
		preConfirmMsg.NewBlock.Hash().TerminalString(), preConfirmMsg.NewBlock.NumberU64(),
		preConfirmMsg.NewBlock.ParentHash().TerminalString(), len(preConfirmMsg.OldestFutures)))
	// Check Signature first
	if err := n.checkMsgSignature(preConfirmMsg); err != nil {
		return nil, err
	}

	// Check request message's time stamp
	if preConfirmMsg.GetReqTimeStamp() <= preConfirmMsg.NewBlock.Time().Uint64() {
		return nil, errors.New(fmt.Sprintf("Check time stamp of request message failed"))
	}

	// Check the height of the new block
	if preConfirmMsg.NewBlock.NumberU64() <= n.EleBackend.GetCurrentBlock().NumberU64() {
		return nil, errors.New(fmt.Sprintf("Check height of new block failed"))
	}

	// Create a new state for the new consensus if the current state is nil.
	elephant := n.EleBackend
	n.createStateForNewConsensus(preConfirmMsg)
	n.CurrentState.AppendBlock(preConfirmMsg.NewBlock)
	addFutureParent := n.checkIfAddFutureParent(preConfirmMsg, elephant)

	// Check the preConfirm message if valid
	currentView := n.GetCurrentViewID(elephant.GetCurrentBlock())
	currentSeq := n.GetCurrentSequenceID(elephant.GetCurrentBlock())
	err := n.CurrentState.CheckView(preConfirmMsg, currentSeq, currentView) // Check view
	if err != nil {
		return nil, err
	}
	errState := n.CurrentState.CheckCurrentState(preConfirmMsg, elephant.GetCurrentBlock(),
		elephant.BFTOldestFutureBlock(), elephant.Get2fRealSealersCnt(), addFutureParent) // Check current state
	if errState != nil {
		return nil, errState
	}
	err = n.ValidateNewBlocks(preConfirmMsg, addFutureParent) // Check new blocks
	if err != nil {
		return nil, errors.New(fmt.Sprintf("ValidateNewBlocks failed, %s", err.Error()))
	}

	n.CurrentState.MsgLogs.PreConfirmMsgs.Store(preConfirmMsg.Hash(), preConfirmMsg)
	n.CurrentState.ViewPrimary = preConfirmMsg.ViewPrimary

	replyMsg, errReply := n.GetReplyMsg(preConfirmMsg)
	if errReply != nil {
		return nil, errReply
	} else {
		n.CurrentState.AppendSignedBlock(preConfirmMsg.NewBlock)
		if addFutureParent {
			n.clearFutureBlocksAndWork()
			n.CurrentState.ClearState(preConfirmMsg.FutureParent.NumberU64(), true)
			n.Log.Warn("The node might have network problem causing data de-synchronisation, " +
				"clear all BFT future blocks and then add a new one in HandlePreConfirmMsg")
			n.addNewFutureBlockAndSetCompleted(preConfirmMsg.FutureParent, preConfirmMsg.CurState.Completed)
		}
		return replyMsg, nil
	}
}

func (n *Node) clearFutureBlocksAndWork() {
	n.EleBackend.ClearAllBFTFutureBlock()
	n.EleBackend.ClearWorkerFutureParent()
}

func (n *Node) addNewFutureBlockAndSetCompleted(newFuture *types.Block, completed []*types.HBFTStageCompleted) {
	newFutureHash := newFuture.Hash()
	if !newFutureHash.Equal(n.CurrentState.SignedBlocks[len(n.CurrentState.SignedBlocks)-1].Hash()) {
		n.CurrentState.AppendSignedBlock(newFuture)
	}

	n.EleBackend.AddBFTFutureBlock(newFuture)
	n.HBFTDebugInfo(fmt.Sprintf("AddBFTFutureBlock block: %s, number: %d, parent: %s",
		newFuture.Hash().TerminalString(), newFuture.NumberU64(), newFuture.ParentHash().TerminalString()))

	for _, completed := range completed {
		n.CurrentState.SetStageCompleted(completed)

		n.HBFTDebugInfo(fmt.Sprintf("SetStageCompleted stage: %s, blockHash: %s",
			completed.Hash().TerminalString(), completed.BlockHash.TerminalString()))
		n.HBFTDebugInfo(fmt.Sprintf("SetStageCompleted parent, %s", completed.ParentStage.TerminalString()))

		if completed.BlockNum == 1 && completed.BlockHash.Equal(newFuture.Hash()) && n.newCompletedChan != nil {
			n.newCompletedChan <- completed
		}
	}
}

func (n *Node) checkIfAddFutureParent(preConfirmMsg *hbft.PreConfirmMsg, elephant Elephant) bool {
	if !types.EmptyBlockHash.Equal(preConfirmMsg.FutureParent.Hash()) {
		if _, exist := n.CurrentState.GetBlock(preConfirmMsg.FutureParent.Hash()); !exist {
			n.CurrentState.AppendBlock(preConfirmMsg.FutureParent)
		}
		oldestFutureBlock := elephant.BFTOldestFutureBlock()
		FutureParentParentHash := preConfirmMsg.FutureParent.ParentHash()
		if oldestFutureBlock == nil && FutureParentParentHash.Equal(elephant.GetCurrentBlock().Hash()) &&
			len(preConfirmMsg.OldestFutures) == elephant.Get2fRealSealersCnt()+1 {
			n.HBFTDebugInfo(fmt.Sprintf(
				"checkIfAddFutureParent, elephant.BFTOldestFutureBlock() == nil, newestStateCompletedStage.ViewID: %d",
				n.CurrentState.GetNewestStateCompletedStage().ViewID))

			canReplace, hash, _ := n.checkIfOldestFuturesCanReplace(preConfirmMsg.OldestFutures,
				preConfirmMsg.SequenceID, preConfirmMsg.ViewID)
			if canReplace && hash.Equal(preConfirmMsg.FutureParent.Hash()) {
				n.HBFTDebugInfo(fmt.Sprintf(
					"checkIfAddFutureParent return true, preConfirmMsg.FutureParent: %s, FutureParentParentHash: %s",
					preConfirmMsg.FutureParent.Hash().TerminalString(), FutureParentParentHash.TerminalString()))
				return true
			}

		} else if oldestFutureBlock != nil && FutureParentParentHash.Equal(elephant.GetCurrentBlock().Hash()) {
			if n.CurrentState.GetNewestStateCompletedStage().BlockHash.Equal(preConfirmMsg.FutureParent.Hash()) &&
				n.CurrentState.GetNewestStateCompletedStage().BlockHash.Equal(oldestFutureBlock.Hash()) {
				return false
			}
			n.HBFTDebugInfo(fmt.Sprintf(
				"checkIfAddFutureParent, elephant.BFTOldestFutureBlock() != nil, newestStateCompletedStage.ReqTimeStamp: %d",
				n.CurrentState.GetNewestStateCompletedStage().ReqTimeStamp%1000000))

			newestCompleted := n.CurrentState.GetNewestStateCompletedStage()
			if newestCompleted == nil {
				n.HBFTDebugInfo("checkIfAddFutureParent return false, newestCompleted is nil")
				return false
			}
			if !newestCompleted.BlockHash.Equal(oldestFutureBlock.Hash()) {
				n.HBFTDebugInfo(fmt.Sprintf("checkIfAddFutureParent return false, newestCompleted block: %s, BFTOldestFutureBlock: %s",
					newestCompleted.BlockHash.TerminalString(), oldestFutureBlock.Hash().TerminalString()))
				return false
			}

			canReplace, hash, replaceAll := n.checkIfOldestFuturesCanReplace(preConfirmMsg.OldestFutures,
				preConfirmMsg.SequenceID, preConfirmMsg.ViewID)
			if canReplace && replaceAll && hash.Equal(preConfirmMsg.FutureParent.Hash()) {
				n.HBFTDebugInfo(fmt.Sprintf(
					"checkIfAddFutureParent return true, preConfirmMsg.FutureParent: %s, FutureParentParentHash: %s",
					preConfirmMsg.FutureParent.Hash().TerminalString(), FutureParentParentHash.TerminalString()))
				return true
			}
		}
	}
	return false
}

// HandleConfirmMsg can be called when the node receives a confirm message.
func (n *Node) HandleConfirmMsg(confirmMsg *hbft.ConfirmMsg) (*hbft.ReplyMsg, error) {
	n.HBFTDebugInfo(fmt.Sprintf("HandleConfirmMsg, newest completed stage: %s, block: %s",
		confirmMsg.CurState.Completed[len(confirmMsg.CurState.Completed)-1].Hash().TerminalString(),
		confirmMsg.CurState.Completed[len(confirmMsg.CurState.Completed)-1].BlockHash.TerminalString()))
	// Check Signature first
	if err := n.checkMsgSignature(confirmMsg); err != nil {
		return nil, err
	}
	// Create a new state for the new consensus if the current state is nil.
	n.createStateForNewConsensus(confirmMsg)
	if n.CurrentState == nil || n.CurrentState.CurrentStage == hbft.Idle {
		return nil, errors.New("the node is in idle state and discard the confirm message received")
	}

	// Check if the preConfirm message is valid
	elephant := n.EleBackend
	err := n.CurrentState.CheckView(confirmMsg, 0, 0) // Check view
	if err != nil {
		return nil, err
	}
	errState := n.CurrentState.CheckCurrentState(confirmMsg, elephant.GetCurrentBlock(), nil,
		elephant.Get2fRealSealersCnt(), false) // Check current state
	if errState != nil {
		return nil, errState
	}

	n.CurrentState.MsgLogs.ConfirmMsgs.Store(confirmMsg.Hash(), confirmMsg)
	n.CurrentState.ViewPrimary = confirmMsg.ViewPrimary
	n.addNewFutureBlockAndSetCompleted(n.CurrentState.SignedBlocks[len(n.CurrentState.SignedBlocks)-1], confirmMsg.CurState.Completed)

	replyMsg, errReply := n.GetReplyMsg(confirmMsg)
	if errReply != nil {
		return nil, errReply
	} else {
		return replyMsg, nil
	}
}

func (n *Node) GetConfirmMsg(replyMsg *hbft.ReplyMsg, blockInReplyHash common.Hash, blockInReplyNum uint64) (*hbft.ConfirmMsg, error) {
	if len(n.CurrentState.SignedBlocks) == 0 {
		return nil, errors.New("GetConfirmMsg failed, there is no any block in current state")
	}

	stageCompleted := &types.HBFTStageCompleted{
		ViewID:       n.CurrentState.ViewID, // TODO: need to be verified
		SequenceID:   n.CurrentState.SequenceID,
		BlockHash:    blockInReplyHash,
		BlockNum:     blockInReplyNum,
		MsgType:      uint(hbft.MsgPreConfirm),
		MsgHash:      n.CurrentState.MsgHash,
		ValidSigns:   make([][]byte, 0),
		ParentStage:  n.CurrentState.GetNewestCompletedStage(n.EleBackend.GetCurrentBlock()).Hash(),
		Timestamp:    hbft.GetTimeStampForSet(replyMsg.ReqTimeStamp, n.CoinBase),
		ReqTimeStamp: replyMsg.ReqTimeStamp,
	}
	for _, reply := range *n.ReplyBuffer { // we already have ReplyBufMutex
		if stageCompleted.MsgHash.Equal(reply.MsgHash) {
			stageCompleted.ValidSigns = append(stageCompleted.ValidSigns, reply.Signature)
		}
	}
	if len(stageCompleted.ValidSigns) <= n.EleBackend.Get2fRealSealersCnt() {
		return nil, errors.New("GetConfirmMsg failed, not enough HBFT pre-confirm stage message")
	}

	curCompleted := n.CurrentState.SliceCompletedStages()
	curCompleted = append(curCompleted, stageCompleted)
	curState := &hbft.StateMsg{
		ViewID:       n.CurrentState.ViewID,
		SequenceID:   n.CurrentState.SequenceID,
		CurrentStage: n.CurrentState.CurrentStage,
		Completed:    curCompleted,
	}

	confirmMsg := &hbft.ConfirmMsg{
		ViewPrimary:  n.CurrentState.ViewPrimary,
		ViewID:       n.CurrentState.ViewID,
		SequenceID:   n.CurrentState.SequenceID,
		CurState:     curState,
		ReqTimeStamp: replyMsg.ReqTimeStamp,
	}

	n.addNewFutureBlockAndSetCompleted(n.CurrentState.SignedBlocks[len(n.CurrentState.SignedBlocks)-1],
		[]*types.HBFTStageCompleted{stageCompleted})

	signature, err := n.getMsgSignature(confirmMsg.Hash())
	if err != nil {
		return nil, err
	}
	confirmMsg.Signature = signature

	return confirmMsg, nil
}

func (n *Node) GetReplyMsg(msg hbft.MsgHbftConsensus) (*hbft.ReplyMsg, error) {
	replyMsg := &hbft.ReplyMsg{
		ViewPrimary:  msg.GetViewPrimary(),
		ViewID:       msg.GetViewID(),
		SequenceID:   msg.GetSequenceID(),
		MsgHash:      msg.Hash(),
		ReqTimeStamp: msg.GetReqTimeStamp(),
	}

	switch msg.(type) {
	case *hbft.PreConfirmMsg:
		replyMsg.MsgType = hbft.MsgPreConfirm

	case *hbft.ConfirmMsg:
		replyMsg.MsgType = hbft.MsgConfirm
	}

	signature, err := n.getMsgSignature(replyMsg.MsgHash)
	if err != nil {
		return nil, err
	}
	replyMsg.Signature = signature

	return replyMsg, nil
}

func (n *Node) createStateForNewConsensus(msg hbft.MsgHbftConsensus) {
	n.confirmReplyCnt = 0
	if n.CurrentState != nil && n.CurrentState.CurrentStage != hbft.Idle {
		return
	}

	// Create a new state for this new consensus process in the Primary
	switch msg.(type) {
	case *hbft.RequestMsg, *hbft.PreConfirmMsg:
		if n.CurrentState == nil {
			n.CurrentState = CreateState(n.CoinBase, msg.GetViewPrimary(), msg.GetViewID(), msg.GetSequenceID(),
				n.EleBackend.Get2fRealSealersCnt(), n.Log)
		} else {
			n.CurrentState.UpdateState(n.CoinBase, msg.GetViewPrimary(), msg.GetViewID(), msg.GetSequenceID(),
				n.EleBackend.GetCurrentBlock().NumberU64())
		}

	case *hbft.ConfirmMsg:
		return
	}

	n.HBFTDebugInfo(fmt.Sprintf("The block hash of newest completed stage is: %s",
		n.CurrentState.GetNewestStateCompletedStage().BlockHash.TerminalString()))
}

func (n *Node) resolveRequestMsg(reqMsg *hbft.RequestMsg) error {
	preConfirmMsg, err := n.HandleReq(reqMsg)
	if err != nil {
		return err
	} else {
		n.SendHBFTMsg(preConfirmMsg, hbft.MsgPreConfirm)
	}

	// Change the stage to pre-confirm.
	n.CurrentState.NodeCoinBase = preConfirmMsg.NewBlock.Coinbase()
	n.CurrentState.CurrentStage = hbft.PreConfirm
	n.CurrentState.ViewID = preConfirmMsg.ViewID
	n.CurrentState.SequenceID = preConfirmMsg.SequenceID
	n.CurrentState.MsgHash = preConfirmMsg.Hash()

	*n.ReplyBuffer = make([]*hbft.ReplyMsg, 0)
	replyMsg, err := n.GetReplyMsg(preConfirmMsg)
	if err != nil {
		n.Error(err.Error())
	} else {
		go n.AddReplyMsg(replyMsg)
	}

	n.LogStage(fmt.Sprintf("Pre-confirm, seq: %d, view: %d, prim: %s",
		preConfirmMsg.SequenceID, preConfirmMsg.ViewID, preConfirmMsg.ViewPrimary), false)

	return nil
}

func (n *Node) resolvePreConfirmMsg(preConfirmMsg *hbft.PreConfirmMsg) error {
	replyMsg, err := n.HandlePreConfirmMsg(preConfirmMsg)
	if err != nil {
		return err
	}

	// Change the stage to pre-confirm.
	n.CurrentState.NodeCoinBase = preConfirmMsg.NewBlock.Coinbase()
	n.CurrentState.CurrentStage = hbft.PreConfirm
	n.CurrentState.ViewID = preConfirmMsg.ViewID
	n.CurrentState.SequenceID = preConfirmMsg.SequenceID
	n.CurrentState.MsgHash = preConfirmMsg.Hash()

	n.LogStage(fmt.Sprintf("Pre-confirm, seq: %d, view: %d, prim: %s",
		preConfirmMsg.SequenceID, preConfirmMsg.ViewID, preConfirmMsg.ViewPrimary), false)

	n.SendHBFTMsg(replyMsg, hbft.MsgReply)

	return nil
}

func (n *Node) resolveConfirmMsg(confirmMsg *hbft.ConfirmMsg) error {
	replyMsg, err := n.HandleConfirmMsg(confirmMsg)
	if err != nil {
		return err
	}

	// Change the stage to pre-confirm.
	n.CurrentState.CurrentStage = hbft.Confirm
	n.CurrentState.ViewID = confirmMsg.ViewID
	n.CurrentState.SequenceID = confirmMsg.SequenceID

	n.LogStage(fmt.Sprintf("Pre-confirm, seq: %d, view: %d, prim: %s",
		confirmMsg.SequenceID, confirmMsg.ViewID, confirmMsg.ViewPrimary), true)
	n.LogStage(fmt.Sprintf("Confirm, seq: %d, view: %d, prim: %s",
		confirmMsg.SequenceID, confirmMsg.ViewID, confirmMsg.ViewPrimary), false)

	n.SendHBFTMsg(replyMsg, hbft.MsgReply)

	return nil
}

func (n *Node) resolveOldestFutureMsg(oldestFutureMsg *hbft.OldestFutureMsg) error {
	if !common.EmptyHash(oldestFutureMsg.Completed.BlockHash) {
		n.HBFTDebugInfo(fmt.Sprintf("HandleOldestFutureMsg, stage: %s, block: %s, reqT: %d, view: %d, prim: %s",
			oldestFutureMsg.Completed.Hash().TerminalString(), oldestFutureMsg.Completed.BlockHash.TerminalString(),
			oldestFutureMsg.Completed.ReqTimeStamp%1000000, oldestFutureMsg.ViewID, oldestFutureMsg.ViewPrimary))
	} else {
		n.HBFTDebugInfo(fmt.Sprintf("HandleOldestFutureMsg, stage: %s, block: %s, reqT: %d, view: %d, prim: %s",
			common.Hash{}.TerminalString(), common.Hash{}.TerminalString(),
			0, oldestFutureMsg.ViewID, oldestFutureMsg.ViewPrimary))
	}
	if n.oldestFutureFull {
		return nil
	}

	// Check Signature first
	if oldestFutureMsg.NoReply {
		if err := n.checkMsgSignature(oldestFutureMsg); err != nil {
			return err
		}
	}

	elephant := n.EleBackend
	// Check view
	currentView := n.GetCurrentViewID(elephant.GetCurrentBlock())
	currentSeq := n.GetCurrentSequenceID(elephant.GetCurrentBlock())
	if currentSeq != oldestFutureMsg.GetSequenceID() || (oldestFutureMsg.NoReply && currentView != oldestFutureMsg.GetViewID()) ||
		(!oldestFutureMsg.NoReply && currentView != oldestFutureMsg.GetViewID() && currentView != oldestFutureMsg.GetViewID()-1) {
		return errors.New(fmt.Sprintf("CheckView failed in oldest-future msg, %s, currentSeq: %d, currentView: %d, statePrim: %s",
			InvalidViewOrSeqIDErr, currentSeq, currentView, n.CoinBase.String()))
	}

	// Check if oldest future block is valid
	err := n.checkIfOldestFutureValid(oldestFutureMsg, elephant)
	if err != nil {
		return err
	}

	newOldestFutureMsgFlag := false
	if !oldestFutureMsg.NoReply {
		defer func() {
			if !newOldestFutureMsgFlag {
				oldestFutureMsgToSend := n.waitForGenMyOldestFutureMsgToReply(oldestFutureMsg.ViewID)
				if oldestFutureMsgToSend != nil {
					n.SendHBFTMsg(oldestFutureMsgToSend, hbft.MsgOldestFuture)
				}
			}
		}()
	}

	// Check if we have future blocks
	newestCompletedStage := n.CurrentState.GetNewestStateCompletedStage()
	if newestCompletedStage == nil || newestCompletedStage.BlockNum <= elephant.GetCurrentBlock().NumberU64() ||
		common.EmptyHash(newestCompletedStage.BlockHash) {
		return nil
	}

	// Check if the the view of the oldest future block is the bigger than ours
	var newOldestFutureMsg *hbft.OldestFutureMsg
	if newOldestFutureMsgFlag, newOldestFutureMsg, err = n.handleNewOldestFuture(oldestFutureMsg, newestCompletedStage, elephant); err != nil {
		return err
	}
	if !oldestFutureMsg.NoReply && newOldestFutureMsgFlag && newOldestFutureMsg != nil {
		n.SendHBFTMsg(newOldestFutureMsg, hbft.MsgOldestFuture)
	}

	return nil
}

func (n *Node) waitForGenMyOldestFutureMsgToReply(viewID uint64) (myOldestFutureMsg *hbft.OldestFutureMsg) {
	myOldestFutureMsg = nil

	n.myOldestFutureMutex.RLock()
	if n.myOldestFutureMsg != nil && n.myOldestFutureMsg.ViewID >= viewID {
		myOldestFutureMsg = n.myOldestFutureMsg
		n.myOldestFutureMutex.RUnlock()
		return
	}
	n.myOldestFutureMutex.RUnlock()

	times := uint32((BlockMaxPeriodHbft - BlockMinPeriodHbft).Nanoseconds() / BlockSealingBeat.Nanoseconds() / 4)
	ticker := time.NewTicker(BlockSealingBeat)
	defer ticker.Stop()
	var current uint32 = 0

loop:
	for {
		select {
		case <-ticker.C:
			n.myOldestFutureMutex.RLock()
			if n.myOldestFutureMsg != nil && n.myOldestFutureMsg.ViewID >= viewID {
				myOldestFutureMsg = n.myOldestFutureMsg
				n.myOldestFutureMutex.RUnlock()
				return
			}
			n.myOldestFutureMutex.RUnlock()

			if current >= times {
				break loop
			}
			current++
		}
	}

	n.Warn(fmt.Sprintf("Generate oldest-future message timed out, view: %d", viewID))
	return
}

func (n *Node) checkIfOldestFutureValid(oldestFutureMsg *hbft.OldestFutureMsg, elephant Elephant) error {
	if oldestFutureMsg.OldestFuture == nil && common.EmptyHash(oldestFutureMsg.Completed.BlockHash) &&
		oldestFutureMsg.NoAnyFutures {
		return nil
	} else if oldestFutureMsg.OldestFuture == nil {
		return errors.New("oldest future block is nil")
	} else if !oldestFutureMsg.Completed.BlockHash.Equal(oldestFutureMsg.OldestFuture.Hash()) {
		return errors.New("the hash of the oldest future block is not valid")
	} else if oldestFutureMsg.Completed.BlockNum != elephant.GetCurrentBlock().NumberU64()+1 {
		return errors.New("the height of the oldest future block is not valid")
	} else {
		parentHash := oldestFutureMsg.OldestFuture.ParentHash()
		if !parentHash.Equal(elephant.GetCurrentBlock().Hash()) {
			return errors.New(fmt.Sprintf("the parent hash of the oldest future block is not valid: %s", parentHash.TerminalString()))
		}
	}

	return nil
}

func (n *Node) handleNewOldestFuture(oldestFutureMsg *hbft.OldestFutureMsg, newestCompletedStage *types.HBFTStageCompleted,
	elephant Elephant) (bool, *hbft.OldestFutureMsg, error) {
	if newestCompletedStage.BlockNum != elephant.GetCurrentBlock().NumberU64()+1 {
		n.HBFTDebugInfo(fmt.Sprintf("the block height of our newestCompletedStage is %d, which is not valid in handleNewOldestFuture",
			newestCompletedStage.BlockNum))
		return false, nil, nil
	}
	bFTOldestFutureBlock := elephant.BFTOldestFutureBlock()
	if newestCompletedStage == nil || common.EmptyHash(newestCompletedStage.BlockHash) || bFTOldestFutureBlock == nil {
		n.HBFTDebugInfo("we do not have a BFT oldest future block")
		return false, nil, nil
	} else if !newestCompletedStage.BlockHash.Equal(bFTOldestFutureBlock.Hash()) {
		return false, nil, errors.New("we do not have a valid BFT oldest future block")
	} else {
		n.HBFTDebugInfo(fmt.Sprintf("our oldest future block info as follows: stage: %s, block: %s, reqT: %d",
			newestCompletedStage.Hash().TerminalString(), newestCompletedStage.BlockHash.TerminalString(),
			newestCompletedStage.ReqTimeStamp%1000000))
	}

	//oldestFutureMsgToReturn := oldestFutureMsg
	appendOldestFutureFlag := true
	defer func() {
		if appendOldestFutureFlag {
			if full := n.appendValidOldestFutureMsgs(oldestFutureMsg); full {
				n.oldestFutureFull = true
			}
		}
	}()

	if oldestFutureMsg.Completed.ReqTimeStamp > newestCompletedStage.ReqTimeStamp {
		if oldestFutureMsg.Completed.BlockHash.Equal(newestCompletedStage.BlockHash) {
			appendOldestFutureFlag = false
			return false, nil, errors.New("we have the same valid BFT oldest future block but our reqTimeStamp is smaller")
		}
	} else if oldestFutureMsg.Completed.ReqTimeStamp == newestCompletedStage.ReqTimeStamp {
		if !oldestFutureMsg.Completed.BlockHash.Equal(newestCompletedStage.BlockHash) {
			appendOldestFutureFlag = false
			return false, nil, errors.New("we do not have same valid BFT oldest future block but the same reqTimeStamp")
		}
	} else {
		if oldestFutureMsg.Completed.BlockHash.Equal(newestCompletedStage.BlockHash) {
			appendOldestFutureFlag = false
			return false, nil, errors.New("we have the same valid BFT oldest future block but our reqTimeStamp is bigger")
		}
	}

	oldestFutureMsgToReturn := n.waitForGenMyOldestFutureMsgToReply(oldestFutureMsg.ViewID)
	return true, oldestFutureMsgToReturn, nil
}

func (n *Node) getMyOldestFutureMsg(newestCompletedStage *types.HBFTStageCompleted, oldestFuture *types.Block,
	noReply bool) (*hbft.OldestFutureMsg, error) {
	myOldestFutureMsg := &hbft.OldestFutureMsg{
		ViewPrimary:  n.CoinBase.String(),
		ViewID:       n.GetCurrentViewID(n.EleBackend.GetCurrentBlock()),
		SequenceID:   n.GetCurrentSequenceID(n.EleBackend.GetCurrentBlock()),
		Completed:    newestCompletedStage,
		NoReply:      noReply,
		NoAnyFutures: false,
		OldestFuture: nil,
	}
	if common.EmptyHash(newestCompletedStage.BlockHash) {
		myOldestFutureMsg.NoAnyFutures = true
	} else {
		myOldestFutureMsg.OldestFuture = oldestFuture
	}

	if noReply {
		var err error
		myOldestFutureMsg.Signature, err = n.getMsgSignature(myOldestFutureMsg.Hash())
		if err != nil {
			return nil, err
		}
	}
	return myOldestFutureMsg, nil
}

func (n *Node) appendValidOldestFutureMsgs(oldestFutureMsg *hbft.OldestFutureMsg) bool {
	n.oldestFutureMutex.Lock()
	defer n.oldestFutureMutex.Unlock()

	if n.oldestFutureFull {
		return true
	}

	for i, oldestFutureMsgHave := range n.oldestFutureMsgs {
		if strings.Compare(oldestFutureMsgHave.ViewPrimary, oldestFutureMsg.ViewPrimary) == 0 {
			if oldestFutureMsgHave.Completed.BlockHash.Equal(oldestFutureMsg.Completed.BlockHash) {
				return false
			} else if oldestFutureMsg.Completed.ReqTimeStamp > oldestFutureMsgHave.Completed.ReqTimeStamp {
				n.oldestFutureMsgs[i] = oldestFutureMsg
				return false
			}
		}
	}

	n.oldestFutureMsgs = append(n.oldestFutureMsgs, oldestFutureMsg)
	if len(n.oldestFutureMsgs) == n.EleBackend.Get2fRealSealersCnt()+1 {
		curViewID, curSequenceID := oldestFutureMsg.ViewID, oldestFutureMsg.SequenceID
		n.HBFTDebugInfo("=========================================================")
		if replace, _, _ := n.checkIfOldestFuturesCanReplace(n.oldestFutureMsgs, curSequenceID, curViewID); replace {
			n.HBFTDebugInfo("preConfirmMsg.OldestFutures reached consensus when appending new oldestFuture")
			return true
		} else {
			n.oldestFutureFull = false
			n.oldestFutureMsgs = make([]*hbft.OldestFutureMsg, 0)
			return false
		}
	} else {
		return false
	}
}

func (n *Node) clearOutDatedOldestFutureMsgs(curViewID, curSequenceID uint64) {
	n.oldestFutureMutex.Lock()
	defer n.oldestFutureMutex.Unlock()

	if n.EleBackend.GetCurrentBlock().NumberU64() == 0 {
		n.oldestFutureFull = false
		n.oldestFutureMsgs = make([]*hbft.OldestFutureMsg, 0)
		return
	}

	for i := 0; i < len(n.oldestFutureMsgs); {
		if n.oldestFutureMsgs[i].SequenceID < curSequenceID || n.oldestFutureMsgs[i].ViewID < curViewID {
			n.oldestFutureMsgs = append(n.oldestFutureMsgs[:i], n.oldestFutureMsgs[i+1:]...)
			n.oldestFutureFull = false
		} else {
			i++
		}
	}
}

func (n *Node) getMsgSignature(msgHash common.Hash) ([]byte, error) {
	passwd := blizparam.GetCsSelfPassphrase()
	if n.passphrase != "" {
		passwd = n.passphrase
	}
	if common.EmptyAddress(n.CoinBase) {
		var err error
		if n.CoinBase, err = n.EleBackend.Etherbase(); err != nil {
			return nil, err
		}
	}
	return n.HbftSign(n.CoinBase, passwd, msgHash.Bytes())
}

func (n *Node) checkMsgSignature(msg hbft.MsgHbftConsensus) error {
	switch msg.(type) {
	case *hbft.PreConfirmMsg, *hbft.ConfirmMsg:
		addr, err := n.HbftEcRecover(msg.Hash().Bytes(), msg.GetSignature())
		if err != nil {
			return err
		}
		sealers, _ := n.EleBackend.GetCurRealSealers()
		curSealer := n.GetCurrentSealer(sealers, false)
		if !addr.Equal(curSealer) {
			return fmt.Errorf("checkMsgSignature failed, the sealer address %s is not valid, which is supposed to be %s",
				addr.String(), curSealer.String())
		}

		primary := common.HexToAddress(msg.GetViewPrimary())
		if !addr.Equal(primary) {
			return fmt.Errorf("checkMsgSignature failed, view primary address %s is not valid", msg.GetViewPrimary())
		}
		return nil

	case *hbft.ReplyMsg:
		_, err := n.HbftEcRecover(msg.GetMsgHash().Bytes(), msg.GetSignature())
		if err != nil {
			return err
		}
		return nil

	case *hbft.OldestFutureMsg:
		addr, err := n.HbftEcRecover(msg.Hash().Bytes(), msg.GetSignature())
		if err != nil {
			return err
		}
		primary := common.HexToAddress(msg.GetViewPrimary())
		if !addr.Equal(primary) {
			return fmt.Errorf("checkMsgSignature failed, view primary address %s is not valid, which is supposed to be %s",
				msg.GetViewPrimary(), addr.String())
		}
		return nil
	}
	return errors.New("checkMsgSignature failed, unknown msg type")
}
