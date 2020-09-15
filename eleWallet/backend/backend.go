package backend

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"math/big"
	"mime/multipart"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"gopkg.in/urfave/cli.v1"

	"github.com/contatract/go-contatract/accounts"
	"github.com/contatract/go-contatract/accounts/keystore"
	"github.com/contatract/go-contatract/blizcore"
	"github.com/contatract/go-contatract/blizparam"
	"github.com/contatract/go-contatract/blizzard/storage"
	"github.com/contatract/go-contatract/blizzard/storagecore"
	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/common/datacrypto"
	"github.com/contatract/go-contatract/crypto"
	"github.com/contatract/go-contatract/eleWallet/flags"
	externgui "github.com/contatract/go-contatract/eleWallet/gui/external"
	"github.com/contatract/go-contatract/eleWallet/node"
	"github.com/contatract/go-contatract/eleWallet/utils"
	"github.com/contatract/go-contatract/ftransfer"
	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/params"
	"github.com/contatract/go-contatract/rlp"
)

const (
	CharSpace               = ' '
	CharEqual               = '='
	StringSpace             = " "
	confirmTxCountMax uint8 = 5
	agentListNet            = "https://www.contatract.org/agentlist"
)

type Elephant struct {
	ctx    *cli.Context
	rpcCli *EthClient
	node   *node.Node
	am     *accounts.Manager
	accUse string

	pwdFlag      bool
	remoteIPFlag bool
	initConnFlag bool
	errorCh      chan string
	signedCs     *common.WalletCSAuthData
	usedFlow     uint64

	log log.Logger `toml:",omitempty"`
}

type paramType struct {
	types  []string
	passwd int
}

type csJson struct { // qiwy: todo, need delete
	CsAddr        string `json:"csAddr"`
	Start         string `json:"start"`
	End           string `json:"end"`
	Flow          string `json:"flow"`
	AuthAllowTime string `json:"authAllowTime"`
	CsPickupTime  string `json:"csPickupTime"`
	AuthAllowFlow string `json:"authAllowFlow"`
	UsedFlow      string `json:"usedFlow"`
	PayMethod     string `json:"payMethod"`
}

type csData struct { // qiwy: todo, need delete
	CsAddr        common.Address `json:"csAddr"`
	Start         uint64         `json:"start"`
	End           uint64         `json:"end"`
	Flow          uint64         `json:"flow"`
	CsPickupTime  uint64         `json:"csPickupTime"`
	AuthAllowFlow uint64         `json:"authAllowFlow"`
	UsedFlow      uint64         `json:"usedFlow"`
	PayMethod     uint8          `json:"payMethod"`
}

var paramTypes = map[string]paramType{
	//"elephant_walletTx":        {[]string{"uint32", "string"}, -1},
	"gui_setAccountInUse":      {[]string{"string", "string"}, 1},
	"gui_getAgentsInfo":        {[]string{}, -1},
	"personal_newAccount":      {[]string{"string"}, 0},
	"eth_accounts":             {[]string{}, -1},
	"elephant_getBalance":      {[]string{"string"}, -1},
	"elephant_getTotalBalance": {[]string{}, -1},
	"elephant_getSignedCs":     {[]string{"string"}, 0},
	"elephant_signCs":          {[]string{"string", "string", "uint64", "string"}, 3},
	"elephant_cancelCs":        {[]string{"string", "string"}, 1},
	"guiback_importKey":        {[]string{"string", "string"}, 1},
	"blizcs_getRentSize":       {[]string{"uint64"}, -1},
	"blizcs_createObject":      {[]string{"uint64", "uint64", "string"}, 2},
	"blizcs_addObjectStorage":  {[]string{"uint64", "uint64", "string"}, 2},
	"elephant_checkTx":         {[]string{"string"}, -1},
	"elephant_checkReceipt":    {[]string{"string"}, -1},
	"blizcs_write":             {[]string{"uint64", "uint64", "string", "string"}, 3},
	"blizcs_read":              {[]string{"uint64", "uint64", "uint64", "string"}, 3},
	"blizcs_writeFile":         {[]string{"uint64", "uint64", "string", "string", "uint64", "*multipart.File"}, 3},
	"blizcs_readFile":          {[]string{"uint64", "uint64", "uint64", "string", "string", "string", "string", "string"}, 7},
	"blizcs_getObjectsInfo":    {[]string{"string"}, 0},
	"gui_getCurAgentAddress":   {[]string{}, -1},
	"gui_waitTx":               {[]string{"string"}, -1},
	"gui_getSharingCode":       {[]string{"uint64", "string", "uint64", "uint64", "string", "string", "string", "string", "string", "string"}, 9},
	"gui_getSignature":         {[]string{"string", "string"}, 1},
	"gui_checkSignature":       {[]string{"string"}, -1},
	"gui_payForSharedFile":     {[]string{"string", "string"}, 1},
	"elephant_sendTransaction": {[]string{"string", "string", "string", "string"}, 3},
	"elephant_sendMail":        {[]string{"string", "string"}, 1},
	"elephant_getMails":        {[]string{"string"}, 0},
}

func New(n *node.Node, ctx *cli.Context) *Elephant {
	var RemoteIP = false
	if ctx.GlobalIsSet(flags.RemoteIPFlag.Name) {
		RemoteIP = ctx.GlobalString(flags.RemoteIPFlag.Name) != ""
	}

	elephant := &Elephant{
		ctx:          ctx,
		node:         n,
		am:           n.GetAccMan(),
		rpcCli:       nil,
		pwdFlag:      false,
		remoteIPFlag: RemoteIP,
		initConnFlag: false,
		errorCh:      make(chan string),
		log:          log.New(),
	}

	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Crit(err.Error())
	}
	fileName := "wemore.log"
	filePath := ""
	if runtime.GOOS == `windows` {
		filePath += dir + "\\" + fileName
	} else if runtime.GOOS == `linux` {
		filePath += dir + "/" + fileName
	} else if runtime.GOOS == `darwin` {
		dir = getParentDirectory(getParentDirectory(getParentDirectory(dir)))
		filePath += dir + "/" + fileName
	}
	if ok, _ := pathExists(filePath); ok {
		bak := filePath + ".bak"
		if okBak, _ := pathExists(bak); okBak {
			os.Remove(bak)
		}
		os.Rename(filePath, bak)
	}

	fileHandle := log.LvlFilterHandler(log.LvlInfo, log.GetDefaultFileHandle(filePath))
	elephant.log.SetHandler(log.MultiHandler(fileHandle))

	return elephant
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func substr(s string, pos, length int) string {
	runes := []rune(s)
	l := pos + length
	if l > len(runes) {
		l = len(runes)
	}
	return string(runes[pos:l])
}

func getParentDirectory(dirctory string) string {
	spec := "\\"
	if runtime.GOOS != `windows` {
		spec = "/"
	}
	return substr(dirctory, 0, strings.LastIndex(dirctory, spec))
}

func (e *Elephant) Log() log.Logger {
	return e.log
}

func (e *Elephant) logInfo(msg string, ctx ...interface{}) {
	log.Info(msg, ctx...)
	e.log.Info(msg, ctx...)
}

func (e *Elephant) logError(msg string, ctx ...interface{}) {
	log.Error(msg, ctx...)
	e.log.Error(msg, ctx...)
}

func (e *Elephant) logCrit(msg string, ctx ...interface{}) {
	e.log.Error(msg, ctx...)
	log.Crit(msg, ctx...)
}

