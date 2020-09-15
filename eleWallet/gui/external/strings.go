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

const (
	programTitle = iota
	accountsLabel
	newAccountBtn
	getBalanceBtn
	getSignedCsBtn
	signCsBtn
	cancelCsBtn
	importPrivkeyBtn
	spaceLabel
	createObjBtn
	addObjStorageBtn
	showSpaceLabel
	getObjInfoCmd
	getRentSizeBtn
	spaceOpLabel
	blizWriteBtn
	blizWriteFileBtn
	selectFileBtn
	selectFileDlg
	blizReadFileBtn
	statusLabel
	clearBtn
	remoteIPCmd
	exitBtn
	confirmBtn
	cancelBtn
	needAccounts
	needAccountpwd
	wrongPassword
	paramsDlgTitle
	errParamInput
	waitForFeedback

	showAccountsCmd
	getAgentsInfoCmd
	setDataDirCmd
	exitExternGUICmd
	newAccountCmd
	setAccountInUseCmd
	blizReadCmd
	blizReadFileCmd
	blizWriteCmd
	blizWriteFileCmd
	addObjStorageCmd
	getCurAgentAddrCmd
	getAgentDetailsCmd
	getAccTBalanceCmd
	getAccBalanceCmd
	getTransactionCmd
	signAgentCmd
	cancelAgentCmd
	waitTXCompleteCmd
	getSharingCodeCmd
	getSignatureCmd
	checkSignatureCmd
	payForSharedFileCmd
	sendTransactionCmd
	sendMailCmd
	getMailsCmd

	okMsg
	errorMsg
	opRspMsg
	rateMsg
	noAccountMsg
)

type RPCCommand struct {
	Cmd     string
	TypeIn  []string
	BaseOut int
}

var RPCCommands = map[int]RPCCommand{
	showAccountsCmd:     {"ele_accounts", []string{}, 0},
	getAgentsInfoCmd:    {"gui_getAgentsInfo", []string{}, 0},
	setDataDirCmd:       {"gui_setDataDir", []string{}, 0},
	exitExternGUICmd:    {"gui_close", []string{}, 0},
	newAccountCmd:       {"personal_newAccount", []string{"string"}, 0},
	setAccountInUseCmd:  {"gui_setAccountInUse", []string{"string", "string"}, 0},
	remoteIPCmd:         {"gui_setRemoteIP", []string{"string"}, 0},
	getObjInfoCmd:       {"blizcs_getObjectsInfo", []string{"string"}, 0},
	blizReadCmd:         {"blizcs_read", []string{"uint64", "uint64", "uint64", "string"}, 0},
	blizReadFileCmd:     {"blizcs_readFile", []string{"uint64", "uint64", "uint64", "string", "string", "string", "string", "string"}, 0},
	blizWriteCmd:        {"blizcs_write", []string{"uint64", "uint64", "string", "string"}, 0},
	blizWriteFileCmd:    {"blizcs_writeFile", []string{"uint64", "uint64", "string", "string", "uint64", "string"}, 0},
	createObjBtn:        {"blizcs_createObject", []string{"uint64", "uint64", "string"}, 0},
	addObjStorageCmd:    {"blizcs_addObjectStorage", []string{"string", "string", "string"}, 0},
	getCurAgentAddrCmd:  {"gui_getCurAgentAddress", []string{}, 0},
	getAgentDetailsCmd:  {"elephant_getSignedCs", []string{"string"}, 0},
	getAccTBalanceCmd:   {"elephant_getTotalBalance", []string{}, 10},
	getAccBalanceCmd:    {"elephant_getBalance", []string{"string"}, 10},
	getTransactionCmd:   {"elephant_checkTx", []string{"string"}, 0},
	signAgentCmd:        {"elephant_signCs", []string{"string", "string", "uint64", "string"}, 0},
	cancelAgentCmd:      {"elephant_cancelCs", []string{"string", "string"}, 0},
	waitTXCompleteCmd:   {"gui_waitTx", []string{"string"}, 0},
	getSharingCodeCmd:   {"gui_getSharingCode", []string{"uint64", "string", "uint64", "uint64", "string", "string", "string", "string", "string", "string"}, 0},
	getSignatureCmd:     {"gui_getSignature", []string{"string", "string"}, 0},
	checkSignatureCmd:   {"gui_checkSignature", []string{"string"}, 0},
	payForSharedFileCmd: {"gui_payForSharedFile", []string{"string", "string"}, 0},
	sendTransactionCmd:  {"elephant_sendTransaction", []string{"string", "string", "string", "string"}, 0},
	sendMailCmd:         {"elephant_sendMail", []string{"string", "string"}, 0},
	getMailsCmd:         {"elephant_getMails", []string{"string"}, 0},
}

var RPCStrCommands = map[string]int{}

var SysMessage = map[int]string{
	okMsg:        "OK",
	errorMsg:     "Error",
	opRspMsg:     "OpRsp",
	rateMsg:      "Rate",
	noAccountMsg: "No any accounts",
}

func init() {
	for i, rpcCmd := range RPCCommands {
		RPCStrCommands[rpcCmd.Cmd] = i
	}
}
