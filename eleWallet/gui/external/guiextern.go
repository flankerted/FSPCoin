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
	"bufio"
	"errors"
	"fmt"
	"math/big"
	"mime/multipart"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"gopkg.in/urfave/cli.v1"

	"github.com/contatract/go-contatract/eleWallet/flags"
	"github.com/contatract/go-contatract/eleWallet/gui"
	"github.com/contatract/go-contatract/eleWallet/utils"
	"github.com/contatract/go-contatract/log"
)

const Spec = "/"

var (
	srv       *guiserver = nil
	guiClose  chan os.Signal
	ele       gui.GUIBackend
	cmds      map[int]RPCCommand
	ctx       *cli.Context
	externGUI string

	DataDirSetFlag = false

	uiLog log.Logger = nil
)

func setExternGUI(port uint16) {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		logCrit(err.Error())
	}
	// fileName := "CCTTFileMan"
	// dirName := "GUI"
	fileName := "CNTDStorage"
	dirName := fileName
	if port != 10309 {
		dirName += strconv.Itoa(int(port))
	}
	if runtime.GOOS == `windows` {
		externGUI += dir + "\\" + dirName + "\\" + fileName + ".exe"
	} else if runtime.GOOS == `linux` {
		externGUI += dir + "/" + dirName + "/" + fileName
	} else if runtime.GOOS == `darwin` {
		externGUI += dir + "/" + fileName //"open " + dir  + "/" + fileName + ".app"
	}
}

type ExternalGUI struct {
	port uint16
}

func NewExternalGUI(context *cli.Context, close chan os.Signal) gui.EleWalletGUI {
	guiClose = close
	cmds = RPCCommands
	srv = NewGUIServer()
	ctx = context
	port := uint16(ctx.GlobalInt(flags.GuiPortFlag.Name))
	return &ExternalGUI{port}
}

func (ui *ExternalGUI) Init(langEN bool, e gui.GUIBackend) {
	if e == nil {
		logCrit("There is a nil pointer to elephant backend")
	}

	ele = e
}

func cmdLog(cmd *exec.Cmd) {
	logError("Cmd log start")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Error("Stdout pipe", "err", err)
		return
	}
	go func() {
		reader := bufio.NewReader(stdout)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				break
			}
			fmt.Println(line)
		}
		logError("Cmd log exit")
	}()
}

func (ui *ExternalGUI) Show() {
	setExternGUI(ui.port)
	portStr := strconv.Itoa(int(ui.port))
	go srv.Serve(portStr)

	//var gui *exec.Cmd
	//if runtime.GOOS == `darwin` {
	//	gui = exec.Command("/bin/sh", "-c", externGUI, portStr)
	//} else {
	//	gui = exec.Command(externGUI, portStr)
	//}

	// cmdLog(gui)

	gui := exec.Command(externGUI, portStr)
	err := gui.Start()
	if err != nil {
		logCrit(err.Error())
	}
}

func (ui *ExternalGUI) SetLogger(log log.Logger) {
	uiLog = log
}

func logInfo(msg string, ctx ...interface{}) {
	log.Info(msg, ctx...)
	if uiLog != nil {
		uiLog.Info(msg, ctx...)
	}
}

func logError(msg string, ctx ...interface{}) {
	log.Error(msg, ctx...)
	if uiLog != nil {
		uiLog.Error(msg, ctx...)
	}
}

func logCrit(msg string, ctx ...interface{}) {
	if uiLog != nil {
		uiLog.Error(msg, ctx...)
	}
	log.Crit(msg, ctx...)
}

func Close() {
	time.Sleep(time.Millisecond * 500)
	guiClose <- os.Interrupt
}

func showAccounts() string {
	for ele == nil {
		utils.SleepDefault()
	}
	accs := ele.GetAccAddresses()
	if len(accs) != 0 {
		strAccounts := ""
		for i, acc := range accs {
			strAccounts += acc
			if i != len(accs)-1 {
				strAccounts += Spec
			}
		}
		return strAccounts
	} else {
		return SysMessage[noAccountMsg]
	}
}

func setDataDir(pDir []string) string {
	dir := ""
	if len(pDir) == 0 {
		return SysMessage[errorMsg]
	} else {
		dir = pDir[0]
	}

	ctx.Set(flags.DataDirFlag.GetName(), dir)
	DataDirSetFlag = true
	return SysMessage[okMsg]
}

