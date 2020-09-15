package nodeHbft

import (
	"fmt"

	"github.com/contatract/go-contatract/bft/consensus/hbft"
)

func (n *Node) LogStage(info string, isDone bool) {
	if isDone {
		n.Log.Info(fmt.Sprintf("【*】[STAGE-DONE] %s", info))
	} else {
		n.Log.Info(fmt.Sprintf("【*】[STAGE-BEGIN] %s", info))
	}
}

func (n *Node) LogStageReset() {
	n.Log.Info("【*】[STAGE-RESET] Current state reset")
}

func (n *Node) Trace(msg string, ctx ...interface{}) {
	n.Log.Trace(msg, ctx...)
}

func (n *Node) Info(msg string, ctx ...interface{}) {
	n.Log.Info(msg, ctx...)
}

func (n *Node) Warn(msg string, ctx ...interface{}) {
	n.Log.Warn(msg, ctx...)
}

func (n *Node) Error(msg string, ctx ...interface{}) {
	n.Log.Error(msg, ctx...)
}

func (n *Node) HBFTDebugInfo(msg string) {
	if hbft.HBFTDebugInfoOn {
		n.Log.Info(fmt.Sprintf("===HBFT Debug===%s", msg))
	}
}
