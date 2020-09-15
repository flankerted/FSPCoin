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

package common

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"
	"reflect"
	"strconv"
	"strings"
	"time"

	"errors"

	"github.com/contatract/go-contatract/blizparam"
	"github.com/contatract/go-contatract/common/hexutil"
	"github.com/contatract/go-contatract/crypto/sha3"
	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/rlp"
)

const (
	HashLength     = 32
	AddressLength  = 20
	DataHashLength = 4
)

var (
	hashT    = reflect.TypeOf(Hash{})
	addressT = reflect.TypeOf(Address{})
)

var (
	ErrInterfaceType = errors.New("interface type not match")
	ErrToDo          = errors.New("need to do")
)

// Hash represents the 32 byte Keccak256 hash of arbitrary data.
type Hash [HashLength]byte
type Hashs []Hash

func BytesToHash(b []byte) Hash {
	var h Hash
	h.SetBytes(b)
	return h
}
func StringToHash(s string) Hash { return BytesToHash([]byte(s)) }
func BigToHash(b *big.Int) Hash  { return BytesToHash(b.Bytes()) }
func HexToHash(s string) Hash    { return BytesToHash(FromHex(s)) }

// Get the string representation of the underlying hash
func (h Hash) Str() string   { return string(h[:]) }
func (h Hash) Bytes() []byte { return h[:] }
func (h Hash) Big() *big.Int { return new(big.Int).SetBytes(h[:]) }
func (h Hash) Hex() string   { return hexutil.Encode(h[:]) }

// a hex string without 0x prefix
func (h Hash) ToHex() string { return hexutil.Encodes(h[:]) }

// TerminalString implements log.TerminalStringer, formatting a string for console
// output during logging.
func (h Hash) TerminalString() string {
	return fmt.Sprintf("%x…%x", h[:3], h[29:])
}

// String implements the stringer interface and is used also by the logger when
// doing full logging into a file.
func (h Hash) String() string {
	return h.Hex()
}

// Format implements fmt.Formatter, forcing the byte slice to be formatted as is,
// without going through the stringer interface used for logging.
func (h Hash) Format(s fmt.State, c rune) {
	fmt.Fprintf(s, "%"+string(c), h[:])
}

// UnmarshalText parses a hash in hex syntax.
func (h *Hash) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedText("Hash", input, h[:])
}

// UnmarshalJSON parses a hash in hex syntax.
func (h *Hash) UnmarshalJSON(input []byte) error {
	return hexutil.UnmarshalFixedJSON(hashT, input, h[:])
}

// MarshalText returns the hex representation of h.
func (h Hash) MarshalText() ([]byte, error) {
	return hexutil.Bytes(h[:]).MarshalText()
}

// Sets the hash to the value of b. If b is larger than len(h), 'b' will be cropped (from the left).
func (h *Hash) SetBytes(b []byte) {
	if len(b) > len(h) {
		b = b[len(b)-HashLength:]
	}

	copy(h[HashLength-len(b):], b)
}

// Set string `s` to h. If s is larger than len(h) s will be cropped (from left) to fit.
func (h *Hash) SetString(s string) { h.SetBytes([]byte(s)) }

// Sets h to other
func (h *Hash) Set(other Hash) {
	for i, v := range other {
		h[i] = v
	}
}

// Generate implements testing/quick.Generator.
func (h Hash) Generate(rand *rand.Rand, size int) reflect.Value {
	m := rand.Intn(len(h))
	for i := len(h) - 1; i > m; i-- {
		h[i] = byte(rand.Uint32())
	}
	return reflect.ValueOf(h)
}

func (a *Hash) Equal(b Hash) bool {
	for i := 0; i < HashLength; i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (a Hash) Empty() bool {
	return a == Hash{}
}

func EmptyHash(h Hash) bool {
	return h == Hash{}
}

// UnprefixedHash allows marshaling a Hash without 0x prefix.
type UnprefixedHash Hash

// UnmarshalText decodes the hash from hex. The 0x prefix is optional.
func (h *UnprefixedHash) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedUnprefixedText("UnprefixedHash", input, h[:])
}