func doOperation(cmd int, paramsIn []string) string {
	var (
		err          error
		ret          interface{}
		params       []interface{}
		transferRate chan int
	)

	if ele == nil {
		logCrit("backend is nil", "operation", RPCCommands[cmd].Cmd)
	}

	params, err = parseAndDo(cmd, paramsIn)
	if err != nil {
		logError(err.Error())
		return SysMessage[errorMsg]
	}

	if cmd == blizWriteFileCmd || cmd == blizReadFileCmd {
		if cmd == blizWriteFileCmd {
			fData, _, err := getFile(paramsIn[len(paramsIn)-1])
			if err != nil {
				logError(fmt.Sprintf("Can not get the file: %s", paramsIn[len(paramsIn)-1]))
				return SysMessage[errorMsg]
			}
			params = append(params[:len(params)-1], &fData)
		}

		// Show file upload rate through a go routine
		transferRate = make(chan int)
		//initRateSending()
		go showTransferRate(transferRate)
		ret, err = ele.DoOperation(RPCCommands[cmd].Cmd, params, transferRate)
	} else {
		ret, err = ele.DoOperation(RPCCommands[cmd].Cmd, params, nil)
	}
	if err != nil {
		logError(err.Error())
		return SysMessage[errorMsg]
	} else {
		//if cmd != getAgentDetailsCmd && cmd != getAccTBalanceCmd {
		//	uiLog.logInfo(fmt.Sprintf("ret = %s", parseResult(ret, 0, RPCCommands[cmd].BaseOut)))
		//}
		return parseResult(ret, 0, RPCCommands[cmd].BaseOut)
	}
}

func parseAndDo(cmd int, paramsIn []string) ([]interface{}, error) {
	var params []interface{}
	if len(paramsIn) != len(RPCCommands[cmd].TypeIn) {
		return nil, errors.New("operation failed for the number of parameters in parseAndDo is not correct")
	}
	for i := 0; i < len(RPCCommands[cmd].TypeIn); i += 1 {
		if val := paramsIn[i]; val != "" {
			switch RPCCommands[cmd].TypeIn[i] {
			case "string":
				params = append(params, val)
			case "int":
				if valT, err := strconv.Atoi(val); err != nil {
					return nil, err
				} else {
					params = append(params, valT)
				}
			case "uint32":
				if valT, err := strconv.ParseUint(val, 0, 32); err != nil {
					return nil, err
				} else {
					params = append(params, uint32(valT))
				}
			case "uint64":
				if valT, err := strconv.ParseUint(val, 0, 64); err != nil {
					return nil, err
				} else {
					params = append(params, valT)
				}
			}
		} else {
			break
		}
	}

	return params, nil
}

func parseResult(ret interface{}, index int, base int) (result string) {
	result = ""
	if index > 0 {
		result = result + strconv.FormatInt(int64(index), 10) + " : "
	} else {
		result = result // + "\n"
	}
	switch vv := ret.(type) {
	case string:
		if base != 0 {
			val, _ := new(big.Int).SetString(vv, 0)
			balance := val.String()
			const weiPos = 19
			intergerNum := "0"
			floatNum := ""
			if len(balance) >= weiPos {
				offset := len(balance) - weiPos + 1
				intergerNum = string(balance[:offset])
				if len(intergerNum) == 0 {
					intergerNum = "0"
				}
				floatNum = "." + strings.TrimRight(string(balance[offset:offset+6]), "0")

			} else {
				floatCnt := 6
				if weiPos-len(balance)-1 < floatCnt {
					floatNum = strings.TrimRight(balance, "0")
					for i := 0; i < weiPos-len(balance)-1; i++ {
						floatNum = "0" + floatNum
					}
					floatNum = "." + floatNum[:floatCnt]
				}
			}

			if len(floatNum) > 1 {
				balance = intergerNum + floatNum
			} else {
				balance = intergerNum
			}
			result = result + balance
		} else {
			result = result + vv
		}
	case []string:
		for i, u := range vv {
			parseResult(u, i+1, base)
		}
	case int:
		result = result + strconv.FormatInt(int64(vv), base)
	case []interface{}:
		for i, u := range vv {
			parseResult(u, i+1, base)
		}
	case bool:
		if vv {
			result = result + SysMessage[okMsg]
		} else {
			result = result + SysMessage[errorMsg]
		}
	default:
		result = result + "unknown type/null"
	}
	if index == 0 {
		result = result // + "\n"
	}

	return result
}

func getFile(filePath string) (multipart.File, uint64, error) {
	f, err := os.OpenFile(filePath, os.O_RDONLY, 0600)
	if err != nil {
		logError(err.Error())
		return nil, 0, err
	} else {
		var size uint64
		err = filepath.Walk(filePath, func(path string, f os.FileInfo, err error) error {
			size = uint64(f.Size())
			return nil
		})
		if err != nil {
			logError(err.Error())
			return nil, 0, err
		}
		return f, size, nil
	}
}

func showTransferRate(rate chan int) {
	for {
		r, ok := <-rate
		if !ok {
			logInfo("Show transfer rate, but rate chan closed")
			return
		}
		if r >= 100 || r < 0 {
			if r >= 100 {
				sendRate(int32(100))
			} else {
				sendRate(int32(0))
			}
			//closeRateSending()
			break
		}
		sendRate(int32(r))
	}
}
