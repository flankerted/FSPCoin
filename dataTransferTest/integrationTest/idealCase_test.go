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

package integrationTest

import (
	"fmt"
	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/dataTransferTest"
	"github.com/contatract/go-contatract/log"
	"math/rand"
	"sync"
	"testing"
	"time"
)

const (
	timeUnit                   = 30 //30
	dataTransferIdealCaseTimes = uint64(100000)
)

var (
	loggerTest log.Logger

	dtNoDBOrBlockIdealServCnt int
	dtNoDBOrBlockIdealCliCnt  int

	servEtherBasesIdeal       []common.Address
	dtNoDBOrBlockIdealClients []*dataTransferTest.BackendNoDiskDBOrBlock
	dtNoDBOrBlockIdealServers []*dataTransferTest.BackendNoDiskDBOrBlock

	ServerGlobalMu    sync.Mutex
	TestStopped       bool
	useRandomDelay    bool
	testTimes         int
	recoverDalayTimes int
)

func LogInfo(msg string, ctx ...interface{}) {
	loggerTest.Info(msg, ctx...)
}

func LogWarn(msg string, ctx ...interface{}) {
	loggerTest.Warn(msg, ctx...)
}

func LogError(msg string, ctx ...interface{}) {
	loggerTest.Error(msg, ctx...)
}

func LogCrit(msg string, ctx ...interface{}) {
	loggerTest.Crit(msg, ctx...)
}

func randomDelayMsForHBFT(nodeIdx int) bool {
	if testTimes >= recoverDalayTimes {
		if testTimes <= recoverDalayTimes+5 {
			LogInfo("---Recover net delay")
		}

		//bftNodeGlobalMu.Lock()
		//if len(bftNodeBlockNumMap)%3000 == 0 {
		//	testTimes = 0
		//}
		//bftNodeGlobalMu.Unlock()

		return true
	}

	if !useRandomDelay {
		return true
	}

	rand.Seed(time.Now().UnixNano())
	random := rand.Intn(100)
	//random = 50
	if random < 60 {
		time.Sleep(time.Millisecond / 10 * timeUnit)
	} else if random < 75 {
		time.Sleep(time.Millisecond * 1 * timeUnit)
	} else if random < 86 {
		time.Sleep(time.Millisecond * 3 * timeUnit)
	} else if random < 90 {
		time.Sleep(time.Millisecond * 5 * timeUnit)
	} else if random < 94 {
		time.Sleep(time.Millisecond * 7 * timeUnit)
	} else if random < 96 {
		time.Sleep(time.Millisecond * 11 * timeUnit)
	} else if random < 98 {
		time.Sleep(time.Millisecond * 20 * timeUnit)
	} else if random < 100 {
		//bftNodesNormal[b.NodeIndex].Info("RandomDelayMsForHBFT() return false", "node",
		//	bftNodeBackendsNormal[nodeIdx].EtherBase.String())
		return false
	}

	return true
}

func init() {
	testTimes = 0
	useRandomDelay = true
	recoverDalayTimes = 3000
	dtNoDBOrBlockIdealServCnt = 4
	dtNoDBOrBlockIdealCliCnt = 1 // 2
	servEtherBasesIdeal = make([]common.Address, 0)
	dtNoDBOrBlockIdealClients = make([]*dataTransferTest.BackendNoDiskDBOrBlock, dtNoDBOrBlockIdealCliCnt)
	dtNoDBOrBlockIdealServers = make([]*dataTransferTest.BackendNoDiskDBOrBlock, dtNoDBOrBlockIdealServCnt)
}

func initIdealCase() error {
	testTimes := dataTransferIdealCaseTimes
	testType := "IntTestNoDiskDBOrBlockTest"
	loggerTest = dataTransferTest.CreateLogger("", testType, "IdealCaseGlobal.log")
	for i := range dtNoDBOrBlockIdealClients {
		nodeName := fmt.Sprintf("IdealCliNode%d", i)
		logger := dataTransferTest.CreateLogger("", testType, fmt.Sprintf("IdealCliNode%d.log", i))

		//var lamp *eth.Ethereum
		client, err := dataTransferTest.NewSimBackendForIntTestWithoutDiskDBOrBlock(false, nodeName, testType, i, logger)
		if err != nil {
			return err
		}

		client.TestTimesTotal = testTimes
		dtNoDBOrBlockIdealClients[i] = client

		LogInfo(fmt.Sprintf("---Init testing client, %s: %s", dtNoDBOrBlockIdealClients[i].NodeName,
			dtNoDBOrBlockIdealClients[i].EtherBase.String()))
	}

	for i := range dtNoDBOrBlockIdealServers {
		nodeName := fmt.Sprintf("IdealServNode%d", i)
		logger := dataTransferTest.CreateLogger("", testType, fmt.Sprintf("IdealServNode%d.log", i))

		//var lamp *eth.Ethereum
		server, err := dataTransferTest.NewSimBackendForIntTestWithoutDiskDBOrBlock(true, nodeName, testType, i, logger)
		if err != nil {
			return err
		}

		server.TestTimesTotal = testTimes
		dtNoDBOrBlockIdealServers[i] = server

		LogInfo(fmt.Sprintf("---Init testing server, %s: %s", dtNoDBOrBlockIdealServers[i].NodeName,
			dtNoDBOrBlockIdealServers[i].EtherBase.String()))
	}

	LogInfo("---Init ideal case testing successfully")

	return nil
}

func TestIdealCase(t *testing.T) {
	tstart := time.Now()
	initIdealCase()
	fmt.Println("---InitIdealCase", "wait", common.PrettyDuration(time.Since(tstart)))

	fmt.Println(dtNoDBOrBlockIdealServers[0].GetBalanceString(dtNoDBOrBlockIdealServers[0].EtherBase))

	dtNoDBOrBlockIdealServers[0].SendTransactionServ("", dtNoDBOrBlockIdealServers[0].EtherBase.String(), "10000")

	fmt.Println(dtNoDBOrBlockIdealServers[0].GetBalanceString(dtNoDBOrBlockIdealServers[0].EtherBase))

	err := dtNoDBOrBlockIdealClients[0].ConnectToServer(dtNoDBOrBlockIdealServers[0])
	if err != nil {
		fmt.Println(err.Error())
	}
	dtNoDBOrBlockIdealClients[0].StartFtansferHandler()
	dtNoDBOrBlockIdealServers[0].StartFtansferHandler()

	_, errW := dtNoDBOrBlockIdealClients[0].WriteStringClient(1, 0, "test", dtNoDBOrBlockIdealClients[0].NodeName)
	if errW != nil {
		fmt.Println(errW.Error())
	}
}
