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
	"fmt"
	"sync"
	"time"

	"github.com/contatract/go-contatract/accounts"
	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/elephant/exporter"
	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/p2p"
	"github.com/contatract/go-contatract/rpc"
)

func ftransDebug(msg string, ctx ...interface{}) {
	log.Info(msg, ctx...)
}

type blizzardCsCross interface {
	ObjReadFtp2CSFunc(req *exporter.ObjReadFtp2CS) *exporter.ObjReadFtp2CSRsp
	UpdateAuthAllowFlow(userAddr common.Address, flow uint64, sign []byte)
	EleObjGetElephantDataFunc(query *exporter.ObjGetElephantData)
	EleObjGetElephantDataRspFunc(query *exporter.ObjGetElephantData) *exporter.ObjGetElephantDataRsp
	ObjWriteFtp2CSFunc(req *exporter.ObjWriteFtp2CS) *exporter.ObjWriteFtp2CSRsp
	ObjAuthFtp2CSFunc(req *exporter.ObjAuthFtp2CS)
}

type FileTransfer struct {
	// Name should contain the official protocol name,
	// often a three-letter word.
	Name string

	// Version should contain the version number of the protocol.
	Version string

	shutdownChan chan struct{} // Channel for shutting down the service

	// Logger is a custom logger
	Logger log.Logger `toml:",omitempty"`

	config *Config

	protocolManager *ProtocolManager
	blizCsCross     blizzardCsCross

	wg   sync.WaitGroup
	lock sync.RWMutex
}

// New creates a new P2P node, ready for protocol registration.
func New(blizCs blizzardCsCross, config *Config, am *accounts.Manager) (*FileTransfer, error) {
	if config.Logger == nil {
		config.Logger = log.New()
	}

	// Note: any interaction with Config that would create/touch files
	// in the data directory or instance directory is delayed until Start.
	ft := &FileTransfer{
		Name:         config.Name,
		Version:      config.Version,
		Logger:       config.Logger,
		config:       config,
		blizCsCross:  blizCs,
		shutdownChan: make(chan struct{}),
	}
	log.Info("Initializing FileTransfer Service protocol", "versions", ProtocolVersions, "network", config.NetworkId)

	var err error
	if ft.protocolManager, err = NewProtocolManager(config.NetworkId, ft, am); err != nil {
		return nil, err
	}

	return ft, nil
}

func getSignData(data *exporter.ObjGetElephantDataRsp) (uint64, string) {
	if data.Ty != exporter.TypeElephantSign {
		log.Error("Get sign data", "ty", data.Ty)
		return 0, ""
	}
	if data.RspParams == nil {
		ftransDebug("Get sign data nil")
		return 0, ""
	}
	if len(data.RspParams) == 1 {
		return 0, data.RspParams[0].(string)
	} else if len(data.RspParams) < 2 {
		return 0, ""
	}
	return data.RspParams[0].(uint64), data.RspParams[1].(string)
}

func (ft *FileTransfer) ObjReadFtp2CSRspFunc(rsp *exporter.ObjReadFtp2CSRsp, seqID uint32) {
	if rsp == nil {
		return
	}
	p := ft.protocolManager.peers.Peer(rsp.PeerId)
	if p == nil {
		log.Error(fmt.Sprintf("The peer %s does not exist for read", rsp.PeerId))
		return
	}
	data := &DataReadRet{
		ObjId:   rsp.ObjId,
		Ret:     rsp.Ret,
		Buff:    rsp.Buff,
		RawSize: rsp.RawSize,
	}
	err := ft.protocolManager.DoReadDataRet(p, data, seqID)
	if err != nil {
		log.Error("SendReadDataRetMsg", "err", err)
	}
}

