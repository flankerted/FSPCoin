package ftransfer

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"

	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/crypto"
	"github.com/pkg/errors"
)

// SignTx signs the transaction using the given signer and private key
func WalletSign(h common.Hash, prv *ecdsa.PrivateKey) (*big.Int, *big.Int, *big.Int, error) {
	sig, err := crypto.Sign(h[:], prv)
	if err != nil {
		return nil, nil, nil, err
	}
	return WithSignature(sig)
}

// WithSignature returns a new transaction with the given signature.
// This signature needs to be formatted as described in the yellow paper (v+27).
func WithSignature(sig []byte) (*big.Int, *big.Int, *big.Int, error) {
	return SignatureValues(sig)
}

// SignatureValues returns signature values. This signature
// needs to be in the [R || S || V] format where V is 0 or 1.
func SignatureValues(sig []byte) (r, s, v *big.Int, err error) {
	if len(sig) != 65 {
		panic(fmt.Sprintf("wrong size for signature: got %d, want 65", len(sig)))
	}
	r = new(big.Int).SetBytes(sig[:32])
	s = new(big.Int).SetBytes(sig[32:64])
	v = new(big.Int).SetBytes([]byte{sig[64] + 27})
	return r, s, v, nil
}

func SignAuthorizeHash(addr common.Address, authDeadline uint64) common.Hash {
	return rlpHash([]interface{}{
		addr,
		authDeadline,
	})
}

func recoverPlain(sighash common.Hash, R, S, Vb *big.Int) (common.Address, error) {
	if Vb.BitLen() > 8 {
		return common.Address{}, errors.New("Signing is Invalid")
	}
	V := byte(Vb.Uint64() - 27)
	homestead := true
	if !crypto.ValidateSignatureValues(V, R, S, homestead) {
		return common.Address{}, errors.New("Signing is Invalid")
	}
	// encode the snature in uncompressed format
	r, s := R.Bytes(), S.Bytes()
	sig := make([]byte, 65)
	copy(sig[32-len(r):32], r)
	copy(sig[64-len(s):64], s)
	sig[64] = V
	// recover the public key from the snature
	pub, err := crypto.Ecrecover(sighash[:], sig)
	if err != nil {
		return common.Address{}, err
	}
	if len(pub) == 0 || pub[0] != 4 {
		return common.Address{}, errors.New("invalid public key")
	}
	var addr common.Address
	copy(addr[:], crypto.Keccak256(pub[1:])[12:])
	return addr, nil
}

func SignAuthorizeHashSender(hash common.Hash, r, s, v *big.Int) (common.Address, error) {
	return recoverPlain(hash, r, s, v)
}
