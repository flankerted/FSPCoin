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
	"bufio"
	"bytes"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/contatract/go-contatract/accounts"
	"github.com/contatract/go-contatract/accounts/keystore"
	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/common/datacrypto"
	"github.com/contatract/go-contatract/crypto"
	"github.com/contatract/go-contatract/elephant/exporter"
	"github.com/contatract/go-contatract/event"
	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/p2p"
)

// Constants to match up protocol versions and messages
const (
	ftransfer1 = 1
	//ftransfer2 = 2
)

const (
	uploadTryCount   uint32 = uint32(time.Minute / oneTimeout) // Uploading data trys count
	downloadTryCount uint32 = uploadTryCount                   // Downloading data trys count
	writeTryCount    uint32 = uploadTryCount                   // Writing data trys count

	oneTimeout time.Duration = time.Millisecond * 200

	seqAckDiff uint32 = 1
	seqStartID uint32 = 1
)

// Official short name of the protocol used during capability negotiation.
var ProtocolName = "ftransfer"

// Supported versions of the eth protocol (first is primary).
var ProtocolVersions = []uint{ftransfer1}

// Number of implemented message corresponding to different protocol versions.
var ProtocolLengths = []uint64{0x17 + 1}

const (
	Version = 0
	// FileTransWritePort = "12791"
	// FileTransReadPort  = "12792"
	// FileTransferNet    = "tcp"
	FileTransferSize   = 1 * 1024 * 1024
	FileSignatureSize  = 64 // [R || S] format
	ProtocolMaxMsgSize = 10 * 1024 * 1024
	AuthCSDeadline     = 3600 * 2
	mSize              = 1024 * 1024
)

// ftransfer protocol message codes
const (
	// Protocol messages belonging to eth/62
	StatusMsg = 0x00
	//ShakehandMsg = 0x01 // 握手，keepalive

	SendFileReqMsg = 0x02
	ReadFileReqMsg = 0x03

	SendFileRspMsg = 0x04
	ReadFileRspMsg = 0x05

	SendFileTerminateMsg = 0x06
	ReadFileTerminateMsg = 0x07

	SendFileCSProgressMsg = 0x08

	GetHashSignMsg = 0x09
	HashSignMsg    = 0x0A

	// 签名授权（c2s）
	SignAuthorizeMsg = 0x0B

	// 写数据(c2s)
	WriteDataMsg = 0x0C
	// 写数据返回(s2c)
	WriteDataRetMsg = 0x0D

	// 读数据(c2s)
	ReadDataMsg = 0x0E
	// 读数据返回(s2c)
	ReadDataRetMsg = 0x0F

	// 数据签名确认(s2c)
	GetDataSignConfirmMsg = 0x10
	// 数据签名确认返回(c2s)
	DataSignConfirmMsg = 0x11

	BaseAddressMsg       = 0x12
	ClientBaseAddressMsg = 0x13

	Information   = 0x14
	AuthAllowFlow = 0x15

	SendFTWriteMsg = 0x16

	SendFTFileMsg = 0x17
)

// FileSendRsp messages
const (
	FileTransferRspStart = "start transfer"
	FileTransferRspError = "invalid req"
)

const (
	rwDataTimeout   = 10 * time.Second
	fileDataTimeout = 20 * time.Second
)

// statusData is the network packet for the status message.
type statusData struct {
	ProtocolVersion uint32
	NetworkId       uint64
}

// bzz represents the swarm wire protocol
// an instance is running on each peer
type ProtocolManager struct {
	networkId    uint64
	SubProtocols []p2p.Protocol
	AccMan       *accounts.Manager

	maxClients int
	peers      *peerSet

	newPeerCh         chan *Peer
	TransferTCPRateRF chan TransferPercent   // Only used for client, only used for reading file
	TransferTCPRateWF chan TransferPercent   // Only used for client, only used for writing file
	TransferCSRate    chan TransferCSPercent // Only used for client

	// DataSendFinishRS     chan DataReadRet // Only used for client, only used for reading string
	scope                event.SubscriptionScope
	dataSendFinishRSSub  event.Subscription
	dataSendFinishRSFeed event.Feed

	// DataSendFinishWS chan DataSendRet // Only used for client, only used for writing string
	dataSendFinishWSSub  event.Subscription
	dataSendFinishWSFeed event.Feed

	DataSendFinishRF chan DataSendRet // Only used for client, only used for reading file
	DataSendFinishWF chan DataSendRet // Only used for client, only used for writing file
	// errorMsgChan    chan string            // Only used for client

	ftrans         *FileTransfer
	fileToTransfer map[string]*multipart.File
	fileHead       map[string]*FileTransferReqData

	baseAddress    common.Address
	basePassphrase string
	authorizedCS   common.Address      // Only used for client
	authorization  *BaseAddressMsgData // Only used for client

	IsClient bool

	// wait group is used for graceful shutdowns during downloading
	// and processing
	wg              sync.WaitGroup
	peerUpDatas     map[*Peer]map[uint32]*FTFileData // Upload
	peerUpDatasLock map[*Peer]sync.Mutex

	peerDownDatas     map[*Peer]map[uint32]*FTFileData // Download
	peerDownDatasLock map[*Peer]sync.Mutex

	localDatas     map[uint32]*FTFileData // Write to local file
	localDatasLock sync.Mutex

	encryptKeys map[string][]byte

	uploadingPeer   map[string]struct{} // For cs controlling flow
	downloadingPeer map[string]struct{} // For cs controlling flow
}

var errIncompatibleConfig = errors.New("incompatible configuration")

func NewProtocolManager(networkId uint64, ftrans *FileTransfer, am *accounts.Manager) (*ProtocolManager, error) {
	// Create the protocol manager with the base fields
	manager := &ProtocolManager{
		AccMan:            am,
		networkId:         networkId,
		ftrans:            ftrans,
		maxClients:        1,
		peers:             newPeerSet(),
		baseAddress:       common.Address{},
		basePassphrase:    "",
		IsClient:          false,
		fileToTransfer:    make(map[string]*multipart.File),
		fileHead:          make(map[string]*FileTransferReqData),
		newPeerCh:         make(chan *Peer),
		TransferTCPRateRF: make(chan TransferPercent),
		TransferTCPRateWF: make(chan TransferPercent),
		TransferCSRate:    make(chan TransferCSPercent),
		// DataSendFinishRS:  make(chan DataReadRet),
		// DataSendFinishWS: make(chan DataSendRet),
		DataSendFinishRF: make(chan DataSendRet),
		DataSendFinishWF: make(chan DataSendRet),
		peerUpDatas:      make(map[*Peer]map[uint32]*FTFileData),
		peerUpDatasLock:  make(map[*Peer]sync.Mutex),

		peerDownDatas:     make(map[*Peer]map[uint32]*FTFileData),
		peerDownDatasLock: make(map[*Peer]sync.Mutex),

		localDatas:  make(map[uint32]*FTFileData),
		encryptKeys: make(map[string][]byte),

		uploadingPeer:   make(map[string]struct{}),
		downloadingPeer: make(map[string]struct{}),
	}

	manager.SubProtocols = make([]p2p.Protocol, 0, len(ProtocolVersions))
	for i, version := range ProtocolVersions {
		// Compatible; initialise the sub-protocol
		version := version // Closure for the run
		manager.SubProtocols = append(manager.SubProtocols, p2p.Protocol{
			Name:    ProtocolName,
			Version: version,
			Length:  ProtocolLengths[i],
			Run: func(p *p2p.Peer, rw p2p.MsgReadWriter) error {
				// Check base account
				if !manager.CheckBase() {
					return errors.New("need at least one base account to start the protocol")
				}

				peer := manager.newPeer(int(version), p, rw)
				manager.wg.Add(1)
				defer manager.wg.Done()

				return manager.handle(peer)
			},
		})
	}

	if len(manager.SubProtocols) == 0 {
		return nil, errIncompatibleConfig
	}
	return manager, nil
}