func (ft *FileTransfer) runServerLoop() {
	// Wait for different events to fire synchronisation operations
	ticker := time.NewTicker(AuthCSDeadline / 2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ft.shutdownChan:
			return

		case <-ticker.C:

		// case req := <-ft.serviceChan.ObjWalletSignChan():
		// 	p := ft.protocolManager.peers.Peer(req.PeerID)
		// 	if p == nil {
		// 		log.Error(fmt.Sprintf("The peer %s does not exist", req.PeerID))
		// 		break
		// 	}
		// 	data := &GetHashSign{Hash: req.Hash}
		// 	err := ft.protocolManager.SendGetHashSign(p, data)
		// 	log.Info("ObjWalletSignChan", "err", err)

		case <-ft.GetProtocolMan().newPeerCh:
			// if err := ft.GetProtocolMan().SendBaseAddress(p); err != nil {
			// 	log.Error(fmt.Sprintf("Failed to send the pubkey: %s", err.Error()))
			// }

		}
	}
}

func (ft *FileTransfer) ObjGetElephantDataRspFunc(rsp *exporter.ObjGetElephantDataRsp) {
	var peer *Peer
	endTime, peerId := getSignData(rsp)
	peer = ft.protocolManager.peers.Peer(peerId)
	if peer == nil {
		log.Error("peer nil")
		return
	}
	if endTime == 0 {
		log.Info("please sign cs")
	} else {
		log.Info("ObjGetElephantDataRspChan", "endTime", common.GetTimeString(endTime))
	}
	if err := ft.GetProtocolMan().SendBaseAddress(peer); err != nil {
		log.Error(fmt.Sprintf("Failed to send the pubkey: %s", err.Error()))
	}
}

func (ft *FileTransfer) runClientLoop() {
	// Wait for different events to fire synchronisation operations
	ticker := time.NewTicker(AuthCSDeadline / 2 * time.Second)
	defer ticker.Stop()

	ft.protocolManager.IsClient = true
	for {
		select {
		case <-ft.shutdownChan:
			return

		case <-ticker.C:
			// 签名授权（c2s） qiwy(TODO)

		// case req := <-ft.serviceChan.ObjWalletSignChan():
		// 	p := ft.protocolManager.peers.Peer(req.PeerID)
		// 	if p == nil {
		// 		log.Error(fmt.Sprintf("The peer %s does not exist", req.PeerID))
		// 		break
		// 	}
		// 	data := &GetHashSign{Hash: req.Hash}
		// 	err := ft.protocolManager.SendGetHashSign(p, data)
		// 	log.Info("ObjWalletSignChan", "err", err)

		case p := <-ft.GetProtocolMan().newPeerCh:
			if err := ft.GetProtocolMan().SendClientBaseAddress(p); err != nil {
				log.Error(fmt.Sprintf("Failed to send client: %s", err.Error()))
			}

			//case <-readDataFromCS:
		}
	}
}

func (ft *FileTransfer) GetProtocolMan() *ProtocolManager {
	return ft.protocolManager
}

// Protocols implements node.Service, returning all the currently configured
// network protocols to start.
func (ft *FileTransfer) Protocols() []p2p.Protocol {
	//protocols := make([]p2p.Protocol, 0)
	//return protocols
	return ft.protocolManager.SubProtocols
}

// Start is called after all services have been constructed and the networking
// layer was also initialized to spawn any goroutines required by the service.
func (ft *FileTransfer) Start(srv *p2p.Server) error {
	ft.protocolManager.Start()

	// 启动各个主要的工作 routine
	go ft.runServerLoop()

	return nil
}

func (ft *FileTransfer) StartClient(dev *p2p.Client) error {
	ft.protocolManager.Start()

	// 启动各个主要的工作 routine
	go ft.runClientLoop()

	return nil
}

// Stop implements node.Service, terminating all internal goroutines used by the
// Contatract protocol.
func (ft *FileTransfer) Stop() error {

	ft.protocolManager.Stop()
	close(ft.shutdownChan)

	return nil
}

// APIs returns the collection of RPC services the ethereum package offers.
// NOTE, some of these services probably need to be moved to somewhere else.
func (ft *FileTransfer) APIs() []rpc.API {
	apis := []rpc.API{
		{
			Namespace: "ftrans",
			Version:   "1.0",
			Service:   NewPrivateFTransAPI(ft),
			Public:    false,
		},
	}
	// Append all the local APIs and return
	return append(apis, []rpc.API{}...)
}
