// Copyright 2014 The go-contatract Authors
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

package datacrypto

import (
	"crypto/ecdsa"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/crypto"
	"github.com/contatract/go-contatract/crypto/ecies"
	"github.com/contatract/go-contatract/crypto/sha3"
	"github.com/contatract/go-contatract/rlp"
)

const (
	EncryptNone uint8 = iota
	EncryptDes
)

const (
	digitCount     = 10 // 0 ~ 9
	lowercaseCount = 26 // a ~ z
	uppercaseCount = 26 // A ~ z
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func Atoi(ty string) uint8 {
	t, err := strconv.Atoi(ty)
	if err != nil {
		return EncryptNone
	}
	return uint8(t)
}

func Encrypt(ty uint8, data []byte, key []byte, final bool) ([]byte, error) {
	switch ty {
	case EncryptDes:
		return crypto.Encrypt(data, key, final)
	default:
		return data, nil
	}
}

func Decrypt(ty uint8, data []byte, key []byte, final bool) ([]byte, error) {
	switch ty {
	case EncryptDes:
		return crypto.Decrypt(data, key, final)
	default:
		return data, nil
	}
}

func randByte() byte {
	num := rand.Intn(digitCount + lowercaseCount + uppercaseCount)
	if num < digitCount {
		return '0' + byte(rand.Intn(digitCount))
	} else if num < digitCount+lowercaseCount {
		return 'a' + byte(rand.Intn(lowercaseCount))
	} else {
		return 'A' + byte(rand.Intn(uppercaseCount))
	}
}

func GenerateKey(ty uint8) []byte {
	switch ty {
	case EncryptDes:
		len := 8
		key := make([]byte, len)
		for i := 0; i < len; i++ {
			key[i] = randByte()
		}
		return key
	default:
		return nil
	}
}

func EncryptKey(ecdsaPub *ecdsa.PublicKey, key []byte) []byte {
	ret, err := ecies.EncryptBytes(ecdsaPub, key)
	if err != nil {
		panic(err)
	}
	return ret
}

func DecryptKey(ecdsaPrv *ecdsa.PrivateKey, key []byte) []byte {
	ret, err := ecies.DecryptBytes(ecdsaPrv, key)
	if err != nil {
		panic(err)
	}
	return ret
}

func rlpHash(x interface{}) (h common.Hash) {
	hw := sha3.NewKeccak256()
	if err := rlp.Encode(hw, x); err != nil {
		fmt.Println("Rlp encode error: ", err)
		return
	}
	hw.Sum(h[:0])
	return
}

func GetEncryptKey(ty uint8, h common.Hash, objID uint64) []byte {
	switch ty {
	case EncryptDes:
		len := 8
		bys := rlpHash([]interface{}{h, objID}).Bytes()
		ret := common.Bytes2Hex(bys)
		return []byte(ret[:len])
	default:
		return nil
	}
}

func GetBlockSize(ty uint8) uint64 {
	switch ty {
	case EncryptDes:
		return 8
	default:
		return 1
	}
}

func alignSize(size uint64, align uint64) uint64 {
	m := size % align
	if m == 0 {
		return size
	}
	return size + (align - m)
}

func GetBlockAlignSize(ty uint8, size uint64) uint64 {
	var blockSize uint64
	switch ty {
	case EncryptDes:
		blockSize = GetBlockSize(ty)
	default:
		blockSize = 1
	}
	return alignSize(size, blockSize)
}
