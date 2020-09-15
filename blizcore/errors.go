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

package blizcore

import (
	"errors"
	"fmt"
)

type ErrCode int

const (
	ErrMsgTooLarge ErrCode = iota
	ErrDecode
	ErrInvalidMsgCode
	ErrInvalidMsgParameter
	ErrProtocolVersionMismatch
	ErrNetworkIdMismatch
	ErrGenesisBlockMismatch
	ErrNoStatusMsg
	ErrExtraStatusMsg
	ErrSuspendedPeer
	ErrNoDailChunkInfo
	ErrNoShakeHandsChunkInfo
	ErrChunkMismatch
	ErrLocalNotChunkFarmer
	ErrPartnerNotChunkFarmer
	ErrAssoAlreadyExist

	ErrRetriveFarmerFail
	ErrInsufficientFarmers
	ErrQueryShardingMinerTimeout
	ErrQueryFarmerTimeout
	ErrQueryObjectMetaTimeout

	ErrObjectSpaceInsufficient
	ErrWouldBlock
	ErrParameter
	ErrHash
	ErrFarmer
	ErrNetwork

	ErrWopIdNotValid
	ErrWopInvalidPermission
	ErrInvalidSig

	ErrOutOfRange
	ErrTimeout
	ErrNoEnoughFarmer
	ErrLackOfData
	ErrCapacityNotEnough
	ErrWriteOperRsp
	ErrFarmerSlice
	ErrReadOperRsp
	ErrSliceHeader

	ErrRsv
	ErrRsvTimeout
	ErrShutdown

	ErrUnknown // 务必放在最后
)

type peerError struct {
	code    ErrCode
	message string
}

func (e ErrCode) String() string {
	return errorToString[e]
}

func (e ErrCode) Error() string {
	return e.String()
}

func ErrorToUInt8(e error) uint8 {
	i := uint8(0)
	maxI := uint8(ErrUnknown)
	for ; i < maxI; i++ {
		if ErrCode(i).String() == e.Error() {
			break
		}
	}
	return i
}

func UInt8ToErrorCode(id uint8) ErrCode {
	errCode := (ErrCode)(id)
	if _, ok := errorToString[errCode]; !ok {
		errCode = ErrUnknown
	}
	return errCode
}

// XXX change once legacy code is out
var errorToString = map[ErrCode]string{
	ErrMsgTooLarge:             "Message too long",
	ErrDecode:                  "Invalid message",
	ErrInvalidMsgCode:          "Invalid message code",
	ErrInvalidMsgParameter:     "Invalid message parameter",
	ErrProtocolVersionMismatch: "Protocol version mismatch",
	ErrNetworkIdMismatch:       "NetworkId mismatch",
	ErrGenesisBlockMismatch:    "Genesis block mismatch",
	ErrNoStatusMsg:             "No status message",
	ErrExtraStatusMsg:          "Extra status message",
	ErrSuspendedPeer:           "Suspended peer",
	ErrNoDailChunkInfo:         "No chunk Info found in blizzard dail",
	ErrNoShakeHandsChunkInfo:   "No chunk Info found in shakehands",
	ErrChunkMismatch:           "ChunkID mismatch",
	ErrLocalNotChunkFarmer:     "Local Host Not Chunk farmer",
	ErrPartnerNotChunkFarmer:   "Incoming Partner Not Chunk farmer",
	ErrAssoAlreadyExist:        "The Chunk Asso already Exist",

	ErrRetriveFarmerFail:         "Query Farmers Failed",
	ErrInsufficientFarmers:       "No Sufficient Farmer for Chunk",
	ErrQueryShardingMinerTimeout: "Query sharding miners timeout",
	ErrQueryFarmerTimeout:        "Query chunk farmers timeout",
	ErrQueryObjectMetaTimeout:    "Query Object Meta timeout",

	ErrObjectSpaceInsufficient: "Blizzard Space for this Object is insufficient",
	ErrWouldBlock:              "Operation is Blocked for network or resource reason",
	ErrParameter:               "Paramerter error",
	ErrHash:                    "Hash error",
	ErrFarmer:                  "Farmer error",
	ErrNetwork:                 "Network error",

	ErrWopIdNotValid:        "WopId is Invalid",
	ErrWopInvalidPermission: "Invalid sender or permission error",
	ErrInvalidSig:           "Signing is Invalid",

	ErrOutOfRange:        "Out of range",
	ErrTimeout:           "Timeout",
	ErrNoEnoughFarmer:    "No enough farmer",
	ErrLackOfData:        "Lack of data",
	ErrCapacityNotEnough: "No enough capacity",
	ErrWriteOperRsp:      "Error of writing Operation response",
	ErrFarmerSlice:       "Error of farmer slice",
	ErrReadOperRsp:       "Error of reading Operation response",
	ErrSliceHeader:       "Error of slice header",

	ErrRsv:        "Error of rsv",
	ErrRsvTimeout: "Error of getting rsv timeout",

	ErrShutdown: "Shutdown",

	ErrUnknown: "Unknown",
}

func newPeerError(code ErrCode, format string, v ...interface{}) *peerError {
	desc, ok := errorToString[code]
	if !ok {
		panic("invalid error code")
	}
	err := &peerError{code, desc}
	if format != "" {
		err.message += ": " + fmt.Sprintf(format, v...)
	}
	return err
}

func (self *peerError) Error() string {
	return self.message
}

var errProtocolReturned = errors.New("protocol returned")

type DiscReason uint

const (
	DiscRequested DiscReason = iota
	DiscNetworkError
	DiscProtocolError
	DiscUselessPeer
	DiscTooManyPeers
	DiscAlreadyConnected
	DiscIncompatibleVersion
	DiscInvalidIdentity
	DiscQuitting
	DiscUnexpectedIdentity
	DiscSelf
	DiscReadTimeout
	DiscSubprotocolError = 0x10
)

var discReasonToString = [...]string{
	DiscRequested:           "disconnect requested",
	DiscNetworkError:        "network error",
	DiscProtocolError:       "breach of protocol",
	DiscUselessPeer:         "useless peer",
	DiscTooManyPeers:        "too many peers",
	DiscAlreadyConnected:    "already connected",
	DiscIncompatibleVersion: "incompatible p2p protocol version",
	DiscInvalidIdentity:     "invalid node identity",
	DiscQuitting:            "client quitting",
	DiscUnexpectedIdentity:  "unexpected identity",
	DiscSelf:                "connected to self",
	DiscReadTimeout:         "read timeout",
	DiscSubprotocolError:    "subprotocol error",
}

func (d DiscReason) String() string {
	if len(discReasonToString) < int(d) {
		return fmt.Sprintf("unknown disconnect reason %d", d)
	}
	return discReasonToString[d]
}

func (d DiscReason) Error() string {
	return d.String()
}

func discReasonForError(err error) DiscReason {
	if reason, ok := err.(DiscReason); ok {
		return reason
	}
	if err == errProtocolReturned {
		return DiscQuitting
	}
	peerError, ok := err.(*peerError)
	if ok {
		switch peerError.code {
		case ErrInvalidMsgCode:
			return DiscProtocolError
		default:
			return DiscSubprotocolError
		}
	}
	return DiscSubprotocolError
}
