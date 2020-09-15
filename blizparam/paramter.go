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

package blizparam

import (
	"math/big"

	"github.com/contatract/go-contatract/common/hexutil"
)

// standard value as follows:

// ChunkFarmerMin = 8

// ChunkClaimMax = 8192 // 最多同时认领 8192 个chunk,将单矿工提供存储容量限制在 8TB

// NeedShardingNodeCount = 3

// QueryFarmerStep2ChanCount = 2

// StandardWopSyncQueueSize = 256 // 同步wop队列的标秤长度（允许超过，达到就需要触发Sweep清理）

var syncCheckTimes = 1

var objMetaQueryCount = 1

var writeCheckCount = 1

var readCheckCount = 1

var minCopyCount = 2

var maxCopyCount = 4

var standardCopyCount = 3

const csSelfDefaultPassphrase = "block"

var csSelfPassphrase = csSelfDefaultPassphrase

const (
	QueryFarmerTimeout int = 20
	QueryMinerTimeout  int = 20
)

const (
	TypeActNone uint8 = iota
	TypeActClaim
	TypeActVerify
	TypeActRent
	TypeActSignCs   // 签约CS
	TypeActCancelCs // 解约CS
	TypeActSignTime
	TypeActSignFlow
	TypeActPickupTime
	TypeActPickupFlow
	TypeActPickupCost
	TypeActUsedFlow
	TypeActShareObj // Send tx for sharing obj id
	TypeActSendMail
	TypeActRegMail
)

const (
	PayMethodTime uint8 = iota
	PayMethodFlow
)

const FromAddrStr = "0xaf8a2ec602ef70c0607b815ee89e2c9916e1cd26"
const DestAddrStr = "0x39f124f6f8911a35f19346d3940908d8f6a29543"

const CSPerDayCharge float64 = 1e18 // time charge
const CSPerMBCharge float64 = 1e18  // flow charge

const CSPickUpTimeMin = 10 // 3600 * 1 // one hour
const CSPickUpFlowMin = 10 // 1048576 * 1 // 1Mb

const SizeForUpdateState uint64 = 1 // qiwy: todo, online, such as 1mbit
const UserAuthTimes uint64 = 20     // 每次授权分成UserAuthTimes次签名

var TxDefaultValue = new(big.Int).SetInt64(1234)
var TxDefaultCost = (*hexutil.Big)(TxDefaultValue)
var TxCost0 = (*hexutil.Big)(big.NewInt(0))

func SetSyncCheckTimes(times int) {
	syncCheckTimes = times
}

func GetSyncCheckTimes() int {
	return syncCheckTimes
}

func SetObjMetaQueryCount(count int) {
	objMetaQueryCount = count
}

func GetObjMetaQueryCount() int {
	return objMetaQueryCount
}

func SetWriteCheckCount(count int) {
	writeCheckCount = count
}

func GetWriteCheckCount() int {
	return writeCheckCount
}

func SetReadCheckCount(count int) {
	readCheckCount = count
}

func GetReadCheckCount() int {
	return readCheckCount
}

func SetMinCopyCount(count int) {
	minCopyCount = count
}

func GetOtherMinFarmerCount(isChunkFarmer bool) int {
	if isChunkFarmer {
		return minCopyCount - 1
	}
	return minCopyCount
}

func GetMinCopyCount() int {
	return minCopyCount
}

func SetMaxCopyCount(count int) {
	maxCopyCount = count
}

func GetMaxCopyCount() int {
	return maxCopyCount
}

func GetStandardCopyCount() int {
	return standardCopyCount
}

func SetCsSelfPassphrase(passphrase string) {
	csSelfPassphrase = passphrase
}

func GetCsSelfPassphrase() string {
	return csSelfPassphrase
}

func GetNeedBalance(startTime, endTime, flow uint64) *big.Int {
	day := float64(endTime-startTime) / 86400
	timeBalance := new(big.Int).SetUint64(uint64(day * /*CSPerDayCharge*/ 1))
	mb := float64(flow) / (1024 * 1024)
	flowBalance := new(big.Int).SetUint64(uint64(mb * /*CSPerMBCharge*/ 1))
	return new(big.Int).Add(timeBalance, flowBalance)
}

func GetFlowBalance(flow uint64) *big.Int {
	// mb := float64(flow) / (1024 * 1024)
	// flowBalance := new(big.Int).SetUint64(uint64(mb * CSPerMBCharge))
	flowBalance := new(big.Int).SetUint64(uint64(20000))
	return flowBalance
}

func GetTimeBalance(seconds uint64) *big.Int {
	// day := float64(math.Ceil(float64(seconds) / 86400))
	// timeBalance := new(big.Int).SetUint64(uint64(day * CSPerDayCharge))
	timeBalance := new(big.Int).SetUint64(uint64(25000))
	return timeBalance
}
