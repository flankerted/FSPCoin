// Copyright 2018 The go-contatract Authors
// This file is part of go-contatract.
//
// go-contatract is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-contatract is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-contatract. If not, see <http://www.gnu.org/licenses/>.

// geth is the official command-line client for Ethereum.
package main

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/contatract/go-contatract/accounts"
	"github.com/contatract/go-contatract/accounts/keystore"
	"github.com/contatract/go-contatract/cmd/utils"
	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/console"
	"github.com/contatract/go-contatract/elephant"
	"github.com/contatract/go-contatract/eth"
	"github.com/contatract/go-contatract/ethclient"
	"github.com/contatract/go-contatract/internal/debug"
	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/metrics"
	"github.com/contatract/go-contatract/node"

	"github.com/mattn/go-colorable"
	"gopkg.in/urfave/cli.v1"
)

const (
	clientIdentifier = "gctt" // Client identifier to advertise over the network
)

var (
	// Git SHA1 commit hash of the release (set via linker flags)
	gitCommit = ""
	// Ethereum address of the Geth release oracle.
	relOracle = common.HexToAddress("0xfa7b9770ca4cb04296cac84f37736d4041251cdf")
	// The app that holds all commands and flags.
	app = utils.NewApp(gitCommit, "the go-contatract command line interface")
	// flags that configure the node
	nodeFlags = []cli.Flag{
		utils.IdentityFlag,
		utils.UnlockedAccountFlag,
		utils.PasswordFileFlag,
		utils.BootnodesFlag,
		utils.BootnodesV4Flag,
		utils.BootnodesV5Flag,
		utils.DataDirFlag,
		utils.KeyStoreDirFlag,
		utils.NoUSBFlag,
		//utils.DashboardEnabledFlag,
		//utils.DashboardAddrFlag,
		//utils.DashboardPortFlag,
		//utils.DashboardRefreshFlag,
		//utils.DashboardAssetsFlag,
		utils.EthashCacheDirFlag,
		utils.EthashCachesInMemoryFlag,
		utils.EthashCachesOnDiskFlag,
		utils.EthashDatasetDirFlag,
		utils.EthashDatasetsInMemoryFlag,
		utils.EthashDatasetsOnDiskFlag,
		utils.TxPoolNoLocalsFlag,
		utils.TxPoolJournalFlag,
		utils.TxPoolRejournalFlag,
		utils.TxPoolPriceLimitFlag,
		utils.TxPoolPriceBumpFlag,
		utils.TxPoolAccountSlotsFlag,
		utils.TxPoolGlobalSlotsFlag,
		utils.TxPoolAccountQueueFlag,
		utils.TxPoolGlobalQueueFlag,
		utils.TxPoolLifetimeFlag,
		utils.FastSyncFlag,
		utils.LightModeFlag,
		utils.SyncModeFlag,
		utils.GCModeFlag,
		utils.LightServFlag,
		utils.LightPeersFlag,
		utils.LightKDFFlag,
		utils.CacheFlag,
		utils.CacheDatabaseFlag,
		utils.CacheGCFlag,
		utils.TrieCacheGenFlag,
		utils.ListenPortFlag,
		utils.MaxPeersFlag,
		utils.MaxPendingPeersFlag,
		utils.EtherbaseFlag,
		utils.GasPriceFlag,
		utils.MinerThreadsFlag,
		utils.EthMiningMainForPBFTTestFlag,
		utils.EthMiningEnabledFlag,
		utils.EleMiningEnabledFlag,
		utils.TargetGasLimitFlag,
		utils.NATFlag,
		utils.NoDiscoverFlag,
		utils.DiscoveryV5Flag,
		utils.NetrestrictFlag,
		utils.NodeKeyFileFlag,
		utils.NodeKeyHexFlag,
		utils.DeveloperFlag,
		utils.DeveloperPeriodFlag,
		utils.TestnetFlag,
		utils.RinkebyFlag,
		utils.VMEnableDebugFlag,
		utils.NetworkIdFlag,
		utils.RPCCORSDomainFlag,
		utils.RPCVirtualHostsFlag,
		utils.EthStatsURLFlag,
		utils.MetricsEnabledFlag,
		utils.FakePoWFlag,
		utils.NoCompactionFlag,
		utils.GpoBlocksFlag,
		utils.GpoPercentileFlag,
		utils.ExtraDataFlag,
		configFileFlag,
		utils.TestBootNodeFlag,
	}

	rpcFlags = []cli.Flag{
		utils.RPCEnabledFlag,
		utils.RPCListenAddrFlag,
		utils.RPCPortFlag,
		utils.RPCApiFlag,
		utils.WSEnabledFlag,
		utils.WSListenAddrFlag,
		utils.WSPortFlag,
		utils.WSApiFlag,
		utils.WSAllowedOriginsFlag,
		utils.IPCDisabledFlag,
		utils.IPCPathFlag,
	}

	/*
		whisperFlags = []cli.Flag{
			utils.WhisperEnabledFlag,
			utils.WhisperMaxMessageSizeFlag,
			utils.WhisperMinPOWFlag,
		}
	*/
	gcttFlags = []cli.Flag{
		utils.TimerTxFlag,
		utils.MinCopyCntFlag,
		utils.MaxCopyCntFlag,
		utils.MinRWFarmerCntFlag,
		utils.CSPassphraseFlag,
		utils.CloseTermLog,
		utils.ShowLine,
	}
)

