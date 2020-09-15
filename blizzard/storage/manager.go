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

package storage

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/contatract/go-contatract/blizcore"
	"github.com/contatract/go-contatract/blizzard/storagecore"
	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/p2p/discover"
	"github.com/syndtr/goleveldb/leveldb"
)

const (
	SplitStr  = "|"
	SplitStr2 = ";"
	SplitStr3 = ","
	SplitStr4 = "-"
)

const (
	dbIpPrefix    = "ip"
	dbClaimPrefix = "claim"
)

type Manager struct {
	sDB           *StorageDB
	claimersDatas map[common.Address]claimerDatas
	node          string // Useless
	etherbase     common.Address
	freeSlices    []storagecore.Slice // 所有可以使用的Slice
	rwLock        sync.RWMutex
}

type ClaimData struct {
	ChunkId uint64
	Content []byte
	Hashs   []common.Hash
	Node    *discover.Node
}

type claimerData struct {
	chunkId uint64
	merkle  *Merkle
}

type claimerDatas []claimerData

type VerifyData struct {
	ChunkId uint64
	Content []byte
	Hashs   []common.Hash
	H       uint64
}

type RentData struct {
	Size   uint64
	ObjId  uint64
	CSAddr common.Address
}

type Segment struct {
	Data [SegmentDataSize]byte
}

// SliceData格式： SliceHeader + Segments
type SliceData struct {
	Header   *storagecore.SliceHeader
	Segments [SegmentCount]*Segment
}

type SliceDatas []*SliceData

type MetaSliceHeader struct {
	ChunkId    uint64
	SliceId    uint32
	OwnerId    common.Address
	Permission uint32
	OpId       uint64
	Hash       common.DataHash
}

func storageDebug(msg string, ctx ...interface{}) {
	log.Info(msg, ctx...)
}

func writeContent(path string, content []byte, off int64) bool {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		log.Error("OpenFile", "err", err)
		return false
	}
	defer file.Close()

	_, err = file.WriteAt(content, off)
	if err != nil {
		log.Error("Write", "err", err)
		return false
	}
	return true
}

func generateConent(chunkId uint64, index int) []byte {
	size := gSize / ChunkSplitCount
	content := make([]byte, 0, size)
	for i := 0; i < size; i++ {
		n := (chunkId+uint64(index+i)+chunkId*uint64(index*i))%26 + 65
		c := rune(n)
		content = append(content, string(c)...)
	}
	return content
}

// get 1M content
func (mgr *Manager) getRandContent(addr common.Address, chunkId uint64, pos int) ([]byte, error) {
	size := ChunkSize / ChunkSplitCount
	content := make([]byte, size)
	if pos == 0 || pos > ChunkSplitCount {
		return content, errIndex
	}

	path := getFilePath(addr, chunkId, mgr.node)
	mgr.rwLock.RLock()
	defer mgr.rwLock.RUnlock()
	file, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		return content, err
	}
	defer file.Close()

	off := int64(size * (pos - 1))
	_, err = file.ReadAt(content, off)
	if err != nil {
		return content, err
	}
	return content, nil
}

func generateIndex() int {
	index := rand.Intn(ChunkSplitCount) + 1
	return index
}

func (mgr *Manager) readAllHeaderHash(addr common.Address, chunkId uint64) []common.Hash {
	headerHashs := make([]common.Hash, 0, ChunkSplitCount)
	path := getFilePath(addr, chunkId, mgr.node)
	mgr.rwLock.RLock()
	defer mgr.rwLock.RUnlock()
	file, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		return headerHashs
	}
	defer file.Close()
	sliceLen := GetSliceLen()
	headerHashOffset := GetSliceHeaderHashOffset()
	content := make([]byte, common.HashLength)
	for i := uint32(0); i < ChunkSplitCount; i++ {
		offset := int64(sliceLen*i + headerHashOffset)
		_, err = file.ReadAt(content, offset)
		if err != nil {
			log.Error("ReadAt", "err", err, "i", i)
			return nil
		}
		headerHash := common.BytesToHash(content)
		headerHashs = append(headerHashs, headerHash)
	}
	return headerHashs
}

func getFilePath(addr common.Address, chunkId uint64, node string) string {
	if common.EmptyAddress(addr) {
		log.Error("Get file path, storage manager address empty")
		return ""
	}
	name := hex.EncodeToString(addr[:]) + SplitStr4 + strconv.Itoa(int(chunkId))
	return filepath.Join(fileDir, name)
}

