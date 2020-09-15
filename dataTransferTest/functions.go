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

package dataTransferTest

import (
	"errors"
	"fmt"
	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/ftransfer"
	"math/big"
	"strings"
)

const WeiAmount = "1000000000000000000"

func (b *BackendNoDiskDBOrBlock) GetBalanceString(addr common.Address) string {
	return convertBalanceToString(b.StateMemDB.GetBalance(addr))
}

func convertBalanceToString(val *big.Int) string {
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

	return balance
}

//////////////////////////////////////////////////////////////////////////////////////
///////////////////////////////////Server functions///////////////////////////////////
//////////////////////////////////////////////////////////////////////////////////////

func (b *BackendNoDiskDBOrBlock) SendTransactionServ(from, to, amount string) error {
	value, err := convertDouble2Wei(amount)
	if err != nil {
		b.Log.Error("SendTransaction failed", "err", err.Error())
		return err
	}

	if to != "" {
		b.StateMemDB.AddBalance(common.HexToAddress(to), value)
	}
	if from != "" {
		b.StateMemDB.SubBalance(common.HexToAddress(from), value)
	}

	return nil
}

func convertDouble2Wei(str string) (*big.Int, error) {
	var value *big.Int
	weiAmount := WeiAmount
	if val, _ := new(big.Int).SetString(str, 0); val != nil {
		mul, _ := new(big.Int).SetString(weiAmount, 0)
		value = val.Mul(val, mul)
	} else {
		vals := strings.Split(str, ".")
		if len(vals) != 2 {
			return nil, errors.New("the amount can not be parsed case 1")
		}
		vals[0] = strings.TrimLeft(vals[0], "0")
		vals[1] = strings.TrimRight(vals[1], "0")
		multiedAmt := vals[0] + vals[1]
		multiedAmt = strings.TrimLeft(multiedAmt, "0")

		val, _ = new(big.Int).SetString(multiedAmt, 0)
		if val == nil {
			return nil, errors.New("the amount can not be parsed case 2")
		}

		for i := 0; i < len(vals[1]); i++ {
			weiAmount = weiAmount[:len(weiAmount)-1]
		}
		mul, _ := new(big.Int).SetString(weiAmount, 0)
		value = val.Mul(val, mul)
	}

	return value, nil
}

func (b *BackendNoDiskDBOrBlock) SignCsServ(addr common.Address, signCsData []byte, update bool) {
	b.StateMemDB.SignCs(addr, signCsData, update)
}

func (b *BackendNoDiskDBOrBlock) CancelCsServ(addr common.Address, cancelCsData []byte, update bool) {
	b.StateMemDB.CancelCs(addr, cancelCsData, update)
}

func (b *BackendNoDiskDBOrBlock) GetSignedCsServ(addr, curCSAddr common.Address) []common.WalletCSAuthData {
	return b.StateMemDB.GetSignedCs(addr, curCSAddr)
}

func (b *BackendNoDiskDBOrBlock) GetUsedFlowServ(address string) (uint64, error) {
	return b.BlizCsApi.GetUsedFlow(address)
}

func (b *BackendNoDiskDBOrBlock) GetSignedCsStringServ(addr, curCSAddr common.Address) string {
	return convertCSAuthData(b.StateMemDB.GetSignedCs(addr, curCSAddr), 1024)
}

func convertCSAuthData(css []common.WalletCSAuthData, usedFlow uint64) string {
	ret := `{"csArray": [{}`
	for i, cs := range css {
		if i == 0 {
			ret = `{"csArray": [`
		} else if i != len(css)-1 {
			ret += ", "
		}
		ret += getDetailedSignedCs(&cs, usedFlow)
	}
	ret += "]}"
	return ret
}

func getDetailedSignedCs(cs *common.WalletCSAuthData, usedFlow uint64) string {
	start := common.GetTimeString(cs.StartTime)
	end := common.GetTimeString(cs.EndTime)
	authAllowTime := common.GetTimeString(0)
	csPickupTime := common.GetTimeString(cs.CSPickupTime)
	ret := fmt.Sprintf("{\"csAddr\": \"%v\", \"start\": \"%v\", \"end\": \"%v\", \"flow\": \"%v\", "+
		"\"authAllowTime\": \"%v\", \"csPickupTime\": \"%v\", \"authAllowFlow\": \"%v\", \"csPickupFlow\": \"%v\", "+
		"\"usedFlow\": \"%v\", \"payMethod\": \"%v\", \"nodeID\": \"%v\"}",
		cs.CsAddr.Hex(), start, end, cs.Flow, authAllowTime, csPickupTime, cs.AuthAllowFlow, cs.CSPickupFlow, usedFlow, cs.PayMethod, cs.NodeID)
	return ret
}

func (b *BackendNoDiskDBOrBlock) CreateObjectStorageServ(addr common.Address, rentData []byte, update bool) {
	b.StateMemDB.RentSize(addr, rentData, update)
}

func (b *BackendNoDiskDBOrBlock) ExpandObjectStorageServ(addr common.Address, rentData []byte, update bool) {
	b.StateMemDB.RentSize(addr, rentData, update)
}

//////////////////////////////////////////////////////////////////////////////////////
///////////////////////////////////Client functions///////////////////////////////////
//////////////////////////////////////////////////////////////////////////////////////

func (b *BackendNoDiskDBOrBlock) WriteStringClient(objId, offset uint64, data, passwd string) (bool, error) {
	p := b.getPeer()
	if p == nil {
		return false, errors.New("there is no correct connection with the FileTransfer device")
	}
	return b.FTransAPI.WriteString(objId, offset, []byte(data), passwd, p)
}

func (b *BackendNoDiskDBOrBlock) getPeer() *ftransfer.Peer {
	ps := b.FTransAPI.GetProtocolMan().GetPeerSet()
	if len(ps) == 0 {
		return nil
	}
	return ps[0]
}