func init() {
	// Initialize the CLI app and start Geth
	app.Action = gctt
	app.HideVersion = true // we have a command to print the version
	app.Copyright = "Copyright 2018 The go-contatract Authors"
	app.Commands = []cli.Command{
		// See chaincmd.go:
		initCommand,
		importCommand,
		exportCommand,
		copydbCommand,
		removedbCommand,
		dumpCommand,
		// See monitorcmd.go:
		monitorCommand,
		// See accountcmd.go:
		accountCommand,
		walletCommand,
		// See consolecmd.go:
		consoleCommand,
		attachCommand,
		javascriptCommand,
		// See misccmd.go:
		makecacheCommand,
		makedagCommand,
		versionCommand,
		bugCommand,
		licenseCommand,
		// See config.go
		dumpConfigCommand,

		// added for elephant
		elephantCommand,
	}
	sort.Sort(cli.CommandsByName(app.Commands))

	app.Flags = append(app.Flags, nodeFlags...)
	app.Flags = append(app.Flags, rpcFlags...)
	app.Flags = append(app.Flags, consoleFlags...)
	app.Flags = append(app.Flags, debug.Flags...)
	//app.Flags = append(app.Flags, whisperFlags...)
	app.Flags = append(app.Flags, gcttFlags...)

	app.Before = func(ctx *cli.Context) error {
		runtime.GOMAXPROCS(runtime.NumCPU())
		if err := debug.Setup(ctx); err != nil {
			return err
		}
		// Start system runtime metrics collection
		go metrics.CollectProcessMetrics(3 * time.Second)

		utils.SetupNetwork(ctx)
		return nil
	}

	app.After = func(ctx *cli.Context) error {
		debug.Exit()
		console.Stdin.Close() // Resets terminal mode.
		return nil
	}
}