// MarshalText encodes the hash as hex.
func (h UnprefixedHash) MarshalText() ([]byte, error) {
	return []byte(hex.EncodeToString(h[:])), nil
}

/////////// Address

// Address represents the 20 byte address of an Ethereum account.
type Address [AddressLength]byte
type Addresss []Address

func BytesToAddress(b []byte) Address {
	var a Address
	a.SetBytes(b)
	return a
}
func StringToAddress(s string) Address { return BytesToAddress([]byte(s)) }
func BigToAddress(b *big.Int) Address  { return BytesToAddress(b.Bytes()) }
func HexToAddress(s string) Address    { return BytesToAddress(FromHex(s)) }

// IsHexAddress verifies whether a string can represent a valid hex-encoded
// Ethereum address or not.
func IsHexAddress(s string) bool {
	if hasHexPrefix(s) {
		s = s[2:]
	}
	return len(s) == 2*AddressLength && isHex(s)
}

// Get the string representation of the underlying address
func (a Address) Str() string   { return string(a[:]) }
func (a Address) Bytes() []byte { return a[:] }
func (a Address) Big() *big.Int { return new(big.Int).SetBytes(a[:]) }
func (a Address) Hash() Hash    { return BytesToHash(a[:]) }

// Hex returns an EIP55-compliant hex string representation of the address.
func (a Address) Hex() string {
	unchecksummed := hex.EncodeToString(a[:])
	sha := sha3.NewKeccak256()
	sha.Write([]byte(unchecksummed))
	hash := sha.Sum(nil)

	result := []byte(unchecksummed)
	for i := 0; i < len(result); i++ {
		hashByte := hash[i/2]
		if i%2 == 0 {
			hashByte = hashByte >> 4
		} else {
			hashByte &= 0xf
		}
		if result[i] > '9' && hashByte > 7 {
			result[i] -= 32
		}
	}
	return "0x" + string(result)
}

// Hex returns an EIP55-compliant hex string representation of the address(a hex string without 0x prefix).
func (a Address) ToHex() string {
	unchecksummed := hex.EncodeToString(a[:])
	sha := sha3.NewKeccak256()
	sha.Write([]byte(unchecksummed))
	hash := sha.Sum(nil)

	result := []byte(unchecksummed)
	for i := 0; i < len(result); i++ {
		hashByte := hash[i/2]
		if i%2 == 0 {
			hashByte = hashByte >> 4
		} else {
			hashByte &= 0xf
		}
		if result[i] > '9' && hashByte > 7 {
			result[i] -= 32
		}
	}
	return string(result)
}

// String implements the stringer interface and is used also by the logger.
func (a Address) String() string {
	return a.Hex()
}

// Format implements fmt.Formatter, forcing the byte slice to be formatted as is,
// without going through the stringer interface used for logging.
func (a Address) Format(s fmt.State, c rune) {
	fmt.Fprintf(s, "%"+string(c), a[:])
}

// Sets the address to the value of b. If b is larger than len(a) it will panic
func (a *Address) SetBytes(b []byte) {
	if len(b) > len(a) {
		b = b[len(b)-AddressLength:]
	}
	copy(a[AddressLength-len(b):], b)
}

// Set string `s` to a. If s is larger than len(a) it will panic
func (a *Address) SetString(s string) { a.SetBytes([]byte(s)) }

// Sets a to other
func (a *Address) Set(other Address) {
	for i, v := range other {
		a[i] = v
	}
}

func (a *Address) Equal(b Address) bool {
	if len(a) != len(b) {
		return false
	}

	var i int
	for i = 0; i < AddressLength; i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func EmptyAddress(a Address) bool {
	return a == Address{}
}

// MarshalText returns the hex representation of a.
func (a Address) MarshalText() ([]byte, error) {
	return hexutil.Bytes(a[:]).MarshalText()
}

// UnmarshalText parses a hash in hex syntax.
func (a *Address) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedText("Address", input, a[:])
}

// UnmarshalJSON parses a hash in hex syntax.
func (a *Address) UnmarshalJSON(input []byte) error {
	return hexutil.UnmarshalFixedJSON(addressT, input, a[:])
}

