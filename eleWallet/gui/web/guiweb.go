package webgui

import (
	"fmt"
	"github.com/contatract/go-contatract/eleWallet/gui"
	"github.com/contatract/go-contatract/log"
	"io"
	"math/big"
	"net/http"
	"strconv"
)

type WebGUI struct {
	result   string
	accounts []string
	ele      gui.GUIBackend
	passwd   string
	langEN   bool
	cmds     map[string]RPCCommand
}

var strWalletPort = ":8086"

func NewWebGUI() gui.EleWalletGUI {
	return &WebGUI{
		result: "",
	}
}

func (ui *WebGUI) Init(langEN bool, e gui.GUIBackend) {
	if e == nil {
		log.Crit("There is a nil pointer to elephant backend")
	}

	ui.ele = e
	ui.langEN = langEN
	g_LangIsEN = langEN
	initStrings()
	if langEN {
		ui.cmds = RPCCommandsEN
	} else {
		ui.cmds = RPCCommandsCN
	}
	accs := e.GetAccAddresses()
	//params := []interface{}{ui.passwd}
	if len(accs) != 0 {
		fmt.Println("Accounts:")
		for i, acc := range e.GetAccAddresses() {
			if i == 0 {
				if !langEN {
					ui.result = fmt.Sprintf("请先输入该账户的密码: \n%s", acc)
				} else {
					ui.result = fmt.Sprintf("Please enter the passphrase of the key first: \n%s", acc)
				}
				//if !ui.ele.SetAccoutInUse(acc, params[0].(string)) {
				//	log.Error("The passphrase is not correct, please enter later")
				//}
			}
			ui.accounts = append(ui.accounts, acc)
			fmt.Println(acc)
		}
	} else {
		if !langEN {
			ui.result = "请先导入秘钥"
		} else {
			ui.result = "Please import an account first"
		}
	}

	//else {
	//	postType := "personal_newAccount"
	//	res, err := ui.ele.DoOperation(postType, params, nil)
	//	if err != nil {
	//		log.Info(err.Error())
	//		ui.showResult(err.Error(), 0, 0)
	//		return
	//	}
	//
	//	// Save and show the result
	//	ui.saveResult(res, postType, nil)
	//	ui.showResult(res, 0, RPCCommands[postType].baseOut)
	//	ui.ele.setBaseAddress(res.(string))
	//	ui.ele.setBasePassphrase(params)
	//	ui.ele.SetAccoutInUse(res.(string))
	//}
}

func (ui *WebGUI) Show() {
	http.HandleFunc("/eleWallet", ui.walletHandler)
	desc := "Started the API server for interaction, please visit http://localhost" + strWalletPort + "/eleWallet"
	log.Info(desc)
	err := http.ListenAndServe(strWalletPort, nil)
	if err != nil {
		log.Crit(err.Error())
	}
}

func (ui *WebGUI) SetLogger(log log.Logger) {

}

func (ui *WebGUI) getAllResult() string {
	return ui.result
}

func (ui *WebGUI) walletHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		s1, s2 := getWebShowString()
		io.WriteString(w, s1+ui.getAllResult()+s2)
	}

	if r.Method == "POST" {
		postType := r.FormValue("type")
		defer http.Redirect(w, r, "/eleWallet", http.StatusFound)
		if postType == "" {
			return
		}
		params, errP := getParamsByType(r, postType)
		if errP != nil {
			log.Info(errP.Error())
			ui.showResult("Error, the input parameters are not correct", 0, ui.cmds[postType].baseOut)
			return
		}
		//post := generatePost(postType, getParams(r))
		//var jsonStr = []byte(post)
		//fmt.Println("request:", postType, "\n")
		//fmt.Println("post:", post)

		// Set the password first
		if postType == "webgui_setPasswd" {
			ui.passwd = params[0].(string)
			if !ui.ele.SetAccoutInUse(ui.accounts[0], params[0].(string)) {
				if ui.langEN {
					ui.result = "The passphrase is not correct, please re-enter later"
				} else {
					ui.result = "密码错误，请再次输入密码"
				}
			} else {
				if ui.langEN {
					ui.result = "The passphrase entered correctly"
				} else {
					ui.result = "密码输入正确"
				}
			}
			return
		} else {
			if ui.passwd == "" && len(ui.accounts) > 0 {
				if !ui.langEN {
					ui.result = fmt.Sprintf("请先输入该账户的密码: \n%s", ui.accounts[0])
				} else {
					ui.result = fmt.Sprintf("Please enter the passphrase of the key first: \n%s", ui.accounts[0])
				}
				return
			}
		}

		// Deal with the file uploaded
		f, h, err := r.FormFile("fileToUpload")
		var uploadRate chan int = nil
		if postType == "blizcs_writeFile" {
			if err != nil {
				ui.showResult("Error, no file selected", 0, ui.cmds[postType].baseOut)
				return
			} else {
				defer f.Close()
				//fmt.Println("The file to upload is:", h.Filename)

				// Show file upload rate through a go routine
				uploadRate = make(chan int)
				go ui.showUploadRate(uploadRate)

				params = append(params, uint64(h.Size))
				params = append(params, &f)

			}
		} else if postType == "blizcs_readFile" {
			uploadRate = make(chan int)
			go ui.showUploadRate(uploadRate)

			params = append(params, ui.passwd)
		}

		// Call functions
		var res interface{}
		ui.result = ""
		res, err = ui.ele.DoOperation(postType, params, uploadRate)
		if err != nil {
			log.Info(err.Error())
			ui.showResult(err.Error(), 0, 0)
			return
		}

		// Save and show the result
		ui.saveResult(res, postType, params)
		ui.showResult(res, 0, ui.cmds[postType].baseOut)
	}
}