func (pm *ProtocolManager) CheckBase() bool {
	ks := fetchKeystore(pm.AccMan)
	if ks != nil {
		if accs := ks.Accounts(); len(accs) != 0 {
			for i := range accs {
				if common.EmptyAddress(pm.baseAddress) {
					if pm.IsClient {
						return false
					} else {
						if err := pm.SetBaseAddress(accs[0].Address.String()); err != nil {
							return false
						} else {
							return true
						}
					}
				} else {
					if pm.IsClient {
						if strings.Compare(accs[i].Address.String(), pm.baseAddress.String()) == 0 {
							if _, err := ks.GetKeyCopy(accs[i], pm.basePassphrase); err == nil {
								return true
							}
						}
					} else {
						return true
					}
				}
			}
		}
	}

	return false
}

func (pm *ProtocolManager) GetCurrentCSAddress() common.Address {
	return pm.authorizedCS
}

func (pm *ProtocolManager) SetBaseAddress(address string) error {
	ks := fetchKeystore(pm.AccMan)
	if ks != nil {
		if len(ks.Accounts()) != 0 {
			if address != "" {
				pm.baseAddress = common.HexToAddress(address)
			} else {
				return errors.New("SetBaseAddress can not set a empty address string")
			}
			return nil
		} else {
			return errors.New("SetBaseAddress need at least one account")
		}
	} else {
		return errors.New("SetBaseAddress need at least one keystore")
	}
}

func (pm *ProtocolManager) GetBaseAddress() common.Address {
	return pm.baseAddress
}

func (pm *ProtocolManager) SetBaseAddrPasswd(passphrase string) {
	if passphrase != "" {
		pm.basePassphrase = passphrase

		if pm.CheckBase() {
			if pm.IsClient {
				if peers := pm.peers.GetPeers(); len(peers) > 0 {
					err := pm.SendSignAuthorizeMsg(peers[0], pm.authorization)
					if err != nil {
						ftransDebug(err.Error())
					}
				} else {
					log.Info("Do SetBaseAddrPasswd function when there is no any peers connected")
				}
			}
		}
	} else {
		log.Error("SetBaseAddrPasswd", "err", "the passphrase is empty")
	}
}

func (pm *ProtocolManager) ZeroBaseAddrPasswd() {
	pm.basePassphrase = ""
}

func (pm *ProtocolManager) GetBaseAddrPasswd() string {
	return pm.basePassphrase
}

// func (pm *ProtocolManager) SetErrorMsgChan(ch chan string) {
// 	if pm.errorMsgChan == nil {
// 		pm.errorMsgChan = ch
// 	}
// }

// fetchKeystore retrives the encrypted keystore from the account manager.
func fetchKeystore(am *accounts.Manager) *keystore.KeyStore {
	return am.Backends(keystore.KeyStoreType)[0].(*keystore.KeyStore)
}

func (pm *ProtocolManager) GetPeerSet() []*Peer {
	return pm.peers.GetPeers()
}

func (pm *ProtocolManager) removePeer(id string) {
	// Short circuit if the peer was already removed
	p := pm.peers.Peer(id)
	if p == nil {
		return
	}
	log.Debug("Removing FileTransfer client", "client", id)

	if err := pm.peers.Unregister(id); err != nil {
		log.Error("FileTransfer Client removal failed", "client", id, "err", err)
	}
	// Hard disconnect at the networking layer
	p.Peer.Disconnect(p2p.DiscUselessPeer)
}

func (pm *ProtocolManager) newPeer(pv int, p *p2p.Peer, rw p2p.MsgReadWriter) *Peer {
	return NewPeer(pv, p, newMeteredMsgWriter(rw))
}

func sendChanPeer(ex chan struct{}, ch chan *Peer, va *Peer) {
	select {
	case <-ex:
	case ch <- va:
	}
}

// handle is the callback invoked to manage the life cycle of an eth peer. When
// this function terminates, the peer is disconnected.
func (pm *ProtocolManager) handle(p *Peer) error {
	log.Info("ftransfer handle", "id", p.id)
	if pm.maxClients == 0 {
		log.Warn("----------??????????----------")
		pm.maxClients = 1
	}

	// Execute the file transfer handshake
	if err := p.Handshake(pm.networkId); err != nil {
		log.Error("Handshake", "err", err)
		return err
	}

	if rw, ok := p.rw.(*meteredMsgReadWriter); ok {
		rw.Init(p.version)
	}

	// Register the peer locally
	// sendChanPeer(pm.ftrans.shutdownChan, pm.newPeerCh, p)
	select {
	case <-pm.ftrans.shutdownChan:
		return errors.New("chan closed")
	case pm.newPeerCh <- p:
	}
	if err := pm.RegisterPeer(p); err != nil {
		log.Error("FileTransfer peer registration failed", "err", err)
		return err
	}
	defer func() {
		delete(pm.peerUpDatas, p)
		delete(pm.peerUpDatasLock, p)

		delete(pm.peerDownDatas, p)
		delete(pm.peerDownDatasLock, p)

		// Exiting when it is uploading the file
		delete(pm.fileToTransfer, p.id)
		delete(pm.fileHead, p.id)

		pm.removePeer(p.id)
	}()
	pm.peerUpDatas[p] = make(map[uint32]*FTFileData)
	pm.peerUpDatasLock[p] = sync.Mutex{}

	pm.peerDownDatas[p] = make(map[uint32]*FTFileData)
	pm.peerDownDatasLock[p] = sync.Mutex{}
	ftransDebug("FileTransfer protocol connected,", "name", p.Name())

	// main loop. handle incoming messages.
	for {
		if err := pm.handleMsg(p); err != nil {
			if err == io.EOF {
				log.Info("FileTransfer protocol message EOF")
				return nil
			}
			// if pm.errorMsgChan != nil {
			// 	pm.errorMsgChan <- err.Error()
			// }
			return err
		}
	}
}

func (pm *ProtocolManager) RegisterPeer(p *Peer) error {
	if err := pm.peers.Register(p); err != nil {
		return err
	}
	return nil
}

func (pm *ProtocolManager) HandleMsgForTest(p *Peer) error {
	// main loop. handle incoming messages.
	//for {
	if err := pm.handleMsg(p); err != nil {
		if err == io.EOF {
			fmt.Println("FileTransfer protocol message EOF")
			log.Info("FileTransfer protocol message EOF")
			return nil
		}
		// if pm.errorMsgChan != nil {
		// 	pm.errorMsgChan <- err.Error()
		// }
		return err
	}
	//}
	return nil
}

