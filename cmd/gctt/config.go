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

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"unicode"

	"gopkg.in/urfave/cli.v1"

	"github.com/contatract/go-contatract/blizcs"
	"github.com/contatract/go-contatract/blizzard"
	"github.com/contatract/go-contatract/cmd/utils"
	"github.com/contatract/go-contatract/elephant"
	"github.com/contatract/go-contatract/eth"
	"github.com/contatract/go-contatract/ftransfer"
	"github.com/contatract/go-contatract/node"
	"github.com/contatract/go-contatract/params"
	"github.com/naoina/toml"
)

var (
	dumpConfigCommand = cli.Command{
		Action:    utils.MigrateFlags(dumpConfig),
		Name:      "dumpconfig",
		Usage:     "Show configuration values",
		ArgsUsage: "",

		// modified by jianghan
		//Flags:       append(append(nodeFlags, rpcFlags...), whisperFlags...),
		Flags: append(nodeFlags, rpcFlags...),

		Category:    "MISCELLANEOUS COMMANDS",
		Description: `The dumpconfig command shows configuration values.`,
	}

	configFileFlag = cli.StringFlag{
		Name:  "config",
		Usage: "TOML configuration file",
	}
)

// These settings ensure that TOML keys use the same names as Go struct fields.
var tomlSettings = toml.Config{
	NormFieldName: func(rt reflect.Type, key string) string {
		return key
	},
	FieldToKey: func(rt reflect.Type, field string) string {
		return field
	},
	MissingField: func(rt reflect.Type, field string) error {
		link := ""
		if unicode.IsUpper(rune(rt.Name()[0])) && rt.PkgPath() != "main" {
			link = fmt.Sprintf(", see https://godoc.org/%s#%s for available fields", rt.PkgPath(), rt.Name())
		}
		return fmt.Errorf("field '%s' is not defined in %s%s", field, rt.String(), link)
	},
}

type ethstatsConfig struct {
	URL string `toml:",omitempty"`
}

type gcttConfig struct {
	Eth      eth.Config
	Elephant elephant.Config
	//Shh       whisper.Config
	Node     node.Config
	Ethstats ethstatsConfig
	Blizzard blizzard.Config
	BlizCS   blizcs.Config
	//Dashboard dashboard.Config
	FileTrans ftransfer.Config
}

func loadConfig(file string, cfg *gcttConfig) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	err = tomlSettings.NewDecoder(bufio.NewReader(f)).Decode(cfg)
	// Add file name to errors that have a line number.
	if _, ok := err.(*toml.LineError); ok {
		err = errors.New(file + ", " + err.Error())
	}
	return err
}

func defaultNodeConfig() node.Config {
	cfg := node.DefaultConfig
	cfg.Name = clientIdentifier
	cfg.Version = params.VersionWithCommit(gitCommit)
	cfg.HTTPModules = append(cfg.HTTPModules, "eth", "shh")
	cfg.WSModules = append(cfg.WSModules, "eth", "shh")
	cfg.IPCPath = "geth.ipc"
	return cfg
}

func defaultBlizzardConfig() blizzard.Config {
	cfg := blizzard.DefaultConfig
	cfg.Name = clientIdentifier
	cfg.Version = params.VersionWithCommit(gitCommit)
	cfg.WSModules = append(cfg.WSModules, "blizzard", "shh")
	return cfg
}

func defaultFTransConfig() ftransfer.Config {
	cfg := ftransfer.DefaultConfig
	cfg.Name = clientIdentifier
	cfg.Version = params.VersionWithCommit(gitCommit)
	return cfg
}