func (e *Elephant) Init() ([]byte, error) {
	// Init the connection to remote
	var err error
	if !e.initConnFlag {
		err = e.initConnection("")
		if err != nil {
			return nil, err
		}
	}

	// Get the node ID of the remote device
	var nodeId []byte
	nodeId, err = e.rpcCli.NodeID(context.Background())
	if err != nil {
		return nil, err
	}

	return nodeId, nil
}

func (e *Elephant) initConnection(remoteIP string) error {
	if remoteIP == "" {
		remoteIP = e.ctx.GlobalString(flags.RemoteIPFlag.Name)
	}
	if strings.Compare(remoteIP, "") == 0 {
		return errors.New("need specify the remote ip by parameter: --ip \"xxx.xxx.xxx.xxx\"")
	}

	// Establish a connection to the server's node
	rpcPort := e.ctx.GlobalUint(flags.RemoteRPCPortFlag.Name)
	remoteAddr := "http://" + remoteIP + ":" + strconv.Itoa(int(rpcPort))
	client, err := NewEthClient(remoteAddr)
	if err != nil {
		return err
	}

	// Get the network ID
	netId := e.ctx.GlobalUint64(flags.NetworkIdFlag.Name)
	id, err := client.NetworkID(context.Background())
	if err != nil {
		return err
	} else {
		if netId != id.Uint64() {
			e.ctx.GlobalSet(flags.NetworkIdFlag.Name, strconv.FormatUint(id.Uint64(), 10))
		}
	}
	e.logInfo(fmt.Sprintf("The network id is: %d", id.Uint64()))
	e.logInfo(fmt.Sprintf("The agent server's ip is: %s", remoteIP))
	e.rpcCli = client

	e.initConnFlag = true

	return nil
}

func (e *Elephant) DoOperation(cmd string, params []interface{}, rate chan int) (interface{}, error) {
	// if e.node.FTransAPI != nil {
	// 	if pm := e.node.FTransAPI.GetProtocolMan(); pm != nil {
	// 		pm.SetErrorMsgChan(e.errorCh)
	// 	}
	// }

	switch cmd {
	case "gui_setRemoteIP":
		return e.setRemoteIP(params)

	case "gui_setAccountInUse":
		return e.SetAccoutInUse2(cmd, params)

	case "gui_getAgentsInfo":
		return e.GetAgentsInfo()

	case "gui_typePassword":
		return e.setBasePassphrase(params), nil

	case "personal_newAccount":
		return e.NewAccount(params)

	case "eth_accounts":
		return e.GetAccAddresses(), nil

	case "elephant_getBalance":
		return e.GetBalance(params)

	//case "elephant_getTotalBalance":
	//	return e.GetTotalBalance()

	case "elephant_getSignedCs":
		return e.GetSignedCs(cmd, params, e.accUse)

	case "elephant_signCs":
		return e.SignCs(cmd, params)

	case "elephant_cancelCs":
		return e.CancelCs(cmd, params)

	case "guiback_importKey":
		return e.ImportKey(cmd, params)

	case "blizcs_getRentSize":
		return e.rpcCli.GetRentSize(params, e.accUse)

	case "blizcs_getObjectsInfo":
		return e.rpcCli.GetObjectsInfo(params, e.accUse)

	case "blizcs_createObject":
		return e.CreateObjectStorage(cmd, params)

	case "blizcs_addObjectStorage":
		return e.CreateObjectStorage(cmd, params)

	case "gui_getCurAgentAddress":
		return e.GetCSAddress()

	case "elephant_checkTx":
		return e.CheckTx(cmd, params)

	case "elephant_checkReceipt":
		return e.CheckReceipt(cmd, params)

	//case "elephant_walletTx":
	//	return e.WalletTx(cmd, params)

	case "blizcs_write":
		return e.WriteString(cmd, params)

	case "blizcs_read":
		return e.ReadString(cmd, params)

	case "blizcs_writeFile":
		return e.WriteFile(cmd, params, rate)

	case "blizcs_readFile":
		return e.ReadFile(cmd, params, rate)

	case "gui_waitTx":
		return e.WaitTXComplete(cmd, params)

	case "gui_getSharingCode":
		return e.getSharingCode(cmd, params)

	case "gui_getSignature":
		return e.getSignature(cmd, params)

	case "gui_checkSignature":
		return e.checkSignature(cmd, params)

	case "gui_payForSharedFile":
		return e.payForSharedFile(cmd, params)

	case "elephant_sendTransaction":
		return e.SendTransaction(cmd, params)

	case "elephant_sendMail":
		return e.SendMail(cmd, params)

	case "elephant_getMails":
		return e.GetMails(cmd, params)

	case "elephant_registerMail":
		return e.RegMail(cmd, params)

	case "elephant_checkRegistered":
		return e.rpcCli.CheckReg(common.HexToAddress(e.accUse), params)
	}

	return nil, errors.New(fmt.Sprintf("the command is not supported yet: %s", cmd))
}

func (e *Elephant) GetClient() *EthClient {
	return e.rpcCli
}

// Accounts returns the collection of accounts this node manages
func (e *Elephant) GetAccouts() []accounts.Account {
	accs := make([]accounts.Account, 0) // return [] instead of nil if empty
	for _, wallet := range e.am.Wallets() {
		return wallet.Accounts()
	}
	return accs
}

// GetAccoutInUse gets the account selected in the GUI
func (e *Elephant) GetAccoutInUse() accounts.Account {
	for _, wallet := range e.am.Wallets() {
		for _, account := range wallet.Accounts() {
			if strings.Compare(e.accUse, account.Address.String()) == 0 {
				return account
			}
		}
	}

	return accounts.Account{}
}

// GetBalance - returns the balance for this account
func (e *Elephant) GetBalance(params []interface{}) (string, error) {
	if len(params) == 1 {
		if address, ok := params[0].(string); ok {
			destAccount := common.HexToAddress(address)

			//go func(acc common.Address) {
			result, err := e.rpcCli.BalanceAt(context.Background(), destAccount, nil)
			if err != nil {
				return "", err
			} else {
				return result.String(), nil
			}
			//}(destAccount)
			//return "", nil
		}
	}

	return "", errors.New("error: The inputs are not correct")
}

//// GetTotalBalance - returns the total balance for all accounts available
//func (e *Elephant) GetTotalBalance() (string, error) {
//	var value, _ = new(big.Int).SetString("0", 0)
//	for _, wallet := range e.am.Wallets() {
//		for _, account := range wallet.Accounts() {
//			valStr, err := e.GetBalance([]interface{}{account.Address.String()})
//			if err != nil {
//				return "", err
//			}
//			balance, _ := new(big.Int).SetString(valStr, 0)
//			value = value.Add(value, balance)
//		}
//	}
//
//	return value.String(), nil
//}

// GetCSAddress gets the agent account connected in the GUI
func (e *Elephant) GetCSAddress() (string, error) {
	if e.node.FTransAPI.GetProtocolMan() != nil {
		return e.node.FTransAPI.GetProtocolMan().GetCurrentCSAddress().String(), nil
	} else {
		return "", errors.New("there has no protocol manager")
	}
}

// GetAccAddresses returns the collection of accounts this node manages
func (e *Elephant) GetAccAddresses() []string {
	addresses := make([]string, 0) // return [] instead of nil if empty
	for _, wallet := range e.am.Wallets() {
		for _, account := range wallet.Accounts() {
			addresses = append(addresses, account.Address.String())
		}
	}
	return addresses
}