// JiangHan 打印产品标题
func printLogo() {
	var Output = colorable.NewColorableStdout()
	var f = 31
	var f1 = 41

	fmt.Fprintf(Output, "\n")
	/*
		    for b := 40; b <= 47; b++ { // 背景色彩 = 40-47
		        for f := 30; f <= 37; f++ { // 前景色彩 = 30-37
		            for d := range []int{0, 1, 4, 5, 7, 8} { // 显示方式 = 0,1,4,5,7,8
		                fmt.Fprintf(Output, " %c[%d;%d;%dm%s(f=%d,b=%d,d=%d)%c[0m ", 0x1B, d, b, f, "", f, b, d, 0x1B)
		            }
		            fmt.Fprintf(Output,"\n")
		        }
		        fmt.Fprintf(Output,"\n")
			}
	*/

	for i := 0; i < 5; i++ {
		//fmt.Fprintf(Output, " ");
		for b := 0; b < 47; b++ {
			// 背景色彩 = 40-47
			if b < 8 {
				f = 31
				f1 = 41
			} else if b < 16 {
				f = 33
				f1 = 43
			} else if b < 24 {
				f = 32
				f1 = 42
			} else if b < 32 {
				f = 36
				f1 = 46
			} else if b < 40 {
				f = 34
				f1 = 44
			} else {
				f = 35
				f1 = 45
			}
			fmt.Fprintf(Output, "%c[%d;%d;%dm%s=%c[0m", 0x1B, 1, f1, f, "", 0x1B)
		}
		fmt.Fprintf(Output, "\n")
	}

	fmt.Fprintf(Output, "%c[%d;%d;%dm%s%c[1m ___________________________________________ %c[0m\n", 0x1B, 1, 40, 33, "", 0x1B, 0x1B)
	fmt.Fprintf(Output, "%c[%d;%d;%dm%s%c[1m|                                           |%c[0m\n", 0x1B, 1, 40, 33, "", 0x1B, 0x1B)
	fmt.Fprintf(Output, "%c[%d;%d;%dm%s%c[1m|                                           |%c[0m\n", 0x1B, 1, 40, 33, "", 0x1B, 0x1B)
	fmt.Fprintf(Output, "%c[%d;%d;%dm%s%c[1m|                                           |%c[0m\n", 0x1B, 1, 40, 33, "", 0x1B, 0x1B)
	fmt.Fprintf(Output, "%c[%d;%d;%dm%s%c[1m|                                           |%c[0m\n", 0x1B, 1, 40, 33, "", 0x1B, 0x1B)
	fmt.Fprintf(Output, "%c[%d;%d;%dm%s%c[1m|              ____                         |%c[0m\n", 0x1B, 1, 40, 33, "", 0x1B, 0x1B)
	fmt.Fprintf(Output, "%c[%d;%d;%dm%s%c[1m|   .d~~@    _@`  `a,   |~~~~~@  |@~~~~@|   |%c[0m\n", 0x1B, 1, 40, 33, "", 0x1B, 0x1B)
	fmt.Fprintf(Output, "%c[%d;%d;%dm%s%c[1m|  ./'  |   .@     `a   |     @  ||    ||   |%c[0m\n", 0x1B, 1, 40, 33, "", 0x1B, 0x1B)
	fmt.Fprintf(Output, "%c[%d;%d;%dm%s%c[1m|  d' _za   d' |/-, |L  |__ ._@  |L_ ._a|   |%c[0m\n", 0x1B, 1, 40, 33, "", 0x1B, 0x1B)
	fmt.Fprintf(Output, "%c[%d;%d;%dm%s%c[1m|  @ .@zaL  a  @ `Lzz[  `~| ||`  `~@ ||~    |%c[0m\n", 0x1B, 1, 40, 33, "", 0x1B, 0x1B)
	fmt.Fprintf(Output, "%c[%d;%d;%dm%s%c[1m|  L `@| || @  L .|`||    | ||     @ ||     |%c[0m\n", 0x1B, 1, 40, 33, "", 0x1B, 0x1B)
	fmt.Fprintf(Output, "%c[%d;%d;%dm%s%c[1m|  |  |b || q, `-~' |'    | ||     @ ||     |%c[0m\n", 0x1B, 1, 40, 33, "", 0x1B, 0x1B)
	fmt.Fprintf(Output, "%c[%d;%d;%dm%s%c[1m|  `b    || `L     .@     | ||     @ ||     |%c[0m\n", 0x1B, 1, 40, 33, "", 0x1B, 0x1B)
	fmt.Fprintf(Output, "%c[%d;%d;%dm%s%c[1m|   ~z  _@   `L_  _/'     | ||     @ ||     |%c[0m\n", 0x1B, 1, 40, 33, "", 0x1B, 0x1B)
	fmt.Fprintf(Output, "%c[%d;%d;%dm%s%c[1m|    `~~~      ~~~~       ~~~'     ~~~'     |%c[0m\n", 0x1B, 1, 40, 33, "", 0x1B, 0x1B)
	fmt.Fprintf(Output, "%c[%d;%d;%dm%s%c[1m|                                           |%c[0m\n", 0x1B, 1, 40, 33, "", 0x1B, 0x1B)
	fmt.Fprintf(Output, "%c[%d;%d;%dm%s%c[1m|                                           |%c[0m\n", 0x1B, 1, 40, 33, "", 0x1B, 0x1B)
	fmt.Fprintf(Output, "%c[%d;%d;%dm%s%c[1m ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~ %c[0m\n", 0x1B, 1, 40, 33, "", 0x1B, 0x1B)

	/*
		//fmt.Fprintf(Output," ===============================================\n")
		fmt.Fprintf(Output,"|%c[%d;%d;%dm%s%c[1m                                               %c[0m|\n",0x1B, 0, 44, 31, "", 0x1B, 0x1B)
		fmt.Fprintf(Output,"|%c[%d;%d;%dm%s%c[1m   The Future's                                %c[0m|\n",0x1B, 0, 44, 31, "", 0x1B, 0x1B)
		fmt.Fprintf(Output,"|%c[%d;%d;%dm%s%c[1m                                               %c[0m|\n",0x1B, 0, 44, 31, "", 0x1B, 0x1B)
		fmt.Fprintf(Output,"|%c[%d;%d;%dm%s%c[1m",0x1B, 1, 44, 31, "", 0x1B)
		fmt.Fprintf(Output,"                   Contatract                  ")
		fmt.Fprintf(Output,"%c[0m|\n", 0x1B);
		fmt.Fprintf(Output,"|%c[%d;%d;%dm%s%c[1m                                               %c[0m|\n",0x1B, 0, 44, 31, "", 0x1B, 0x1B)
		fmt.Fprintf(Output,"|%c[%d;%d;%dm%s%c[1m                                               %c[0m|\n",0x1B, 0, 44, 31, "", 0x1B, 0x1B)
		fmt.Fprintf(Output,"|%c[%d;%d;%dm%s%c[1m                                 ver 0.1       %c[0m|\n",0x1B, 0, 44, 31, "", 0x1B, 0x1B)
		fmt.Fprintf(Output,"|%c[%d;%d;%dm%s%c[1m                                               %c[0m|\n",0x1B, 0, 44, 31, "", 0x1B, 0x1B)
		//fmt.Fprintf(Output," =============================================== \n")
	*/
	for i := 0; i < 5; i++ {
		//fmt.Fprintf(Output, " ");
		for b := 47; b > 0; b-- {
			// 背景色彩 = 40-47
			if b < 8 {
				f = 31
				f1 = 41
			} else if b < 16 {
				f = 33
				f1 = 43
			} else if b < 24 {
				f = 32
				f1 = 42
			} else if b < 32 {
				f = 36
				f1 = 46
			} else if b < 40 {
				f = 34
				f1 = 44
			} else {
				f = 35
				f1 = 45
			}
			fmt.Fprintf(Output, "%c[%d;%d;%dm%s=%c[0m", 0x1B, 1, f1, f, "", 0x1B)
		}
		fmt.Fprintf(Output, "\n")
	}
	fmt.Fprintf(Output, "\n\n")
}