// UnprefixedHash allows marshaling an Address without 0x prefix.
type UnprefixedAddress Address

// UnmarshalText decodes the address from hex. The 0x prefix is optional.
func (a *UnprefixedAddress) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedUnprefixedText("UnprefixedAddress", input, a[:])
}

// MarshalText encodes the address as hex.
func (a UnprefixedAddress) MarshalText() ([]byte, error) {
	return []byte(hex.EncodeToString(a[:])), nil
}

type DataHash [DataHashLength]byte

// Sets the hash to the value of b. If b is larger than len(h), 'b' will be cropped (from the left).
func (h *DataHash) SetBytes(b []byte) {
	if len(b) > len(h) {
		b = b[len(b)-DataHashLength:]
	}

	copy(h[DataHashLength-len(b):], b)
}

func BytesToDataHash(b []byte) DataHash {
	var h DataHash
	h.SetBytes(b)
	return h
}

func (h *DataHash) Equal(d DataHash) bool {
	if len(h) != len(d) {
		return false
	}

	var i int
	for i = 0; i < DataHashLength; i++ {
		if h[i] != d[i] {
			return false
		}
	}
	return true
}

type CSAuthData struct {
	Authorizer Address // authorizer
	StartTime  uint64  // authorize starttime
	EndTime    uint64  // authorize endtime
	Flow       uint64  // authorize flow
	// AuthAllowTime uint64  // authorizer allow time
	CSPickupTime  uint64 // pickup time
	AuthAllowFlow uint64 // authorizer allow flow
	CSPickupFlow  uint64 // pickup flow
	// UsedFlow      uint64 // used flow
	PayMethod uint8 // payment method, 0-time, 1-flow
}

func GetTimeString(sec uint64) string {
	t := time.Unix(int64(sec), 0)
	year, month, day := t.Date()
	hour, minute, second := t.Clock()
	return fmt.Sprintf("%04v-%02v-%02v %02v:%02v:%02v", year, int(month), day, hour, minute, second)
}

func GetUnixTime(str string) (uint64, error) {
	layout := "2006-01-02 15:04:05"
	t, err := time.ParseInLocation(layout, str, time.Local)
	if err != nil {
		log.Error("ParseInLocation", "str", str, "err", err)
	}
	return uint64(t.Unix()), err
}

type WalletCSAuthData struct {
	CsAddr        Address // CsAddr
	StartTime     uint64  // authorize starttime
	EndTime       uint64  // authorize endtime
	Flow          uint64  // authorize flow
	CSPickupTime  uint64  // pickup time
	AuthAllowFlow uint64  // authorizer allow flow
	CSPickupFlow  uint64  // pickup flow
	PayMethod     uint8   // payment method, 0-time, 1-flow
	NodeID        string
}

func CheckChan() interface{} {
	return recover()
}

func EleSyncRule(remoteHeightEle, localHeightEle, remoteHeightEth, localHeightEth int64) bool {
	ret := false
	if remoteHeightEth == localHeightEth && remoteHeightEle > localHeightEle {
		ret = true
	}
	if ret {
		log.Info("EleSyncRule", "remoteHeightEle", remoteHeightEle, "localHeightEle", localHeightEle, "remoteHeightEth", remoteHeightEth, "localHeightEth", localHeightEth)
	} else {
		log.Warn("EleSyncRule", "remoteHeightEle", remoteHeightEle, "localHeightEle", localHeightEle, "remoteHeightEth", remoteHeightEth, "localHeightEth", localHeightEth)
	}
	return ret
}

func CheckEleSharding(rShard, lShard uint16, rEthHeight, lEthHeight uint64, rEleHeight, lEleHeight uint64) bool {
	if rEthHeight != lEthHeight {
		//log.Info("Check ele sharding", "remote eth height", rEthHeight, "local eth height", lEthHeight)
		return false
	}
	if rEleHeight <= lEleHeight {
		log.Info("Check ele sharding", "remote ele height", rEleHeight, "local ele height", lEleHeight)
		return false
	}
	shardHeight := GetEleShardHeight(rShard, lShard)
	if rEthHeight > shardHeight {
		//log.Info("Check ele sharding", "shard height", shardHeight)
		return false
	}
	return true
}

