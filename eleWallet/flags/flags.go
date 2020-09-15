package flags

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/contatract/go-contatract/eleWallet/node"
	"github.com/contatract/go-contatract/eleWallet/utils"
	"github.com/contatract/go-contatract/ftransfer"
	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/p2p"
	"github.com/contatract/go-contatract/p2p/discover"
	"github.com/contatract/go-contatract/p2p/discv5"
	"github.com/contatract/go-contatract/p2p/nat"
	"github.com/contatract/go-contatract/p2p/netutil"
	"github.com/contatract/go-contatract/params"
	"gopkg.in/urfave/cli.v1"
)

// DefaultConfig contains default settings for use on the Ethereum main net.
var DefaultConfig = Config{
	NetworkId:        1,
	RemoteIP:         "",
	RemoteFTransPort: 30303,
	RemoteRPCPort:    8545,
	GuiPort:          10309,
}

type Config struct {
	// Protocol options
	NetworkId        uint64 // Network ID to use for selecting peers to connect to
	RemoteIP         string // Remote connection's IP address
	RemoteFTransPort uint   // Remote connection's port
	RemoteRPCPort    uint   // Remote http rpc's port
	DatasetDir       DirectoryString
	GuiPort          uint // Connection's port for gui
}

func init() {
	home := os.Getenv("HOME")
	if home == "" {
		if user, err := user.Current(); err == nil {
			home = user.HomeDir
		}
	}
	if runtime.GOOS == "windows" {
		DefaultConfig.DatasetDir = DirectoryString{filepath.Join(home, "AppData", "EleWallet")}
	} else {
		DefaultConfig.DatasetDir = DirectoryString{filepath.Join(home, ".eleWallet")}
	}
}

// NewApp creates an app with sane defaults.
func NewApp(gitCommit, usage string) *cli.App {
	app := cli.NewApp()
	app.Name = filepath.Base(os.Args[0])
	app.Author = ""
	//app.Authors = nil
	app.Email = ""
	app.Version = params.Version
	if len(gitCommit) >= 8 {
		app.Version += "-" + gitCommit[:8]
	}
	app.Usage = usage
	return app
}

var (
	// General settings
	//PasswordFlag = cli.StringFlag {
	//    Name:  "pw",
	//    Usage: `The password of the first account`,
	//    Value: "block",
	//}

	LanguageFlag = cli.StringFlag{
		Name:  "lang",
		Usage: "Select the language of the GUI",
		Value: "en",
	}
	RemoteIPFlag = cli.StringFlag{
		Name:  "ip",
		Usage: "Remote connection's IP address",
		Value: DefaultConfig.RemoteIP,
	}
	RemoteFTransPortFlag = cli.UintFlag{
		Name:  "ftpport",
		Usage: "Remote connection's port for file transfer protocol",
		Value: DefaultConfig.RemoteFTransPort,
	}
	RemoteRPCPortFlag = cli.UintFlag{
		Name:  "rpcport",
		Usage: "Remote connection's port for http rpc communication",
		Value: DefaultConfig.RemoteRPCPort,
	}
	DataDirFlag = DirectoryFlag{
		Name:  "dir",
		Usage: "Directory for the data storage",
		Value: DefaultConfig.DatasetDir,
	}
	NetworkIdFlag = cli.Uint64Flag{
		Name:  "networkid",
		Usage: "Network identifier (integer, 1=Frontier, 2=Morden (disused), 3=Ropsten, 4=Rinkeby)",
		Value: DefaultConfig.NetworkId,
	}
	TestnetFlag = cli.BoolFlag{
		Name:  "testnet",
		Usage: "Ropsten network: pre-configured proof-of-work test network",
	}
	RinkebyFlag = cli.BoolFlag{
		Name:  "rinkeby",
		Usage: "Rinkeby network: pre-configured proof-of-authority test network",
	}
	DeveloperFlag = cli.BoolFlag{
		Name:  "dev",
		Usage: "Ephemeral proof-of-authority network with a pre-funded developer account, mining enabled",
	}
	LightModeFlag = cli.BoolFlag{ //JiangHan：轻节点，只下载各级hash
		Name:  "light",
		Usage: "Enable light client mode",
	}

	// Network Settings
	MaxPeersFlag = cli.IntFlag{
		Name:  "maxpeers",
		Usage: "Maximum number of network peers (network disabled if set to 0)",
		Value: 25,
	}
	MaxPendingPeersFlag = cli.IntFlag{
		Name:  "maxpendpeers",
		Usage: "Maximum number of pending connection attempts (defaults used if set to 0)",
		Value: 0,
	}
	UDPListenPortFlag = cli.IntFlag{
		Name:  "udpport",
		Usage: "UDP network listening port",
		Value: 30303,
	}
	NATFlag = cli.StringFlag{
		Name:  "nat",
		Usage: "NAT port mapping mechanism (any|none|upnp|pmp|extip:<IP>)",
		Value: "any",
	}
	BootnodesFlag = cli.StringFlag{
		Name:  "bootnodes",
		Usage: "Comma separated enode URLs for P2P discovery bootstrap (set v4+v5 instead for light servers)",
		Value: "",
	}
	BootnodesV4Flag = cli.StringFlag{
		Name:  "bootnodesv4",
		Usage: "Comma separated enode URLs for P2P v4 discovery bootstrap (light server, full nodes)",
		Value: "",
	}
	BootnodesV5Flag = cli.StringFlag{
		Name:  "bootnodesv5",
		Usage: "Comma separated enode URLs for P2P v5 discovery bootstrap (light server, light nodes)",
		Value: "",
	}
	NoDiscoverFlag = cli.BoolFlag{
		Name:  "nodiscover",
		Usage: "Disables the peer discovery mechanism (manual peer addition)",
	}
	DiscoveryV5Flag = cli.BoolFlag{
		Name:  "v5disc",
		Usage: "Enables the experimental RLPx V5 (Topic Discovery) mechanism",
	}
	GuiPortFlag = cli.UintFlag{
		Name:  "guiport",
		Usage: "Connection's port for gui",
		Value: DefaultConfig.GuiPort,
	}
)

