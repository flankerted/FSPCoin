package backend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"sync"

	"github.com/contatract/go-contatract/core/types_elephant"

	"github.com/contatract/go-contatract/accounts"
	"github.com/contatract/go-contatract/common/hexutil"
	"github.com/contatract/go-contatract/eleWallet/wallettx"
	"github.com/contatract/go-contatract/internal/elephantapi"
	"github.com/contatract/go-contatract/log"

	ethereum "github.com/contatract/go-contatract"
	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/eleWallet/utils"
	"github.com/contatract/go-contatract/ethclient"
)

// EthClient represents the eth client
type EthClient struct {
	*ethclient.Client
	url string
}

//var txns []wi.Txn
var txnsLock sync.RWMutex

// NewEthClient returns a new eth client
func NewEthClient(url string) (*EthClient, error) {
	var conn *ethclient.Client
	var err error
	if conn, err = ethclient.Dial(url); err != nil {
		return nil, err
	}
	return &EthClient{
		Client: conn,
		url:    url,
	}, nil

}

// GetRentSize - returns the rent size
func (client *EthClient) GetRentSize(params []interface{}, address string) (string, error) {
	if len(params) == 1 {
		if objId, ok := params[0].(uint64); ok {
			result, err := client.GetRentSizeAt(context.Background(), objId, address)
			if err != nil {
				return "", err
			} else {
				return result, nil
			}
		}
	}

	return "", errors.New("error: The inputs are not correct")
}

// GetObjectsInfo - returns the objects information
func (client *EthClient) GetObjectsInfo(params []interface{}, address string) (string, error) {
	if len(params) == 1 {
		if passwd, ok := params[0].(string); ok {
			result, err := client.GetObjectsInfoAt(context.Background(), passwd, address)
			if err != nil {
				return "", err
			} else {
				return result, nil
			}
		}
	}

	return "", errors.New("error: The inputs are not correct")
}

//// CreateObject - creates a storage object
//func (client *EthClient) CreateObject(postType string, params []interface{}) (bool, error) {
//	var (
//		result      bool
//		objId, size uint64
//		password    string
//		err         error
//	)
//
//	if err := e.parseParams(postType, params, &objId, &size, &password); err != nil {
//		return false, err
//	}
//
//	result, err = client.CreateObjectAt(context.Background(), objId, size, password)
//	if err == nil {
//		return result, nil
//	} else {
//		return false, err
//	}
//}

//// AddObjectStorage - adds the storage size of the object
//func (client *EthClient) AddObjectStorage(postType string, params []interface{}) (bool, error) {
//	var (
//		result      bool
//		objId, size uint64
//		password    string
//		err         error
//	)
//
//	if err := parseParams(postType, params, &objId, &size, &password); err != nil {
//		return false, err
//	}
//
//	result, err = client.AddObjectStorageAt(context.Background(), objId, size, password)
//	if err == nil {
//		return result, nil
//	} else {
//		return false, err
//	}
//}

func (client *EthClient) GetUsedFlow(address string) (uint64, error) {
	return client.UsedFlowAt(context.Background(), address)
}

func (client *EthClient) SendTx(from, to, passphrase string, am *accounts.Manager, actType uint8, actData []byte, value *big.Int) (common.Hash, error) {
	hash := common.Hash{}
	fromAddr := common.HexToAddress(from)
	nonce, err := client.ElePendingNonceAt(context.Background(), fromAddr)
	if err != nil {
		log.Error("ElePendingNonceAt", "err", err)
		return hash, err
	}
	log.Info("ElePendingNonceAt", "nonce", nonce)

	var args elephantapi.SendTxArgs
	args.From = fromAddr
	toAddr := common.HexToAddress(to)
	args.To = &toAddr
	args.Value = (*hexutil.Big)(value)
	args.Nonce = (*hexutil.Uint64)(&nonce)
	gasPrice := big.NewInt(3)
	args.GasPrice = (*hexutil.Big)(gasPrice)
	fmt.Printf("from: %s, to: %s\n", from, to)
	args.ActType = actType
	args.ActData = actData

	result, err := client.BalanceAt(context.Background(), fromAddr, nil)
	if err != nil {
		return hash, errors.New("BalanceAt error")
	}
	if result.Cmp(value) < 0 {
		str := fmt.Sprintf("error: lack of gctt, left %v, but need %v", result.String(), value.String())
		return hash, errors.New(str)
	}
	tx, err := wallettx.SignTransaction(am, args, passphrase)
	if err != nil {
		log.Error("SignTransaction", "err", err)
		return hash, err
	}
	hash = tx.Hash()

	return hash, client.EleSendTransaction(context.Background(), tx)
}