// handleMsg is invoked whenever an inbound message is received from a remote
// peer. The remote connection is torn down upon returning any error.
func (pm *ProtocolManager) handleMsg(p *Peer) error {
	// Read the next message from the remote peer, and ensure it's fully consumed
	msg, err := p.rw.ReadMsg()
	if err != nil {
		return err
	}
	if msg.Size > ProtocolMaxMsgSize {
		return errResp(ErrMsgTooLarge, "%v > %v", msg.Size, ProtocolMaxMsgSize)
	}
	defer msg.Discard()

	log.Msg("FT handle msg start", "code", msg.Code)
	defer log.Msg("FT handle msg end", "code", msg.Code)

	// Handle the message depending on its contents
	switch {
	case msg.Code == StatusMsg:
		// Status messages should never arrive after the handshake
		return errResp(ErrExtraStatusMsg, "uncontrolled status message")

	case msg.Code == Information:
		var info InteractiveInfo
		if err := msg.Decode(&info); err != nil {
			// maybe attacker, discard
			return err
		}

		if err = pm.HandleInteractiveInfo(p, &info, pm.IsClient); err != nil {
			return err
		}

		return nil

		// File writing to blizcs operations
	case msg.Code == SendFileReqMsg:
		ret, err := pm.HandleTransferReq(p, &msg)
		if err != nil {
			return errResp(ErrDecode, "%v", msg)
		} else if !ret {
			log.Error(errorToString[ErrVerifyReqSig], "%v", msg)
		}

		err = pm.DoFileSendRsp(p)
		if err != nil {
			return errResp(ErrDecode, "%v: %v", msg, err)
		}
		return nil

	case msg.Code == SendFileRspMsg:
		var rsp string
		if err := msg.Decode(&rsp); err != nil {
			// maybe attacker, discard
			return err
		}
		if rsp == FileTransferRspStart {
			if err := pm.DoFileSendOp(p, &msg); err != nil {
				delete(pm.fileToTransfer, p.id)
				delete(pm.fileHead, p.id)
				return err
			}
		} else {
			delete(pm.fileToTransfer, p.id)
			delete(pm.fileHead, p.id)
			return errResp(ErrFileHead, "%v", msg)
		}
		return nil

	case msg.Code == SendFileTerminateMsg:
		var data WriteFileRet
		if err := msg.Decode(&data); err != nil {
			return err
		}

		log.Info("SendFileTerminateMsg", "file name", data.FileName, " Ret", data.Ret)
		fh, ok := pm.fileHead[p.id]
		if ok {
			fh.SetRetWithCS(true)
		}
		ret := DataSendRet{data.Ret, data.ErrInfo}
		go ret.SafeSendChTo(pm.DataSendFinishWF)
		return nil

	case msg.Code == SendFileCSProgressMsg:
		var data WriteFileCSRate
		if err := msg.Decode(&data); err != nil {
			return err
		}
		log.Info("File progress", "data", data)

		fh, ok := pm.fileHead[p.id]
		if ok {
			fh.SetAckID(data.SeqID)
		}

		transpercentCS := TransferCSPercent(data.CSRate)
		go transpercentCS.SafeSendChTo(pm.TransferCSRate)
		return nil

		// File reading from blizcs operations
	case msg.Code == ReadFileReqMsg:
		ret, err := pm.HandleTransferReq(p, &msg)
		if err != nil {
			return errResp(ErrDecode, "%v", msg)
		} else if !ret {
			log.Error(errorToString[ErrVerifyReqSig], "%v", msg)
		}
		err = pm.DoFileReadRsp(p)
		if err != nil {
			return errResp(ErrDecode, "%v: %v", msg, err)
		}
		return nil

	case msg.Code == ReadFileRspMsg:
		var rsp string
		if err := msg.Decode(&rsp); err != nil {
			// maybe attacker, discard
			return err
		}
		if rsp == FileTransferRspStart {
			if err := pm.DoFileReadOp(p); err != nil {
				delete(pm.fileHead, p.id)
				return err
			}
		} else {
			delete(pm.fileHead, p.id)
			ret := DataSendRet{false, errorToString[ErrFileHead]}
			go ret.SafeSendChTo(pm.DataSendFinishRF)
			return errResp(ErrFileHead, "%v", msg)
		}
		return nil

	case msg.Code == ReadFileTerminateMsg:
		var data ReadFileRet
		if err := msg.Decode(&data); err != nil {
			return err
		}

		log.Info("Read file terminate msg", "filename", data.FileName, " ret", data.Ret, "errinfo", data.ErrInfo)
		fh, ok := pm.fileHead[p.id]
		//fh := pm.fileHead.GetFileHead(p.id)
		if ok {
			fh.SetRetWithCS(true)
		}
		ret := DataSendRet{data.Ret, data.ErrInfo}
		go ret.SafeSendChTo(pm.DataSendFinishRF)
		return nil

	//case msg.Code == GetHashSignMsg:
	//	fmt.Println("GetHashSignMsg")
	//	var data GetHashSign
	//	if err := msg.Decode(&data); err != nil {
	//		fmt.Println("Decode", "err", err)
	//		return err
	//	}
	//	privKey, err := pm.GetPrivateKey()
	//	defer zeroKey(privKey)
	//	if err != nil {
	//		fmt.Println("GetPrivateKey", "err", err)
	//		return err
	//	}
	//	r, s, v, err := WalletSign(data.Hash, privKey)
	//	if err != nil {
	//		fmt.Println("WalletSign", "err", err)
	//		return err
	//	}
	//
	//	rsp := &HashSign{R: r, S: s, V: v}
	//	err = pm.SendHashSign(p, rsp)
	//	fmt.Println("SendHashSign", "err", err)
	//	return err

	case msg.Code == HashSignMsg:
		var data HashSign
		if err := msg.Decode(&data); err != nil {
			log.Error("Decode", "err", err)
			return err
		}

		// req := &exporter.ObjWalletSignRsp{R: data.R, S: data.S, V: data.V, PeerID: p.id}
		// pm.ftrans.serviceChan.Send(req)
		return nil

	case msg.Code == BaseAddressMsg:
		if !pm.IsClient {
			return nil
		}
		var data BaseAddressMsgData
		if err := msg.Decode(&data); err != nil {
			return err
		}
		pm.authorization = &data
		pm.authorizedCS = data.Address
		log.Info("Got the base address of FTP from the remote device successfully", "pm.authorizedCS", pm.authorizedCS.Hex())

		// Send first signed authorization to the remote
		err = pm.SendSignAuthorizeMsg(p, &data)
		if err != nil {
			log.Error("SendSignAuthorizeMsg", "err", err)
			return err
		}
		return nil

	case msg.Code == ClientBaseAddressMsg: //current is cs server
		var data BaseAddressMsgData
		if err := msg.Decode(&data); err != nil {
			return err
		}
		// whether authorized
		params := make([]interface{}, 0, 2)
		params = append(params, pm.baseAddress)
		params = append(params, p.id)
		req := &exporter.ObjGetElephantData{Address: data.Address, Ty: exporter.TypeElephantSign, Params: params}
		rsp := pm.ftrans.blizCsCross.EleObjGetElephantDataRspFunc(req)
		pm.ftrans.ObjGetElephantDataRspFunc(rsp)
		return nil

	// 签名授权（c2s）
	case msg.Code == SignAuthorizeMsg:
		var data SignAuthorizeData
		if err := msg.Decode(&data); err != nil {
			log.Error("Decode", "err", err)
			return err
		}
		req := &exporter.ObjAuthFtp2CS{
			CsAddr:       data.CsAddr,
			UserAddr:     data.UserAddr,
			AuthDeadline: data.AuthDeadline,
			R:            data.R,
			S:            data.S,
			V:            data.V,
			Peer:         data.Peer,
		}
		pm.ftrans.blizCsCross.ObjAuthFtp2CSFunc(req)
		log.Info("Got the signed authorization successfully")
		return nil

	// 写数据(c2s)
	case msg.Code == WriteDataMsg:
		var data WriteDataMsgData
		if err := msg.Decode(&data); err != nil {
			return err
		}
		go pm.TransmitToCS(data.ObjId, data.Offset, p.id, data.Buff, 0)
		return nil

	// 写数据返回(s2c)
	case msg.Code == WriteDataRetMsg:
		var data WriteDataRet
		if err := msg.Decode(&data); err != nil {
			return err
		}

		log.Info("WriteDataRetMsg", "ObjId", data.ObjId, "Ret", common.GetErrString(data.RetVal))
		ret := DataSendRet{len(data.RetVal) == 0, common.GetErrString(data.RetVal)}
		go pm.dataSendFinishWSFeed.Send(ret)
		return nil

	// 读数据(c2s)
	case msg.Code == ReadDataMsg:
		var data ReadDataMsgData
		if err := msg.Decode(&data); err != nil {
			return err
		}
		go func() {
			req := &exporter.ObjReadFtp2CS{
				ObjId:          data.ObjId,
				Offset:         data.Offset,
				Len:            data.Len,
				PeerId:         p.id,
				SharerCSNodeID: data.SharerCSNodeID,
				Sharer:         data.Sharer,
			}
			rsp := pm.ftrans.blizCsCross.ObjReadFtp2CSFunc(req)
			pm.ftrans.ObjReadFtp2CSRspFunc(rsp, 0)
		}()
		return nil

	// 读数据返回(s2c)
	case msg.Code == ReadDataRetMsg:
		var data DataReadRet
		if err := msg.Decode(&data); err != nil {
			return err
		}
		go pm.dataSendFinishRSFeed.Send(data)
		log.Info("Read data ret msg", "objid", data.ObjId, "ret", common.GetErrString(data.Ret), "bufflen", len(data.Buff))
		return nil

	// 数据签名确认(s2c)
	case msg.Code == GetDataSignConfirmMsg:
		return err

	// 数据签名确认返回(c2s)
	case msg.Code == DataSignConfirmMsg:
		return err

	case msg.Code == AuthAllowFlow: // cs server
		var data AuthAllowFlowMsgData
		if err := msg.Decode(&data); err != nil {
			return err
		}
		err = pm.handleAuthAllowFlow(&data)
		return err

	case msg.Code == SendFTWriteMsg:
		var data FTFileMsgData
		if err := msg.Decode(&data); err != nil {
			return err
		}
		go pm.pushLocalData(&data)
		return nil

	case msg.Code == SendFTFileMsg:
		var data FTFileMsgData
		if err := msg.Decode(&data); err != nil {
			return err
		}

		go pm.pushFileData(&data, p)
		return nil

	default:
		return errResp(ErrInvalidMsgCode, "%v", msg.Code)
	}
}