func SetP2PConfig(ctx *cli.Context, cfg *p2p.ClientCfg) {
	lightClient := ctx.GlobalBool(LightModeFlag.Name)
	setUDPListenAddress(ctx, cfg)
	setNAT(ctx, cfg)
	setBootstrapNodes(ctx, cfg)
	setBootstrapNodesV5(ctx, cfg)

	//if ctx.GlobalIsSet(MaxPeersFlag.Name) {
	//	cfg.MaxPeers = ctx.GlobalInt(MaxPeersFlag.Name)
	//}
	//ethPeers := cfg.MaxPeers
	//log.Info("Maximum peer count", "ETH", ethPeers, "total", cfg.MaxPeers)

	if ctx.GlobalIsSet(MaxPendingPeersFlag.Name) {
		cfg.MaxPendingPeers = ctx.GlobalInt(MaxPendingPeersFlag.Name)
	}
	if ctx.GlobalIsSet(NoDiscoverFlag.Name) {
		cfg.NoDiscovery = true
	}
	if ctx.GlobalIsSet(NetworkIdFlag.Name) {
		cfg.NetworkId = ctx.GlobalUint64(NetworkIdFlag.Name)
	}
	if ctx.GlobalIsSet(RemoteIPFlag.Name) {
		cfg.RemoteIP = ctx.GlobalString(RemoteIPFlag.Name)

		list, err := netutil.ParseNetlist(cfg.RemoteIP + "/32")
		if err != nil {
			log.Crit(fmt.Sprintf("Option %q: %v", cfg.RemoteIP, err))
		}
		cfg.NetRestrict = list
	}
	cfg.RemotePort = uint16(ctx.GlobalUint(RemoteFTransPortFlag.Name))

	// if we're running a light client or server, force enable the v5 peer discovery
	// unless it is explicitly disabled with --nodiscover note that explicitly specifying
	// --v5disc overrides --nodiscover, in which case the later only disables v4 discovery
	forceV5Discovery := (lightClient) && !ctx.GlobalBool(NoDiscoverFlag.Name)
	if ctx.GlobalIsSet(DiscoveryV5Flag.Name) {
		cfg.DiscoveryV5 = ctx.GlobalBool(DiscoveryV5Flag.Name)
	} else if forceV5Discovery {
		cfg.DiscoveryV5 = true
	}

	if ctx.GlobalBool(DeveloperFlag.Name) {
		// --dev mode can't use p2p networking.
		//cfg.MaxPeers = 0
		cfg.UDPListenAddr = ":0"
		cfg.NoDiscovery = true
		cfg.DiscoveryV5 = false
	}
}

// setBootstrapNodes creates a list of bootstrap nodes from the command line
// flags, reverting to pre-configured ones if none have been specified.
func setBootstrapNodes(ctx *cli.Context, cfg *p2p.ClientCfg) {
	urls := params.MainnetBootnodes
	switch {
	case ctx.GlobalIsSet(BootnodesFlag.Name) || ctx.GlobalIsSet(BootnodesV4Flag.Name):
		if ctx.GlobalIsSet(BootnodesV4Flag.Name) {
			urls = strings.Split(ctx.GlobalString(BootnodesV4Flag.Name), ",")
		} else {
			urls = strings.Split(ctx.GlobalString(BootnodesFlag.Name), ",")
		}
	case ctx.GlobalBool(TestnetFlag.Name):
		urls = params.TestnetBootnodes
	case ctx.GlobalBool(RinkebyFlag.Name):
		urls = params.RinkebyBootnodes
	case cfg.BootstrapNodes != nil:
		return // already set, don't apply defaults.
	}

	//log.Info("[******************************]");
	cfg.BootstrapNodes = make([]*discover.Node, 0, len(urls))
	for _, url := range urls {
		log.Info("【BootTrapNodes】", "[url]=", url)
		node, err := discover.ParseNode(url)
		if err != nil {
			log.Error("Bootstrap URL invalid", "enode", url, "err", err)
			continue
		}
		cfg.BootstrapNodes = append(cfg.BootstrapNodes, node)
	}
}