func NewManager(eBase *common.Address) *Manager {
	m := &Manager{
		sDB:           NewDB(),
		claimersDatas: make(map[common.Address]claimerDatas),
		etherbase:     *eBase,
		freeSlices:    make([]storagecore.Slice, 0),
	}
	return m
}

func (mgr *Manager) LoadClaimDatas() {
	// return // qiwy: for test, TODO
	rd, err := ioutil.ReadDir(fileDir)
	if err != nil {
		fmt.Println("Load claim datas", "err", err)
		return
	}
	for _, fi := range rd {
		if fi.IsDir() {
			fmt.Println("Load claim datas, is  dir", "name", fi.Name())
			continue
		}
		//log.Info("range file", "name", fi.Name())
		datas := strings.Split(fi.Name(), SplitStr4)
		if len(datas) < 2 {
			// fmt.Println("Load claim datas", "count", len(datas))
			//return
			continue
		}
		tmp, err := hex.DecodeString(datas[0])
		if err != nil {
			fmt.Println("Load claim datas", "err", err)
			return
		}
		addr := common.BytesToAddress(tmp)
		chunkId := ParseUint64(datas[1])
		mgr.LoadClaimData(addr, chunkId)
	}
}

func (mgr *Manager) CheckClaimFile(chunkId uint64) bool {
	if !common.FileExist(fileDir) {
		err := os.Mkdir(fileDir, os.ModePerm)
		if err != nil {
			log.Error("Mkdir", "err", err)
			return false
		}
	}

	path := getFilePath(mgr.etherbase, chunkId, mgr.node)
	mgr.rwLock.Lock()
	defer mgr.rwLock.Unlock()
	// 判断文件是否已存在
	if common.FileExist(path) {
		var fileSize int64
		filepath.Walk(path, func(path string, f os.FileInfo, err error) error {
			fileSize = f.Size()
			return nil
		})
		if fileSize == int64(GetTotalChunkSize()) {
			return true
		}
		err := os.Remove(path)
		if err != nil {
			log.Error("os remove", "err", err)
		}
		log.Warn("Remove", "path", path)
	}
	// 判断剩余空间
	size := GetDiskFreeSize(".")
	if size < GetTotalChunkSize() {
		log.Error("Get disk free size", "size", size)
		return false
		// 写文件
	}
	off := int64(0)
	sliceLen := GetSliceLen()
	data := make([]byte, 0, sliceLen)
	for i := 0; i < ChunkSplitCount; i++ {
		// header
		header := storagecore.SliceHeader{ChunkId: chunkId, SliceId: uint32(i + 1)}
		headerData := GetSliceHeaderBytes(&header)
		data = append(data[:0], headerData...)
		// data
		contentData := generateConent(chunkId, i)
		data = append(data, contentData...)
		// write
		ret := writeContent(path, data, off)
		if !ret {
			return ret
		}
		off += int64(sliceLen)
	}
	return true
}

func (mgr *Manager) LoadClaimData(addr common.Address, chunkId uint64) {
	hashs := mgr.readAllHeaderHash(addr, chunkId)
	if len(hashs) != ChunkSplitCount {
		fmt.Println("Load claim data", "hashcount", len(hashs))
		return
	}

	if mgr.claimersDatas[addr] == nil {
		data := claimerData{
			chunkId: chunkId,
			merkle:  NewMerkle(hashs),
		}
		mgr.claimersDatas[addr] = claimerDatas{data}
	} else {
		index := -1
		for i, data := range mgr.claimersDatas[addr] {
			if data.chunkId == chunkId {
				index = int(i)
				break
			}
		}
		if index != -1 {
			mgr.claimersDatas[addr][index].merkle = NewMerkle(hashs)
		} else {
			data := claimerData{
				chunkId: chunkId,
				merkle:  NewMerkle(hashs),
			}
			mgr.claimersDatas[addr] = append(mgr.claimersDatas[addr], data)
		}
	}
}

// func (mgr *Manager) LoadClaimData(addr common.Address, chunkId uint64) {
// 	hashs, err := CalculateAllContentHash(addr, chunkId)
// 	if err != nil {
// 		log.Error("CalculateAllContentHash", "err", err)
// 		return
// 	}
// 	count := len(hashs)
// 	if count != gSize/kSize {
// 		log.Error("LoadMerkel", "count", count)
// 		return
// 	}
// 	for {
// 		if count <= ChunkSplitCount {
// 			break
// 		}
// 		cnt := count / 2
// 		for i := 0; i < cnt; i++ {
// 			left := hashs[i*2]
// 			right := hashs[i*2+1]
// 			hashs[i] = CalculateParentHash(left, right)
// 		}
// 		count = cnt
// 	}