// fetchKeystore retrives the encrypted keystore from the account manager.
func fetchKeystore(am *accounts.Manager) *keystore.KeyStore {
	return am.Backends(keystore.KeyStoreType)[0].(*keystore.KeyStore)
}

// NewAccount will create a new account and returns the address for the new account.
func (e *Elephant) NewAccount(params []interface{}) (string, error) {
	if len(params) == 1 {
		if pwd, ok := params[0].(string); ok {
			acc, err := fetchKeystore(e.am).NewAccount(pwd)
			if err == nil {
				return acc.Address.String(), nil
			} else {
				return "", err
			}
		}
	}

	return "", errors.New("the inputs are not correct")
}

// ImportKey will import a new account and returns the address for the new account.
func (e *Elephant) ImportKey(postType string, params []interface{}) (string, error) {
	var key, passphrase string
	if err := e.parseParams(postType, params, &key, &passphrase); err != nil {
		return "", err
	}

	data, errd := hex.DecodeString(key)
	if errd != nil {
		return "", errd
	}
	acc, err := fetchKeystore(e.am).Import(data, passphrase, passphrase)
	if err == nil {
		return acc.Address.String(), nil
	} else {
		return "", err
	}
}

func (e *Elephant) getPeer() *ftransfer.Peer {
	ps := e.node.FTransAPI.GetProtocolMan().GetPeerSet()
	if len(ps) == 0 {
		return nil
	}
	return ps[0]
}

func (e *Elephant) ReadString(postType string, params []interface{}) (string, error) {
	p := e.getPeer()
	if p == nil {
		err := errors.New("there is no correct connection with the FileTransfer device")
		return "", err
	}

	var (
		objId, offset uint64
		len           uint64
		passwd        string
		err           error
	)
	if err := e.parseParams(postType, params, &objId, &offset, &len, &passwd); err != nil {
		return "", err
	}

	ret := ""
	ret, err = e.node.FTransAPI.ReadString(objId, offset, len, passwd, p)
	if len == 4096 {
		ret = strings.TrimRight(ret, " ")
	}
	return ret, err
}

func (e *Elephant) WriteString(postType string, params []interface{}) (bool, error) {
	p := e.getPeer()
	if p == nil {
		err := errors.New("there is no correct connection with the FileTransfer device")
		return false, err
	}

	var (
		objId, offset uint64
		data, passwd  string
		err           error
	)
	if err := e.parseParams(postType, params, &objId, &offset, &data, &passwd); err != nil {
		return false, err
	}

	ret := false
	ret, err = e.node.FTransAPI.WriteString(objId, offset, []byte(data), passwd, p)
	//err = errors.New("Error")
	return ret, err
}

func (e *Elephant) ReadFile(postType string, params []interface{}, rate chan int) (string, error) {
	var (
		objId, offset    uint64
		length           uint64
		sharerCSNodeID   string
		sharerAddr       string
		filePath, passwd string
		pubKey           []byte
		privKey          *ecdsa.PrivateKey
		sig, sigrsv      []byte
		err              error
		key              string
	)

	if err := e.parseParams(postType, params, &objId, &offset, &length, &sharerAddr, &sharerCSNodeID, &key, &filePath, &passwd); err != nil {
		return "", err
	}

	// Sign file head request
	acc := e.GetAccoutInUse()
	ks := fetchKeystore(e.am)
	if privKey, err = ks.GetKeyCopy(acc, passwd); err == nil {
		defer zeroKey(privKey)
		pubKey = crypto.CompressPubkey(&privKey.PublicKey)
		hash := utils.RlpHash([]interface{}{filePath, objId, length, pubKey}).Bytes()
		sigrsv, err = crypto.Sign(hash, privKey) // [R || S || V] format
		sig = sigrsv[:64]
		if err != nil {
			return "", err
		}
		var addr string
		var encKey []byte
		if sharerAddr == "0" {
			addr = e.accUse
			h := utils.RlpHash(privKey.D.Bytes())
			encKey = datacrypto.GetEncryptKey(datacrypto.EncryptDes, h, objId)
		} else {
			addr = sharerAddr
			encKey = []byte(key)
		}
		k := common.KeyID(common.HexToAddress(addr), objId)
		e.node.FTransAPI.SetEncryptKey(k, encKey)
	}

	// Get the peer used to connect to the FileTransfer device
	if p := e.getPeer(); p != nil {
		return e.node.FTransAPI.ReadFile(objId, offset, length, filePath, pubKey, sig, p, rate, sharerCSNodeID, sharerAddr)
	} else {
		return "", errors.New("there is no correct connection with the FileTransfer device")
	}
}

func (e *Elephant) WriteFile(postType string, params []interface{}, rate chan int) (bool, error) {
	var (
		objId, offset    uint64
		fileName, passwd string
		fileSize         uint64
		data             *multipart.File
		pubKey           []byte
		privKey          *ecdsa.PrivateKey
		sig, sigrsv      []byte
		err              error
	)
	if err := e.parseParams(postType, params, &objId, &offset, &fileName, &passwd, &fileSize, &data); err != nil {
		return false, err
	}
	_, fName := filepath.Split(fileName)

	// Only for test
	//file, err := os.Create(fName)
	//if err != nil {
	//	fmt.Println(err.Error())
	//}
	//FileRead:
	//for {
	//	buff := make([]byte, 64*1024)
	//	fileBuffer := bufio.NewReader(*data)
	//	rsize, rerr := fileBuffer.Read(buff)
	//	if rerr != nil {
	//		if rerr != io.EOF {
	//			break FileRead
	//		} else {
	//			fmt.Println(rerr.Error())
	//		}
	//	}
	//
	//	_, err2 := file.Write(buff[:rsize])
	//	if err2 != nil {
	//		fmt.Println(err2.Error())
	//	}
	//}

	// Sign file head request
	acc := e.GetAccoutInUse()
	ks := fetchKeystore(e.am)
	if privKey, err = ks.GetKeyCopy(acc, passwd); err == nil {
		defer zeroKey(privKey)
		pubKey = crypto.CompressPubkey(&privKey.PublicKey)
		hash := utils.RlpHash([]interface{}{fName, objId, fileSize, pubKey}).Bytes()
		sigrsv, err = crypto.Sign(hash, privKey)
		if err != nil {
			return false, err
		}
		sig = sigrsv[:64] //append(sig[:64], sig[65:]...) // [R || S || V] format to [R || S] format
	} else {
		return false, err
	}

	// Get the peer used to connect to the FileTransfer device
	if p := e.getPeer(); p != nil {
		return e.node.FTransAPI.WriteFile(data, fName, objId, offset, uint64(fileSize), pubKey, sig, p, rate)
	} else {
		return false, errors.New("there is no correct connection with the FileTransfer device")
	}
}