// No sharding
const BitTotal = 0         // 13
const MaxShardingTotal = 1 // (2 << (BitTotal - 1))
const lampHeightMax uint64 = 0xFFFFFFFF

// var ethShardingHeight [MaxShardingTotal]uint64 = [MaxShardingTotal]uint64{5, 50, 500, lampHeightMax}
var ethShardingHeight [MaxShardingTotal]uint64 = [MaxShardingTotal]uint64{lampHeightMax /*, lampHeightMax, lampHeightMax, lampHeightMax*/}

func GetTotalSharding(lampHeight uint64) uint16 {
	for i, v := range ethShardingHeight {
		if lampHeight <= v {
			if i == 0 {
				return 1
			}
			return 2 << uint16(i-1)
		}
	}
	return MaxShardingTotal
}

func GetShardingHeight(s uint16) uint64 {
	if s > MaxShardingTotal {
		return lampHeightMax
	}
	return ethShardingHeight[s-1]
}

func GetEleShardHeight(rShard, lShard uint16) uint64 {
	var m uint64
	for _, v := range ethShardingHeight {
		totalShard := GetTotalSharding(v)
		if GetSharding2(rShard, totalShard) != GetSharding2(lShard, totalShard) {
			break
		}
		m = v
	}
	return m
}

func GetNextShardingHeight(lampHeight uint64) uint64 {
	s := GetTotalSharding(lampHeight)
	next := s + 1
	h := GetShardingHeight(next)
	return h
}

// Real sharding by address, sharding [0, 8191]
func GetRealSharding(address Address) uint16 {
	var id uint16
	for i := 0; i < BitTotal; i++ {
		c := address[AddressLength-1-i]
		if c%2 == 0 {
			continue
		}
		id |= (1 << uint32(i))
	}
	return id
}

func GetSharding(address Address, lampHeight uint64) uint16 {
	realShard := GetRealSharding(address)
	totalShard := GetTotalSharding(lampHeight)
	return GetSharding2(realShard, totalShard)
}

func GetSharding2(realShard, totalShard uint16) uint16 {
	return realShard % totalShard
}

func GetSharding3(realShard uint16, lampHeight uint64) uint16 {
	totalShard := GetTotalSharding(lampHeight)
	return GetSharding2(realShard, totalShard)
}

func IsSameSharding(a, b Address, lampHeight uint64) bool {
	as := GetSharding(a, lampHeight)
	bs := GetSharding(b, lampHeight)
	log.Debug("IsSameSharding", "a", a.Hex(), "as", as, "b", b.Hex(), "bs", bs)
	return as == bs
}

// extend txs
const (
	LocalTxs   = 0
	InputTxs   = 1
	OutputTxs  = 2
	MaxTxs     = OutputTxs + 1
	InvalidTxs = 4
)

func GetTxsType(eBase, from, to *Address, lampHeight uint64) int {
	if from == nil {
		panic("Tx from error")
	}
	t := *from
	if to != nil {
		t = *to
	}
	a := IsSameSharding(*eBase, *from, lampHeight)
	b := IsSameSharding(*eBase, t, lampHeight)
	if a && b {
		return LocalTxs
	} else if !a && b {
		return InputTxs
	} else if a && !b {
		return OutputTxs
	} else {
		return InvalidTxs
	}
}

// convenient for test
var (
	IsConsensusBft = true
	IsTest         = true
)

func GetConsensusBft() bool {
	return IsConsensusBft
}

func GetConsensusBftTest() bool {
	return IsTest
}

func GetEleSharding(eBase *Address, lampHeight uint64) uint16 {
	return GetSharding(*eBase, lampHeight)
}

// Sharding level n as followings:
// level 1( 1): 0
// level 2( 2): 0,                         1
// level 3( 4): 0,           2,            1,           3
// level 4( 8): 0,    4,     2,     6,     1,    5,     3,     7
// level 5(16): 0, 8, 4, 12, 2, 10, 6, 14, 1, 9, 5, 13, 3, 11, 7, 15
var shardingIndex [BitTotal + 1][]uint16

