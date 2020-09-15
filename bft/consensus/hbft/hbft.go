package hbft

import (
	"sync"

	"github.com/contatract/go-contatract/common"
	types "github.com/contatract/go-contatract/core/types_elephant"
)

const (
	Idle         Stage = iota // Node is created successfully, but the consensus process is not started yet.
	PreConfirm                // The `preConfirmMsg` is processed successfully. The node is ready to head to the Confirm stage.
	Confirm                   // The `confirmMsg` is processed successfully. The node is ready to head to the next block processing.
	InvalidStage              // Unavailable stage
)

// stage represents that the node in which state in the consensus process
type Stage uint

type MsgHbftConsensus interface {
	GetViewPrimary() string
	GetViewID() uint64
	GetSequenceID() uint64
	GetNewBlock() *types.Block
	GetCurStage() Stage
	GetCompleted() []*types.HBFTStageCompleted
	GetMsgType() TypeHBFTMsg
	GetMsgHash() common.Hash
	GetReqTimeStamp() uint64
	GetSignature() []byte
	Hash() common.Hash
}

type BFTMsg struct {
	ViewPrimary     string
	MsgType         TypeHBFTMsg
	MsgPreConfirm   *PreConfirmMsg
	MsgConfirm      *ConfirmMsg
	MsgReply        *ReplyMsg
	MsgOldestFuture *OldestFutureMsg
}

func NewBFTMsg(msg MsgHbftConsensus, msgType uint) *BFTMsg {
	bftMsg := &BFTMsg{
		MsgType: TypeHBFTMsg(msgType),
	}
	switch msg.(type) {
	case *PreConfirmMsg:
		bftMsg.ViewPrimary = msg.(*PreConfirmMsg).ViewPrimary
		bftMsg.MsgPreConfirm = msg.(*PreConfirmMsg)
		bftMsg.MsgConfirm = EmptyConfirmMsg
		bftMsg.MsgReply = EmptyReplyMsg
		bftMsg.MsgOldestFuture = EmptyOldestFutureMsg

	case *ConfirmMsg:
		bftMsg.ViewPrimary = msg.(*ConfirmMsg).ViewPrimary
		bftMsg.MsgPreConfirm = EmptyPreConfirmMsg
		bftMsg.MsgConfirm = msg.(*ConfirmMsg)
		bftMsg.MsgReply = EmptyReplyMsg
		bftMsg.MsgOldestFuture = EmptyOldestFutureMsg

	case *ReplyMsg:
		bftMsg.ViewPrimary = msg.(*ReplyMsg).ViewPrimary
		bftMsg.MsgPreConfirm = EmptyPreConfirmMsg
		bftMsg.MsgConfirm = EmptyConfirmMsg
		bftMsg.MsgReply = msg.(*ReplyMsg)
		bftMsg.MsgOldestFuture = EmptyOldestFutureMsg

	case *OldestFutureMsg:
		bftMsg.ViewPrimary = msg.(*OldestFutureMsg).ViewPrimary
		bftMsg.MsgPreConfirm = EmptyPreConfirmMsg
		bftMsg.MsgConfirm = EmptyConfirmMsg
		bftMsg.MsgReply = EmptyReplyMsg
		bftMsg.MsgOldestFuture = msg.(*OldestFutureMsg)

	}

	return bftMsg
}

func (msg *BFTMsg) IsReply() bool {
	return msg.MsgType == TypeHBFTMsg(MsgReply)
}

type StateMsg struct {
	ViewID       uint64
	SequenceID   uint64
	CurrentStage Stage
	Completed    []*types.HBFTStageCompleted
}

type MsgLogs struct {
	PreConfirmMsgs *sync.Map
	ConfirmMsgs    *sync.Map
}

func (log *MsgLogs) ClearOutdated(height uint64) {
	log.PreConfirmMsgs.Range(func(k, v interface{}) bool {
		if v.(*PreConfirmMsg).NewBlock.NumberU64() < height {
			log.PreConfirmMsgs.Delete(k.(common.Hash))
		}
		return true
	})
	log.ConfirmMsgs = new(sync.Map)
}

var HBFTDebugInfoOn = true

func IsDebugMode() bool {
	return HBFTDebugInfoOn
}