func (pm *ProtocolManager) Start() {
}

func (pm *ProtocolManager) Stop() {
	log.Info("Stopping FileTransfer protocol")
	pm.scope.Close()

	// Disconnect existing sessions.
	// This also closes the gate for any new registrations on the peer set.
	// sessions which are already established but not added to pm.peers yet
	// will exit when they try to register.
	pm.peers.Close()

	// Wait for all peer handler goroutines and the loops to come down.
	pm.wg.Wait()
	log.Info("FileTransfer protocol stopped")

	// log.Info("Stopping exporter chan")
	// exporter.Close()
	// log.Info("exporter chan stopped")
}

func (pm *ProtocolManager) DoFileSendReq(reqHead FileTransferReqData, data *multipart.File, peer *Peer) error {
	if _, ok := pm.fileToTransfer[peer.id]; ok {
		return errors.New("can not transfer two files simultaneously")
	}

	var err error = nil
	pm.fileToTransfer[peer.id] = data
	pm.fileHead[peer.id] = &reqHead
	if err = p2p.Send(peer.rw, SendFileReqMsg, reqHead); err != nil {
		delete(pm.fileToTransfer, peer.id)
		delete(pm.fileHead, peer.id)
	}

	return err
}

func (pm *ProtocolManager) HandleTransferReq(p *Peer, msg *p2p.Msg) (bool, error) {
	var reqHead FileTransferReqData
	if err := msg.Decode(&reqHead); err != nil {
		// maybe attacker, discard
		log.Error("HandleTransferReq", "err", err)
		return false, err
	}

	if !reqHead.VerifyHash() {
		return false, nil
	}
	reqHead.SetTransferCSSize(0)
	reqHead.SetRetWithCS(false)
	pm.fileHead[p.id] = &reqHead
	return true, nil
}

func (pm *ProtocolManager) DoFileSendRsp(peer *Peer) error {
	rsp := FileTransferRspStart
	if err := p2p.Send(peer.rw, SendFileRspMsg, rsp); err != nil {
		delete(pm.fileHead, peer.id)
		return err
	}
	if err := pm.DoFileReceiveOp(peer); err != nil {
		delete(pm.fileHead, peer.id)
		return err
	}
	return nil
}

func (pm *ProtocolManager) DoFileSendOp(peer *Peer, msg *p2p.Msg) error {
	fh, ok := pm.fileHead[peer.id]
	if !ok {
		return errors.New(fmt.Sprintf("%s: %s", "error in DoFileSendOp", "file head does not exist"))
	}
	fdata, ok := pm.fileToTransfer[peer.id]
	if !ok {
		return errors.New(fmt.Sprintf("%s: %s", "error in DoFileSendOp", "file data does not exist"))
	}
	size := fh.Size
	fh.SetRetWithCS(false)
	ftransDebug("File transferring...", "name", fh.Name)

	go func(id string) {
		defer delete(pm.fileHead, id)
		defer delete(pm.fileToTransfer, id)
		pm.sendFile(fh, fdata, size, true, peer)
	}(peer.id)

	return nil
}

func (pm *ProtocolManager) DoFileReceiveOp(p *Peer) error {
	fh, ok := pm.fileHead[p.id]
	//fh := pm.fileHead.GetFileHead(p.id)
	if !ok {
		return errors.New(fmt.Sprintf("%s: %s", "error in DoFileReceiveOp", "file head does not exist"))
	}
	fileName := fh.Name
	if fileName == "" {
		return errors.New("the file name is invalid")
	}
	size := fh.Size
	ftransDebug("File transferring...", "name", fileName)
	go pm.receiveFile2CS(p.id, fileName, size, true, p)
	return nil
}

func (pm *ProtocolManager) GetEncryptKeyForFile(fh *FileTransferReqData) (*ecdsa.PrivateKey, []byte, error) {
	key, kerr := pm.GetPrivateKey()
	if kerr != nil {
		return nil, nil, kerr
	}
	h := rlpHash(key.D.Bytes())
	encKey := datacrypto.GetEncryptKey(datacrypto.EncryptDes, h, fh.ObjId)
	if encKey == nil {
		return nil, nil, errors.New("datacrypto.GetEncryptKey() returned nil")
	}
	return key, encKey, nil
}

// Wemore server sends the data to cs
func (pm *ProtocolManager) sendFile(fh *FileTransferReqData, fdata *multipart.File, fsize uint64, signed bool, peer *Peer) {
	var (
		transferedSize uint64 = 0
		rsize                 = 0
		rerr, werr     error
		fileBuffer     = bufio.NewReader(*fdata)
	)

	pKey, encKey, kerr := pm.GetEncryptKeyForFile(fh)
	if kerr != nil {
		ftransDebug("File transfer failed", "error", kerr)
		return
	}
	defer zeroKey(pKey)

	var hash []byte
	msgDataBak := make(map[uint32]*FTFileMsgData)
	seqID := seqStartID

	for {
		// The remote
		if fh.interactWithCSfailed {
			return
		}

		buff := make([]byte, FileTransferSize)
		// buffSinged := make([]byte, FileTransferSize+FileSignatureSize)
		rsize, rerr = fileBuffer.Read(buff)
		if rerr != nil {
			if rerr != io.EOF {
				ftransDebug("File transfer failed,", "error", rerr)
				return
			} else {
				return
			}
		}

		sendAgain := false
		fail := false
		waitCnt := uint32(0)
		ackID := atomic.LoadUint32(&fh.AckID)
		for seqID > ackID+seqAckDiff {
			waitCnt++
			if waitCnt%5 == 0 {
				d := oneTimeout * time.Duration(waitCnt)
				ftransDebug("File transfer, wait", "seqid", seqID, "ackid", ackID, "waittime", d)
			}
			if waitCnt < uploadTryCount {
				time.Sleep(oneTimeout)
				ackID = atomic.LoadUint32(&fh.AckID)
				continue
			}
			if sendAgain {
				fail = true
				break
			}
			v, ok := msgDataBak[ackID+1]
			if !ok {
				fail = true
				break
			}
			werr = pm.SendFTFileMsg(peer, v)
			if werr != nil {
				ftransDebug("File transfer failed, again", "error", werr)
				fail = true
				break
			}
			sendAgain = true
			waitCnt = 0
			ftransDebug("File transfer failed, ack timeout, send again")
			ackID = atomic.LoadUint32(&fh.AckID)
		}
		if fail {
			goto _fail
		}

		ftransDebug("File transfer, req", "seqid", seqID, "ackid", ackID)

		final := false
		if transferedSize+uint64(rsize) >= fsize {
			final = true
		}
		// Sign
		fileBuff, err := datacrypto.Encrypt(datacrypto.EncryptDes, buff[:rsize], encKey, final)
		if err != nil {
			ftransDebug("File transfer failed", "err", err, "size", rsize)
			goto _fail
		}
		hash = rlpHash([]interface{}{fileBuff}).Bytes()
		sig, serr := crypto.Sign(hash, pKey) // [R || S || V] format
		if serr != nil {
			ftransDebug("File transfer failed,", "error", serr)
			goto _fail
		}
		buffSinged := append(sig[:FileSignatureSize], fileBuff...)
		ftFileData := &FTFileMsgData{
			Buff:    buffSinged,
			RawSize: uint32(rsize),
			SeqID:   seqID,
		}

		delete(msgDataBak, ackID)
		msgDataBak[seqID] = ftFileData

		seqID++
		// Transfer
		werr = pm.SendFTFileMsg(peer, ftFileData)
		if werr != nil {
			ftransDebug("File transfer failed,", "error", werr)
			goto _fail
		}

		transferedSize += uint64(rsize)
		if transferedSize >= fsize {
			transferedSize = fsize
			ftransDebug("File transfer completed, waiting for results of blizcs")
		}
		transpercent := TransferPercent(float64(transferedSize) / float64(fsize) * 100)
		go transpercent.SafeSendChTo(pm.TransferTCPRateWF)
		if transferedSize >= fsize {
			ftransDebug("File transfer completed")
			return
		}
	}

_fail:
	pm.SendTransferFileRet(peer.id, fh.Name, "File transfer failed abnormally", false)
}