// parseParams parses the parameters
func (e *Elephant) parseParams(postType string, params []interface{}, args ...interface{}) error {
	types := paramTypes[postType].types
	panic := "Parsing the parameter "
	var err error
	defer func() {
		if err := recover(); err != nil {
			err = errors.New(panic + "in backend cashed")
		}
	}()
	if len(types) != len(params) {
		return fmt.Errorf("the count of parameters is not correct, type: %v, demand: %v", postType, len(types))
	}
	for i := 0; i < len(types); i += 1 {
		panic += fmt.Sprintf("%d ", i)
		switch types[i] {
		case "string":
			*args[i].(*string) = params[i].(string)
			if postType != "elephant_getTotalBalance" && postType != "elephant_getSignedCs" {
				if i == paramTypes[postType].passwd {
					e.logInfo(fmt.Sprintf("arg%d: ****", i))
				} else {
					e.logInfo(fmt.Sprintf("arg%d: %s", i, params[i].(string)))
				}
			}

		case "uint32":
			*args[i].(*uint32) = params[i].(uint32)
			if postType != "elephant_getTotalBalance" && postType != "elephant_getSignedCs" {
				e.logInfo(fmt.Sprintf("arg%d: %d", i, params[i].(uint32)))
			}

		case "uint64":
			*args[i].(*uint64) = params[i].(uint64)
			if postType != "elephant_getTotalBalance" && postType != "elephant_getSignedCs" {
				e.logInfo(fmt.Sprintf("arg%d: %d", i, params[i].(uint64)))
			}

		case "*multipart.File":
			*args[i].(**multipart.File) = params[i].(*multipart.File)

		default:
			return errors.New(fmt.Sprintf("the params %d is not valid", i+1))
		}
	}
	return err
}

// zeroKey zeroes a private key in memory.
func zeroKey(k *ecdsa.PrivateKey) {
	b := k.D.Bits()
	for i := range b {
		b[i] = 0
	}
}

func (e *Elephant) SendTransaction(postType string, params []interface{}) (string, error) {
	var (
		from, to string
		amount   string
		password string
	)

	if err := e.parseParams(postType, params, &from, &to, &amount, &password); err != nil {
		return "", err
	}

	value, err := utils.ConvertDouble2Wei(amount)
	if err != nil {
		e.logError("SendTransaction failed", "err", err.Error())
		return "", err
	}

	nonce, err := e.GetClient().ElePendingNonceAt(context.Background(), common.HexToAddress(from))
	if err != nil {
		e.logError("SendTransaction failed", "err", err)
		return "", err
	}

	hash, err := e.GetClient().SendTxWithNonce(from, to, password, value, e.am, blizparam.TypeActNone, nil, nonce)
	if err != nil {
		e.logError("SendTransaction failed", "err", err)
		return "", err
	}

	return hash.String(), nil
}

func txMailDataEncrypt(e *Elephant, data *common.TxMailData, passwd string) {
	ty := datacrypto.Atoi(data.EncryptType)
	key := datacrypto.GenerateKey(ty)
	acc := e.GetAccoutInUse()
	ks := fetchKeystore(e.am)
	pubKey, err := ks.GetMailPubKey(acc, passwd)
	if err != nil {
		return
	}
	data.Key = string(datacrypto.EncryptKey(pubKey, key))

	enc := func(ty uint8, str string, key []byte) string {
		ret, err := datacrypto.Encrypt(ty, []byte(str), key, true)
		if err != nil {
			panic(err)
		}
		return string(ret)
	}
	data.Title = enc(ty, data.Title, key)
	data.Content = enc(ty, data.Content, key)
	data.FileSharing = enc(ty, data.FileSharing, key)
}

func sDBMailDataDecrypt(e *Elephant, data *common.SDBMailData, passwd string) {
	ty := datacrypto.Atoi(data.EncryptType)
	key := datacrypto.GenerateKey(ty)
	acc := e.GetAccoutInUse()
	ks := fetchKeystore(e.am)
	priKey, err := ks.GetMailPriKey(acc, passwd)
	if err == nil || priKey == nil {
		return
	}
	zeroKey(priKey)
	key = datacrypto.DecryptKey(priKey, []byte(data.Key))
	data.Key = string(key)

	dec := func(ty uint8, str string, key []byte) string {
		ret, err := datacrypto.Decrypt(ty, []byte(str), key, true)
		if err != nil {
			panic(err)
		}
		return string(ret)
	}
	data.Title = dec(ty, data.Title, key)
	data.Content = dec(ty, data.Content, key)
	data.FileSharing = dec(ty, data.FileSharing, key)
}

func (e *Elephant) SendMail(postType string, params []interface{}) (string, error) {
	var (
		mailStr  string
		password string
		receiver string
	)

	if err := e.parseParams(postType, params, &mailStr, &password); err != nil {
		return "", err
	}

	mailData := common.TxMailData{}
	err := mailData.UnmarshalJson(mailStr)
	if err != nil {
		e.logError("SendMail failed", "err", err)
		return "", err
	}
	txMailDataEncrypt(e, &mailData, password)
	receiver = mailData.Receiver
	sender := e.accUse
	nonce, err := e.GetClient().ElePendingNonceAt(context.Background(), common.HexToAddress(sender))
	if err != nil {
		e.logError("Ele pending nonce at", "err", err)
		return "", err
	}

	actType := blizparam.TypeActSendMail
	actData := []byte(mailStr)
	value := blizparam.TxDefaultValue
	hash, err := e.GetClient().SendTxWithNonce(sender, receiver, password, value, e.am, actType, actData, nonce)
	if err != nil {
		return "", err
	}
	return hash.String(), nil
}

func (e *Elephant) GetMails(postType string, params []interface{}) (string, error) {
	var (
		password string
	)

	if err := e.parseParams(postType, params, &password); err != nil {
		return "", err
	}

	destAccount := common.HexToAddress(e.accUse) //"0x5f663f10f12503cb126eb5789a9b5381f594a0eb") //
	result, err := e.rpcCli.GetMailsAt(context.Background(), destAccount)
	if err != nil {
		return "", err
	}

	var mails []*common.SDBMailData
	err = json.Unmarshal([]byte(result), &mails)
	if err != nil {
		e.logError("Get mails", "err", err)
		return "", err
	}
	ret := ""
	for i, mail := range mails {
		sDBMailDataDecrypt(e, mail, password)
		data, err := json.Marshal(mail)
		if err != nil {
			e.logError("Get mails", "err", err)
			return "", err
		}
		c := len(data)
		if c >= 2 && data[0] == '[' && data[c-1] == ']' {
			data = data[1 : c-1]
		}
		if i != 0 {
			ret += "/"
		}
		ret += string(data)
	}
	return ret, nil
}

func getCreateObjectBalance(size uint64) *big.Int {
	s := size / (1024 * 1024)
	if s*(1024*1024) != size {
		s++
	}
	const MBalance = params.Ether
	a := new(big.Int).SetInt64(int64(s))
	oneCost := params.Ether
	b := new(big.Int).SetInt64(int64(oneCost))
	return new(big.Int).Mul(a, b)
}

func (e *Elephant) CreateObjectStorage(postType string, params []interface{}) (string, error) {
	const oneTimeSize = storagecore.GSize
	var (
		objId    uint64
		size     uint64
		password string
	)

	if err := e.parseParams(postType, params, &objId, &size, &password); err != nil {
		return "", err
	}

	// Check balance
	b, err := e.GetClient().BalanceAt(context.Background(), common.HexToAddress(e.accUse), nil)
	if err != nil {
		return "", err
	}
	need := getCreateObjectBalance(size)
	if b.Cmp(need) < 0 {
		str := fmt.Sprintf("Error: lack of gctt, left %v, but need %v", b.String(), need.String())
		return "", errors.New(str)
	}

	from := e.accUse
	to := blizparam.DestAddrStr
	actType := blizparam.TypeActRent
	nonce, err := e.GetClient().ElePendingNonceAt(context.Background(), common.HexToAddress(from))
	if err != nil {
		e.logError("ElePendingNonceAt", "err", err)
		return "", err
	}

	var ret string
	for i := 0; i < 1; i++ {
		// for i := 0; ; i++ {
		ord := strconv.Itoa(i + 1)
		cur := size
		if cur > oneTimeSize {
			// cur = oneTimeSize
		}

		rentData := storage.RentData{
			Size:   cur,
			ObjId:  objId,
			CSAddr: e.getSignedCSAddr(),
		}
		actData, err := rlp.EncodeToBytes(rentData)
		if err != nil {
			e.logError("Create object storage", "err", err)
			return "", err
		}

		hash, err := e.GetClient().SendTxWithNonce(from, to, password, need, e.am, actType, actData, nonce+uint64(i))
		if err != nil {
			ret += (ord + ", err: " + err.Error() + "\n")
			return "", errors.New(ret)
		}
		ret += (hash.Hex() + externgui.Spec)
		size -= cur
		if size == 0 {
			break
		}
	}
	return ret, nil
}