func main() {
	printLogo()
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// geth is the main entry point into the system if no special subcommand is ran.
// It creates a default node based on the command line arguments and runs it in
// blocking mode, waiting for it to be shut down.
func gctt(ctx *cli.Context) error {
	node := makeFullNode(ctx)
	startNode(ctx, node)
	node.Wait()
	return nil
}

// startNode boots up the system node and all registered protocols, after which
// it unlocks any requested accounts, and starts the RPC/IPC interfaces and the
// miner.
func startNode(ctx *cli.Context, stack *node.Node) {
	// Start up the node itself
	utils.StartNode(stack)

	// Unlock any account specifically requested
	ks := stack.AccountManager().Backends(keystore.KeyStoreType)[0].(*keystore.KeyStore)

	passwords := utils.MakePasswordList(ctx)
	unlocks := strings.Split(ctx.GlobalString(utils.UnlockedAccountFlag.Name), ",")
	for i, account := range unlocks {
		if trimmed := strings.TrimSpace(account); trimmed != "" {
			unlockAccount(ctx, ks, trimmed, i, passwords)
		}
	}
	// Register wallet event handlers to open and auto-derive wallets
	events := make(chan accounts.WalletEvent, 16)
	stack.AccountManager().Subscribe(events)

	go func() {
		// Create an chain state reader for self-derivation
		rpcClient, err := stack.Attach()
		if err != nil {
			utils.Fatalf("Failed to attach to self: %v", err)
		}
		stateReader := ethclient.NewClient(rpcClient)

		// Open any wallets already attached
		for _, wallet := range stack.AccountManager().Wallets() {
			if err := wallet.Open(""); err != nil {
				log.Warn("Failed to open wallet", "url", wallet.URL(), "err", err)
			}
		}
		// Listen for wallet event till termination
		for event := range events {
			switch event.Kind {
			case accounts.WalletArrived:
				if err := event.Wallet.Open(""); err != nil {
					log.Warn("New wallet appeared, failed to open", "url", event.Wallet.URL(), "err", err)
				}
			case accounts.WalletOpened:
				status, _ := event.Wallet.Status()
				log.Info("New wallet appeared", "url", event.Wallet.URL(), "status", status)

				if event.Wallet.URL().Scheme == "ledger" {
					event.Wallet.SelfDerive(accounts.DefaultLedgerBaseDerivationPath, stateReader)
				} else {
					event.Wallet.SelfDerive(accounts.DefaultBaseDerivationPath, stateReader)
				}

			case accounts.WalletDropped:
				log.Info("Old wallet dropped", "url", event.Wallet.URL())
				event.Wallet.Close()
			}
		}
	}()

	// Start auxiliary services if enabled
	if ctx.GlobalBool(utils.EthMiningEnabledFlag.Name) || ctx.GlobalBool(utils.DeveloperFlag.Name) {
		// Mining only makes sense if a full Ethereum node is running
		if ctx.GlobalBool(utils.LightModeFlag.Name) || ctx.GlobalString(utils.SyncModeFlag.Name) == "light" {
			utils.Fatalf("Light clients do not support mining")
		}

		// JiangHan:
		// 从注册的服务中找到 Ethereum 服务并根据 flag 自动开启挖矿
		log.Info("[ETH]开始挖矿")
		var ethereum *eth.Ethereum
		if err := stack.Service(&ethereum); err != nil {
			utils.Fatalf("Ethereum service not running: %v", err)
		}
		// Use a reduced number of threads if requested
		if threads := ctx.GlobalInt(utils.MinerThreadsFlag.Name); threads > 0 {
			type threaded interface {
				SetThreads(threads int)
			}
			if th, ok := ethereum.Engine().(threaded); ok {
				th.SetThreads(threads)
			}
		}
		// Set the gas price to the limits from the CLI and start mining
		ethereum.TxPool().SetGasPrice(utils.GlobalBig(ctx, utils.GasPriceFlag.Name))
		if err := ethereum.StartMining(true); err != nil {
			utils.Fatalf("Failed to start mining: %v", err)
		}

		if ctx.GlobalBool(utils.EthMiningMainForPBFTTestFlag.Name) {
			log.Info("Eth enable mining for BFT test as a main node")
			ethereum.SetMainNodeForBFTFlag(true)
		}
	}

	if ctx.GlobalBool(utils.EleMiningEnabledFlag.Name) || ctx.GlobalBool(utils.DeveloperFlag.Name) {
		// Mining only makes sense if a full Ethereum node is running
		if ctx.GlobalBool(utils.LightModeFlag.Name) || ctx.GlobalString(utils.SyncModeFlag.Name) == "light" {
			utils.Fatalf("Light clients do not support mining")
		}

		// 从注册的服务中找到 Elephant 服务并根据 flag 自动开启挖矿
		log.Info("[ELEPHANT]开始打包区块")
		var elephant *elephant.Elephant
		if err := stack.Service(&elephant); err != nil {
			utils.Fatalf("Elephant service not running: %v", err)
		}

		// Set the gas price to the limits from the CLI and start mining
		// elephant.TxPool().SetGasPrice(utils.GlobalBig(ctx, utils.GasPriceFlag.Name)) // Must not drop claim tx
		if err := elephant.StartMining(true); err != nil {
			utils.Fatalf("Failed to start mining: %v", err)
		}
	}
}
