package hbft

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/contatract/go-contatract/common"
)

func Hash(content []byte) string {
	h := sha256.New()
	h.Write(content)
	return hex.EncodeToString(h.Sum(nil))
}

func GetTimeNowMs(address common.Address) int64 {
	// return time.Now().UnixNano() / 1e6 + delay(address) // only for test(monkey patch)
	return time.Now().UnixNano() / 1e6
}

func GetTimeStampForSet(oldTimeStamp uint64, address common.Address) uint64 {
	timeStamp := uint64(GetTimeNowMs(address))
	if timeStamp <= oldTimeStamp {
		time.Sleep(time.Millisecond * time.Duration(oldTimeStamp-timeStamp))
		timeStamp = oldTimeStamp + 1
	}
	return timeStamp
}
