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

package externgui

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"
)

const (
	netType = "tcp"
	// port        = "10309"
	connTimeout = 60 * time.Second
	maxCmdSize  = 1024 * 1024
)

//var (
//	connRate   *grpc.ClientConn   = nil
//	rateClient pb.OperationClient = nil
//)

// The operation command request
type OpRequestMsg struct {
	OpCmd    string   `json:"OpCmd,omitempty"`
	OpParams []string `json:"OpParams,omitempty"`
	OpIndex  string   `json:"OpIndex,omitempty"`
}

// The result of the operation command request
type BackendMsg struct {
	MsgType string `json:"MsgType,omitempty"`
	OpCmd   string `json:"OpCmd,omitempty"`
	OpRet   string `json:"OpRet,omitempty"`
	OpIndex string `json:"OpIndex,omitempty"`
}

type guiserver struct {
	lis     net.Listener
	connect net.Conn

	quitConn chan os.Signal
	quitOp   chan os.Signal
}

func NewGUIServer() *guiserver {
	logInfo("Starting RPC server for external GUI")
	return &guiserver{quitConn: make(chan os.Signal)}
}

func (srv *guiserver) Serve(port string) {
	var (
		err         error
		failedTimes = 0
		connected   = false
	)

	go func(connected *bool) {
		for {
			if !(*connected) {
				if failedTimes > 30 {
					logCrit("failed to listen in gui server: timed out")
				}

				sleepDefault()
				failedTimes += 1
			} else {
				return
			}
		}
	}(&connected)

	srv.lis, err = net.Listen(netType, ":"+port)
	if err != nil {
		logCrit("failed to listen in gui server: %v", err)
	}

	logInfo("RPC server Listening for external GUI")
	srv.connect, err = srv.lis.Accept()
	if err != nil {
		logCrit("failed to accept connection from external gui: %v", err)
	}
	connected = true

	go srv.handleGUIRPCConn()
}

func sleepDefault() {
	time.Sleep(time.Second / 3)
}

func recoverJsonStr(i, len int, s string) string {
	if i == 0 {
		s += "}"
	} else if i == len-1 {
		s = "{" + s
	} else {
		s = "{" + s + "}"
	}

	return s
}

func (srv *guiserver) handleGUIRPCConn() {
	//if err := srv.connect.SetDeadline(time.Now().Add(connTimeout)); err != nil {
	//	logCrit("failed to set deadline of the connection from external gui: %v", err)
	//}
	logInfo("Established a connection for RPC server with external GUI")

	var rerr error
	var rsize int
	buffOpCmd := make([]byte, maxCmdSize)
	errorCnt := 0

	signal.Notify(srv.quitConn, os.Interrupt)
	defer srv.lis.Close()
	for {
		select {
		case <-srv.quitConn:
			// User forcefully quite the console
			return

		default:
			rsize, rerr = srv.connect.Read(buffOpCmd)
			if rerr != nil {
				errorCnt++
				if errorCnt == 3 {
					logCrit("We've got connection error from the external GUI")
				} else if errorCnt > 1 {
					logError("We've got connection error from the external GUI")
				}
				time.Sleep(time.Second)
			} else {
				//logInfo(fmt.Sprintf("New incoming msg: %s", string(buffOpCmd[:rsize])))

				buffOpCmdStr := string(buffOpCmd[:rsize])
				strs := strings.Split(buffOpCmdStr, "}{")
				if len(strs) > 1 {
					for i, s := range strs {
						s = recoverJsonStr(i, len(strs), s)
						srv.parseAndHandle([]byte(s))
					}
				} else {
					srv.parseAndHandle(buffOpCmd[:rsize])
				}
			}
		}
	}
}

func (srv *guiserver) sendMsg2GUI(msg BackendMsg) {
	var rErr error
	retBuff := []byte(SysMessage[errorMsg])

	if retBuff, rErr = json.Marshal(&msg); rErr != nil {
		logError(fmt.Sprintf("sendMsg2GUI failed, %s", rErr.Error()))
	}

	_, werr := srv.connect.Write(retBuff)
	if werr != nil {
		logError(fmt.Sprintf("sendMsg2GUI failed, %s", werr.Error()))
	}
	time.Sleep(time.Millisecond * 50)
}

func (srv *guiserver) parseAndHandle(jsonBuff []byte) {
	var opCmdJson OpRequestMsg
	if err := json.Unmarshal(jsonBuff, &opCmdJson); err != nil {
		logError(fmt.Sprintf("parseAndHandle failed, %s", err.Error()))
	} else {
		go srv.watingForOperationRet(opCmdJson)
	}
}

func (srv *guiserver) watingForOperationRet(req OpRequestMsg) {
	if req.OpCmd == RPCCommands[exitExternGUICmd].Cmd {
		go func() {
			srv.quitLoop()
			Close()
		}()
		return
	}

	errStr := make(chan error)
	retStr := make(chan string)
	if req.OpCmd != "elephant_getTotalBalance" && req.OpCmd != "elephant_getBalance" && req.OpCmd != "elephant_getSignedCs" {
		logInfo(fmt.Sprintf("New incoming operation %s: %s", req.OpIndex, req.OpCmd))
	}
	go func(err chan error, result chan string) {
		if ret, errH := srv.handleRPCOperation(req); errH != nil {
			err <- errH
		} else {
			result <- ret
		}
	}(errStr, retStr)

	srv.quitOp = make(chan os.Signal)
	signal.Notify(srv.quitOp, os.Interrupt)
	defer func() {
		srv.quitOp = nil
	}()
	for {
		select {
		case <-srv.quitOp:
			// User forcefully quite the console
			return

		case ret := <-retStr:
			if req.OpCmd != "elephant_getTotalBalance" && req.OpCmd != "elephant_getBalance" && req.OpCmd != "elephant_getSignedCs" {
				logInfo(fmt.Sprintf("handleRPCOperation %s: %s, result: %s", req.OpIndex, req.OpCmd, ret))
			}
			srv.returnOpRet(req, ret)
			return

		case err := <-errStr:
			logInfo(fmt.Sprintf("handleRPCOperation %s: %s, result: failed, error: %s", req.OpIndex, req.OpCmd, err))
			srv.returnOpRet(req, err.Error())
			return
		}
	}
}

func (srv *guiserver) returnOpRet(req OpRequestMsg, str string) {
	ret := BackendMsg{SysMessage[opRspMsg], req.OpCmd, str, req.OpIndex}
	srv.sendMsg2GUI(ret)
}

func (srv *guiserver) quitLoop() {
	srv.quitConn <- os.Interrupt
	if srv.quitOp != nil {
		srv.quitOp <- os.Interrupt
	}
}

func (srv *guiserver) handleRPCOperation(req OpRequestMsg) (string, error) {
	switch req.OpCmd {
	case RPCCommands[showAccountsCmd].Cmd:
		return showAccounts(), nil

	case RPCCommands[setDataDirCmd].Cmd:
		return setDataDir(req.OpParams), nil

	default:
		return doOperation(RPCStrCommands[req.OpCmd], req.OpParams), nil

	}
}

func sendRate(rate int32) {
	ret := BackendMsg{SysMessage[rateMsg], "", strconv.FormatInt(int64(rate), 10), "0"}
	srv.sendMsg2GUI(ret)
}
