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

package blizcs

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/elephant/exporter"
	"github.com/contatract/go-contatract/log"
)

type PublicBlizCSAPI struct {
	blizcs *BlizCS
}

const rwDir = "httpTest"

func NewPublicBlizCSAPI(b *BlizCS) *PublicBlizCSAPI {
	return &PublicBlizCSAPI{b}
}

func (api *PublicBlizCSAPI) Test() bool {
	api.Log().Info("It works")
	return true
}

// discard
// func (api *PublicBlizCSAPI) CreateObject(objId uint64, size uint64, password string) bool {
// 	ret := api.blizcs.CreateObject(objId, size, password)
// 	return ret == nil
// }

// discard
// func (api *PublicBlizCSAPI) AddObjectStorage(objId uint64, size uint64, password string) bool {
// 	ret := api.blizcs.CreateObject(objId, size, password)
// 	return ret == nil
// }

// discard
// func (api *PublicBlizCSAPI) OpenObject(objId uint64) {
// 	api.blizcs.OpenObject(objId)
// }

// discard
// offset: ObjId的起始位置
// func (api *PublicBlizCSAPI) DoSeek(objId uint64, offset uint64) {
// 	clnt := api.blizcs.GetObject(objId)
// 	if clnt == nil {
// 		api.Log().Error("clnt nil", "objId", objId)
// 		return
// 	}
// 	clnt.DoSeek(offset)
// }

// discard
// func (api *PublicBlizCSAPI) Write(objId uint64, offset uint64, data string, address string) bool {
// 	base := common.HexToAddress(address)
// 	clnt := api.blizcs.GetObject(objId, base)
// 	if clnt == nil {
// 		api.Log().Error("clnt nil", "objId", objId)
// 		return false
// 	}
// 	clnt.DoSeek(offset)
// 	_, err := clnt.Write([]byte(data))
// 	if err != nil {
// 		api.Log().Error("Write", "err", err)
// 		return false
// 	}
// 	return true
// }

// discard
// func (api *PublicBlizCSAPI) Read(objId uint64, offset uint64, length uint32) string {
// 	clnt := api.blizcs.GetObject(objId)
// 	if clnt == nil {
// 		api.Log().Error("clnt nil", "objId", objId)
// 		return ""
// 	}
// 	clnt.DoSeek(offset)
// 	buffer := make([]byte, length)
// 	clnt.Read(buffer)
// 	logLen := len(buffer)
// 	if logLen > 10 {
// 		logLen = 10
// 	}
// 	return string(buffer)
// }

// discard
// func (api *PublicBlizCSAPI) Dail(objId uint64, cId uint64, count uint32) {
// 	clnt := api.blizcs.GetObject(objId)
// 	if clnt == nil {
// 		api.Log().Error("clnt nil", "objId", objId)
// 		return
// 	}
// 	chunkId := storage.Int2ChunkIdentity(cId)
// 	clnt.trickConnFarmers(&chunkId, int(count))
// }

// discard
// func (api *PublicBlizCSAPI) WriteFile(objId uint64, offset uint64, fromFileName string) string {
// 	tip := "upload file " + fromFileName + " fail"
// 	clnt := api.blizcs.GetObject(objId)
// 	if clnt == nil {
// 		api.Log().Error("clnt nil", "objId", objId)
// 		return tip
// 	}
// 	clnt.DoSeek(offset)
// 	filePath := filepath.Join(rwDir, fromFileName)
// 	data, err := ioutil.ReadFile(filePath)
// 	if err != nil {
// 		api.Log().Error("ReadFile", "err", err)
// 		return tip
// 	}
// 	dataLen := len(data)
// 	_, err = clnt.Write(data)
// 	if err != nil {
// 		api.Log().Error("Write", "err", err)
// 		return tip
// 	}
// 	tip = "upload file " + fromFileName + " success, the length is " + strconv.Itoa(dataLen)
// 	return tip
// }

// discard
// func (api *PublicBlizCSAPI) ReadFile(objId uint64, offset uint64, length uint32, toFileName string) bool {
// 	clnt := api.blizcs.GetObject(objId)
// 	if clnt == nil {
// 		api.Log().Error("clnt nil", "objId", objId)
// 		return false
// 	}
// 	clnt.DoSeek(offset)
// 	buffer := make([]byte, length)
// 	err := clnt.Read(buffer)
// 	if err != nil {
// 		api.Log().Error("Read", "err", err)
// 		return false
// 	}
// 	toFileName = filepath.Join(rwDir, toFileName)
// 	err = ioutil.WriteFile(toFileName, buffer, 0644)
// 	if err != nil {
// 		api.Log().Error("WriteFile", "err", err)
// 		return false
// 	}
// 	return true
// }

func (api *PublicBlizCSAPI) Log() log.Logger {
	return api.blizcs.Log()
}

func (api *PublicBlizCSAPI) GetRentSize(objId uint64, address string) string {
	var result string
	base := common.HexToAddress(address)
	clnt, err := api.blizcs.GetObject(objId, base)
	if clnt == nil {
		api.Log().Error("clnt nil", "objId", objId)
		result = err.Error()
		return result
	}
	slices := clnt.GetRentStorage()
	api.Log().Info("GetRentStorage ok", "size(M)", len(slices))
	result = result + "rent size: " + strconv.Itoa(len(slices)) + "M"
	var describeStr string
	for i, v := range slices {
		describeStr += v.String()
		if i < len(slices)-1 {
			describeStr += ","
		}
	}
	if len(describeStr) > 0 {
		api.Log().Info("storage as following", "describeStr", describeStr)
		result = result + ", list as following(chunkId:sliceId): " + describeStr
	}
	return result
}