// 	hashs = hashs[:count]
// 	if mgr.claimersDatas[addr] == nil {
// 		data := claimerData{
// 			chunkId: chunkId,
// 			merkle:  NewMerkle(hashs),
// 		}
// 		mgr.claimersDatas[addr] = claimerDatas{data}
// 	} else {
// 		index := -1
// 		for i, data := range mgr.claimersDatas[addr] {
// 			if data.chunkId == chunkId {
// 				index = int(i)
// 				break
// 			}
// 		}
// 		if index != -1 {
// 			mgr.claimersDatas[addr][index].merkle = NewMerkle(hashs)
// 		} else {
// 			data := claimerData{
// 				chunkId: chunkId,
// 				merkle:  NewMerkle(hashs),
// 			}
// 			mgr.claimersDatas[addr] = append(mgr.claimersDatas[addr], data)
// 		}
// 	}
// }

func (mgr *Manager) PrintChunkData(addr common.Address, chunkId uint64, from int, count int) {
	if mgr.claimersDatas[addr] == nil {
		return
	}
	for _, claimerData := range mgr.claimersDatas[addr] {
		if claimerData.chunkId != chunkId {
			continue
		}
		log.Info("GetChunk", "chunkId", chunkId)
		for i, hash := range claimerData.merkle.hashs {
			if i < from || i >= from+count {
				continue
			}
			log.Info("chunk data", "i", i, "hash", hash.Hex())
		}
	}
}

func GetRootHash(content []byte, hashs []common.Hash) common.Hash {
	parent := CalculateContentHash(content)
	for _, hash := range hashs {
		parent = CalculateParentHash(parent, hash)
	}
	return parent
}

func (mgr *Manager) PutIp(addr common.Address, ip []byte) {
	if len(ip) == 0 {
		return
	}
	key := dbIpPrefix + addr.Str()
	mgr.sDB.Put([]byte(key), ip)
}

func (mgr *Manager) GetIp(addr common.Address) []byte {
	key := dbIpPrefix + addr.Str()
	value, err := mgr.sDB.Get([]byte(key))
	if err != nil {
		return []byte{}
	}
	return value
}

func (mgr *Manager) PutClaim(addr common.Address, value []byte) error {
	key := dbClaimPrefix + addr.Str()
	return mgr.sDB.Put([]byte(key), value)
}

func (mgr *Manager) GetClaim(addr common.Address) ([]byte, error) {
	key := dbClaimPrefix + addr.Str()
	value, err := mgr.sDB.Get([]byte(key))
	if err == leveldb.ErrNotFound {
		return []byte("0"), nil
	}
	return value, err
}

func NewSliceHeader(addr common.Address, slice *storagecore.Slice, opId uint64) *storagecore.SliceHeader {
	header := &storagecore.SliceHeader{
		ChunkId: slice.ChunkId,
		SliceId: slice.SliceId,
		OwnerId: addr,
		OpId:    opId,
	}
	return header
}

func (mgr *Manager) RewriteSliceHeader(farmerAddr common.Address, slice *storagecore.Slice, opId uint64, ownerAddr common.Address) {
	header := mgr.readSliceHeader(farmerAddr, slice)
	if header == nil {
		return
	}
	header.OwnerId = ownerAddr
	header.ChunkId = slice.ChunkId
	header.SliceId = slice.SliceId
	header.OpId = opId
	mgr.WriteSliceHeaderData(header)
}

func (mgr *Manager) WriteSliceHeader(addr common.Address, slice *storagecore.Slice, opId uint64) {
	oldHeader := mgr.readSliceHeader(mgr.etherbase, slice)
	if oldHeader == nil {
		return
	}
	if oldHeader.OpId >= opId {
		return
	}

	header := NewSliceHeader(addr, slice, opId)
	mgr.WriteSliceHeaderData(header)
}

func (mgr *Manager) WriteSliceHeaderData(header *storagecore.SliceHeader) {
	if header == nil {
		return
	}
	buf := new(bytes.Buffer)
	buf.Grow(int(GetSliceHeaderLen()))
	err := binary.Write(buf, binary.LittleEndian, header)
	if err != nil {
		log.Error("write", "err", err)
		return
	}
	path := getFilePath(mgr.etherbase, header.ChunkId, mgr.node)
	mgr.rwLock.Lock()
	defer mgr.rwLock.Unlock()
	file, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		// log.Error("open file", "err", err)
		return
	}
	defer file.Close()
	_, err = file.WriteAt(buf.Bytes(), int64(GetSliceLen()*(header.SliceId-1)))
	if err != nil {
		log.Error("write at", "err", err)
		return
	}
}

