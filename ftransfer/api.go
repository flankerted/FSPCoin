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
	"mime/multipart"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"

	"github.com/contatract/go-contatract/log"
)

const timeoutS = 30             // 5 seconds
const timeoutF = 3600 * 24 * 30 // one month

var errRateClosed = errors.New("Rate closed")

type PrivateFTransAPI struct {
	ftrans *FileTransfer
}

func NewPrivateFTransAPI(ft *FileTransfer) *PrivateFTransAPI {
	return &PrivateFTransAPI{ft}
}

func (api *PrivateFTransAPI) GetProtocolMan() *ProtocolManager {
	if api != nil {
		if api.ftrans != nil {
			if api.ftrans.protocolManager != nil {
				return api.ftrans.protocolManager
			}
		}
	}

	return nil
}

func (api *PrivateFTransAPI) Test() bool {
	log.Info("It works")
	return true
}

func (api *PrivateFTransAPI) ReadString(objId, offset uint64, len uint64, passphrase string, p *Peer) (string, error) {
	if len > FileTransferSize {
		return "", errors.New(fmt.Sprintf("read string too long, longer than %d KB", FileTransferSize/1024))
	}
	req := ReadDataMsgData{
		ObjId:          objId,
		Offset:         offset,
		Len:            len,
		SharerCSNodeID: "", // Self's cs
		Sharer:         "",
	}
	pm := api.ftrans.protocolManager
	err := pm.SendReadDataMsg(p, &req)
	if err != nil {
		return "", err
	}

	rsCh := make(chan DataReadRet)
	rsSub := pm.scope.Track(pm.dataSendFinishRSFeed.Subscribe(rsCh))
	defer rsSub.Unsubscribe()

	select {
	case <-time.After(time.Second * timeoutS):
		return "", errors.New("Operation failed, read string timeout")

	case ret := <-rsCh:
		return string(ret.Buff), nil

	case err := <-rsSub.Err():
		return "", err
	}
}

func (api *PrivateFTransAPI) WriteString(objId, offset uint64, data []byte, passphrase string, p *Peer) (bool, error) {
	req := WriteDataMsgData{
		ObjId:  objId,
		Offset: offset,
		Buff:   []byte(data),
	}
	pm := api.ftrans.protocolManager
	err := pm.SendWriteDataMsg(p, &req)
	if err != nil {
		return false, err
	}

	wsCh := make(chan DataSendRet)
	wsSub := pm.scope.Track(pm.dataSendFinishWSFeed.Subscribe(wsCh))
	defer wsSub.Unsubscribe()

	select {
	case <-time.After(time.Second * timeoutS):
		return false, errors.New("Operation failed, write string timeout")

	case ret := <-wsCh:
		return ret.Ret, nil

	case err := <-wsSub.Err():
		return false, err
	}
}