func init() {
	var count uint16
	for i := uint16(0); i <= BitTotal; i++ {
		count = (1 << i)
		shardingIndex[i] = make([]uint16, count)
		if i == 0 {
			shardingIndex[i][0] = 0
			continue
		}
		for j := uint16(0); j < count; j++ {
			if (j % 2) == 0 {
				shardingIndex[i][j] = shardingIndex[i-1][j/2]
			} else {
				shardingIndex[i][j] = shardingIndex[i-1][j/2] + count/2
			}
			// fmt.Println("i", i, "j", j, "v", shardingIndex[i][j])
		}
	}
}

func GetParentSharding(currentShard, totalShard uint16) int16 {
	for i := uint16(0); i <= BitTotal; i++ {
		if uint16(len(shardingIndex[i])) != totalShard {
			continue
		}
		if i == 0 {
			return -1
		}
		for j := uint16(0); j < totalShard; j++ {
			if shardingIndex[i][j] != currentShard {
				continue
			}
			return int16(shardingIndex[i-1][j/2])
		}
	}
	return -1
}

func GetChildSharding(currentShard, totalShard uint16) (int16, int16) {
	for i := uint16(0); i <= BitTotal; i++ {
		if uint16(len(shardingIndex[i])) != totalShard {
			continue
		}
		if i == BitTotal {
			return -1, -1
		}
		for j := uint16(0); j < totalShard; j++ {
			if shardingIndex[i][j] != currentShard {
				continue
			}
			return int16(shardingIndex[i+1][j*2]), int16(shardingIndex[i+1][j*2+1])
		}
	}
	return -1, -1
}

func GetBrotherSharding(currentShard, totalShard uint16) int16 {
	s := GetParentSharding(currentShard, totalShard)
	if s == -1 {
		return s
	}
	left, right := GetChildSharding(uint16(s), totalShard/2)
	if left == -1 || right == -1 {
		return -1
	}
	if currentShard == uint16(left) {
		return right
	} else if currentShard == uint16(right) {
		return left
	} else {
		return -1
	}
}

func Max(x, y int64) int64 {
	if x > y {
		return x
	}
	return y
}

func Min(x, y int64) int64 {
	if x < y {
		return x
	}
	return y
}

type BlockIDNode struct {
	BlockID uint64
	NodeStr string
}

type AddressNode struct {
	Address Address
	NodeStr string
}

func GetShareObjData(sharingCode string) (objId uint64, offset uint64, length uint64, price uint64, timeStart string, timeStop string, fileName string, sharerCSNodeID string, receiver string, key string, err error) {
	var sharingData interface{}
	err = json.Unmarshal([]byte(sharingCode), &sharingData)
	if err != nil {
		return
	}
	sharingMap, ok := sharingData.(map[string]interface{})
	if !ok {
		err = errors.New("can not unmarshal json string")
		return
	}
	objId = uint64(sharingMap["ObjectID"].(float64))

	offsetInt64, e := strconv.ParseInt(sharingMap["FileOffset"].(string), 10, 64)
	if e != nil {
		err = errors.New("can not unmarshal json string: FileOffset")
		return
	}
	offset = uint64(offsetInt64)

	lengthInt64, e := strconv.ParseInt(sharingMap["FileSize"].(string), 10, 64)
	if e != nil {
		err = errors.New("can not unmarshal json string: FileSize")
		return
	}
	length = uint64(lengthInt64)

	priceInt64, e := strconv.ParseInt(sharingMap["Price"].(string), 10, 64)
	if e != nil {
		err = errors.New("can not unmarshal json string: Price")
		return
	}
	price = uint64(priceInt64)

	timeStart = sharingMap["TimeStart"].(string)
	_, e = GetUnixTime(timeStart)
	if e != nil {
		err = errors.New("The start time format is not right")
		return
	}

	timeStop = sharingMap["TimeStop"].(string)
	_, e = GetUnixTime(timeStop)
	if e != nil {
		err = errors.New("The end time format is not right")
		return
	}

	fileName = sharingMap["FileName"].(string)

	sharerCSNodeID = sharingMap["SharerCSNodeID"].(string)

	receiver = sharingMap["Receiver"].(string)

	key = sharingMap["Key"].(string)

	fmt.Println(objId, offset, length, price, timeStart, timeStop, err, sharingMap)
	return
}

