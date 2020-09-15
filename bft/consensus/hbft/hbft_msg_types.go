package hbft

import (
	"github.com/contatract/go-contatract/common"
	types "github.com/contatract/go-contatract/core/types_elephant"
	"github.com/contatract/go-contatract/crypto/sha3"
	"github.com/contatract/go-contatract/rlp"
)

var (
	EmptyPreConfirmMsg = &PreConfirmMsg{
		NewBlock: types.NewBlockWithHeader(types.EmptyHeader),
		CurState: &StateMsg{
			Completed: make([]*types.HBFTStageCompleted, 0),
		},
		OldestFutures: make([]*OldestFutureMsg, 0),
		FutureParent:  types.NewBlockWithHeader(types.EmptyHeader),
	}

	EmptyConfirmMsg = &ConfirmMsg{
		CurState: &StateMsg{
			Completed: make([]*types.HBFTStageCompleted, 0),
		},
	}

	EmptyReplyMsg = &ReplyMsg{
		MsgHash: common.Hash{},
	}

	EmptyOldestFutureMsg = &OldestFutureMsg{
		Completed: &types.HBFTStageCompleted{
			BlockHash:   common.Hash{},
			MsgHash:     common.Hash{},
			ParentStage: common.Hash{},
		},
		OldestFuture: types.NewBlockWithHeader(types.EmptyHeader),
	}
)

type RequestMsg struct {
	ViewPrimary string       `json:"viewPrimary"`
	ViewID      uint64       `json:"viewID"`
	Timestamp   uint64       `json:"timestamp"`
	SequenceID  uint64       `json:"sequenceID"`
	NewBlock    *types.Block `json:"newBlock"`
}

func (m *RequestMsg) Hash() common.Hash {
	return rlpHash([]interface{}{
		m.ViewPrimary,
		m.ViewID,
		m.Timestamp,
		m.SequenceID,
		m.NewBlock,
	})
}

func (m *RequestMsg) GetViewPrimary() string {
	return m.ViewPrimary
}

func (m *RequestMsg) GetViewID() uint64 {
	return m.ViewID
}

func (m *RequestMsg) GetSequenceID() uint64 {
	return m.SequenceID
}

func (m *RequestMsg) GetNewBlock() *types.Block {
	return m.NewBlock
}

func (m *RequestMsg) GetCurStage() Stage {
	return InvalidStage
}

func (m *RequestMsg) GetCompleted() []*types.HBFTStageCompleted {
	return nil
}

func (m *RequestMsg) GetMsgType() TypeHBFTMsg {
	return MsgInvalid
}

func (m *RequestMsg) GetMsgHash() common.Hash {
	return common.Hash{}
}

func (m *RequestMsg) GetReqTimeStamp() uint64 {
	return m.Timestamp
}

func (m *RequestMsg) GetSignature() []byte {
	return nil
}

type PreConfirmMsg struct {
	ViewPrimary   string             `json:"viewPrimary"`
	ViewID        uint64             `json:"viewID"`
	SequenceID    uint64             `json:"sequenceID"`
	NewBlock      *types.Block       `json:"newBlock"`
	FutureParent  *types.Block       `json:"futureParent"`
	CurState      *StateMsg          `json:"curState"`
	ReqTimeStamp  uint64             `json:"reqTimeStamp"`
	OldestFutures []*OldestFutureMsg `json:"oldestFutures"`
	Signature     []byte             `json:"signature"`
}

func (m *PreConfirmMsg) Hash() common.Hash {
	return rlpHash([]interface{}{
		m.ViewPrimary,
		m.ViewID,
		m.SequenceID,
		m.NewBlock,
		m.FutureParent,
		m.CurState,
		m.ReqTimeStamp,
	})
}

func (m *PreConfirmMsg) GetViewPrimary() string {
	return m.ViewPrimary
}

func (m *PreConfirmMsg) GetViewID() uint64 {
	return m.ViewID
}

func (m *PreConfirmMsg) GetSequenceID() uint64 {
	return m.SequenceID
}

func (m *PreConfirmMsg) GetNewBlock() *types.Block {
	return m.NewBlock
}

func (m *PreConfirmMsg) GetCurStage() Stage {
	return m.CurState.CurrentStage
}

func (m *PreConfirmMsg) GetCompleted() []*types.HBFTStageCompleted {
	return m.CurState.Completed
}

func (m *PreConfirmMsg) GetMsgType() TypeHBFTMsg {
	return MsgInvalid
}

func (m *PreConfirmMsg) GetMsgHash() common.Hash {
	return common.Hash{}
}

func (m *PreConfirmMsg) GetReqTimeStamp() uint64 {
	return m.ReqTimeStamp
}

func (m *PreConfirmMsg) GetSignature() []byte {
	return m.Signature
}

type ConfirmMsg struct {
	ViewPrimary  string    `json:"viewPrimary"`
	ViewID       uint64    `json:"viewID"`
	SequenceID   uint64    `json:"sequenceID"`
	CurState     *StateMsg `json:"curState"`
	ReqTimeStamp uint64    `json:"reqTimeStamp"`
	Signature    []byte    `json:"signature"`
}