// setBootstrapNodesV5 creates a list of bootstrap nodes from the command line
// flags, reverting to pre-configured ones if none have been specified.
func setBootstrapNodesV5(ctx *cli.Context, cfg *p2p.ClientCfg) {
	urls := params.DiscoveryV5Bootnodes
	switch {
	case ctx.GlobalIsSet(BootnodesFlag.Name) || ctx.GlobalIsSet(BootnodesV5Flag.Name):
		if ctx.GlobalIsSet(BootnodesV5Flag.Name) {
			urls = strings.Split(ctx.GlobalString(BootnodesV5Flag.Name), ",")
		} else {
			urls = strings.Split(ctx.GlobalString(BootnodesFlag.Name), ",")
		}
	case ctx.GlobalBool(RinkebyFlag.Name):
		urls = params.RinkebyBootnodes
	case cfg.BootstrapNodesV5 != nil:
		return // already set, don't apply defaults.
	}

	cfg.BootstrapNodesV5 = make([]*discv5.Node, 0, len(urls))
	for _, url := range urls {
		node, err := discv5.ParseNode(url)
		if err != nil {
			log.Error("Bootstrap URL invalid", "enode", url, "err", err)
			continue
		}
		cfg.BootstrapNodesV5 = append(cfg.BootstrapNodesV5, node)
	}
}

// setUDPListenAddress creates a UDP listening address string from set command
// line flags.
func setUDPListenAddress(ctx *cli.Context, cfg *p2p.ClientCfg) {
	if ctx.GlobalIsSet(UDPListenPortFlag.Name) {
		cfg.UDPListenAddr = fmt.Sprintf(":%d", ctx.GlobalInt(UDPListenPortFlag.Name))
	}
}

// setNAT creates a port mapper from command line flags.
func setNAT(ctx *cli.Context, cfg *p2p.ClientCfg) {
	if ctx.GlobalIsSet(NATFlag.Name) {
		natif, err := nat.Parse(ctx.GlobalString(NATFlag.Name))
		if err != nil {
			utils.Fatalf("Option %s: %v", NATFlag.Name, err)
		}
		cfg.NAT = natif
	}
}

// SetNodeConfig applies node-related command line flags to the config.
func SetNodeConfig(ctx *cli.Context, cfg *node.NodeConfig) {
	//SetP2PConfig(ctx, &cfg.P2P)

	switch {
	case ctx.GlobalIsSet(DataDirFlag.Name):
		cfg.DataDir = ctx.GlobalString(DataDirFlag.Name)
	case ctx.GlobalBool(TestnetFlag.Name): //JiangHan：对于两种测试模式网，这里在主datapath后面附加了子文件夹，以示区分，避免跟主模式目录数据冲突
		cfg.DataDir = filepath.Join(node.DefaultDataDir(), "testnet")
	case ctx.GlobalBool(RinkebyFlag.Name):
		cfg.DataDir = filepath.Join(node.DefaultDataDir(), "rinkeby")
	}
}

// SetFTransConfig applies file-transfer module's flags to the config.
func SetFTransConfig(ctx *cli.Context, cfg *ftransfer.Config) {
	if ctx.GlobalIsSet(DataDirFlag.Name) {
		dir := ctx.GlobalString(DataDirFlag.Name)
		cfg.FilesDir = filepath.Join(dir, "files")
	}
	if ctx.GlobalIsSet(NetworkIdFlag.Name) {
		cfg.NetworkId = ctx.GlobalUint64(NetworkIdFlag.Name)
	}
	if ctx.GlobalIsSet(RemoteIPFlag.Name) {
		cfg.RemoteIP = ctx.GlobalString(RemoteIPFlag.Name)
	}
}

func RegisterFTransferService(stack *node.Node, cfg *ftransfer.Config) {
	var err error

	err = stack.Register(func(ctx *node.FTransServiceContext) (node.FTransService, error) {
		fullNode, err := ftransfer.New(nil, cfg, ctx.AccountManager)
		return fullNode, err
	})
	if err != nil {
		utils.Fatalf("Failed to register the Contatract service: %v", err)
	}
}
