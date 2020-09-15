package utils

import (
	"errors"
	"fmt"
	"github.com/contatract/go-contatract/crypto/sha3"
	"github.com/contatract/go-contatract/rlp"
	"io"
	"math/big"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/contatract/go-contatract/common"
)

//// ExtractChaincode used to get the chaincode out of extended key
//func ExtractChaincode(key *hdkeychain.ExtendedKey) []byte {
//	return base58.Decode(key.String())[13:45]
//}

// Fatalf formats a message to standard error and exits the program.
// The message is also printed to standard output if standard error
// is redirected to a different file.
const WeiAmount = "1000000000000000000"

func Fatalf(format string, args ...interface{}) {
	w := io.MultiWriter(os.Stdout, os.Stderr)
	if runtime.GOOS == "windows" {
		// The SameFile check below doesn't work on Windows.
		// stdout is unlikely to get redirected though, so just print there.
		w = os.Stdout
	} else {
		outf, _ := os.Stdout.Stat()
		errf, _ := os.Stderr.Stat()
		if outf != nil && errf != nil && os.SameFile(outf, errf) {
			w = os.Stderr
		}
	}
	fmt.Fprintf(w, "Fatal: "+format+"\n", args...)
	os.Exit(1)
}

// IsValidAddress validate hex address
func IsValidAddress(iaddress interface{}) bool {
	re := regexp.MustCompile("^0x[0-9a-fA-F]{40}$")
	switch v := iaddress.(type) {
	case string:
		return re.MatchString(v)
	case common.Address:
		return re.MatchString(v.Hex())
	default:
		return false
	}
}

func RlpHash(x interface{}) (h common.Hash) {
	hw := sha3.NewKeccak256()
	rlp.Encode(hw, x)
	hw.Sum(h[:0])
	return h
}

func SleepDefault() {
	time.Sleep(time.Second / 3)
}

func ConvertDouble2Wei(str string) (*big.Int, error) {
	var value *big.Int
	weiAmount := WeiAmount
	if val, _ := new(big.Int).SetString(str, 0); val != nil {
		mul, _ := new(big.Int).SetString(weiAmount, 0)
		value = val.Mul(val, mul)
	} else {
		vals := strings.Split(str, ".")
		if len(vals) != 2 {
			return nil, errors.New("the amount can not be parsed case 1")
		}
		vals[0] = strings.TrimLeft(vals[0], "0")
		vals[1] = strings.TrimRight(vals[1], "0")
		multiedAmt := vals[0] + vals[1]
		multiedAmt = strings.TrimLeft(multiedAmt, "0")

		val, _ = new(big.Int).SetString(multiedAmt, 0)
		if val == nil {
			return nil, errors.New("the amount can not be parsed case 2")
		}

		for i := 0; i < len(vals[1]); i++ {
			weiAmount = weiAmount[:len(weiAmount)-1]
		}
		mul, _ := new(big.Int).SetString(weiAmount, 0)
		value = val.Mul(val, mul)
	}

	return value, nil
}

//// IsZeroAddress validate if it's a 0 address
//func IsZeroAddress(iaddress interface{}) bool {
//	var address common.Address
//	switch v := iaddress.(type) {
//	case string:
//		address = common.HexToAddress(v)
//	case common.Address:
//		address = v
//	default:
//		return false
//	}
//
//	zeroAddressBytes := common.FromHex("0x0000000000000000000000000000000000000000")
//	addressBytes := address.Bytes()
//	return reflect.DeepEqual(addressBytes, zeroAddressBytes)
//}

//// ToDecimal wei to decimals
//func ToDecimal(ivalue interface{}, decimals int) decimal.Decimal {
//	value := new(big.Int)
//	switch v := ivalue.(type) {
//	case string:
//		value.SetString(v, 10)
//	case *big.Int:
//		value = v
//	}
//
//	mul := decimal.NewFromFloat(float64(10)).Pow(decimal.NewFromFloat(float64(decimals)))
//	num, _ := decimal.NewFromString(value.String())
//	result := num.Div(mul)
//
//	return result
//}
//
//// ToWei decimals to wei
//func ToWei(iamount interface{}, decimals int) *big.Int {
//	amount := decimal.NewFromFloat(0)
//	switch v := iamount.(type) {
//	case string:
//		amount, _ = decimal.NewFromString(v)
//	case float64:
//		amount = decimal.NewFromFloat(v)
//	case int64:
//		amount = decimal.NewFromFloat(float64(v))
//	case decimal.Decimal:
//		amount = v
//	case *decimal.Decimal:
//		amount = *v
//	}
//
//	mul := decimal.NewFromFloat(float64(10)).Pow(decimal.NewFromFloat(float64(decimals)))
//	result := amount.Mul(mul)
//
//	wei := new(big.Int)
//	wei.SetString(result.String(), 10)
//
//	return wei
//}

//// CalcGasCost calculate gas cost given gas limit (units) and gas price (wei)
//func CalcGasCost(gasLimit uint64, gasPrice *big.Int) *big.Int {
//	gasLimitBig := big.NewInt(int64(gasLimit))
//	return gasLimitBig.Mul(gasLimitBig, gasPrice)
//}
//
//// SigRSV signatures R S V returned as arrays
//func SigRSV(isig interface{}) ([32]byte, [32]byte, uint8) {
//	var sig []byte
//	switch v := isig.(type) {
//	case []byte:
//		sig = v
//	case string:
//		sig, _ = hexutil.Decode(v)
//	}
//
//	sigstr := common.Bytes2Hex(sig)
//	rS := sigstr[0:64]
//	sS := sigstr[64:128]
//	R := [32]byte{}
//	S := [32]byte{}
//	copy(R[:], common.FromHex(rS))
//	copy(S[:], common.FromHex(sS))
//	vStr := sigstr[128:130]
//	vI, _ := strconv.Atoi(vStr)
//	V := uint8(vI + 27)
//
//	return R, S, V
//}
