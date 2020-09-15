package main

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/contatract/go-contatract/rpc"
)

var url = "ws://127.0.0.1:8546"

const kSize = 1024
const onceLen = kSize * 1024

func main() {
	client, err := rpc.Dial(url)
	if err != nil {
		fmt.Println("Dial", err)
		return
	}
	modules, err := client.SupportedModules()
	if err != nil {
		fmt.Println("SupportedModules", err)
		return
	}
	var supported string
	for k, _ := range modules {
		if len(supported) != 0 {
			supported += ", "
		}
		supported += k
	}
	fmt.Printf("supported modules are as following: %s\n", supported)

	in := bufio.NewReader(os.Stdin)
	for {
		fmt.Println("please input method id")
		met, err := in.ReadString('\n')
		if err != nil {
			fmt.Println("ReadString, err: ", err)
			return
		}
		if len(met) < 2 {
			fmt.Println("met length is less than 2")
			continue
		}
		met = strings.Replace(met, "\r\n", "", -1)
		metId, err := strconv.Atoi(met)
		if err != nil {
			fmt.Println("Atoi err: ", err)
			continue
		}
		if metId == 5 {
			ret := handle_005(client)
			fmt.Println("handle_005, ret: ", ret)
			continue
		} else if metId == 6 {
			ret := handle_006(client)
			fmt.Println("handle_006, ret: ", ret)
			continue
		} else if metId == 7 {
			ret := handle_test(client)
			fmt.Println("handle_test, ret: ", ret)
			continue
		} else {
		}

		var result interface{}
		var method string
		var param []interface{}
		switch metId {
		case 1:
			method, param = method_001()
		case 2:
			method, param = method_002()
		case 3:
			method, param = method_003()
		case 4:
			method, param = method_004()
		default:
			fmt.Println("method default")
		}

		fmt.Println("method: ", method)
		err = client.Call(&result, method, param...)
		if err != nil {
			fmt.Println("Call, err", err)
			continue
		}

		switch metId {
		case 1:
			result_001(result)
		case 2:
			result_002(result)
		case 3:
			result_003(result)
		case 4:
			result_004(result)
		default:
			fmt.Println("resut default")
		}
		fmt.Println()
	}
}

func method_001() (method string, param []interface{}) {
	method = "storage_readRawData"

	chundId := uint64(1)
	param = append(param, chundId)

	sliceId := uint32(1)
	param = append(param, sliceId)

	offset := uint32(0)
	param = append(param, offset)

	length := 3
	param = append(param, length)
	return
}
func result_001(result interface{}) {
	fmt.Println("no return value")
}

func method_002() (method string, param []interface{}) {
	method = "blizcs_read"

	objId := uint64(2)
	param = append(param, objId)

	length := uint32(3)
	param = append(param, length)
	return
}
func result_002(result interface{}) {
	res, ok := result.(string)
	if !ok {
		fmt.Println("type not match, type: ", result.(string))
		return
	}
	fmt.Println("res: ", res)
}

func method_003() (method string, param []interface{}) {
	method = "blizcs_wSWrite"

	objId := uint64(2)
	param = append(param, objId)

	data := "abc"
	param = append(param, data)

	offset := uint64(2)
	param = append(param, offset)
	return
}
func result_003(result interface{}) {
	res, ok := result.(bool)
	if !ok {
		fmt.Println("type not match, type: ", result.(string))
		return
	}
	fmt.Println("res: ", res)
}

func method_004() (method string, param []interface{}) {
	method = "blizcs_wSRead"

	objId := uint64(2)
	param = append(param, objId)

	length := uint32(5)
	param = append(param, length)

	offset := uint64(1)
	param = append(param, offset)
	return
}
func result_004(result interface{}) {
	res, ok := result.(string)
	if !ok {
		fmt.Println("type not match, type: ", result.(string))
		return
	}
	fmt.Println("res: ", res)
}

var readLenTmp int64