func (mgr *Manager) ReadSliceRawHeader(chunkId uint64, sliceId uint32) ([]byte, error) {
	slice := &storagecore.Slice{
		ChunkId: chunkId,
		SliceId: sliceId,
	}
	return mgr.readSliceRawHeader(mgr.etherbase, slice)
}

func (mgr *Manager) readSliceRawHeader(addr common.Address, slice *storagecore.Slice) ([]byte, error) {
	path := getFilePath(addr, slice.ChunkId, mgr.node)
	mgr.rwLock.RLock()
	defer mgr.rwLock.RUnlock()
	file, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		// log.Error("open file", "err", err)
		return nil, err
	}
	defer file.Close()
	content := make([]byte, GetSliceHeaderLen())
	if slice.SliceId == 0 {
		log.Error("read slice header", "slice", *slice)
		return nil, errors.New("parameter error")
	}
	_, err = file.ReadAt(content, int64(GetSliceLen()*(slice.SliceId-1)))
	if err != nil {
		log.Error("read at", "err", err)
		return nil, err
	}
	//log.Info("read", "content", content, "len", len(content))
	return content, nil
}

func (mgr *Manager) readSliceHeader(addr common.Address, slice *storagecore.Slice) *storagecore.SliceHeader {
	content, err := mgr.readSliceRawHeader(addr, slice)
	if err != nil {
		return nil
	}

	header := new(storagecore.SliceHeader)
	buf := bytes.NewBuffer(content)
	err = binary.Read(buf, binary.LittleEndian, header)
	if err != nil {
		log.Error("read", "err", err)
	}
	// log.Info("read", "header.ChunkId", header.ChunkId, "header.SliceId", header.SliceId, "header.OpId", header.OpId, "header.OwnerId", header.OwnerId)
	return header
}

func GetSliceHeaderLen() uint32 {
	var sh storagecore.SliceHeader
	return uint32(binary.Size(sh))
}

func GetSegmentLen() uint32 {
	var sg Segment
	return uint32(binary.Size(sg))
}

func GetSliceLen() uint32 {
	return GetSliceHeaderLen() + SliceSize
}

func GetSliceSegmentHashLen() uint32 {
	var sh storagecore.SliceHeader
	return uint32(binary.Size(sh.SegmentHash))
}

// segment hash起始offset
func GetSliceSegmentHashOffset(sliceId uint32) uint32 {
	return GetSliceLen()*(sliceId-1) + GetSliceHeaderLen() - GetSliceSegmentHashLen()
}

// segment data 起始offset
func GetSliceSegmentDataOffset(sliceId uint32) uint32 {
	return GetSliceLen()*(sliceId-1) + GetSliceHeaderLen()
}

// HeaderHash offset, 随SliceHeader结构调整
func GetSliceHeaderHashOffset() uint32 {
	var sh storagecore.SliceHeader
	return uint32(binary.Size(sh.ChunkId) + binary.Size(sh.SliceId) + binary.Size(sh.OwnerId) + binary.Size(sh.Permission) + binary.Size(sh.OpId))
}

func (mgr *Manager) PrintChunkVerifyData(addr common.Address, chunkId uint64) {
	content, hashs, err := mgr.GetVerifyData(addr, chunkId)
	if err != nil {
		log.Error("chunk verify data", "err", err)
		return
	}
	var str string
	hashsCount := len(hashs)
	for i, v := range hashs {
		str = str + strconv.Itoa(i) + ": " + v.Hex()
		if i != hashsCount-1 {
			str += ", "
		}
	}
	log.Info("chunk verify data", "content len", len(content), "content[0-9]", content[:10], "hashs", str)
}