func (e *Elephant) CheckTx(postType string, params []interface{}) (string, error) {
	var (
		err    error
		hashId string
	)

	if err := e.parseParams(postType, params, &hashId); err != nil {
		return "", err
	}

	hash := common.HexToHash(hashId)
	tx, _, err := e.GetClient().GetEleTransaction(hash)
	var ret string
	if err == nil {
		ret = tx.String()
	}
	return ret, err
}

func (e *Elephant) CheckReceipt(postType string, params []interface{}) (string, error) {
	var (
		err    error
		hashId string
	)

	if err := e.parseParams(postType, params, &hashId); err != nil {
		return "", err
	}

	hash := common.HexToHash(hashId)
	receipt, err := e.GetClient().TransactionReceipt(context.Background(), hash)
	var ret string
	if err == nil {
		ret = receipt.String2()
	}
	return ret, err
}

func (e *Elephant) CheckTxIsPending(hashId string) (bool, error) {
	hash := common.HexToHash(hashId)
	_, pending, err := e.GetClient().GetEleTransaction(hash)
	if err == nil {
		return pending, nil
	}
	return false, err
}

func (e *Elephant) SignCs(postType string, params []interface{}) (string, error) {
	var (
		err          error
		addr         string
		startEndTime string
		flow         uint64
		password     string
		payMethod    uint8
		startTime    uint64
		endTime      uint64
	)

	if err = e.parseParams(postType, params, &addr, &startEndTime, &flow, &password); err != nil {
		return "", err
	}
	if strings.Compare(startEndTime, "0000-00-00 00:00:00~0000-00-00 00:00:00") != 0 {
		parts := strings.Split(startEndTime, "~")
		if len(parts) != 2 {
			return "", errors.New("must including start and end time")
		}
		startTime, err = common.GetUnixTime(parts[0])
		if err != nil {
			return "", errors.New("The start time format is not right")
		}
		endTime, err = common.GetUnixTime(parts[1])
		if err != nil {
			return "", errors.New("The end time format is not right")
		}
	} else {
		payMethod = 1
		if flow == 0 {
			return "", errors.New("The flow is not right")
		}
	}

	data := blizparam.AddrTimeFlow{
		Addr:      addr,
		StartTime: startTime,
		EndTime:   endTime,
		Flow:      flow,
		PayMethod: payMethod,
	}
	ret, err := rlp.EncodeToBytes(data)
	if err != nil {
		e.logError("Sign cs", "err", err)
		return "", err
	}
	user := e.accUse

	// check need balance
	result, err := e.GetClient().BalanceAt(context.Background(), common.HexToAddress(user), nil)
	if err != nil {
		return "", errors.New("BalanceAt error")
	}
	signNeed := blizparam.GetNeedBalance(startTime, endTime, flow)
	txNeed := blizparam.TxDefaultValue
	value := new(big.Int).Add(signNeed, txNeed)
	if result.Cmp(value) < 0 {
		str := fmt.Sprintf("error: lack of gctt, left %v, but sign need %v and tx need %v", result.String(), txNeed.String(), txNeed.String())
		return "", errors.New(str)
	}

	actType := blizparam.TypeActSignCs
	actData := ret
	hash, err := e.GetClient().SendTx(user, addr, password, e.am, actType, actData, value)
	if err != nil {
		return "", err
	}

	res := e.confirmTx(hash)
	if res {
		e.notifyCsAuthChange()
	}

	return hash.Hex(), nil
}

func (e *Elephant) CancelCs(postType string, params []interface{}) (string, error) {
	var (
		err      error
		addr     string
		password string
	)

	if err := e.parseParams(postType, params, &addr, &password); err != nil {
		return "", err
	}

	user := e.accUse
	actType := blizparam.TypeActCancelCs
	data := blizparam.CancelCsParam{
		CsAddr: addr,
		Now:    uint64(time.Now().Unix()),
	}
	ret, err := rlp.EncodeToBytes(data)
	if err != nil {
		e.logError("Cancel cs", "err", err)
		return "", err
	}
	actData := ret
	hash, err := e.GetClient().SendTx(user, addr, password, e.am, actType, actData, blizparam.TxDefaultValue)
	if err != nil {
		return "", err
	}

	res := e.confirmTx(hash)
	if res {
		e.notifyCsAuthChange()
	}

	return hash.Hex(), nil
}

// SetAccoutInUse sets the account selected in the GUI
func (e *Elephant) SetAccoutInUse(address, passphrase string) bool {
	if address == "" || passphrase == "" {
		return false
	}
	if e.checkPassphrase(address, passphrase) {
		e.accUse = address
		e.setBaseAddress(address)
		e.setBasePassphrase([]interface{}{passphrase})
		e.pwdFlag = true
		return e.pwdFlag
	}

	return false
}

func (e *Elephant) SetAccoutInUse2(postType string, params []interface{}) (bool, error) {
	var (
		err      error
		address  string
		password string
	)

	if err = e.parseParams(postType, params, &address, &password); err != nil {
		return false, err
	}

	if address == "" || password == "" {
		return false, errors.New("Error")
	}
	if e.checkPassphrase(address, password) {
		e.accUse = address
		e.setBaseAddress(address)
		e.setBasePassphrase([]interface{}{password})
		e.pwdFlag = true
		return true, nil
	}

	return false, errors.New("Error")
}

// setBaseAddress sets the base address of the backend's FTP
func (e *Elephant) setBaseAddress(addr string) {
	if e.node.FTransAPI != nil {
		if pm := e.node.FTransAPI.GetProtocolMan(); pm != nil {
			pm.SetBaseAddress(addr)
		}
	}
	e.node.GetConfig().BaseAddress = addr
}

// setBasePassphrase sets the base passphrase of the backend's FTP
func (e *Elephant) setBasePassphrase(data []interface{}) bool {
	passphrase := data[0].(string)
	if e.node.FTransAPI != nil {
		if pm := e.node.FTransAPI.GetProtocolMan(); pm != nil {
			pm.SetBaseAddrPasswd(passphrase)
		}
	}
	e.node.GetConfig().BasePassphrase = passphrase
	return true
}

// setRemoteIP sets the ip address of the remote
func (e *Elephant) setRemoteIP(data []interface{}) (string, error) {
	ip := data[0].(string)
	if err := e.ctx.GlobalSet(flags.RemoteIPFlag.Name, ip); err != nil {
		return "", err
	}
	err := e.initConnection(ip)
	if err != nil {
		return "", err
	}
	e.remoteIPFlag = true

	// Waiting for a ftp work
	abort := make(chan os.Signal, 1)
	signal.Notify(abort, os.Interrupt)
	for e.node.FTransAPI.GetProtocolMan() == nil {
		select {
		case <-abort:
			// User forcefully quite the console
			return "", nil
		default:
			utils.SleepDefault()
		}
	}
	i := 0
	for common.EmptyAddress(e.node.FTransAPI.GetProtocolMan().GetCurrentCSAddress()) {
		select {
		case <-abort:
			// User forcefully quite the console
			return "", nil
		default:
			utils.SleepDefault()
			i++
			if i > 360 { // 2 minutes, when it is a cs server that is reading or writing a big file, it need more time
				return "", errors.New(fmt.Sprintf("connection failed, get agent account timeout, ip = %s", ip))
			}
		}
	}
	return e.node.FTransAPI.GetProtocolMan().GetCurrentCSAddress().String(), nil
}

