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

package ftransfer

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/bouk/monkey"
	"github.com/contatract/go-contatract/accounts"
	"github.com/contatract/go-contatract/accounts/keystore"
	"github.com/contatract/go-contatract/common/datacrypto"
	"github.com/contatract/go-contatract/log"
	"github.com/prashantv/gostub"
	. "github.com/smartystreets/goconvey/convey"
)

///////////////////////////////////////////////////////////////////////
/////////////////////////////only for test/////////////////////////////
///////////////////////////////////////////////////////////////////////
//func TestWaitGroup(t *testing.T) {
//	stop := make(chan struct{})
//	var pend sync.WaitGroup
//	pend.Add(1)
//	go func() {
//		defer pend.Done()
//		seal(stop)
//	}()
//
//	time.Sleep(time.Millisecond * 250)
//	//time.Sleep(time.Millisecond * 1250)
//	fmt.Println("===========close(stop)")
//	close(stop)
//
//	pend.Wait()
//}

//func seal(stop <-chan struct{}) int {
//	abort := make(chan struct{})
//	found := make(chan int)
//
//	var pend sync.WaitGroup
//	pend.Add(1)
//	go func() {
//		defer pend.Done()
//		mine(abort, found)
//	}()
//
//	// Wait until sealing is terminated or a nonce is found
//	var result = 0
//	select {
//	case <-stop:
//		// Outside abort, stop all miner threads
//		close(abort)
//		fmt.Println("===========close(abort)")
//		a := <-found
//		fmt.Println("===========<-found", a)
//	case result = <-found:
//		// One of the threads found a block, abort all others
//		close(abort)
//	}
//
//	// Wait for all miners to terminate and return the block
//	pend.Wait()
//	fmt.Println("===========return result", result)
//	return result
//}
//
//func mine(abort chan struct{}, found chan int) {
//	ticker := time.NewTicker(time.Second)
//	defer ticker.Stop()
//
//seal:
//	for {
//		select {
//		case <-abort:
//			fmt.Println("===========break seal")
//			close(found)
//			break seal
//
//		case <-ticker.C:
//			ticker.Stop()
//			fmt.Println("===========ticker.C")
//			time.Sleep(time.Second * 5)
//			found <- 10
//		}
//	}
//}

func StringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	if (a == nil) != (b == nil) {
		return false
	}

	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func TestStringSliceEqualA(t *testing.T) {
	Convey("TestStringSliceEqual should return true when a != nil  && b != nil", t, func() {
		a := []string{"hello", "goconvey"}
		b := []string{"hello", "goconvey"}
		So(StringSliceEqual(a, b), ShouldBeTrue)
	})
}

func TestStringSliceEqualB(t *testing.T) {
	Convey("TestStringSliceEqual", t, func() {
		Convey("should return true when a != nil  && b != nil", func() {
			a := []string{"hello", "goconvey"}
			b := []string{"hello", "goconvey"}
			So(StringSliceEqual(a, b), ShouldBeTrue)
		})

		Convey("should return true when a ＝= nil  && b ＝= nil", func() {
			So(StringSliceEqual(nil, nil), ShouldBeTrue)
		})

		Convey("should return false when a ＝= nil  && b != nil", func() {
			a := []string(nil)
			b := []string{}
			So(StringSliceEqual(a, b), ShouldBeFalse)
		})

		Convey("should return false when a != nil  && b != nil", func() {
			a := []string{"hello", "world"}
			b := []string{"hello", "goconvey"}
			So(StringSliceEqual(a, b), ShouldBeFalse)
		})
	})
}

var stringSliceEqual = StringSliceEqual

func StringSliceEqualForStub(a, b []string) bool {
	return stringSliceEqual(a, b)
}

func TestStringSliceEqualForStub(t *testing.T) {
	stubs := gostub.Stub(&stringSliceEqual, func(a, b []string) bool {
		return false
	})
	defer stubs.Reset()

	Convey("TestStringSliceEqual", t, func() {
		Convey("should return true when a != nil  && b != nil", func() {
			a := []string{"hello", "goconvey"}
			b := []string{"hello", "goconvey"}
			So(StringSliceEqualForStub(a, b), ShouldBeFalse)
		})

		Convey("should return true when a ＝= nil  && b ＝= nil", func() {
			So(StringSliceEqualForStub(nil, nil), ShouldBeFalse)
		})

		Convey("should return false when a ＝= nil  && b != nil", func() {
			a := []string(nil)
			b := []string{}
			So(StringSliceEqualForStub(a, b), ShouldBeFalse)
		})

		Convey("should return false when a != nil  && b != nil", func() {
			a := []string{"hello", "world"}
			b := []string{"hello", "goconvey"}
			So(StringSliceEqualForStub(a, b), ShouldBeFalse)
		})
	})
}

