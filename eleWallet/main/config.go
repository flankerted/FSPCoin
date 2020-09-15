package main

import (
	"github.com/contatract/go-contatract/eleWallet/flags"
	"github.com/contatract/go-contatract/eleWallet/node"
	"github.com/contatract/go-contatract/eleWallet/utils"
	"github.com/contatract/go-contatract/ftransfer"
	"github.com/contatract/go-contatract/params"
	"gopkg.in/urfave/cli.v1"
)

const (
	clientIdentifier = "eleWallet" // Client identifier to advertise over the network
)

type eleWalletConfig struct {
	Node      node.NodeConfig
	FileTrans ftransfer.Config
}

func defaultNodeConfig() node.NodeConfig {
	cfg := node.DefaultConfig
	cfg.Name = clientIdentifier
	cfg.Version = params.VersionWithCommit(gitCommit)
	return cfg
}

func defaultFTransConfig() ftransfer.Config {
	cfg := ftransfer.DefaultConfig
	cfg.Name = clientIdentifier
	cfg.Version = params.VersionWithCommit(gitCommit)
	return cfg
}

func NewClientNode(ctx *cli.Context) (*node.Node, *eleWalletConfig) {
	// Load defaults.
	cfg := eleWalletConfig{
		Node:      defaultNodeConfig(),
		FileTrans: defaultFTransConfig(),
	}

	// Apply flags.
	flags.SetNodeConfig(ctx, &cfg.Node)
	stack, err := node.New(&cfg.Node)
	if err != nil {
		utils.Fatalf("Failed to create the protocol stack: %v", err)
	}

	return stack, &cfg
}

func setConfigNode(ctx *cli.Context, cfg *eleWalletConfig) {
	flags.SetP2PConfig(ctx, &cfg.Node.P2P)
	flags.SetFTransConfig(ctx, &cfg.FileTrans)
}

func initClientNode(ctx *cli.Context, stack *node.Node, cfg *eleWalletConfig, remoteId []byte) {
	setConfigNode(ctx, cfg)
	setRemoteID(&cfg.Node, remoteId)
	if stack.GetConfig().BaseAddress != "" && stack.GetConfig().BasePassphrase != "" {
		cfg.Node.BaseAddress = stack.GetConfig().BaseAddress
		cfg.Node.BasePassphrase = stack.GetConfig().BasePassphrase
	}
	stack.SetConfig(&cfg.Node)
	flags.RegisterFTransferService(stack, &cfg.FileTrans)
}

// setRemoteID applies the remote ID to the config.
func setRemoteID(cfg *node.NodeConfig, remoteId []byte) {
	cfg.P2P.RemoteID = remoteId
}
