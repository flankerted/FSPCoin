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

// NodeID 和 publicKey 之间的互相转换参见 /p2p/discover 下的 node.go
// func PubkeyID(pub *ecdsa.PublicKey) NodeID {}
// func (id NodeID) Pubkey() (*ecdsa.PublicKey, error) {}
// 两个函数
package blizcore

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"

	"github.com/contatract/go-contatract/crypto"
	"github.com/contatract/go-contatract/crypto/secp256k1"
	"github.com/contatract/go-contatract/log"

	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/crypto/sha3"
	"github.com/contatract/go-contatract/p2p/discover"
	"github.com/contatract/go-contatract/rlp"
)

func BlizHash(data []byte) []byte {
	msg := fmt.Sprintf("\x19Blizzard Signed Message:\n%d%s", len(data), data)
	return crypto.Keccak256([]byte(msg))[0:4]
}

func DataHash(data ...[]byte) common.Hash {
	return crypto.Keccak256Hash(data...)
}

func rlpHash(x interface{}) (h common.Hash) {
	hw := sha3.NewKeccak256()
	// JiangHan: 意思是将x首先进行rlp编码，然后将编码输入 hw 进行散列，用hw 的散列输出当成结果
	rlp.Encode(hw, x)
	hw.Sum(h[:0])
	return h
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

func (wop *WopInfo) Sender() (common.Address, error) {
	return recoverPlain(wop.Hash(), wop.R, wop.S, wop.V)
}

func (wop *WopInfo) AuthSender() (common.Address, error) {
	return recoverPlain(wop.AuthHash(), wop.RC, wop.SC, wop.VC)
}

func RecoverBase(addr common.Address, deadline uint64, r, s, v *big.Int) (common.Address, error) {
	hash := rlpHash([]interface{}{
		addr,
		deadline,
	})
	return recoverPlain(hash, r, s, v)
}

// Hash returns the hash to be signed by the sender.
// It does not uniquely identify the transaction.
func (wop *WopInfo) Hash() common.Hash {
	return rlpHash([]interface{}{
		wop.WopId,
		wop.WopTs,
		wop.Chunk,
		wop.Slice,
		wop.Infos,
		wop.Type,
		wop.HeaderHash,
		wop.AuthAddress,
		wop.AuthDeadlineC,
		wop.RC,
		wop.SC,
		wop.VC,
	})
}

func (wop *WopInfo) AuthHash() common.Hash {
	return rlpHash([]interface{}{
		wop.AuthAddress,
		wop.AuthDeadlineC,
	})
}

// SignTx signs the transaction using the given signer and private key
func SignWop(wop *WriteOperReqData, prv *ecdsa.PrivateKey) error {
	h := wop.Hash()
	sig, err := crypto.Sign(h[:], prv)
	if err != nil {
		return err
	}
	return wop.WithSignature(sig)
}

// WithSignature returns a new transaction with the given signature.
// This signature needs to be formatted as described in the yellow paper (v+27).
func (wop *WriteOperReqData) WithSignature(sig []byte) error {
	r, s, v, err := wop.SignatureValues(sig)
	if err != nil {
		return err
	}
	wop.Info.R, wop.Info.S, wop.Info.V = r, s, v
	return nil
}

// SignatureValues returns signature values. This signature
// needs to be in the [R || S || V] format where V is 0 or 1.
func (wop *WriteOperReqData) SignatureValues(sig []byte) (r, s, v *big.Int, err error) {
	if len(sig) != 65 {
		panic(fmt.Sprintf("wrong size for signature: got %d, want 65", len(sig)))
	}
	r = new(big.Int).SetBytes(sig[:32])
	s = new(big.Int).SetBytes(sig[32:64])
	v = new(big.Int).SetBytes([]byte{sig[64] + 27})
	return r, s, v, nil
}

func (req *WriteOperReqData) Sender() (common.Address, error) {
	return req.Info.Sender()
}

func (req *WriteOperReqData) AuthSender() (common.Address, error) {
	return req.Info.AuthSender()
}

// Hash returns the hash to be signed by the sender.
// It does not uniquely identify the transaction.
func (req *WriteOperReqData) Hash() common.Hash {
	return req.Info.Hash()
}

// ----------------------------------------------------------------------------------

// recoverNodeID computes the public key used to sign the
// given hash from the signature.
func recoverNodeID(hash, sig []byte) (id discover.NodeID, err error) {
	pubkey, err := secp256k1.RecoverPubkey(hash, sig)
	if err != nil {
		return id, err
	}
	if len(pubkey)-1 != len(id) {
		return id, fmt.Errorf("recovered pubkey has %d bits, want %d bits", len(pubkey)*8, (len(id)+1)*8)
	}
	for i := range id {
		id[i] = pubkey[i+1]
	}
	return id, nil
}

func (hbdata *HeartBeatMsgData) SignHeartbeat(prv *ecdsa.PrivateKey) error {
	sig, err := crypto.Sign(hbdata.Hash().Bytes(), prv)
	if err != nil {
		log.Error("Can't sign discv4 packet", "err", err)
		return err
	}
	hbdata.Signature = sig
	return nil
}

func (hbdata *HeartBeatMsgData) Sender() (discover.NodeID, error) {
	return recoverNodeID(hbdata.Hash().Bytes(), hbdata.Signature)
}

// Hash returns the hash to be signed by the sender.
// It does not uniquely identify the transaction.
func (hbdata *HeartBeatMsgData) Hash() common.Hash {
	return rlpHash([]interface{}{
		hbdata.Progs,
		hbdata.ChunkId,
		hbdata.Ts,
	})
}

// common sign start
func CommonHash(x interface{}) (h common.Hash) {
	return rlpHash(x)
}

func CommonSign(h common.Hash, prv *ecdsa.PrivateKey) ([]byte, error) {
	return crypto.Sign(h[:], prv)
}

func CommonSender(h common.Hash, sign []byte) (common.Address, error) {
	if len(sign) != 65 {
		return common.Address{}, errors.New("wrong size for signature, want 65")
	}
	r := new(big.Int).SetBytes(sign[:32])
	s := new(big.Int).SetBytes(sign[32:64])
	v := new(big.Int).SetBytes([]byte{sign[64] + 27})
	return recoverPlain(h, r, s, v)
}

// common sign end

func RlpHash(x interface{}) (h common.Hash) {
	hw := sha3.NewKeccak256()
	rlp.Encode(hw, x)
	hw.Sum(h[:0])
	return h
}

func EcRecover(data, sig []byte) (common.Address, error) {
	if len(sig) != 65 {
		return common.Address{}, fmt.Errorf("signature must be 65 bytes long")
	}
	signature := make([]byte, len(sig))
	copy(signature, sig)

	rpk, err := crypto.Ecrecover(data, signature)
	if err != nil {
		return common.Address{}, err
	}
	pubKey := crypto.ToECDSAPub(rpk)
	recoveredAddr := crypto.PubkeyToAddress(*pubKey)
	return recoveredAddr, nil
}