func TestStringSliceEqualForMonkey(t *testing.T) {
	guard := monkey.Patch(StringSliceEqual, func(a, b []string) bool {
		return true
	})
	defer guard.Unpatch()

	Convey("TestStringSliceEqual", t, func() {
		Convey("should return false when a != nil  && b != nil", func() {
			a := []string{"hello", "world"}
			b := []string{"hello", "goconvey"}
			So(StringSliceEqualForStub(a, b), ShouldBeTrue)
		})
	})
}

///////////////////////////////////////////////////////////////////////
/////////////////////////////only for test/////////////////////////////
///////////////////////////////////////////////////////////////////////

const (
	datadirPrivateKey      = "nodekey"  // Path within the datadir to the node's private key
	datadirDefaultKeyStore = "keystore" // Path within the datadir to the keystore
)

var (
	pm *ProtocolManager
)

type testNodeConfig struct {
	// Name sets the instance name of the node. It must not contain the / character and is
	// used in the devp2p node identifier. The instance name of geth is "geth". If no
	// value is specified, the basename of the current executable is used.
	Name string `toml:"-"`

	// BasePassphrase used for the base passphrase of the FTP
	BaseAddress    string
	BasePassphrase string

	//// UserIdent, if set, is used as an additional component in the devp2p node identifier.
	//UserIdent string `toml:",omitempty"`

	// Version should be set to the version number of the program. It is used
	// in the devp2p node identifier.
	Version string `toml:"-"`

	// DataDir is the file system folder the node should use for any data storage
	// requirements. The configured data directory will not be directly shared with
	// registered services, instead those can use utility methods to create/access
	// databases or flat files. This enables ephemeral nodes which can fully reside
	// in memory.
	DataDir string

	// Configuration of peer-to-peer networking.
	//P2P p2p.ClientCfg

	// Logger is a custom logger to use with the p2p.Server.
	Logger log.Logger `toml:",omitempty"`
}

// AccountConfig determines the settings for scrypt and keydirectory
func (c *testNodeConfig) AccountConfig() (int, int, string) {
	scryptN := keystore.StandardScryptN
	scryptP := keystore.StandardScryptP
	keydir := filepath.Join(c.DataDir, datadirDefaultKeyStore)
	return scryptN, scryptP, keydir
}

func defaultNodeConfig() *testNodeConfig {
	cfg := &testNodeConfig{DataDir: defaultDataDir()}
	cfg.Name = "ftpTest"
	cfg.Version = "ftpTestVersion"
	return cfg
}

type testNode struct {
	accman *accounts.Manager
	config *testNodeConfig
}

// New creates a new P2P node, ready for protocol registration.
func newTestNode(conf *testNodeConfig) (*testNode, error) {
	// Copy config and resolve the datadir so future changes to the current
	// working directory don't affect the node.
	confCopy := *conf
	conf = &confCopy
	if conf.DataDir != "" {
		absdatadir, err := filepath.Abs(conf.DataDir)
		if err != nil {
			return nil, err
		}
		conf.DataDir = absdatadir
	}
	// Ensure that the AccountManager method works before the node has started.
	// We rely on this in cmd/geth.
	am, err := makeAccountManager(conf)
	if err != nil {
		return nil, err
	}
	if conf.Logger == nil {
		conf.Logger = log.New()
	}
	// Ensure that the instance name doesn't cause weird conflicts with
	// other files in the data directory.
	if strings.ContainsAny(conf.Name, `/\`) {
		return nil, errors.New(`Config.Name must not contain '/' or '\'`)
	}
	if conf.Name == datadirDefaultKeyStore {
		return nil, errors.New(`Config.Name cannot be "` + datadirDefaultKeyStore + `"`)
	}
	if strings.HasSuffix(conf.Name, ".ipc") {
		return nil, errors.New(`Config.Name cannot end in ".ipc"`)
	}

	// Note: any interaction with Config that would create/touch files
	// in the data directory or instance directory is delayed until Start.
	return &testNode{
		accman: am,
		config: conf,
		//serviceFuncs: []FTransServiceConstructor{},
		//eventmux:     new(event.TypeMux),
	}, nil
}