func (ui *WebGUI) showUploadRate(uploadRate chan int) {
	for {
		r := <-uploadRate
		log.Info(fmt.Sprintf("File transfering: %d percent completed", r))
		if r >= 100 || r < 0 {
			close(uploadRate)
			break
		}
	}
}

func (ui *WebGUI) uploadFile(postType string, params []interface{}, uploadRate chan int) bool {
	res, errUp := ui.ele.DoOperation(postType, params, uploadRate)
	if errUp != nil {
		ui.showResult(fmt.Sprintf("Error, %s", errUp.Error()), 0, ui.cmds[postType].baseOut)
		return false
	} else {
		if ret, ok := res.(bool); ok {
			if !ret {
				ui.showResult("Error, the file selected failed to upload", 0, ui.cmds[postType].baseOut)
				return false
			}
		} else {
			ui.showResult("Error, the file selected upload function crashed", 0, ui.cmds[postType].baseOut)
			return false
		}
	}

	return true
}

func (ui *WebGUI) showResult(ret interface{}, index int, base int) {
	if index > 0 {
		ui.result = ui.result + strconv.FormatInt(int64(index), 10) + " : "
	} else {
		ui.result = ui.result + "\n"
	}
	switch vv := ret.(type) {
	case string:
		if base != 0 {
			val, _ := new(big.Int).SetString(vv, 0)
			balance := val.String()
			const weiPos = 18
			if len(balance) >= weiPos {
				tmp := []byte(balance)
				offset := len(balance) - weiPos
				balance = string(tmp[:offset]) + "," + string(tmp[offset:])
			}
			ui.result = ui.result + balance + "\n"
		} else {
			ui.result = ui.result + vv + "\n"
		}
	case []string:
		for i, u := range vv {
			ui.showResult(u, i+1, base)
		}
	case int:
		ui.result = ui.result + strconv.FormatInt(int64(vv), base) + "\n"
	case []interface{}:
		for i, u := range vv {
			ui.showResult(u, i+1, base)
		}
	case bool:
		if vv {
			ui.result = ui.result + "true" + "\n"
		} else {
			ui.result = ui.result + "false" + "\n"
		}
	default:
		ui.result = ui.result + "unknown type/null" + "\n"
	}
	if index == 0 {
		ui.result = ui.result + "\n"
	}
}

func (ui *WebGUI) saveResult(ret interface{}, typeInput string, params []interface{}) {
	switch typeInput {
	case "personal_newAccount":
		ui.accounts = append(ui.accounts, ret.(string))
	case "eth_accounts":
		if accounts, ok := ret.([]string); ok {
			ui.accounts = accounts
		}
	case "guiback_importKey":
		ui.accounts = append(ui.accounts, ret.(string))
		ui.ele.SetAccoutInUse(ret.(string), params[1].(string))
	}
}

func getParamsByType(r *http.Request, typeInput string) ([]interface{}, error) {
	var params []interface{}
	strName := []string{"param1", "param2", "param3", "param4"}
	for i := 0; i < len(RPCCommandsEN[typeInput].typeNameIn) && i < 4; i += 1 {
		if val := r.FormValue(strName[i]); val != "" {
			switch RPCCommandsEN[typeInput].typeNameIn[i] {
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