func (mgr *Manager) GetVerifyData(addr common.Address, chunkId uint64) ([]byte, []common.Hash, error) {
	hashRet := make([]common.Hash, 0, relateHashCount)
	rand1 := 1
	rand2 := 1
	content, err := mgr.getRandContent(addr, chunkId, rand1)
	if err != nil {
		log.Error("Get verify data", "err", err)
		return content, hashRet, err
	}

	count := len(content) / kSize
	hash1 := make([]common.Hash, count)
	for i := 0; i < count; i++ {
		from := kSize * i
		to := from + kSize
		hash1[i] = CalculateContentHash(content[from:to])
	}
	hashRet1 := GetBrotherHashs(hash1, rand2)
	// fmt.Println("hash1 as follows: ")
	// for i, h := range hashRet1 {
	// 	fmt.Println(i, ", hash: ", h)
	// }

	hashRet2, err := mgr.GetLocalHash(addr, chunkId, rand2)
	if err != nil {
		log.Error("GetLocalHash", "err", err)
		return content, hashRet2, err
	}
	// fmt.Println("hash2 as follows: ")
	// for i, h := range hashRet2 {
	// 	fmt.Println(i, ", hash: ", h)
	// }
	from := (rand2 - 1) * kSize
	to := from + kSize
	return content[from:to], append(hashRet1, hashRet2...), nil
}

func (mgr *Manager) GetLocalHash(addr common.Address, chunkId uint64, pos int) ([]common.Hash, error) {
	var hss []common.Hash
	if mgr.claimersDatas[addr] == nil {
		return hss, errNotFound
	}
	index := -1
	for i, v := range mgr.claimersDatas[addr] {
		if v.chunkId == chunkId {
			index = i
			break
		}
	}
	if index == -1 {
		return hss, errNotFound
	}
	hashs := mgr.claimersDatas[addr][index].merkle.hashs
	if len(hashs) != ChunkSplitCount {
		log.Error("local hash", "errCount", errCount, "len(hashs)", len(hashs))
		return hss, errCount
	}
	hss = GetBrotherHashs(hashs, pos)
	return hss, nil
}

// including header and hash
func GetTotalChunkSize() uint64 {
	len := GetSliceLen() * SliceCount
	return uint64(len)
}

func GetSliceHeaderBytes(header *storagecore.SliceHeader) []byte {
	buf := new(bytes.Buffer)
	buf.Grow(int(GetSliceHeaderLen()))
	err := binary.Write(buf, binary.LittleEndian, header)
	if err != nil {
		return []byte{}
	}
	return buf.Bytes()
}

func GetDataHash(data []byte) []byte {
	hash := sha256.Sum256(data)
	return hash[:common.DataHashLength]
}

func NewMetaSliceHeader(addr common.Address, slice *storagecore.Slice) *MetaSliceHeader {
	header := &MetaSliceHeader{
		ChunkId: slice.ChunkId,
		SliceId: slice.SliceId,
		OwnerId: addr,
	}
	return header
}

func GetMetaSliceHeaderLen() int {
	var sh MetaSliceHeader
	return binary.Size(sh)
}

func (mgr *Manager) writeMetaSliceHeader(addr common.Address, slice *storagecore.Slice) {
	header := NewMetaSliceHeader(addr, slice)
	buf := new(bytes.Buffer)
	buf.Grow(GetMetaSliceHeaderLen())
	err := binary.Write(buf, binary.LittleEndian, header)
	if err != nil {
		log.Error("write", "err", err)
		return
	}
	//log.Info("write", "data", buf.Bytes(), "len", len(buf.Bytes()))

	path := getFilePath(addr, slice.ChunkId, mgr.node)
	mgr.rwLock.Lock()
	defer mgr.rwLock.Unlock()
	file, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		// log.Error("open file", "err", err)
		return
	}
	defer file.Close()
	_, err = file.WriteAt(buf.Bytes(), 0)
	if err != nil {
		log.Error("write at", "err", err)
		return
	}
}

func (mgr *Manager) GetDataHash(data *GetDataHashProto) *DataHashProto {
	var ret DataHashProto
	path := getFilePath(mgr.etherbase, data.ChunkId, mgr.node)
	if !common.FileExist(path) {
		return nil
	}
	return &ret
}

// func getFileSize(path string) (int64, error) {
// 	file, err := os.OpenFile(path, os.O_RDONLY, 0644)
// 	if err != nil {
// 		fmt.Println("open file, err: ", err)
// 		return 0, err
// 	}
// 	defer file.Close()
// 	return file.Seek(0, os.SEEK_END)
// }

// func waitForFull(path string) bool {
// 	var lastLen int64
// 	for i := 0; i < 60; i++ {
// 		len, err := getFileSize(path)
// 		if err != nil {
// 			return false
// 		}
// 		if uint64(len) >= GetTotalChunkSize() {
// 			return true
// 		}
// 		if len <= lastLen {
// 			return false
// 		}
// 		lastLen = len
// 		time.Sleep(time.Millisecond * 500)
// 	}
// 	return false
// }