func (api *PrivateFTransAPI) WriteFile(data *multipart.File, name string, objId uint64, offset uint64, size uint64, pubKey, sig []byte,
	peer *Peer, rate chan int) (bool, error) {
	var reqHead = FileTransferReqData{
		Name:      name,
		ObjId:     objId,
		Offset:    offset,
		Size:      size,
		PubKey:    pubKey,
		Signature: sig,
	}

	if name == "" {
		return false, errors.New("the file name is invalid")
	}
	if size == 0 {
		return false, errors.New("the file size is invalid")
	}

	pm := api.ftrans.GetProtocolMan()
	err := pm.DoFileSendReq(reqHead, data, peer)
	if err != nil {
		return false, err
	}

	// Wait for transfer to complete
	// r := TransferPercent(0)
	// rCS := TransferCSPercent(0)
	var rCSCh int32
	go func() {
		for {
			select {
			case rCS, _ := <-pm.TransferCSRate:
				if rCS == TransferCSPercentExit {
					log.Info("Write file, receive transfer cs rate chan exit")
					return
				}
				atomic.StoreInt32(&rCSCh, int32(rCS/2))
				if rCS >= 100 {
					log.Info("Write file, transfer cs rate 100")
					return
				}
			}
		}
	}()
	for {
		select {
		case r, _ := <-pm.TransferTCPRateWF:
			if r == TransferPercentExit {
				close(rate)
				log.Info("Write file, receive transfer percent chan exit")
				return false, errRateClosed
			}
			rCSChTmp := atomic.LoadInt32(&rCSCh)
			rate <- int(r/2) + int(rCSChTmp)

			if r >= 100 {
				for {
					select {
					case <-time.After(time.Second * timeoutF):
						close(rate)
						return false, errors.New("Operation failed, transmit to blizcs timeout")

					case ret := <-pm.DataSendFinishWF:
						close(rate)
						if !ret.Ret {
							return ret.Ret, errors.New(fmt.Sprintf("false: %s", ret.Err))
						} else {
							log.Info("File transfer completed")
							return true, nil
						}

					default:
						rate <- int(r/2) + int(rCSChTmp)
						time.Sleep(time.Millisecond * 100)
					}
				}
			}

		case ret := <-pm.DataSendFinishWF:
			ftransDebug("Write file, start DataSendFinishWF")
			if ret.Ret {
				ftransDebug("Write file failed,", "error", "there can not be true", "ret", ret)
				log.Crit("there can not be true in case PrivateFTransAPI:WriteFile:DataSendFinish")
			}
			go func() {
				transpercent := TransferPercentExit
				transpercent.SafeSendChTo(pm.TransferTCPRateWF) // Let the thread of sendFile in Ftp go on and to stop
				ftransDebug("Send TransferTCPRateWF successfully")
			}()
			go func() {
				transpercentCS := TransferCSPercentExit
				transpercentCS.SafeSendChTo(pm.TransferCSRate)
				ftransDebug("Send TransferCSRate successfully")
			}()
			rate <- -100
			ftransDebug("Write file, end DataSendFinishWF")
		}
	}
}

func (api *PrivateFTransAPI) ReadFile(objId uint64, offset uint64, length uint64, path string, pubKey, sig []byte,
	peer *Peer, rate chan int, sharerCSNodeID string, sharer string) (string, error) {
	var reqHead = FileTransferReqData{
		Name:           path,
		ObjId:          objId,
		Offset:         offset,
		Size:           length,
		PubKey:         pubKey,
		Signature:      sig,
		SharerCSNodeID: sharerCSNodeID,
		Sharer:         sharer,
	}
	err := api.ftrans.GetProtocolMan().SendReadFileReq(peer, path, reqHead)
	if err != nil {
		return "The file transfer failed", err
	}
	log.Info("File downloading ...")

	for {
		select {
		case r, _ := <-api.ftrans.GetProtocolMan().TransferTCPRateRF:
			if r == TransferPercentExit {
				close(rate)
				log.Info("Read file, receive transfer tcp rate chan exit")
				return "", errRateClosed
			}
			rate <- int(r)
			//log.Info("ReadFile", "r", r)
			if r >= 100 {
				close(rate)
				log.Info(fmt.Sprintf("The file saved in %s successfully", path))
				return fmt.Sprintf("The file saved in %s successfully", path), nil
			}

		case ret := <-api.ftrans.GetProtocolMan().DataSendFinishRF:
			ftransDebug("Read file, start DataSendFinishRF")
			if ret.Ret {
				ftransDebug("Read file failed,", "error", "there can not be true", "ret", ret)
				log.Crit("there can not be true in case PrivateFTransAPI:WriteFile:DataSendFinish")
			}
			rate <- -100
			go func() {
				transpercent := TransferPercentExit
				transpercent.SafeSendChTo(api.ftrans.GetProtocolMan().TransferTCPRateRF)
				ftransDebug("Send TransferTCPRateRF successfully")
			}()
			ftransDebug("Read file, end DataSendFinishRF")
		}
	}
}

func (api *PrivateFTransAPI) SetEncryptKey(k string, v []byte) {
	api.ftrans.protocolManager.SetEncryptKey(k, v)
}