// Cs receives the data which comes from wework
func (pm *ProtocolManager) receiveFile2CS(id string, path string, fsize uint64, signed bool, p *Peer) {
	fh, ok := pm.fileHead[id]
	if !ok {
		ftransDebug("File transfer failed,", "error", "file head does not exist")
		pm.sendWriteFileRet(id, fh.Name, "file head does not exist", false)
		return
	}

	pm.uploadingPeer[id] = struct{}{}
	defer delete(pm.uploadingPeer, id)

	var (
		transferedSize uint64 = 0
		fragmentBuf           = make([]byte, 0)
	)

	fh.SetTransferCSSize(0)
	var leftBuf []byte
	var actualSize uint64 // actual transfer size
	var ret bool

	seqID := seqStartID
	for {
		if fh.interactWithCSfailed {
			delete(pm.fileHead, id)
			mu, ok := pm.peerUpDatasLock[p]
			if ok {
				mu.Lock()
				defer mu.Unlock()
				pm.peerUpDatas[p] = make(map[uint32]*FTFileData)
			}

			ftransDebug("File transfer failed,", "error", "transmit to blizcs failed")
			pm.sendWriteFileRet(id, fh.Name, "transmit to blizcs failed", false)
			return
		}

		// t := len(pm.uploadingPeer) + len(pm.downloadingPeer)
		// if t > 1 {
		// 	ftransDebug("Uploading, bandwidth for other peers", "second", t-1)
		// 	time.Sleep(time.Second * time.Duration(t-1))
		// }

		// Transfer
		ftFileData := pm.getUpData(p, seqID)
		if ftFileData == nil {
			ftransDebug("Receive file to cs fail, buff nil")
			return
		}

		// Handle
		buffs, fragment := pm.reorganizeBuff(ftFileData.Buff, fragmentBuf)
		fragmentBuf = fragment
		for _, buffSinged := range buffs {
			// Transmit buffer to CS
			if leftBuf, ret = pm.handleReceivedBuff(buffSinged, transferedSize, fh, id, leftBuf); !ret {
				return
			}
			leftBuf = pm.handleReceivedBuff2(leftBuf, &actualSize, fh, id, false, seqID)
			transferedSize += uint64(len(buffSinged) - FileSignatureSize)
			if transferedSize >= fsize {
				transferedSize = fsize
				ftransDebug("File received completed, waiting for results of blizcs")
				leftBuf = pm.handleReceivedBuff2(leftBuf, &actualSize, fh, id, true, seqID)
				return
			}
		}
		if fsize-transferedSize == uint64(len(fragment)-FileSignatureSize-(len(ftFileData.Buff)-FileSignatureSize-int(ftFileData.RawSize))) {
			// Transmit buffer to CS
			if leftBuf, ret = pm.handleReceivedBuff(fragment, transferedSize, fh, id, leftBuf); !ret {
				return
			}
			ftransDebug("File received completed, waiting for results of blizcs")
			leftBuf = pm.handleReceivedBuff2(leftBuf, &actualSize, fh, id, true, seqID)
			return
		}

		seqID++
	}
}

func (pm *ProtocolManager) handleReceivedBuff(buffSigned []byte, transferedSize uint64, fh *FileTransferReqData,
	id string, leftBuf []byte) ([]byte, bool) {
	// Verify signature
	buff := buffSigned[FileSignatureSize:]
	hash := rlpHash([]interface{}{buff}).Bytes()
	if !crypto.VerifySignature(fh.PubKey, hash, buffSigned[:FileSignatureSize]) {
		ftransDebug("File transfer failed,", "error", "invalid file signature")
		pm.sendWriteFileRet(id, fh.Name, "invalid file signature", false)
		return nil, false
	}
	return append(leftBuf, buff...), true
}

// MB alignment
func (pm *ProtocolManager) handleReceivedBuff2(leftBuf []byte, actualSize *uint64, fh *FileTransferReqData,
	id string, finish bool, seqID uint32) []byte {
	var leftRet []byte
	leftBufSize := uint64(len(leftBuf))
	var buff []byte
	size := *actualSize
	off := fh.Offset + size + leftBufSize
	left := off - off/mSize*mSize
	if left == 0 || finish {
		buff = leftBuf
		leftRet = leftBuf[:0]
	} else {
		count := leftBufSize - left
		buff = leftBuf[:count]
		leftRet = leftBuf[count:]
	}
	*actualSize = size + uint64(len(buff))

	go pm.TransmitToCS(fh.ObjId, fh.Offset+size, id, buff, seqID)
	return leftRet
}

func (pm *ProtocolManager) reorganizeBuff(buff []byte, remFragment []byte) ([][]byte, []byte) {
	buffers := make([][]byte, 0)
	fragment := make([]byte, 0)
	bagSize := FileTransferSize + FileSignatureSize
	offset := 0
	remBuffSize := len(buff)
	remFragSize := len(remFragment)

	if remFragSize > 0 && remFragSize+remBuffSize >= bagSize {
		buffers = append(buffers, append(remFragment, buff[:bagSize-remFragSize]...))
		remBuffSize -= bagSize - remFragSize
		offset += bagSize - remFragSize
		remFragment = make([]byte, 0)
		remFragSize = 0
	}

	for ; remBuffSize >= bagSize; remBuffSize -= bagSize {
		buffers = append(buffers, buff[offset:offset+bagSize])
		offset += bagSize
	}

	if remFragSize > 0 {
		fragment = append(fragment, remFragment...)
	}
	if remBuffSize > 0 {
		fragment = append(fragment, buff[offset:]...)
	}

	return buffers, fragment
}

func (pm *ProtocolManager) TransmitToCS(objId uint64, offset uint64, pid string, buff []byte, seqID uint32) {
	req := &exporter.ObjWriteFtp2CS{
		ObjId:  objId,
		Offset: offset,
		Buff:   buff,
		PeerId: pid,
	}
	log.Info("Transmit to cs", "objid", req.ObjId, "offset", req.Offset, "len", len(req.Buff), "seqid", seqID)
	rsp := pm.ftrans.blizCsCross.ObjWriteFtp2CSFunc(req)
	if rsp == nil {
		log.Info("Transmit to cs rsp nil")
		return
	}
	log.Info("Transmit to cs", "rsp", rsp)
	pm.ObjWriteFtp2CSRspFunc(rsp, seqID)
	log.Info("Transmit to cs end")
}