func (client *EthClient) SendTxWithNonce(from, to, passphrase string, value *big.Int, am *accounts.Manager, actType uint8, actData []byte,
	nonce uint64) (common.Hash, error) {
	hash := common.Hash{}
	fromAddr := common.HexToAddress(from)
	var args elephantapi.SendTxArgs
	args.From = fromAddr
	toAddr := common.HexToAddress(to)
	args.To = &toAddr
	args.Value = (*hexutil.Big)(value)
	args.Nonce = (*hexutil.Uint64)(&nonce)
	gasPrice := big.NewInt(3)
	args.GasPrice = (*hexutil.Big)(gasPrice)
	//fmt.Printf("from: %s, to: %s, value: %s\n", from, to, value.String())
	args.ActType = actType
	args.ActData = actData

	result, err := client.BalanceAt(context.Background(), fromAddr, nil)
	if err != nil {
		return hash, errors.New("BalanceAt error")
	}
	if result.Cmp(value) < 0 {
		str := fmt.Sprintf("error: lack of gctt, left %v, but need %v", result.String(), value.String())
		return hash, errors.New(str)
	}
	tx, err := wallettx.SignTransaction(am, args, passphrase)
	if err != nil {
		log.Error("SignTransaction", "err", err)
		return hash, err
	}
	hash = tx.Hash()

	return hash, client.EleSendTransaction(context.Background(), tx)
}

// GetUnconfirmedBalance - returns the unconfirmed balance for this account
func (client *EthClient) GetUnconfirmedBalance(destAccount common.Address) (*big.Int, error) {
	return client.PendingBalanceAt(context.Background(), destAccount)
}

// GetEleTransaction - returns a elephant txn for the specified hash
func (client *EthClient) GetEleTransaction(hash common.Hash) (*types_elephant.Transaction, bool, error) {
	return client.EleTransactionByHash(context.Background(), hash)
}

// GetLatestBlock - returns the latest block
func (client *EthClient) GetLatestBlock() (uint32, common.Hash, error) {
	header, err := client.HeaderByNumber(context.Background(), nil)
	if err != nil {
		return 0, common.BytesToHash([]byte{}), err
	}
	return uint32(header.Number.Int64()), header.Hash(), nil
}

// EstimateTxnGas - returns estimated gas
func (client *EthClient) EstimateTxnGas(from, to common.Address, value *big.Int) (*big.Int, error) {
	gas := big.NewInt(0)
	if !(utils.IsValidAddress(from.String()) && utils.IsValidAddress(to.String())) {
		return gas, errors.New("invalid address")
	}

	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		return gas, err
	}
	msg := ethereum.CallMsg{From: from, To: &to, Value: value}
	gasLimit, err := client.EstimateGas(context.Background(), msg)
	if err != nil {
		return gas, err
	}
	return gas.Mul(big.NewInt(int64(gasLimit)), gasPrice), nil
}

// EstimateGasSpend - returns estimated gas
func (client *EthClient) EstimateGasSpend(from common.Address, value *big.Int) (*big.Int, error) {
	gas := big.NewInt(0)
	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		return gas, err
	}
	msg := ethereum.CallMsg{From: from, Value: value}
	gasLimit, err := client.EstimateGas(context.Background(), msg)
	if err != nil {
		return gas, err
	}
	return gas.Mul(big.NewInt(int64(gasLimit)), gasPrice), nil
}

// GetTxnNonce - used to fetch nonce for a submitted txn
func (client *EthClient) GetTxnNonce(txID string) (int32, error) {
	//txnsLock.Lock()
	//for _, txn := range txns {
	//	if txn.Txid == txID {
	//		return txn.Height, nil
	//	}
	//}
	return 0, errors.New("nonce not found")
}

// EthGasStationData represents ethgasstation api data
// https://ethgasstation.info/json/ethgasAPI.json
// {"average": 20.0, "fastestWait": 0.4, "fastWait": 0.4, "fast": 200.0,
// "safeLowWait": 10.6, "blockNum": 6684733, "avgWait": 2.0,
// "block_time": 13.056701030927835, "speed": 0.7529715304081577,
// "fastest": 410.0, "safeLow": 17.0}
type EthGasStationData struct {
	Average     float64 `json:"average"`
	FastestWait float64 `json:"fastestWait"`
	FastWait    float64 `json:"fastWeight"`
	Fast        float64 `json:"Fast"`
	SafeLowWait float64 `json:"safeLowWait"`
	BlockNum    int64   `json:"blockNum"`
	AvgWait     float64 `json:"avgWait"`
	BlockTime   float64 `json:"block_time"`
	Speed       float64 `json:"speed"`
	Fastest     float64 `json:"fastest"`
	SafeLow     float64 `json:"safeLow"`
}

// GetEthGasStationEstimate get the latest data
// from https://ethgasstation.info/json/ethgasAPI.json
func (client *EthClient) GetEthGasStationEstimate() (*EthGasStationData, error) {
	res, err := http.Get("https://ethgasstation.info/json/ethgasAPI.json")
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	var s = new(EthGasStationData)
	err = json.Unmarshal(body, &s)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// CheckReg - returns whether the mail nickname have already been registered
func (client *EthClient) CheckReg(addr common.Address, params []interface{}) (bool, error) {
	paramErr := errors.New("The parameter is not right")
	if len(params) != 1 {
		return false, paramErr
	}
	name, ok := params[0].(string)
	if !ok {
		return false, paramErr
	}
	return client.CheckRegAt(context.Background(), addr, name)
}

func init() {
	//txns = []wi.Txn{}
}