const cLocalIP = "127.0.0.1"

// Parameter ip format: *.*.*.*:*, or [::]:*
func GetIPPort(ip string) (string, string) {
	ipPort := strings.Split(ip, ":")
	c := len(ipPort)
	if c == 0 {
		return "", ""
	} else if c == 1 {
		return ipPort[0], ""
	} else if c == 2 {
		return ipPort[0], ipPort[1]
	} else {
		return cLocalIP, ipPort[c-1]
	}
}

func IsSameIP(ip, ip2 string) bool {
	if ip == ip2 {
		return true
	}
	i, p := GetIPPort(ip)
	i2, p2 := GetIPPort(ip2)
	if i != i2 {
		return false
	}
	if p == "" || p2 == "" || p == p2 {
		return true
	}
	return false
}

func GetErrBytes(err error) []byte {
	if err == nil {
		return nil
	}
	bytes := []byte(err.Error())
	if len(bytes) == 0 {
		bytes = []byte("unknown")
	}
	return bytes
}

func GetErrString(errBytes []byte) string {
	if len(errBytes) == 0 {
		return "success"
	}
	return string(errBytes)
}

const freeHeight uint64 = 1000

func IsFreeAct(h uint64, t uint8) bool {
	if h > freeHeight {
		return false
	}
	if t != blizparam.TypeActClaim && t != blizparam.TypeActVerify {
		return false
	}
	return true
}

type TxMailData struct {
	Title       string `json:"Title,omitempty"`
	Sender      string `json:"Sender,omitempty"`
	Receiver    string `json:"Receiver,omitempty"`
	Content     string `json:"Content,omitempty"`
	TimeStamp   string `json:"TimeStamp,omitempty"`
	FileSharing string `json:"FileSharing,omitempty"`

	Signature   string `json:"Signature,omitempty"`
	EncryptType string `json:"EncryptType,omitempty"`
	Key         string `json:"Key,omitempty"`
}

func (mail *TxMailData) UnmarshalJson(jsonStr string) error {
	err := json.Unmarshal([]byte(jsonStr), mail)
	if err != nil {
		return err
	}

	return nil
}

type SDBMailData struct {
	Sender      string
	Title       string
	Content     string
	TimeStamp   string
	FileSharing string
	Signature   string
	EncryptType string
	Key         string
}

func KeyID(addr Address, objID uint64) string {
	return addr.ToHex() + strconv.FormatInt(int64(objID), 10)
}

func GetAlign64(pos int64) int64 {
	m := pos % 64
	if m == 0 {
		return pos
	}
	return pos + (64 - m)
}

type SharingCodeST struct {
	FileObjID         uint64 `json:"ObjectID"`
	FileName          string `json:"FileName"`
	FileOffset        uint64 `json:"FileOffset"`
	FileSize          uint64 `json:"FileSize"`
	Receiver          string `json:"Receiver"`
	StartTime         string `json:"TimeStart"`
	StopTime          string `json:"TimeStop"`
	Price             string `json:"Price"`
	SharerAgentNodeID string `json:"SharerCSNodeID"`
	Key               string `json:"Key"`
	Signature         string `json:"Signature"`
}

func rlpHash(x interface{}) (h Hash) {
	hw := sha3.NewKeccak256()
	rlp.Encode(hw, x)
	hw.Sum(h[:0])
	return h
}

func (s *SharingCodeST) HashNoSignature() Hash {
	return rlpHash([]interface{}{
		s.FileObjID,
		s.FileName,
		s.FileOffset,
		s.FileSize,
		s.Receiver,
		s.StartTime,
		s.StopTime,
		s.Price,
		s.SharerAgentNodeID,
		s.Key,
	})
}