func (pm *ProtocolManager) ObjWriteFtp2CSRspFunc(rsp *exporter.ObjWriteFtp2CSRsp, seqID uint32) {
	p := pm.peers.Peer(rsp.PeerId)
	if p == nil {
		log.Error(fmt.Sprintf("The peer %s does not exist", rsp.PeerId))
		return
	}
	data := &WriteDataRet{
		ObjId:  rsp.ObjId,
		Size:   rsp.Size,
		RetVal: rsp.RetVal,
		SeqID:  seqID,
	}
	err := pm.DoWriteDataRet(p, data)
	if err != nil {
		log.Error("SendWriteDataRetMsg", "err", err)
	}
}

func (pm *ProtocolManager) DoWriteDataRet(peer *Peer, ret *WriteDataRet) error {
	fh, ok := pm.fileHead[peer.id]
	if ok {
		// Write file results
		msg := WriteFileRet{
			FileName: fh.Name,
		}

		if len(ret.RetVal) == 0 {
			pm.fileHead[peer.id].SetTransferCSSize(fh.TransferCSSize + ret.Size)
			csRate := uint32(float32(fh.TransferCSSize) / float32(fh.Size) * 100)
			msgCSRate := WriteFileCSRate{
				FileName: fh.Name,
				CSRate:   csRate,
				ErrInfo:  common.GetErrString(ret.RetVal),
				SeqID:    ret.SeqID,
			}
			if fh.TransferCSSize >= fh.Size {
				msg.Ret = true
				ftransDebug("File transmit to blizcs successfully")
				log.Info("File transfer completed", "name", fh.Name)
				delete(pm.fileHead, peer.id)
				msgCSRate.CSRate = 100
				pm.SendWriteFileCSProgressMsg(peer, &msgCSRate)
				return pm.SendWriteFileRetMsg(peer, &msg)
			} else {
				if err := pm.SendWriteFileCSProgressMsg(peer, &msgCSRate); err != nil {
					log.Error("File transfer rate of CS transmission error,", "error", err)
				}
			}
		} else {
			fh.SetRetWithCS(true)
			msg.Ret = false
			delete(pm.fileHead, peer.id)
			return pm.SendWriteFileRetMsg(peer, &msg)
		}
	} else {
		return pm.SendWriteDataRetMsg(peer, ret)
	}

	return nil
}

func (pm *ProtocolManager) sendWriteFileRet(id, fName, err string, ret bool) error {
	msg := WriteFileRet{fName, ret, err}
	p := pm.peers.Peer(id)
	if p == nil {
		return errors.New("Peer is nil")
	}
	return pm.SendWriteFileRetMsg(p, &msg)
}

func (pm *ProtocolManager) GetPrivateKey() (*ecdsa.PrivateKey, error) {
	ks := fetchKeystore(pm.AccMan)
	if ks != nil {
		if accs := ks.Accounts(); len(accs) != 0 {
			for i := range accs {
				if strings.Compare(accs[i].Address.String(), pm.baseAddress.String()) == 0 {
					return ks.GetKeyCopy(accs[i], pm.basePassphrase)
				}
			}
		}
	}

	return nil, errors.New("GetPrivateKey failed")
}

func (pm *ProtocolManager) SendHashSign(peer *Peer, req *HashSign) error {
	return p2p.Send(peer.rw, HashSignMsg, req)
}

func (pm *ProtocolManager) SendGetHashSign(peer *Peer, req *GetHashSign) error {
	return p2p.Send(peer.rw, GetHashSignMsg, req)
}

// 签名授权（c2s）
func (pm *ProtocolManager) SendSignAuthorizeMsg(peer *Peer, baseAddr *BaseAddressMsgData) error {
	privKey, err := pm.GetPrivateKey()
	if err != nil {
		return err
	}
	defer zeroKey(privKey)
	authDeadline := uint64(time.Now().Unix() + AuthCSDeadline)
	hash := SignAuthorizeHash(pm.authorizedCS, authDeadline)
	r, s, v, err := WalletSign(hash, privKey)
	if err != nil {
		log.Error("WalletSign", "err", err)
		return err
	}
	authData := &SignAuthorizeData{
		CsAddr:       baseAddr.Address,
		UserAddr:     pm.GetBaseAddress(),
		AuthDeadline: authDeadline,
		R:            r,
		S:            s,
		V:            v,
		Peer:         baseAddr.Peer,
	}
	return p2p.Send(peer.rw, SignAuthorizeMsg, authData)
}

// 写数据(c2s)
func (pm *ProtocolManager) SendWriteDataMsg(peer *Peer, req *WriteDataMsgData) error {
	return p2p.Send(peer.rw, WriteDataMsg, req)
}

// 写数据返回(s2c)
func (pm *ProtocolManager) SendWriteDataRetMsg(peer *Peer, req *WriteDataRet) error {
	return p2p.Send(peer.rw, WriteDataRetMsg, req)
}

func (pm *ProtocolManager) SendWriteFileRetMsg(peer *Peer, msg *WriteFileRet) error {
	return p2p.Send(peer.rw, SendFileTerminateMsg, msg)
}

func (pm *ProtocolManager) SendWriteFileCSProgressMsg(peer *Peer, msg *WriteFileCSRate) error {
	return p2p.Send(peer.rw, SendFileCSProgressMsg, msg)
}

// 读数据(c2s)
func (pm *ProtocolManager) SendReadDataMsg(peer *Peer, req *ReadDataMsgData) error {
	return p2p.Send(peer.rw, ReadDataMsg, req)
}

func (pm *ProtocolManager) SendReadFileRetMsg(peer *Peer, msg *ReadFileRet) error {
	return p2p.Send(peer.rw, ReadFileTerminateMsg, msg)
}

func (pm *ProtocolManager) SendReadFileReq(peer *Peer, filePath string, reqHead FileTransferReqData) error {
	var err error = nil
	if reqHead.Name == "" {
		return errors.New("the file name is invalid")
	}
	if reqHead.Size == 0 {
		return errors.New("the file size is invalid")
	}

	pm.fileHead[peer.id] = &reqHead
	if err = p2p.Send(peer.rw, ReadFileReqMsg, reqHead); err != nil {
		delete(pm.fileHead, peer.id)
	}

	return err
}

func (pm *ProtocolManager) DoFileReadRsp(peer *Peer) error {
	rsp := FileTransferRspStart
	if err := p2p.Send(peer.rw, ReadFileRspMsg, rsp); err != nil {
		delete(pm.fileHead, peer.id)
		return err
	}

	fh, ok := pm.fileHead[peer.id]
	if !ok {
		return errors.New(fmt.Sprintf("%s: %s", "error in DoFileSendOp", "file head does not exist"))
	}
	size := fh.Size
	fh.SetRetWithCS(false)
	go pm.sendCSFile(size, true, peer)

	return nil
}

func (pm *ProtocolManager) DoFileReadOp(peer *Peer) error {
	fh, ok := pm.fileHead[peer.id]
	if !ok {
		return errors.New(fmt.Sprintf("%s: %s", "error in DoFileSendOp", "file head does not exist"))
	}
	fh.SetRetWithCS(false)
	go pm.receiveFile2Disk(peer.id, fh.Name, fh.Size, fh, peer.RemoteAddr().String(), peer)

	return nil
}