func (api *PublicBlizCSAPI) Claim() string {
	return api.blizcs.Claim()
}

func (api *PublicBlizCSAPI) GetObjectsInfo(password string, address string) string {
	ret := "object id not find"
	objsInfo := api.blizcs.GetObjectsInfo(password, address, 3)
	for i, info := range objsInfo {
		var obj *exporter.RetObjectInfo
		if v, ok := info.(*exporter.SelfObjectInfo); ok {
			obj = v.NewRetObjectInfo()
		} else if v, ok := info.(*exporter.ShareObjectInfo); ok {
			obj = v.NewRetObjectInfo()
		}

		if obj == nil {
			log.Error("Get objects info, obj nil")
			return ret
		}
		data, err := json.Marshal(obj)
		if err != nil {
			log.Error("Get objects info", "err", err)
			return ret
		}
		c := len(data)
		if c >= 2 && data[0] == '[' && data[c-1] == ']' {
			data = data[1 : c-1]
		}
		if i == 0 {
			ret = ""
		} else {
			ret += "/"
		}
		ret += string(data)
	}
	api.Log().Info("Get objects info", "ret", ret)
	return ret
}

// discard
// func (api *PublicBlizCSAPI) WSWrite(objId uint64, data string, offset uint64) bool {
// 	data2, _ := hex.DecodeString(data)
// 	ret := api.Write(objId, offset, string(data2))
// 	return ret
// }

// discard
// func (api *PublicBlizCSAPI) WSRead(objId uint64, length uint32, offset uint64) string {
// 	ret := api.Read(objId, offset, length)
// 	ret2 := hex.EncodeToString([]byte(ret))
// 	return ret2
// }

func (api *PublicBlizCSAPI) TestString(objId uint64, data1 string, data2 string, offset uint64) bool {
	data3, err := hex.DecodeString(data2)
	if err != nil {
		api.Log().Error("DecodeString", "err", err)
		return false
	}
	api.Log().Info("", "objId", objId, "data1", data1, "data2", data2, "offset", offset, "data3 len", len(data3))
	return true
}

// discard
// func (api *PublicBlizCSAPI) WriteSign(ts uint64, signPeer string) bool {
// 	data := new(blizcore.WriteOperReqData)
// 	data.Info = &blizcore.WopInfo{WopTs: ts}
// 	_, err := api.blizcs.GetClientBase()
// 	if err != nil {
// 		api.Log().Error("GetClientBase", "err", err)
// 		return false
// 	}
// 	r, s, v, err := signWop(data.Hash(), api.blizcs.basePrivateKey, signPeer)
// 	if err != nil {
// 		api.Log().Error("signWop", "err", err)
// 		return false
// 	}
// 	data.Info.R, data.Info.S, data.Info.V = r, s, v
// 	api.Log().Info("signWop", "R", data.Info.R, "S", data.Info.S, "V", data.Info.V)
// 	addr, err := data.Sender()
// 	if err != nil {
// 		api.Log().Error("Sender", "err", err)
// 		return false
// 	}
// 	api.Log().Info("Sender", "addr", addr)
// 	return true
// }

// used flow is stored in cs
// user auth flow is also stored in cs
func (s *PublicBlizCSAPI) GetCsServeCost(cliAddr string, payMethod uint8) bool {
	csbase := s.blizcs.csbase
	addr := common.HexToAddress(cliAddr)
	var usedFlow uint64
	var authAllowFlow uint64
	var sign []byte
	if payMethod == 1 {
		usedFlow = s.blizcs.getUsedFlow(addr)
		if usedFlow == 0 {
			s.Log().Info("usedFlow 0")
			return false
		}
		data := s.blizcs.getAuthAllowFlowData(addr)
		if data == nil {
			s.Log().Info("getAuthAllowFlowData nil")
			return false
		}
		authAllowFlow = data.Flow
		sign = data.Sign
		s.Log().Info("GetCsServeCost", "usedFlow", usedFlow, "authAllowFlow", authAllowFlow, "sign len", len(sign))
	}
	params := make([]interface{}, 0, 6)
	params = append(params, csbase)
	params = append(params, addr)
	params = append(params, payMethod)
	params = append(params, usedFlow)
	params = append(params, authAllowFlow)
	params = append(params, sign)
	req := &exporter.ObjGetElephantData{Ty: exporter.TypeCsToElephantServeCost, Params: params}
	s.blizcs.bliz.EleCross.ObjGetElephantDataFunc(req)
	return true
}

func (api *PublicBlizCSAPI) GetUsedFlow(address string) (uint64, error) {
	addr := common.HexToAddress(address)
	flow := api.blizcs.getUsedFlow(addr)
	return flow, nil
}

func (api *PublicBlizCSAPI) GetAuthAllowFlowData(address string) string {
	ret := "empty"
	addr := common.HexToAddress(address)
	data := api.blizcs.getAuthAllowFlowData(addr)
	if data == nil {
		return ret
	}
	return fmt.Sprintf("CsAddr:%s, Flow:%d, Sign len:%d", data.CsAddr.Hex(), data.Flow, len(data.Sign))
}

func (api *PublicBlizCSAPI) ShowAsso(chunkId uint64) {
	api.blizcs.bliz.ShowAsso(chunkId)
}

func (api *PublicBlizCSAPI) ShareObj() {
}