//checkPassphrase returns if the passphrase is wrong or not
func (e *Elephant) checkPassphrase(address string, passphrase string) bool {
	ks := fetchKeystore(e.am)
	if accs := ks.Accounts(); len(accs) != 0 {
		for i := range accs {
			if strings.Compare(accs[i].Address.String(), address) == 0 {
				_, err := ks.GetKeyCopy(accs[i], passphrase)
				if err == nil {
					return true
				} else {
					return false
				}
			}
		}
	}
	return false
}

func (e *Elephant) GetAgentsInfo() (string, error) {
	return "", errors.New("For test")
	res, _ := http.Get(agentListNet)
	defer func(resp *http.Response) {
		if resp != nil {
			resp.Body.Close()
		}
	}(res)
	if res == nil {
		return "", errors.New("get agent information failed due to bad connection")
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	errStr := "Bad Gateway" // 502 Bad Gateway
	if strings.Contains(string(body), errStr) {
		return "", errors.New(errStr)
	}
	bodyStr := strings.Replace(string(body), "&#34;", "\"", -1)
	return bodyStr, nil
}

// PassPhraseEntered return if the passphrase has been entered
func (e *Elephant) PassphraseEntered() bool {
	return e.pwdFlag
}

// RemoteIPEntered return if the remote IP has been entered
func (e *Elephant) RemoteIPEntered() bool {
	return e.remoteIPFlag
}

func analyzeString(s, substring string) string {
	var ret string
	n := strings.Index(s, substring)
	if n == -1 {
		return ret
	}
	if n > 0 {
		if []byte(s)[n-1] != CharSpace {
			return ret
		}
	}
	start := n + len(substring)
	if start+1 > len(s) {
		return ret
	}
	content := []byte(s)[start:]
	if content[0] != CharEqual {
		return ret
	}
	content = content[1:]
	end := len(content)
	n2 := strings.Index(string(content), StringSpace)
	if n2 != -1 {
		end = n2
	}
	return string(content[:end])
}

func (e *Elephant) WalletTx(postType string, params []interface{}) (string, error) {
	var (
		err       error
		txType    uint32
		txContent string
	)

	if err := e.parseParams(postType, params, &txType, &txContent); err != nil {
		return "", err
	}

	user := e.accUse
	addr := analyzeString(txContent, "to")
	passwd := analyzeString(txContent, "passwd")
	data := analyzeString(txContent, "data")
	var actData []byte
	var actType uint8
	if txType == 0 {
		actType = blizparam.TypeActSignTime
		subdata := analyzeString(txContent, "subdata")
		allowTime, err := common.GetUnixTime(data + " " + subdata)
		if err != nil {
			return "", errors.New("The sign time format is not right")
		}
		param := blizparam.SignTimeParam{
			Addr:      addr,
			AllowTime: allowTime,
		}
		ret, err := rlp.EncodeToBytes(param)
		if err != nil {
			e.logError("Wallet tx", "err", err)
			return "", err
		}
		actData = ret
	} else if txType == 1 {
		actType = blizparam.TypeActSignFlow
		allowFlow, err := strconv.Atoi(data)
		if err != nil {
			return "", errors.New("The sign flow format is not right")
		}
		param := blizparam.SignFlowParam{
			Addr:      addr,
			AllowFlow: (uint64)(allowFlow),
		}
		ret, err := rlp.EncodeToBytes(param)
		if err != nil {
			e.logError("Wallet tx", "err", err)
			return "", err
		}
		actData = ret
	} else if txType == 2 {
		actType = blizparam.TypeActPickupTime
		subdata := analyzeString(txContent, "subdata")
		pickupTime, err := common.GetUnixTime(data + " " + subdata)
		if err != nil {
			return "", errors.New("The pickup time format is not right")
		}
		param := blizparam.PickupTimeParam{
			Addr:       addr,
			PickupTime: pickupTime,
		}
		ret, err := rlp.EncodeToBytes(param)
		if err != nil {
			e.logError("Wallet tx", "err", err)
			return "", err
		}
		actData = ret
	} else if txType == 3 {
		actType = blizparam.TypeActPickupFlow
		pickupFlow, err := strconv.Atoi(data)
		if err != nil {
			return "", errors.New("The pickup flow format is not right")
		}
		param := blizparam.PickupFlowParam{
			Addr:       addr,
			PickupFlow: (uint64)(pickupFlow),
		}
		ret, err := rlp.EncodeToBytes(param)
		if err != nil {
			e.logError("Wallet tx", "err", err)
			return "", err
		}
		actData = ret
	}

	hash, err := e.GetClient().SendTx(user, addr, passwd, e.am, actType, actData, blizparam.TxDefaultValue)
	if err != nil {
		return "", err
	}
	return "\"WalletTx\", tx hash: " + hash.Hex(), nil
}

func (e *Elephant) WaitTXComplete(postType string, params []interface{}) (bool, error) {
	var (
		err    error
		txHash string
	)

	if err = e.parseParams(postType, params, &txHash); err != nil {
		return false, err
	}

	abort := make(chan os.Signal, 1)
	signal.Notify(abort, os.Interrupt)
	for i := 50; i > 0; i-- {
		select {
		case <-abort:
			// User forcefully quite the console
			e.logInfo("caught interrupt, exiting")
			return true, nil
		default:
			pending, _ := e.CheckTxIsPending(params[0].(string))
			if !pending {
				return true, nil
			}
			utils.SleepDefault()
		}
	}

	return false, errors.New("waiting for transaction timed out !")
}

func (e *Elephant) confirmTx(hash common.Hash) bool {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	count := uint8(0)

	for {
		select {
		case <-ticker.C:
			_, _, err := e.GetClient().GetEleTransaction(hash)
			if err == nil {
				return true
			}
			count++
			if count >= confirmTxCountMax {
				return false
			}
		}
	}
}

func (e *Elephant) notifyCsAuthChange() {
	p := e.getPeer()
	if p == nil {
		e.logInfo("there is no correct connection with the FileTransfer device")
		return
	}
	if err := e.node.FTransAPI.GetProtocolMan().SendClientBaseAddress(p); err != nil {
		e.logError(fmt.Sprintf("Failed to send client: %s", err.Error()))
		return
	}
}

func (e *Elephant) SignAuthAllowFlow(signer, csAddr common.Address, flow uint64) ([]byte, error) {
	pm := e.node.FTransAPI.GetProtocolMan()
	if pm == nil {
		return nil, errors.New("no pm")
	}
	acc := e.GetAccoutInUse()
	ks := fetchKeystore(e.am)
	passwd := pm.GetBaseAddrPasswd()
	privKey, err := ks.GetKeyCopy(acc, passwd)
	if err != nil {
		return nil, err
	}
	defer zeroKey(privKey)
	hash := blizcore.CommonHash([]interface{}{csAddr, flow})
	sigrsv, err := blizcore.CommonSign(hash, privKey)
	return sigrsv, err
}

func csJsonToData(csj *csJson) *csData {
	var err error
	var tmp int
	csd := new(csData)
	csd.CsAddr = common.HexToAddress(csj.CsAddr)
	csd.Start, err = common.GetUnixTime(csj.Start)
	if err != nil {
		return nil
	}
	csd.End, err = common.GetUnixTime(csj.End)
	if err != nil {
		return nil
	}
	tmp, err = strconv.Atoi(csj.Flow)
	if err != nil {
		return nil
	}
	csd.Flow = uint64(tmp)
	csd.CsPickupTime, err = common.GetUnixTime(csj.CsPickupTime)
	if err != nil {
		return nil
	}
	tmp, err = strconv.Atoi(csj.AuthAllowFlow)
	if err != nil {
		return nil
	}
	csd.AuthAllowFlow = uint64(tmp)
	tmp, err = strconv.Atoi(csj.UsedFlow)
	if err != nil {
		return nil
	}
	csd.UsedFlow = uint64(tmp)
	tmp, err = strconv.Atoi(csj.PayMethod)
	if err != nil {
		return nil
	}
	csd.PayMethod = uint8(tmp)
	// e.logInfo("csJsonToData", "csd", *csd)
	return csd
}

func (e *Elephant) ParseUpdateSignedCs(str string) error {
	ret := errors.New("format is not right")
	sub1 := "{\"csArray\":"
	n1 := strings.Index(str, sub1)
	if n1 != 0 {
		return ret
	}
	sub2 := "}"
	n2 := strings.LastIndex(str, sub2)
	if n2 == -1 {
		return ret
	}
	by := []byte(str)
	by = by[len(sub1):n2]
	empty := "[{}]"
	if strings.LastIndex(string(by), empty) != -1 {
		e.logInfo("signed cs empty")
		return nil
	}
	var csj []csJson
	if err := json.Unmarshal(by, &csj); err != nil {
		return err
	}
	e.logInfo("", "css len", len(csj))
	if len(csj) > 0 {
		e.logInfo("", "csAddr", csj[0].CsAddr, "payMethod", csj[0].PayMethod)
		csd := csJsonToData(&csj[0])
		usedFlow, err := e.rpcCli.GetUsedFlow(e.accUse)
		if err == nil {
			csd.UsedFlow = usedFlow
		}
		// e.checkAuthFlow(e.signedCs, csd)
		// e.signedCs = csd
	}
	return nil
}

func (e *Elephant) convertCSAuthData(css []common.WalletCSAuthData) string {
	ret := `{"csArray": [{}`
	for i, cs := range css {
		if i == 0 {
			ret = `{"csArray": [`
		} else if i != len(css)-1 {
			ret += ", "
		}
		ret += e.getDetailedSignedCs(&cs)
	}
	ret += "]}"
	return ret
}

func (e *Elephant) getDetailedSignedCs(cs *common.WalletCSAuthData) string {
	start := common.GetTimeString(cs.StartTime)
	end := common.GetTimeString(cs.EndTime)
	authAllowTime := common.GetTimeString(0)
	csPickupTime := common.GetTimeString(cs.CSPickupTime)
	ret := fmt.Sprintf("{\"csAddr\": \"%v\", \"start\": \"%v\", \"end\": \"%v\", \"flow\": \"%v\", "+
		"\"authAllowTime\": \"%v\", \"csPickupTime\": \"%v\", \"authAllowFlow\": \"%v\", \"csPickupFlow\": \"%v\", "+
		"\"usedFlow\": \"%v\", \"payMethod\": \"%v\", \"nodeID\": \"%v\"}",
		cs.CsAddr.Hex(), start, end, cs.Flow, authAllowTime, csPickupTime, cs.AuthAllowFlow, cs.CSPickupFlow, e.usedFlow, cs.PayMethod, cs.NodeID)
	return ret
}

func (e *Elephant) checkAuthFlow(oldCs *csData, newCs *csData) { // qiwy: todo, need delete
	if oldCs == nil || newCs == nil {
		return
	}
	if *oldCs == *newCs {
		return
	}
	if oldCs.PayMethod != blizparam.PayMethodFlow || newCs.PayMethod != blizparam.PayMethodFlow {
		return
	}
	times := blizparam.UserAuthTimes
	oneFlow := oldCs.Flow / blizparam.UserAuthTimes
	oldIndex := oldCs.UsedFlow / oneFlow
	if oldIndex*oneFlow != oldCs.UsedFlow {
		oldIndex++
	}
	newIndex := newCs.UsedFlow / oneFlow
	if newIndex*oneFlow != newCs.UsedFlow {
		newIndex++
	}
	if oldIndex == newIndex {
		return
	}
	if newIndex+1 > times {
		return
	}
	pm := e.node.FTransAPI.GetProtocolMan()
	if pm == nil {
		e.logError("pm nil")
		return
	}
	p := e.getPeer()
	if p == nil {
		e.logError("there is no correct connection with the FileTransfer device")
		return
	}
	signer := pm.GetBaseAddress()
	csAddr := pm.GetCurrentCSAddress()
	flow := (newIndex + 1) * oneFlow
	sign, err := e.SignAuthAllowFlow(signer, csAddr, flow)
	if err != nil {
		e.logError("SignAuthAllowFlow", "err", err)
		return
	}
	err = pm.SendAuthAllowFlow(p, flow, sign)
	if err != nil {
		e.logError("SendAuthAllowFlow", "err", err)
	}
}

func (e *Elephant) checkAuthAllowFlow(cs *common.WalletCSAuthData, usedFlow uint64) {
	if e.signedCs == nil {
		return
	}
	if cs.PayMethod != 1 {
		// e.logInfo("no need auth allow flow")
		return
	}
	oldPickupFlow := e.signedCs.CSPickupFlow
	perSize := cs.Flow / blizparam.UserAuthTimes
	if perSize == 0 {
		perSize = 1
	}
	index := e.usedFlow / perSize
	newPickupFlow := cs.Flow / blizparam.UserAuthTimes * index
	// e.logInfo("", "newPickupFlow", newPickupFlow, "oldPickupFlow", oldPickupFlow)
	if newPickupFlow == oldPickupFlow {
		return
	}

	err := e.sendAuthAllowFlow(newPickupFlow)
	if err != nil {
		e.logError("sendAuthAllowFlow", "err", err)
	}
}

func (e *Elephant) sendAuthAllowFlow(flow uint64) error {
	pm := e.node.FTransAPI.GetProtocolMan()
	if pm == nil {
		return errors.New("pm nil")
	}
	p := e.getPeer()
	if p == nil {
		return errors.New("p nil")
	}
	signer := pm.GetBaseAddress()
	csAddr := pm.GetCurrentCSAddress()
	sign, err := e.SignAuthAllowFlow(signer, csAddr, flow)
	if err != nil {
		return err
	}
	return pm.SendAuthAllowFlow(p, flow, sign)
}

// GetSignedCs - returns the signed CS of this account
func (e *Elephant) GetSignedCs(postType string, params []interface{}, address string) (string, error) {
	var (
		accPwd string
		errP   error
	)
	if errP = e.parseParams(postType, params, &accPwd); errP != nil {
		return "", errP
	}

	destAccount := common.HexToAddress(address)
	signedCss, err := e.rpcCli.SignedCsAt(context.Background(), destAccount, nil)
	if err != nil {
		e.logError("getSignedCs", "err", err)
	} else if len(signedCss) > 0 {
		cs := signedCss[0]
		newUsedFlow := uint64(0)
		tmp, err := e.rpcCli.GetUsedFlow(e.accUse)
		if err == nil {
			newUsedFlow = tmp
		}
		e.checkAuthAllowFlow(&cs, newUsedFlow)
		e.usedFlow = newUsedFlow
		e.signedCs = &cs
		if e.signedCs.PayMethod == 0 {
			if time.Now().Unix() < int64(e.signedCs.StartTime) || time.Now().Unix() > int64(e.signedCs.EndTime) {
				signedCss = []common.WalletCSAuthData{}
				e.CancelCs("elephant_cancelCs", []interface{}{e.signedCs.CsAddr.String(), accPwd})
			}
		} else {
			if e.signedCs.Flow <= e.usedFlow {
				signedCss = []common.WalletCSAuthData{}
				e.CancelCs("elephant_cancelCs", []interface{}{e.signedCs.CsAddr.String(), accPwd})
			}
		}
	}
	ret := e.convertCSAuthData(signedCss)
	return ret, err
}

func (e *Elephant) getSharingCode(postType string, params []interface{}) (string, error) {
	var (
		fileObjID         uint64
		fileName          string
		fileOffset        uint64
		fileSize          uint64
		receiver          string
		startTime         string
		stopTime          string
		price             string
		sharerAgentNodeID string
		accountPwd        string
	)

	if err := e.parseParams(postType, params, &fileObjID, &fileName, &fileOffset, &fileSize, &receiver,
		&startTime, &stopTime, &price, &sharerAgentNodeID, &accountPwd); err != nil {
		return "", err
	}

	key, err := e.getEncryptKey(fileObjID, accountPwd)
	if err != nil {
		return "", err
	}

	codeST := common.SharingCodeST{
		FileObjID:         fileObjID,
		FileName:          fileName,
		FileOffset:        fileOffset,
		FileSize:          fileSize,
		Receiver:          receiver,
		StartTime:         startTime,
		StopTime:          stopTime,
		Price:             price,
		SharerAgentNodeID: sharerAgentNodeID,
		Key:               key,
	}

	hash := codeST.HashNoSignature()
	sig, err := e.getCommonSig(hash, accountPwd)
	if err != nil {
		return "", err
	}
	codeST.Signature = sig

	code, err := json.Marshal(codeST)
	return string(code), err
}

func (e *Elephant) getEncryptKey(objID uint64, passwd string) (string, error) {
	acc := e.GetAccoutInUse()
	ks := fetchKeystore(e.am)
	privKey, err := ks.GetKeyCopy(acc, passwd)
	if err != nil {
		return "", err
	}
	defer zeroKey(privKey)
	h := utils.RlpHash(privKey.D.Bytes())
	key := datacrypto.GetEncryptKey(datacrypto.EncryptDes, h, objID)
	return string(key), nil
}

func (e *Elephant) getCommonSig(hash common.Hash, passwd string) (string, error) {
	acc := e.GetAccoutInUse()
	ks := fetchKeystore(e.am)
	privKey, err := ks.GetKeyCopy(acc, passwd)
	if err != nil {
		return "", err
	}
	defer zeroKey(privKey)
	sigrsv, err := crypto.Sign(hash.Bytes(), privKey)
	if err != nil {
		return "", err
	}
	return common.ToHex(sigrsv), nil
}

func (e *Elephant) getSignature(postType string, params []interface{}) (string, error) {
	var (
		data   string
		passwd string
	)

	if err := e.parseParams(postType, params, &data, &passwd); err != nil {
		return "", err
	}

	hash := utils.RlpHash([]interface{}{data})
	return e.getCommonSig(hash, passwd)
}

func (e *Elephant) checkSignature(postType string, params []interface{}) (string, error) {
	var (
		data string
	)

	if err := e.parseParams(postType, params, &data); err != nil {
		return "", err
	}

	var codeST common.SharingCodeST
	if err := json.Unmarshal([]byte(data), &codeST); err != nil {
		return "", err
	}

	hash := codeST.HashNoSignature()
	sig := codeST.Signature
	recoveredAddr, err := blizcore.EcRecover(hash.Bytes(), common.FromHex(sig))
	if err != nil {
		return "", err
	}
	return recoveredAddr.String(), nil
}

func (e *Elephant) payForSharedFile(postType string, params []interface{}) (string, error) {
	var (
		sharingCode string
		passwd      string
	)
	if err := e.parseParams(postType, params, &sharingCode, &passwd); err != nil {
		return "", err
	}

	var codeST common.SharingCodeST
	if err := json.Unmarshal([]byte(sharingCode), &codeST); err != nil {
		return "", err
	}

	var to string
	hash := codeST.HashNoSignature()
	if recoveredAddr, err := blizcore.EcRecover(hash.Bytes(), common.FromHex(codeST.Signature)); err != nil {
		return "", err
	} else {
		fmt.Println(recoveredAddr.String())
		to = recoveredAddr.String()
	}

	// Check balance
	v, err := strconv.ParseFloat(codeST.Price, 64)
	if err != nil {
		e.logError("Parse float", "err", err)
		return "", err
	}
	var value *big.Int
	// Avoid overflowing, 1e18 = 1e10 * 1e8
	if math.Abs(v) > 9.0 {
		v1 := new(big.Int).SetInt64(int64(v * 1e10))
		v2 := new(big.Int).SetInt64(1e8)
		value = new(big.Int).Mul(v1, v2)
	} else {
		value = new(big.Int).SetInt64(int64(v * 1e18))
	}
	if value.Cmp(new(big.Int).SetInt64(0)) < 0 {
		str := "Price should not be negative number"
		e.logError(str)
		return "", errors.New(str)
	}

	user := e.accUse
	actType := blizparam.TypeActShareObj

	data := &blizparam.TxShareObjData{
		SharingCode: sharingCode,
	}
	ret, err := rlp.EncodeToBytes(data)
	if err != nil {
		e.logError("Pay for shared file", "err", err)
		return "", err
	}
	actData := ret
	h, err := e.GetClient().SendTx(user, to, passwd, e.am, actType, actData, value)
	if err != nil {
		return "", err
	}

	return h.String(), nil
}

func (e *Elephant) getSignedCSAddr() common.Address {
	if e.signedCs == nil {
		return common.Address{}
	}
	return e.signedCs.CsAddr
}

func (e *Elephant) RegMail(postType string, params []interface{}) (string, error) {
	var (
		name   string
		passwd string
	)

	if err := e.parseParams(postType, params, &name, &passwd); err != nil {
		return "", err
	}

	acc := e.GetAccoutInUse()
	ks := fetchKeystore(e.am)
	pubKey, err := ks.GetMailPubKey(acc, passwd)
	if err != nil {
		return "", err
	}

	sender := e.accUse
	nonce, err := e.GetClient().ElePendingNonceAt(context.Background(), common.HexToAddress(sender))
	if err != nil {
		e.logError("Ele pending nonce at", "err", err)
		return "", err
	}

	actType := blizparam.TypeActRegMail
	data := blizparam.RegMailData{
		Name:   name,
		PubKey: crypto.CompressPubkey(pubKey),
	}
	actData, err := rlp.EncodeToBytes(data)
	if err != nil {
		e.logError("Encode to bytes", "err", err)
		return "", err
	}
	to := blizparam.DestAddrStr
	value := blizparam.TxDefaultValue
	hash, err := e.GetClient().SendTxWithNonce(sender, to, passwd, value, e.am, actType, actData, nonce)
	if err != nil {
		return "", err
	}
	return hash.String(), nil
}
