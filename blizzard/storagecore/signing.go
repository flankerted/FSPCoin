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

package storagecore

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"

	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/crypto"
)

type SliceHeaderSign struct {
	HeaderHash common.Hash
	V          *big.Int //`json:"v" gencodec:"required"`
	R          *big.Int //`json:"r" gencodec:"required"`
	S          *big.Int //`json:"s" gencodec:"required"`
}

func recoverPlain(sighash common.Hash, R, S, Vb *big.Int) (common.Address, error) {
	if Vb.BitLen() > 8 {
		return common.Address{}, ErrInvalidSig
	}
	V := byte(Vb.Uint64() - 27)
	homestead := true
	if !crypto.ValidateSignatureValues(V, R, S, homestead) {
		return common.Address{}, ErrInvalidSig
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

// SignTx signs the transaction using the given signer and private key
func SignSliceHeader(shh *SliceHeaderSign, prv *ecdsa.PrivateKey) ([]byte, error) {
	h := shh.Hash()
	sig, err := crypto.Sign(h[:], prv)
	if err != nil {
		return nil, err
	}
	return sig, shh.WithSignature(sig)
}

// WithSignature returns a new transaction with the given signature.
// This signature needs to be formatted as described in the yellow paper (v+27).
func (shh *SliceHeaderSign) WithSignature(sig []byte) error {
	r, s, v, err := shh.SignatureValues(sig)
	if err != nil {
		return err
	}
	shh.R, shh.S, shh.V = r, s, v
	return nil
}

// SignatureValues returns signature values. This signature
// needs to be in the [R || S || V] format where V is 0 or 1.
func (shh *SliceHeaderSign) SignatureValues(sig []byte) (r, s, v *big.Int, err error) {
	if len(sig) != 65 {
		panic(fmt.Sprintf("wrong size for signature: got %d, want 65", len(sig)))
	}
	r = new(big.Int).SetBytes(sig[:32])
	s = new(big.Int).SetBytes(sig[32:64])
	v = new(big.Int).SetBytes([]byte{sig[64] + 27})
	return r, s, v, nil
}

func (shs *SliceHeaderSign) Sender() (common.Address, error) {
	return recoverPlain(shs.Hash(), shs.R, shs.S, shs.V)
}

// Hash returns the hash to be signed by the sender.
// It does not uniquely identify the transaction.
func (shh *SliceHeaderSign) Hash() common.Hash {
	return shh.HeaderHash
}