func (pm *ProtocolManager) sendCSFile(fsize uint64, signed bool, peer *Peer) {
	id := peer.id
	fh, ok := pm.fileHead[id]
	if !ok {
		ftransDebug("File transfer failed,", "error", "file head does not exist")
		return
	}
	defer delete(pm.fileHead, id)

	pm.downloadingPeer[id] = struct{}{}
	defer delete(pm.downloadingPeer, id)

	var (
		transferedSize uint64 = 0
		wsize                 = 0
		werr           error
	)
	defer delete(pm.fileToTransfer, id)
	msgDataBak := make(map[uint32]*FTFileMsgData)
	seqID := seqStartID
	// first := true // For less reading count from farmer server
	for {
		// The remote
		if fh.interactWithCSfailed {
			return
		}

		// Transmit from CS
		length := fh.Size - transferedSize
		if length > FileTransferSize {
			length = FileTransferSize
		} else if length <= 0 {
			return
		}

		// t := len(pm.uploadingPeer) + len(pm.downloadingPeer)
		// if t > 1 {
		// 	ftransDebug("Downloading, bandwidth for other peers", "second", t-1)
		// 	time.Sleep(time.Second * time.Duration(t-1))
		// }

		sendAgain := false
		fail := false
		waitCnt := uint32(0)
		ackID := atomic.LoadUint32(&fh.AckID)
		for seqID > ackID+seqAckDiff {
			waitCnt++
			if waitCnt%5 == 0 {
				d := oneTimeout * time.Duration(waitCnt)
				ftransDebug("File read, wait", "seqid", seqID, "ackid", ackID, "waittime", d)
			}
			if waitCnt < downloadTryCount {
				time.Sleep(oneTimeout)
				ackID = atomic.LoadUint32(&fh.AckID)
				continue
			}
			if sendAgain {
				fail = true
				break
			}
			v, ok := msgDataBak[ackID+1]
			if !ok {
				fail = true
				break
			}
			werr = pm.SendFTWriteMsg(peer, v)
			if werr != nil {
				ftransDebug("File transfer failed,", "error", werr)
				fail = true
				break
			}
			sendAgain = true
			waitCnt = 0
			ftransDebug("File read failed, ack timeout, send again")
			ackID = atomic.LoadUint32(&fh.AckID)
		}
		if fail {
			goto _fail
		}

		ftransDebug("File read, req", "seqid", seqID, "ackid", ackID)

		// if first {
		// 	left := mSize - fh.Offset%mSize
		// 	if length > left {
		// 		length = left
		// 	}
		// 	first = false
		// }
		tmpSeqID := seqID
		go pm.TransmitFromCS(fh.ObjId, fh.Offset+transferedSize, length, id, fh.SharerCSNodeID, fh.Sharer, tmpSeqID)
		// chClosed, fileBuff := func() (closed bool, buff []byte) {
		// 	defer func() {
		// 		if recover() != nil {
		// 			closed = true
		// 			buff = []byte{}
		// 		}
		// 	}()
		// 	buff = <-pm.FileDataFromCS[id]
		// 	return false, buff
		// }()
		// if chClosed {
		// 	return
		// }

		fileDataFromCS := pm.getDownData(peer, seqID)
		// Error
		if fileDataFromCS == nil {
			ftransDebug("File transfer failed, cs error")
			goto _fail
		}

		// Transfer
		msgData := &FTFileMsgData{
			Buff:    fileDataFromCS.Buff,
			RawSize: fileDataFromCS.RawSize,
			SeqID:   seqID,
		}

		delete(msgDataBak, ackID)
		msgDataBak[seqID] = msgData

		werr = pm.SendFTWriteMsg(peer, msgData)
		if werr != nil {
			ftransDebug("File transfer failed,", "error", werr)
			goto _fail
		}
		wsize = int(fileDataFromCS.RawSize)

		transferedSize += uint64(wsize)
		if transferedSize >= fsize {
			transferedSize = fsize
			ftransDebug("File transfer completed")
			return
		}

		seqID++
	}

_fail:
	msg := &ReadFileRet{
		FileName: fh.Name,
		ErrInfo:  "File transfer failed abnormally",
	}
	pm.SendReadFileRetMsg(peer, msg)
}

func (pm *ProtocolManager) TransmitFromCS(objId, offset uint64, length uint64, pid string, sharerCSNodeID string, sharer string, seqID uint32) {
	req := &exporter.ObjReadFtp2CS{
		ObjId:          objId,
		Offset:         offset,
		Len:            length,
		PeerId:         pid,
		SharerCSNodeID: sharerCSNodeID,
		Sharer:         sharer,
	}
	rsp := pm.ftrans.blizCsCross.ObjReadFtp2CSFunc(req)
	pm.ftrans.ObjReadFtp2CSRspFunc(rsp, seqID)
}

func (pm *ProtocolManager) receiveFail(errStr string) {
	ret := DataSendRet{
		Ret: false,
		Err: errStr,
	}
	go ret.SafeSendChTo(pm.DataSendFinishRF)
}

// Wemore server sends the data to local pc
// receiveFile2Disk receives the file from other node and save in the disk of the client
func (pm *ProtocolManager) receiveFile2Disk(id, path string, fsize uint64, fh *FileTransferReqData,
	remoteAddr string, p *Peer) bool {
	defer delete(pm.fileHead, id)

	if fh == nil {
		log.Warn("File transfer failed, FileTransferReqData is nil")
		pm.receiveFail("FileTransferReqData is nil")
		return false
	}

	var (
		transferedSize uint64 = 0
		rsize                 = 0
	)
	fh.SetTransferCSSize(0)
	file, cerr := os.Create(path)
	if cerr != nil {
		log.Warn("File transfer failed,", "error", cerr.Error())
		pm.receiveFail(cerr.Error())
		return false
	}
	defer file.Close()

	seqID := seqStartID
	for {
		if fh.interactWithCSfailed {
			log.Warn("File transfer failed,", "error", "interact with blizcs failed")
			// pm.receiveFail("Interact with blizcs failed") // It is handled in ReadFileTerminateMsg msg
			return false
		}

		// Transfer
		fileDataFromCS := pm.GetLocalData(seqID)
		if fileDataFromCS == nil {
			log.Warn("File transfer failed, file data from cs nil")
			pm.receiveFail("File data from cs nil")
			return false
		}
		rsize = int(fileDataFromCS.RawSize)

		addr := pm.baseAddress
		if fh.Sharer != "0" {
			addr = common.HexToAddress(fh.Sharer)
		}
		k := common.KeyID(addr, fh.ObjId)
		encKey := pm.GetEncryptKey(k)
		if encKey == nil {
			log.Warn("File transfer failed, encrypt key nil")
			pm.receiveFail("Encrypt key nil")
			return false
		}

		final := false
		if transferedSize+uint64(rsize) >= fsize {
			final = true
		}
		fileBuff, err := datacrypto.Decrypt(datacrypto.EncryptDes, fileDataFromCS.Buff, encKey, final)
		// Continue to handle, but data is not right
		if err != nil {
			log.Warn("File transfer, decrypt failed", "err", err)
			pm.receiveFail(err.Error())
			return false
		}

		// Continue to handle, but data is not right
		if len(fileBuff) < rsize {
			ftransDebug("File transfer, decrypt failed", "rawsize", rsize, "decryptedsize", len(fileBuff))
			r := bytes.Repeat([]byte{0}, rsize-len(fileBuff))
			fileBuff = append(fileBuff, r...)
		}

		// Save to disk
		//if _, werr := file.Write(fileBuff[:rsize]); werr != nil {
		if _, werr := SaveFileBuff2Disk(file, fileBuff, rsize); werr != nil {
			log.Warn("File transfer failed,", "error", werr.Error())
			pm.receiveFail(werr.Error())
			return false
		}

		transferedSize += uint64(rsize)
		if transferedSize >= fsize {
			transferedSize = fsize
			ftransDebug(fmt.Sprintf("File transfer completed, saved in %s", path))
		}
		transpercent := TransferPercent(float64(transferedSize) / float64(fsize) * 100)
		go transpercent.SafeSendChTo(pm.TransferTCPRateRF)
		if transferedSize >= fsize {
			return true
		}

		seqID++
	}
}

func SaveFileBuff2Disk(file *os.File, fileBuff []byte, rsize int) (int, error) {
	return file.Write(fileBuff[:rsize])
}

func (pm *ProtocolManager) SendTransferFileRet(id, fName, err string, ret bool) {
	msg := TransferRet{fName, ret, err}
	if pm.peers.Peer(id) != nil {
		p2p.Send(pm.peers.Peer(id).rw, SendFileTerminateMsg, msg)
	} else {
		log.Error("can not find the peer", "id", id)
	}
}