// GetAccMan retrieves the account manager.
func (n *testNode) GetAccMan() *accounts.Manager {
	return n.accman
}

// GetAccAddresses returns the collection of accounts this node manages
func (n *testNode) GetAccAddresses() []string {
	addresses := make([]string, 0) // return [] instead of nil if empty
	for _, wallet := range n.accman.Wallets() {
		for _, account := range wallet.Accounts() {
			addresses = append(addresses, account.Address.String())
		}
	}
	return addresses
}

func defaultDataDir() string {
	// Try to place the data folder in the user's home dir
	home := homeDir()
	if home != "" {
		if runtime.GOOS == "darwin" {
			return filepath.Join(home, "Library", "ContatractTest")
		} else if runtime.GOOS == "windows" {
			return filepath.Join(home, "AppData", "Roaming", "ContatractTest")
		} else {
			return filepath.Join(home, ".contatractTest")
		}
	}
	// As we cannot guess a stable location, return empty and handle later
	return ""
}

func homeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if usr, err := user.Current(); err == nil {
		return usr.HomeDir
	}
	return ""
}

func makeAccountManager(conf *testNodeConfig) (*accounts.Manager, error) {
	var err error
	scryptN, scryptP, keydir := conf.AccountConfig()

	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(keydir, 0700); err != nil {
		return nil, err
	}
	// Assemble the account manager and supported backends
	backends := []accounts.Backend{
		keystore.NewKeyStore(keydir, scryptN, scryptP),
	}

	return accounts.NewManager(backends...), nil
}

func showAccounts(node *testNode) {
	accs := node.GetAccAddresses()
	if len(accs) != 0 {
		strAccounts := ""
		for i, acc := range accs {
			strAccounts += acc
			if i != len(accs)-1 {
				strAccounts += "/"
			}
		}
		fmt.Println(strAccounts)
	} else {

	}
}

func init() {
	conf := defaultNodeConfig()
	nodeTest, err := newTestNode(conf)
	if err != nil {
		log.Crit("Failed to create the test node: %v", err)
	}

	cfg := &Config{
		Name:    conf.Name,
		Version: conf.Version,
		Logger:  conf.Logger,
	}
	ftp, err := New(nil, cfg, nodeTest.GetAccMan())
	if pm, err = NewProtocolManager(100, ftp, nodeTest.GetAccMan()); err != nil {
		log.Crit("Failed to create the protocol manager: %v", err)
	}

	showAccounts(nodeTest)
}