func (mgr *Manager) ReadChunkSliceOps(chunkId uint64) []*blizcore.SliceWopProg {
	progs := make([]*blizcore.SliceWopProg, 0, SliceCount)
	path := getFilePath(mgr.etherbase, chunkId, mgr.node)
	// if !waitForFull(path) {
	// 	log.Error("Read chunk slice ops, is not full file", "chunk", chunkId)
	// 	return progs
	// }
	mgr.rwLock.RLock()
	defer mgr.rwLock.RUnlock()
	file, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		// log.Error("open file", "err", err)
		return progs
	}
	defer file.Close()

	sliceLen := GetSliceLen()
	content := make([]byte, GetSliceHeaderLen())
	header := new(storagecore.SliceHeader)
	for i := uint32(1); i <= SliceCount; i++ {
		_, err = file.ReadAt(content, int64(sliceLen*(i-1)))
		if err != nil {
			log.Error("ReadAt", "err", err)
			break
		}

		buf := bytes.NewBuffer(content)
		err = binary.Read(buf, binary.LittleEndian, header)
		if err != nil {
			log.Error("read", "err", err)
			break
		}

		if common.EmptyAddress(header.OwnerId) {
			continue
		}
		// 分配了owner，（就算wop=0也)需要asso
		prog := &blizcore.SliceWopProg{Slice: i, WopId: header.OpId}
		progs = append(progs, prog)
	}

	return progs
}

func (mgr *Manager) ReadChunkSliceHeader(chunkId uint64, sliceId uint32) *storagecore.SliceHeader {
	slice := &storagecore.Slice{
		ChunkId: chunkId,
		SliceId: sliceId,
	}
	return mgr.readSliceHeader(mgr.etherbase, slice)
}

func (mgr *Manager) writeChunkSliceData(chunkId uint64, sliceId uint32, offset uint32, data []byte) error {
	path := getFilePath(mgr.etherbase, chunkId, mgr.node)
	mgr.rwLock.Lock()
	defer mgr.rwLock.Unlock()
	file, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		// log.Error("open file", "err", err)
		return err
	}
	defer file.Close()

	off := GetSliceSegmentDataOffset(sliceId) + offset
	_, err = file.WriteAt(data, int64(off))
	if err != nil {
		log.Error("Write", "err", err)
		return err
	}
	return nil
}

// sliceId下表从1开始
func (mgr *Manager) WriteChunkSlice(chunkId uint64, sliceId uint32, offset uint32, data []byte, wopId uint64, ownerAddr common.Address) error {
	length := len(data)
	if offset+uint32(length) > SliceSize {
		return errDataLen
	}
	if sliceId == 0 {
		return errDataLen
	}
	err := mgr.writeChunkSliceData(chunkId, sliceId, offset, data)
	if err != nil {
		log.Error("Write chunk slice data", "err", err)
		return err
	}
	log.Info("Write", "c", chunkId, "s", sliceId, "w", wopId)

	sliceHeader := mgr.ReadChunkSliceHeader(chunkId, sliceId)
	if sliceHeader == nil {
		log.Error("sliceHeader nil")
		return errors.New("read slice header error")
	}
	sliceHeader.OpId = wopId
	sliceHeader.OwnerId = ownerAddr
	// update SegmentHash
	// start, end := getRelatedSegmanetIndex(offset, uint32(length))
	// for i := start; i < end; i++ {
	// 	dat := make([]byte, SegmentDataSize)
	// 	of := GetSliceSegmentDataOffset(sliceId) + i*SegmentDataSize
	// 	_, err := file.ReadAt(dat, int64(of))
	// 	if err != nil {
	// 		log.Error("ReadAt", "err", err)
	// 		return err
	// 	}
	// 	sliceHeader.SegmentHash[i] = storagecore.DataHash(dat)
	// }
	mgr.WriteSliceHeaderData(sliceHeader)

	return nil
}

func getRelatedSegmanetIndex(offset uint32, length uint32) (uint32, uint32) {
	start := offset / SegmentDataSize
	end := (offset + length) / SegmentDataSize
	if end*SegmentDataSize < offset+length {
		end++
	}
	if end > SegmentCount {
		log.Error("getRelatedSegmanetIndex", "offset", offset, "length", length, "start", start, "end", end)
		end = SegmentCount - 1
		if start > end {
			start = end
		}
	}
	return start, end
}