func (m *ConfirmMsg) Hash() common.Hash {
	return rlpHash([]interface{}{
		m.ViewPrimary,
		m.ViewID,
		m.SequenceID,
		m.CurState,
		m.ReqTimeStamp,
	})
}

func (m *ConfirmMsg) GetViewPrimary() string {
	return m.ViewPrimary
}

func (m *ConfirmMsg) GetViewID() uint64 {
	return m.ViewID
}

func (m *ConfirmMsg) GetSequenceID() uint64 {
	return m.SequenceID
}

func (m *ConfirmMsg) GetNewBlock() *types.Block {
	return nil
}

func (m *ConfirmMsg) GetCurStage() Stage {
	return m.CurState.CurrentStage
}

func (m *ConfirmMsg) GetCompleted() []*types.HBFTStageCompleted {
	return m.CurState.Completed
}

func (m *ConfirmMsg) GetMsgType() TypeHBFTMsg {
	return MsgInvalid
}

func (m *ConfirmMsg) GetMsgHash() common.Hash {
	return common.Hash{}
}

func (m *ConfirmMsg) GetReqTimeStamp() uint64 {
	return m.ReqTimeStamp
}

func (m *ConfirmMsg) GetSignature() []byte {
	return m.Signature
}

type ReplyMsg struct {
	ViewPrimary  string      `json:"viewPrimary"`
	ViewID       uint64      `json:"viewID"`
	SequenceID   uint64      `json:"sequenceID"`
	MsgType      TypeHBFTMsg `json:"MsgType"`
	MsgHash      common.Hash `json:"msgHash"`
	ReqTimeStamp uint64      `json:"reqTimeStamp"`
	Signature    []byte      `json:"signature"`
}

func (m *ReplyMsg) Hash() common.Hash {
	return rlpHash([]interface{}{
		m.ViewPrimary,
		m.ViewID,
		m.SequenceID,
		m.MsgType,
		m.MsgHash,
		//m.ReqTimeStamp, // is already included by msg.MsgHash
	})
}

func (m *ReplyMsg) GetViewPrimary() string {
	return m.ViewPrimary
}

func (m *ReplyMsg) GetViewID() uint64 {
	return m.ViewID
}

func (m *ReplyMsg) GetSequenceID() uint64 {
	return m.SequenceID
}

func (m *ReplyMsg) GetNewBlock() *types.Block {
	return nil
}

func (m *ReplyMsg) GetCurStage() Stage {
	return InvalidStage
}

func (m *ReplyMsg) GetCompleted() []*types.HBFTStageCompleted {
	return nil
}

func (m *ReplyMsg) GetMsgType() TypeHBFTMsg {
	return m.MsgType
}

func (m *ReplyMsg) GetMsgHash() common.Hash {
	return m.MsgHash
}

func (m *ReplyMsg) GetReqTimeStamp() uint64 {
	return m.ReqTimeStamp
}

func (m *ReplyMsg) GetSignature() []byte {
	return m.Signature
}

type OldestFutureMsg struct {
	ViewPrimary  string                    `json:"viewPrimary"`
	ViewID       uint64                    `json:"viewID"`
	SequenceID   uint64                    `json:"sequenceID"`
	Completed    *types.HBFTStageCompleted `json:"completed"` // completed stage of pre-confirm messages
	NoReply      bool                      `json:"noReply"`
	NoAnyFutures bool                      `json:"noAnyFutures"`
	Signature    []byte                    `json:"signature"`

	OldestFuture *types.Block `json:"oldestFuture"`
}

func (m *OldestFutureMsg) Hash() common.Hash {
	return rlpHash([]interface{}{
		m.ViewPrimary,
		m.ViewID,
		m.SequenceID,
		m.Completed,
		m.NoReply,
		m.NoAnyFutures,
	})
}

func (m *OldestFutureMsg) GetViewPrimary() string {
	return m.ViewPrimary
}

func (m *OldestFutureMsg) GetViewID() uint64 {
	return m.ViewID
}

func (m *OldestFutureMsg) GetSequenceID() uint64 {
	return m.SequenceID
}

func (m *OldestFutureMsg) GetNewBlock() *types.Block {
	return m.OldestFuture
}

func (m *OldestFutureMsg) GetCurStage() Stage {
	return InvalidStage
}

func (m *OldestFutureMsg) GetCompleted() []*types.HBFTStageCompleted {
	return []*types.HBFTStageCompleted{m.Completed}
}

func (m *OldestFutureMsg) GetMsgType() TypeHBFTMsg {
	return MsgInvalid
}

func (m *OldestFutureMsg) GetMsgHash() common.Hash {
	return common.Hash{}
}

func (m *OldestFutureMsg) GetReqTimeStamp() uint64 {
	return 0
}

func (m *OldestFutureMsg) GetSignature() []byte {
	return m.Signature
}

type TypeHBFTMsg uint

const (
	MsgPreConfirm TypeHBFTMsg = iota
	MsgConfirm
	MsgReply
	MsgOldestFuture
	MsgInvalid
)

func rlpHash(x interface{}) (h common.Hash) {
	hw := sha3.NewKeccak256()
	rlp.Encode(hw, x)
	hw.Sum(h[:0])
	return h
}
