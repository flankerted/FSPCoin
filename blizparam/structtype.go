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

package blizparam

type AddrTimeFlow struct {
	Addr      string
	StartTime uint64
	EndTime   uint64
	Flow      uint64
	PayMethod uint8
}

type CancelCsParam struct {
	CsAddr string
	Now    uint64
}

type SignTimeParam struct {
	Addr      string
	AllowTime uint64
}

type SignFlowParam struct {
	Addr      string
	AllowFlow uint64
}

type PickupTimeParam struct {
	Addr       string
	PickupTime uint64
}

type PickupFlowParam struct { // useless
	Addr       string
	PickupFlow uint64
}

type CsPickupCostParam struct {
	Addr          string
	UsedFlow      uint64
	AuthAllowFlow uint64
	Sign          []byte
	Now           uint64
}

type CsUsedFlowParam struct {
	Addr string
	Flow uint64
}

type UserAuthAllowFlowParam struct { // useless
	CsAddr string
	Flow   uint64
	Sign   []byte
}

type TxShareObjData struct {
	SharingCode string
}

type RegMailData struct {
	Name   string
	PubKey []byte
}