// 读数据返回(s2c)
func (pm *ProtocolManager) SendReadDataRetMsg(peer *Peer, data *DataReadRet) error {
	return p2p.Send(peer.rw, ReadDataRetMsg, data)
}

func (pm *ProtocolManager) DoReadDataRet(peer *Peer, data *DataReadRet, seqID uint32) error {
	fh, ok := pm.fileHead[peer.id]
	if ok {
		down := &FTFileMsgData{
			Buff:    data.Buff,
			RawSize: data.RawSize,
			SeqID:   seqID,
		}
		pm.pushDownData(down, peer)
		if data.Ret == nil {
			pm.fileHead[peer.id].SetTransferCSSize(fh.TransferCSSize + uint64(len(data.Buff)))
			pm.fileHead[peer.id].SetAckID(seqID)
			ftransDebug("File read, ack", "ackid", seqID)
		} else {
			fh.SetRetWithCS(true)
			msg := ReadFileRet{
				FileName: fh.Name,
				Ret:      false,
				ErrInfo:  common.GetErrString(data.Ret),
			}
			delete(pm.fileHead, peer.id)
			return pm.SendReadFileRetMsg(peer, &msg)
		}
	} else {
		return pm.SendReadDataRetMsg(peer, data)
	}

	return nil
}

func (pm *ProtocolManager) pushFileData(msg *FTFileMsgData, p *Peer) {
	mu, ok := pm.peerUpDatasLock[p]
	if !ok {
		return
	}
	mu.Lock()
	defer mu.Unlock()

	datas, ok := pm.peerUpDatas[p]
	if !ok {
		return
	}
	datas[msg.SeqID] = &FTFileData{
		Buff:    msg.Buff,
		RawSize: msg.RawSize,
	}
	pm.peerUpDatas[p] = datas
}

func (pm *ProtocolManager) popUpData(p *Peer, seqID uint32) (*FTFileData, bool) {
	mu, ok := pm.peerUpDatasLock[p]
	if !ok {
		return nil, false
	}
	mu.Lock()
	defer mu.Unlock()

	datas, ok := pm.peerUpDatas[p]
	if !ok {
		return nil, false
	}
	data, ok := datas[seqID]
	if !ok {
		return nil, true
	}
	ret := &FTFileData{
		Buff:    data.Buff,
		RawSize: data.RawSize,
	}

	delete(datas, seqID)
	pm.peerUpDatas[p] = datas

	return ret, false
}

func (pm *ProtocolManager) getUpData(p *Peer, seqID uint32) *FTFileData {
	for i := seqStartID; i < uploadTryCount; i++ {
		data, wait := pm.popUpData(p, seqID)
		if !wait {
			return data
		}
		time.Sleep(oneTimeout)
	}
	log.Warn("Get file data, timeout", "seqid", seqID)
	return nil
}

func (pm *ProtocolManager) pushDownData(msg *FTFileMsgData, p *Peer) {
	mu, ok := pm.peerDownDatasLock[p]
	if !ok {
		return
	}
	mu.Lock()
	defer mu.Unlock()

	datas, ok := pm.peerDownDatas[p]
	if !ok {
		return
	}
	datas[msg.SeqID] = &FTFileData{
		Buff:    msg.Buff,
		RawSize: msg.RawSize,
	}
	pm.peerDownDatas[p] = datas
}

func (pm *ProtocolManager) popDownData(p *Peer, seqID uint32) (*FTFileData, bool) {
	mu, ok := pm.peerDownDatasLock[p]
	if !ok {
		return nil, false
	}
	mu.Lock()
	defer mu.Unlock()

	datas, ok := pm.peerDownDatas[p]
	if !ok {
		return nil, false
	}
	data, ok := datas[seqID]
	if !ok {
		return nil, true
	}

	// Error
	if len(data.Buff) == 0 {
		return nil, false
	}

	buff := make([]byte, len(data.Buff))
	copy(buff, data.Buff)
	ret := &FTFileData{
		Buff:    buff,
		RawSize: data.RawSize,
	}

	delete(datas, seqID)
	pm.peerDownDatas[p] = datas

	return ret, false
}

func (pm *ProtocolManager) getDownData(p *Peer, seqID uint32) *FTFileData {
	for i := seqStartID; i < downloadTryCount; i++ {
		data, wait := pm.popDownData(p, seqID)
		if !wait {
			return data
		}
		time.Sleep(oneTimeout)
	}
	log.Warn("Get down data, timeout", "seqid", seqID)
	return nil
}

func (pm *ProtocolManager) pushLocalData(msg *FTFileMsgData) {
	pm.localDatasLock.Lock()
	defer pm.localDatasLock.Unlock()

	data := &FTFileData{
		Buff:    msg.Buff,
		RawSize: msg.RawSize,
	}
	pm.localDatas[msg.SeqID] = data
}

func (pm *ProtocolManager) popLocalData(seqID uint32) (*FTFileData, bool) {
	pm.localDatasLock.Lock()
	defer pm.localDatasLock.Unlock()

	data, ok := pm.localDatas[seqID]
	if !ok {
		return nil, true
	}

	buff := make([]byte, len(data.Buff))
	copy(buff, data.Buff)
	ret := &FTFileData{
		Buff:    buff,
		RawSize: data.RawSize,
	}

	delete(pm.localDatas, seqID)
	return ret, false
}

func (pm *ProtocolManager) GetLocalData(seqID uint32) *FTFileData {
	for i := seqStartID; i < writeTryCount; i++ {
		data, wait := pm.popLocalData(seqID)
		if !wait {
			return data
		}
		time.Sleep(oneTimeout)
	}
	log.Warn("Get local data, timeout", "seqid", seqID)
	return nil
}

func (pm *ProtocolManager) SetEncryptKey(k string, v []byte) {
	pm.encryptKeys[k] = v
}

func (pm *ProtocolManager) GetEncryptKey(k string) []byte {
	return pm.encryptKeys[k]
}

// 数据签名确认(s2c)
func (pm *ProtocolManager) SendGetDataSignConfirmMsg(peer *Peer, req *GetDataSignConfirmMsgData) error {
	return p2p.Send(peer.rw, GetDataSignConfirmMsg, req)
}

// 数据签名确认返回(c2s)
func (pm *ProtocolManager) SendDataSignConfirmMsg(peer *Peer, req *DataSignConfirmMsgData) error {
	return p2p.Send(peer.rw, DataSignConfirmMsg, req)
}

// 发送公钥
func (pm *ProtocolManager) SendBaseAddress(peer *Peer) error {
	return p2p.Send(peer.rw, BaseAddressMsg, &BaseAddressMsgData{Address: pm.baseAddress, Peer: peer.id})
}

func (pm *ProtocolManager) SendClientBaseAddress(peer *Peer) error {
	return p2p.Send(peer.rw, ClientBaseAddressMsg, &BaseAddressMsgData{Address: pm.baseAddress, Peer: peer.id})
}

func (pm *ProtocolManager) HandleInteractiveInfo(peer *Peer, info *InteractiveInfo, isClient bool) error {

	return nil
}

func (pm *ProtocolManager) SendAuthAllowFlow(peer *Peer, flow uint64, sign []byte) error {
	req := &AuthAllowFlowMsgData{Signer: pm.baseAddress, Flow: flow, Sign: sign}
	return p2p.Send(peer.rw, AuthAllowFlow, req)
}

func (pm *ProtocolManager) handleAuthAllowFlow(data *AuthAllowFlowMsgData) error {
	pm.ftrans.blizCsCross.UpdateAuthAllowFlow(data.Signer, data.Flow, data.Sign)
	return nil
}

func (pm *ProtocolManager) SendFTWriteMsg(peer *Peer, data *FTFileMsgData) error {
	return p2p.Send(peer.rw, SendFTWriteMsg, data)
}

func (pm *ProtocolManager) SendFTFileMsg(peer *Peer, data *FTFileMsgData) error {
	return p2p.Send(peer.rw, SendFTFileMsg, data)
}