func TestReceiveFile2Disk(t *testing.T) {
	id := "0"
	path := ""
	fsize := uint64(1024)
	fh := &FileTransferReqData{}
	remoteAddr := "0"
	var p *Peer = nil
	guard1 := monkey.PatchInstanceMethod(reflect.TypeOf(pm),
		"SendTransferFileRet", func(_ *ProtocolManager, _, _, _ string, _ bool) {
			return
		})
	defer guard1.Unpatch()

	//case1: os.Create_false
	guard2 := monkey.Patch(os.Create, func(name string) (*os.File, error) {
		return nil, errors.New("error")
	})

	Convey("TestReceiveFile2Disk case1: os.Create_false", t, func() {
		So(pm.receiveFile2Disk(id, path, fsize, fh, remoteAddr, p), ShouldBeFalse)
	})
	guard2.Unpatch()

	//case2: os.Create_true fh_nil
	fh = nil
	Convey("TestReceiveFile2Disk case2: os.Create_true fh_nil", t, func() {
		So(pm.receiveFile2Disk(id, path, fsize, fh, remoteAddr, p), ShouldBeFalse)
	})

	//case3: fh.interactWithCSfailed_true (os.Create_true)
	fh = &FileTransferReqData{}
	fh.interactWithCSfailed = true
	guard3 := monkey.Patch(os.Create, func(name string) (*os.File, error) {
		return &os.File{}, nil
	})
	defer guard3.Unpatch()
	var f *os.File
	guard4 := monkey.PatchInstanceMethod(reflect.TypeOf(f), "Close", func(_ *os.File) error {
		return nil
	})
	defer guard4.Unpatch()
	Convey("TestReceiveFile2Disk case3: fh.interactWithCSfailed_true", t, func() {
		So(pm.receiveFile2Disk(id, path, fsize, fh, remoteAddr, p), ShouldBeFalse)
	})

	// case4: pm.getLocalData_nil (os.Create_true fh.interactWithCSfailed_false)
	fh.interactWithCSfailed = false
	guard5 := monkey.PatchInstanceMethod(reflect.TypeOf(pm), "GetLocalData", func(_ *ProtocolManager, _ uint32) *FTFileData {
		return nil
	})
	Convey("TestReceiveFile2Disk case4: pm.getLocalData_nil", t, func() {
		So(pm.receiveFile2Disk(id, path, fsize, fh, remoteAddr, p), ShouldBeFalse)
	})
	guard5.Unpatch()

	// case5: pm.GetEncryptKey()_nil (os.Create_true fh.interactWithCSfailed_false pm.getLocalData()_!nil)
	guard6 := monkey.PatchInstanceMethod(reflect.TypeOf(pm), "GetLocalData", func(_ *ProtocolManager, _ uint32) *FTFileData {
		return &FTFileData{RawSize: 1024}
	})
	defer guard6.Unpatch()
	guard7 := monkey.PatchInstanceMethod(reflect.TypeOf(pm), "GetEncryptKey", func(_ *ProtocolManager, _ string) []byte {
		return nil
	})
	Convey("TestReceiveFile2Disk case5: pm.getLocalData()_!nil pm.GetEncryptKey_nil", t, func() {
		So(pm.receiveFile2Disk(id, path, fsize, fh, remoteAddr, p), ShouldBeFalse)
	})
	guard7.Unpatch()

	// case6: datacrypto.Decrypt()_nil (os.Create_true fh.interactWithCSfailed_false pm.getLocalData()_!nil pm.GetEncryptKey()_!nil)
	guard8 := monkey.PatchInstanceMethod(reflect.TypeOf(pm), "GetEncryptKey", func(_ *ProtocolManager, _ string) []byte {
		return []byte("test")
	})
	defer guard8.Unpatch()
	guard9 := monkey.Patch(datacrypto.Decrypt, func(_ uint8, _ []byte, _ []byte, _ bool) ([]byte, error) {
		return nil, errors.New("error")
	})
	Convey("TestReceiveFile2Disk case6: datacrypto.Decrypt()_nil", t, func() {
		So(pm.receiveFile2Disk(id, path, fsize, fh, remoteAddr, p), ShouldBeFalse)
	})
	guard9.Unpatch()

	//case7: SaveFileBuff2Disk()_error (os.Create_true fh.interactWithCSfailed_false pm.getLocalData()_!nil pm.GetEncryptKey()_!nil datacrypto.Decrypt()_!nil)
	guard10 := monkey.Patch(datacrypto.Decrypt, func(_ uint8, _ []byte, _ []byte, _ bool) ([]byte, error) {
		return []byte("test"), nil
	})
	defer guard10.Unpatch()
	guard11 := monkey.Patch(SaveFileBuff2Disk, func(file *os.File, fileBuff []byte, rsize int) (int, error) {
		return 0, errors.New("error")
	})
	Convey("TestReceiveFile2Disk case7: file.Write()_error", t, func() {
		So(pm.receiveFile2Disk(id, path, fsize, fh, remoteAddr, p), ShouldBeFalse)
	})
	guard11.Unpatch()

	//case8: fsize_1024 (os.Create_true fh.interactWithCSfailed_false pm.getLocalData()_!nil pm.GetEncryptKey()_!nil datacrypto.Decrypt()_!nil SaveFileBuff2Disk_!error)
	// In this case, function file.Write(fileBuff[:rsize]) can not be mocked correctly, so we add function SaveFileBuff2Disk()
	fsize = 1024
	guard12 := monkey.Patch(SaveFileBuff2Disk, func(file *os.File, fileBuff []byte, rsize int) (int, error) {
		return 0, nil
	})
	defer guard12.Unpatch()
	var percent *TransferPercent
	guard13 := monkey.PatchInstanceMethod(reflect.TypeOf(percent), "SafeSendChTo", func(_ *TransferPercent, _ chan TransferPercent) bool {
		return false
	})
	defer guard13.Unpatch()
	Convey("TestReceiveFile2Disk case8: fsize_1024", t, func() {
		fsize = 1024
		So(pm.receiveFile2Disk(id, path, fsize, fh, remoteAddr, p), ShouldBeTrue)
	})
}
