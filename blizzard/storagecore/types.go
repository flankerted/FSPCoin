package storagecore

import (
	"crypto/ecdsa"
	"errors"
	"fmt"

	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/crypto"
	"github.com/contatract/go-contatract/crypto/sha3"
	"github.com/contatract/go-contatract/rlp"
)

const (
	SegmentCount  = 32
	HeaderSignLen = 65
	MSize         = 1024 * 1024
	GSize         = MSize * 1024
)

type Slice struct {
	ChunkId uint64
	SliceId uint32
}

// SliceHeader要求固定大小
type SliceHeader struct {
	ChunkId     uint64
	SliceId     uint32
	OwnerId     common.Address
	Permission  uint32
	OpId        uint64
	HeaderHash  common.Hash
	OwnerSign   [HeaderSignLen]byte
	SegmentHash [SegmentCount]common.DataHash
}

func transform(data []byte) []byte {
	msg := fmt.Sprintf("\x19Storage Signed Message:\n%d%s", len(data), data)
	return crypto.Keccak256([]byte(msg))
}

func Hash(data []byte) common.Hash {
	return common.BytesToHash(transform(data))
}

func DataHash(data []byte) common.DataHash {
	return common.BytesToDataHash(transform(data))
}

func RlpHash(x interface{}) (h common.Hash) {
	hw := sha3.NewKeccak256()
	rlp.Encode(hw, x)
	hw.Sum(h[:0])
	return h
}

func (sh *SliceHeader) Hash() common.Hash {
	return RlpHash([]interface{}{
		sh.ChunkId,
		sh.SliceId,
		sh.OwnerId,
		sh.Permission,
		sh.OpId,
		sh.SegmentHash,
	})
}

func (sh *SliceHeader) GetSign(prv *ecdsa.PrivateKey) ([]byte, error) {
	h := sh.Hash()
	sig, err := crypto.Sign(h[:], prv)
	if err != nil {
		return nil, err
	}
	if len(sig) != HeaderSignLen {
		return nil, errors.New("sign len error")
	}
	return sig, nil
}