// sliceId下表从1开始
// offset是数据的偏移量
func (mgr *Manager) ReadChunkSliceSegmentHash(chunkId uint64, sliceId uint32, offset uint32, length uint32) ([]byte, error) {
	// storageDebug("Read chunk slice segment hash", "slice", sliceId, "length", length, "offset", offset)
	if length == 0 || offset+length > SliceSize {
		return nil, errDataLen
	}
	path := getFilePath(mgr.etherbase, chunkId, mgr.node)
	mgr.rwLock.RLock()
	defer mgr.rwLock.RUnlock()
	file, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		// log.Error("open file", "err", err)
		return nil, err
	}
	defer file.Close()

	segmentStart := offset / SegmentDataSize
	segmentEnd := (offset + length) / SegmentDataSize
	if segmentEnd*SegmentDataSize < offset+length {
		segmentEnd++
	}

	off := GetSliceSegmentHashOffset(sliceId) + segmentStart*common.DataHashLength
	len := (segmentEnd - segmentStart) * common.DataHashLength
	data := make([]byte, len)
	_, err = file.ReadAt(data, int64(off))
	if err != nil {
		log.Error("read", "err", err)
		return nil, err
	}
	return data, nil
}

func (mgr *Manager) ReadChunkSliceSegmentData(chunkId uint64, sliceId uint32, offset uint32, length uint32) ([]byte, error) {
	// storageDebug("Read chunk slice segment data", "slice", sliceId, "length", length, "offset", offset)
	if length == 0 || offset+length > SliceSize {
		return nil, errDataLen
	}
	path := getFilePath(mgr.etherbase, chunkId, mgr.node)
	mgr.rwLock.RLock()
	defer mgr.rwLock.RUnlock()
	file, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		// log.Error("open file", "err", err)
		return nil, err
	}
	defer file.Close()

	off := GetSliceSegmentDataOffset(sliceId) + offset
	data := make([]byte, length)
	_, err = file.ReadAt(data, int64(off))
	if err != nil {
		log.Error("read", "err", err)
		return nil, err
	}
	return data, nil
}

func (mgr *Manager) GetVerifyDataHash(chunkId uint64, sliceId uint32, size uint32) (common.DataHash, error) {
	var dataHash common.DataHash
	data, err := mgr.ReadChunkSliceSegmentData(chunkId, sliceId, 0, size)
	if err != nil {
		log.Error("GetVerifyDataHash", "err", err)
		return dataHash, err
	}
	hash := blizcore.BlizHash(data)
	dataHash.SetBytes(hash[:])
	return dataHash, nil
}

func (mgr *Manager) GetHeaderHash(chunkId uint64, sliceId uint32) (common.Hash, error) {
	sliceHeader := mgr.ReadChunkSliceHeader(chunkId, sliceId)
	if sliceHeader == nil {
		return common.Hash{}, errors.New("sliceHeader nil")
	}
	return sliceHeader.Hash(), nil
}

// func (mgr *Manager) UpdateChunkSliceDataHash(chunkId uint64, sliceId uint32, dataHash common.DataHash) {
// 	key := writeVerifyHash + strconv.Itoa(int(chunkId)) + strconv.Itoa(int(sliceId))
// 	mgr.sDB.Put([]byte(key), dataHash[:])
// }

func (mgr *Manager) DoSliceMeta(address common.Address) (uint64, uint32) {
	if len(mgr.freeSlices) == 0 {
		return 0, 0
	}
	slice := mgr.freeSlices[0]
	mgr.freeSlices = mgr.freeSlices[1:]
	mgr.writeMetaSliceHeader(address, &slice)
	return slice.ChunkId, slice.SliceId
}

func (mgr *Manager) GetFarmerSliceOp(sliceInfos []blizcore.SliceInfo) []blizcore.FarmerSliceOp {
	ops := make([]blizcore.FarmerSliceOp, 0, len(sliceInfos))
	for _, sliceInfo := range sliceInfos {
		sliceHeader := mgr.ReadChunkSliceHeader(sliceInfo.ChunkId, sliceInfo.SliceId)
		if sliceHeader == nil {
			// log.Info("ReadChunkSliceHeader", "sliceInfo.ChunkId", sliceInfo.ChunkId, "sliceInfo.SliceId", sliceInfo.SliceId)
			continue
		}
		op := blizcore.FarmerSliceOp{ChunkId: sliceInfo.ChunkId, SliceId: sliceInfo.SliceId, OpId: sliceHeader.OpId}
		ops = append(ops, op)
	}
	return ops
}