// 本地文件写到服务器数据
func handle_005(client *rpc.Client) bool {
	method := "blizcs_wSWrite"

	fileName := "x1"
	file, err := os.OpenFile(fileName, os.O_RDONLY, 0)
	if err != nil {
		fmt.Println("OpenFile, err: ", err)
		return false
	}
	defer file.Close()

	fileSize, err := file.Seek(0, os.SEEK_END)
	if err != nil {
		fmt.Println("Seek, err: ", err)
		return false
	}
	// fileSize := int64(20)
	fmt.Println("fileSize: ", fileSize)
	readLenTmp = fileSize

	data := make([]byte, onceLen)
	left := fileSize
	off := int64(0)
	readLen := int64(0)
	for {
		if left == 0 {
			break
		}
		if left > onceLen {
			readLen = onceLen
		} else {
			readLen = left
		}
		if readLen != int64(len(data)) {
			data = data[:readLen]
		}
		// data := make([]byte, readLen)
		_, err = file.ReadAt(data, off)
		if err != nil {
			fmt.Println("read, err: ", err)
			return false
		}
		if off == 0 {
			logLen := len(data)
			if logLen > 20 {
				logLen = 20
			}
			fmt.Println("data: ", data[:logLen])
		}
		var param []interface{}
		objId := uint64(2)
		param = append(param, objId)
		data2 := hex.EncodeToString(data)
		param = append(param, string(data2))
		offset := uint64(off)
		param = append(param, offset)

		err = client.Call(nil, method, param...)
		if err != nil {
			fmt.Println("Call, err", err)
			return false
		}
		off += readLen
		left -= readLen
		fmt.Println("write to server, length: ", off/kSize, "k, left: ", left/kSize, "k")
	}

	return true
}

// 服务器数据保存到本地文件
func handle_006(client *rpc.Client) bool {
	method := "blizcs_wSRead"

	fileName := "x1bak"
	file, err := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		fmt.Println("OpenFile, err: ", err)
		return false
	}
	defer file.Close()

	fileSize := readLenTmp
	fmt.Println("fileSize: ", fileSize)

	data := make([]byte, onceLen)
	left := fileSize
	off := int64(0)
	readLen := int64(0)
	for {
		if left == 0 {
			break
		}
		if left > onceLen {
			readLen = onceLen
		} else {
			readLen = left
		}
		if readLen != int64(len(data)) {
			data = data[:readLen]
		}

		var param []interface{}
		objId := uint64(2)
		param = append(param, objId)
		length := uint32(readLen)
		param = append(param, length)
		offset := uint64(off)
		param = append(param, offset)

		var result interface{}
		err = client.Call(&result, method, param...)
		if err != nil {
			fmt.Println("Call, err", err)
			return false
		}
		res, ok := result.(string)
		if !ok {
			fmt.Println("type not match, type: ", result.(string))
			return false
		}
		res2, err := hex.DecodeString(res)
		if err != nil {
			fmt.Println("DecodeString, err: ", err)
			return false
		}
		_, err = file.WriteAt(res2, off)
		if err != nil {
			fmt.Println("Write, err: ", err)
			return false
		}

		off += readLen
		left -= readLen
		fmt.Println("write to local, length: ", off/kSize, "k, left: ", left/kSize, "k")
	}

	return true
}

func handle_test(client *rpc.Client) bool {
	method := "blizcs_testString"

	var param []interface{}
	objId := uint64(2)
	param = append(param, objId)

	data := "\001\002\003\004\005\006\007"
	param = append(param, string(data))

	const strLen = 255
	bys := make([]byte, strLen)
	for i := 0; i < strLen; i++ {
		bys[i] = byte(i)
	}
	data2 := hex.EncodeToString(bys[:])
	fmt.Println("data2 len", len(data2))
	param = append(param, data2)

	offset := uint64(0)
	param = append(param, offset)

	err := client.Call(nil, method, param...)
	if err != nil {
		fmt.Println("Call, err", err)
		return false
	}

	fmt.Println("end")
	return true
}
