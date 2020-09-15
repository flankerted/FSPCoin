package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"time"

	"gopkg.in/urfave/cli.v1"

	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/console"
	"github.com/contatract/go-contatract/eleWallet/backend"
	"github.com/contatract/go-contatract/eleWallet/flags"
	"github.com/contatract/go-contatract/eleWallet/gui"
	externgui "github.com/contatract/go-contatract/eleWallet/gui/external"
	webgui "github.com/contatract/go-contatract/eleWallet/gui/web"
	"github.com/contatract/go-contatract/eleWallet/node"
	"github.com/contatract/go-contatract/eleWallet/utils"
	"github.com/contatract/go-contatract/internal/debug"
	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/metrics"
)

const ethToWei = 1 << 17

var (
	// Git SHA1 commit hash of the release (set via linker flags)
	gitCommit = ""
	// Ethereum address of the Geth release oracle.
	relOracle = common.HexToAddress("0xfa7b9770ca4cb04296cac84f37736d4041251cdf")
	// The app that holds all commands and flags.
	app = flags.NewApp(gitCommit, "the go-contatract command line interface")
	// The ele that holds all backend commands.
	ele *backend.Elephant = nil
	// flags that configure the node
	nodeFlags = []cli.Flag{
		flags.LanguageFlag,
		flags.RemoteIPFlag,
		flags.RemoteFTransPortFlag,
		flags.RemoteRPCPortFlag,
		flags.DataDirFlag,
		flags.TestnetFlag,
		flags.NetworkIdFlag,
		flags.RinkebyFlag,
		flags.DeveloperFlag,
		flags.LightModeFlag,
		flags.MaxPeersFlag,
		flags.MaxPendingPeersFlag,
		flags.UDPListenPortFlag,
		flags.NATFlag,
		flags.BootnodesFlag,
		flags.BootnodesV4Flag,
		flags.BootnodesV5Flag,
		flags.NoDiscoverFlag,
		flags.DiscoveryV5Flag,
		flags.GuiPortFlag,
	}

	mainLog log.Logger = nil
)

func init() {
	// Initialize the CLI app and start eleWallet
	app.Action = eleWallet
	app.HideVersion = true // we have a command to print the version
	app.Copyright = "Copyright 2018 The eleWallet Authors"
	app.Flags = append(app.Flags, nodeFlags...)
	app.Flags = append(app.Flags, debug.Flags...)

	app.Before = func(ctx *cli.Context) error {
		runtime.GOMAXPROCS(runtime.NumCPU())
		if err := debug.Setup(ctx); err != nil {
			return err
		}
		// Start system runtime metrics collection
		go metrics.CollectProcessMetrics(3 * time.Second)

		//utils.SetupNetwork(ctx)
		return nil
	}

	app.After = func(ctx *cli.Context) error {
		debug.Exit()
		console.Stdin.Close() // Resets terminal mode.
		return nil
	}
}

func logInfo(msg string, ctx ...interface{}) {
	log.Info(msg, ctx...)
	if mainLog != nil {
		mainLog.Info(msg, ctx...)
	}
}

func logError(msg string, ctx ...interface{}) {
	log.Error(msg, ctx...)
	if mainLog != nil {
		mainLog.Error(msg, ctx...)
	}
}

func logCrit(msg string, ctx ...interface{}) {
	if mainLog != nil {
		mainLog.Error(msg, ctx...)
	}
	log.Crit(msg, ctx...)
}

func main() {
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func sleepDefault() {
	time.Sleep(time.Second / 3)
}

func eleWallet(ctx *cli.Context) error {
	var (
		n      *node.Node
		cfg    *eleWalletConfig
		ui     gui.EleWalletGUI
		abort  = make(chan os.Signal, 1)
		sigc   = make(chan os.Signal, 1)
		enLang = strings.ToLower(ctx.GlobalString(flags.LanguageFlag.Name)) == "en"
	)

	// Create a new GUI
	signal.Notify(abort, os.Interrupt)
	if ctx.GlobalBool(flags.DeveloperFlag.GetName()) {
		ui = webgui.NewWebGUI()

		// Show webGUI
		go ui.Show()
	} else {
		ui = externgui.NewExternalGUI(ctx, abort)

		// Show GUI
		go ui.Show()

		ctx.Set(flags.RemoteIPFlag.GetName(), "") // Use GUI to set remote ip address
		for !externgui.DataDirSetFlag {
			select {
			case <-abort:
				// User forcefully quite the console
				logInfo("caught interrupt, exiting")
				return nil
			default:
				sleepDefault()
			}
		}
	}
	// ui = qt.NewQtGUI(abort)

	// Init the backend
	n, cfg = NewClientNode(ctx)
	ele = backend.New(n, ctx)
	ui.Init(enLang, ele)
	ui.SetLogger(ele.Log())
	mainLog = ele.Log()

	// Waiting for the remote IP setting
	for !ele.RemoteIPEntered() {
		select {
		case <-abort:
			// User forcefully quite the console
			logInfo("caught interrupt, exiting")
			return nil
		default:
			sleepDefault()
		}
	}

	// Then initialize the backend
	nodeId, err := ele.Init()
	if err != nil {
		logCrit("Backend initialization failure", "err", err.Error())
	}
	initClientNode(ctx, n, cfg, nodeId)

	// Waiting for a new account
	for !ele.PassphraseEntered() {
		select {
		case <-abort:
			// User forcefully quite the console
			logInfo("caught interrupt, exiting")
			return nil
		default:
			sleepDefault()
		}
	}

	// Start the client node
	startNode(n, sigc)
	logInfo("We have successfully started the node")

	// Waiting for interrupt
	// Monitor Ctrl-C too in case the input is empty and we need to bail
	for {
		select {
		case <-abort:
			// User forcefully quite the console
			sigc <- os.Interrupt
			time.Sleep(time.Millisecond * 500)
			return nil
		}
	}
}

// startNode boots up the system node to find and connect the gctt nodes
func startNode(stack *node.Node, sigc chan os.Signal) {
	// Start up the node itself
	if err := stack.Start(); err != nil {
		utils.Fatalf("Error starting protocol stack: %v", err)
	}
	go func(sigc chan os.Signal) {
		signal.Notify(sigc, os.Interrupt)
		defer signal.Stop(sigc)
		<-sigc
		logInfo("Got interrupt, shutting down...")
		stack.Stop()
		//go stack.Stop()
		//for i := 3; i > 0; i-- {
		// <-sigc
		// if i > 1 {
		//    log.Warn("Already shutting down, interrupt more to panic.", "times", i-1)
		// }
		//}
		//debug.Exit() // ensure trace and CPU profile data is flushed.
		//debug.LoudPanic("boom")
	}(sigc)
}
