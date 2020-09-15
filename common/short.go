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

import "strings"

const (
	addrShortLen = 8
	hashShortLen = 8
)

func (a Address) ShortString() string {
	str := a.String()
	return strings.ToLower(str[len(str)-addrShortLen:])
}

func (a *Address) PShortString() string {
	if a == nil {
		return "nil"
	}
	return (*a).ShortString()
}

func (h Hash) ShortString() string {
	str := h.String()
	return strings.ToLower(str[len(str)-addrShortLen:])
}

func (h *Hash) PShortPointer() string {
	if h == nil {
		return "nil"
	}
	return (*h).ShortString()
}