func (mgr *Manager) WriteRawData(chunkId uint64, sliceId uint32, offset uint32, data []byte, wopId uint64, ownerAddr common.Address) error {
	return mgr.WriteChunkSlice(chunkId, sliceId, offset, data, wopId, ownerAddr)
}

func (mgr *Manager) ReadRawData(chunkId uint64, sliceId uint32, offset uint32, length uint32) ([]byte, error) {
	data, err := mgr.ReadChunkSliceSegmentData(chunkId, sliceId, offset, length)
	if err != nil {
		storageDebug("ReadRawData", "err", err)
	}
	// logLen := len(data)
	// if logLen > 32 {
	// 	logLen = 32
	// }
	// storageDebug("ReadRawData", "data[:logLen]", string(data[:logLen]))
	return data, err
}

func (mgr *Manager) WriteHeaderData(chunkId uint64, sliceId uint32, opId uint64) {
	farmerAddr := mgr.etherbase
	ownerAddr := farmerAddr
	slice := &storagecore.Slice{ChunkId: chunkId, SliceId: sliceId}
	mgr.RewriteSliceHeader(farmerAddr, slice, opId, ownerAddr)
}

func (mgr *Manager) ReadHeaderData(chunkId uint64, sliceId uint32) {
	slice := &storagecore.Slice{ChunkId: chunkId, SliceId: sliceId}
	header := mgr.readSliceHeader(mgr.etherbase, slice)
	if header == nil {
		return
	}
	log.Info("ReadHeaderData", "header.ChunkId", header.ChunkId, "header.SliceId", header.SliceId, "header.OpId", header.OpId, "header.OwnerId", header.OwnerId)
	log.Info("ReadHeaderData", "header", header)
}

func (mgr *Manager) LogHeaderData(chunkId uint64, fromSlice, toSlice uint32) {
	var cnt uint32
	for i := fromSlice; i <= toSlice; i++ {
		slice := &storagecore.Slice{ChunkId: chunkId, SliceId: i}
		header := mgr.readSliceHeader(mgr.etherbase, slice)
		if header == nil {
			return
		}
		fmt.Printf("c: %d, w: %d", i, header.OpId)
		cnt++
		if cnt%10 == 0 || i == toSlice {
			fmt.Printf("\n")
		} else {
			fmt.Printf("; ")
		}
	}
}

func GetChunkIdList() []uint64 {
	idList := make([]uint64, 0)
	rd, err := ioutil.ReadDir(fileDir)
	if err != nil {
		// log.Info("read dir", "err", err)
		return idList
	}
	for _, fi := range rd {
		if fi.IsDir() {
			continue
		}
		datas := strings.Split(fi.Name(), SplitStr4)
		if len(datas) < 2 {
			continue
		}
		id := ParseUint64(datas[1])
		idList = append(idList, id)
	}
	return idList
}

func GetChunkIds() []ChunkIdentity {
	cIds := make([]ChunkIdentity, 0)
	rd, err := ioutil.ReadDir(fileDir)
	if err != nil {
		// log.Info("read dir", "err", err)
		return cIds
	}

_range:
	for _, fi := range rd {
		if fi.IsDir() {
			continue
		}
		datas := strings.Split(fi.Name(), SplitStr4)
		if len(datas) < 2 {
			continue
		}
		chunkId := Int2ChunkIdentity(ParseUint64(datas[1]))
		for _, id := range cIds {
			if chunkId.Equal(id) {
				continue _range
			}
		}
		cIds = append(cIds, chunkId)
	}
	return cIds
}

func (mgr *Manager) SignChunkSlice(chunkId uint64, sliceId uint32, sig []byte) {
	if sig == nil || len(sig) != storagecore.HeaderSignLen {
		log.Error("SignChunkSlice", "sig", sig)
		return
	}
	sliceHeader := mgr.ReadChunkSliceHeader(chunkId, sliceId)
	if sliceHeader == nil {
		log.Error("sliceHeader nil", "chunkId", chunkId, "sliceId", sliceId)
		return
	}
	for i := 0; i < storagecore.HeaderSignLen; i++ {
		sliceHeader.OwnerSign[i] = sig[i]
	}
	mgr.WriteSliceHeaderData(sliceHeader)
	return
}

func HasChunk(chunkID uint64) bool {
	for _, id := range GetChunkIds() {
		if chunkID == ChunkIdentity2Int(id) {
			return true
		}
	}
	return false
}