func makeConfigNode(ctx *cli.Context) (*node.Node, gcttConfig) {
	// Load defaults.
	cfg := gcttConfig{
		Eth:      eth.DefaultConfig,
		Elephant: elephant.DefaultConfig,
		//Shh:      whisper.DefaultConfig,
		Node:      defaultNodeConfig(),
		Blizzard:  defaultBlizzardConfig(),
		FileTrans: defaultFTransConfig(),
		//Dashboard: dashboard.DefaultConfig,
	}

	// Load config file.
	if file := ctx.GlobalString(configFileFlag.Name); file != "" {
		if err := loadConfig(file, &cfg); err != nil {
			utils.Fatalf("%v", err)
		}
	}

	// Apply flags.
	utils.SetNodeConfig(ctx, &cfg.Node)
	stack, err := node.New(&cfg.Node)
	if err != nil {
		utils.Fatalf("Failed to create the protocol stack: %v", err)
	}

	utils.SetEthConfig(ctx, stack, &cfg.Eth)
	if ctx.GlobalIsSet(utils.EthStatsURLFlag.Name) {
		cfg.Ethstats.URL = ctx.GlobalString(utils.EthStatsURLFlag.Name)
	}

	utils.SetElephantConfig(ctx, stack, &cfg.Elephant)

	utils.SetBlizzardConfig(ctx, stack, &cfg.Blizzard)

	utils.SetBlizCSConfig(ctx, stack, &cfg.BlizCS)

	utils.SetFTransConfig(ctx, &cfg.FileTrans)

	//utils.SetShhConfig(ctx, stack, &cfg.Shh)
	//utils.SetDashboardConfig(ctx, &cfg.Dashboard)

	return stack, cfg
}

/*
// enableWhisper returns true in case one of the whisper flags is set.
func enableWhisper(ctx *cli.Context) bool {
	for _, flag := range whisperFlags {
		if ctx.GlobalIsSet(flag.GetName()) {
			return true
		}
	}
	return false
}
*/

func makeFullNode(ctx *cli.Context) *node.Node {
	stack, cfg := makeConfigNode(ctx)

	utils.RegisterEthService(stack, &cfg.Eth)
	utils.RegisterElephantService(stack, &cfg.Elephant)
	utils.RegisterBlizzardService(stack, &cfg.Blizzard)
	utils.RegisterBlizCSService(stack, &cfg.BlizCS)
	utils.RegisterFTransferService(stack, &cfg.FileTrans)
	/*
		modified by JiangHan
		if ctx.GlobalBool(utils.DashboardEnabledFlag.Name) {
			utils.RegisterDashboardService(stack, &cfg.Dashboard, gitCommit)
		}
		// Whisper must be explicitly enabled by specifying at least 1 whisper flag or in dev mode
		shhEnabled := enableWhisper(ctx)
		shhAutoEnabled := !ctx.GlobalIsSet(utils.WhisperEnabledFlag.Name) && ctx.GlobalIsSet(utils.DeveloperFlag.Name)
		if shhEnabled || shhAutoEnabled {
			if ctx.GlobalIsSet(utils.WhisperMaxMessageSizeFlag.Name) {
				cfg.Shh.MaxMessageSize = uint32(ctx.Int(utils.WhisperMaxMessageSizeFlag.Name))
			}
			if ctx.GlobalIsSet(utils.WhisperMinPOWFlag.Name) {
				cfg.Shh.MinimumAcceptedPOW = ctx.Float64(utils.WhisperMinPOWFlag.Name)
			}
			utils.RegisterShhService(stack, &cfg.Shh)
		}

		// Add the Ethereum Stats daemon if requested.
		if cfg.Ethstats.URL != "" {
			utils.RegisterEthStatsService(stack, cfg.Ethstats.URL)
		}
	*/
	//io.WriteString(os.Stdout, "【江寒】test")
	if cfg.Eth.Genesis != nil {
		// 普通运行状态是 Genesis == nil, 下面是不打印的
		//io.WriteString(os.Stdout, "【江寒】test------------------1")
	}

	return stack
}

// dumpConfig is the dumpconfig command.
func dumpConfig(ctx *cli.Context) error {
	_, cfg := makeConfigNode(ctx)
	comment := ""

	if cfg.Eth.Genesis != nil {
		cfg.Eth.Genesis = nil
		comment += "# Note: this config doesn't contain the genesis block.\n\n"
	}

	out, err := tomlSettings.Marshal(&cfg)
	if err != nil {
		return err
	}
	io.WriteString(os.Stdout, comment)
	os.Stdout.Write(out)
	return nil
}
