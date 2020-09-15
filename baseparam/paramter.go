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

package baseparam

const (
	TypeActNone uint8 = iota
	TypeSealersElection
	TypePoliceElection
	TypeValidateElection
)

const (
	MaxElectionPoolLen    = 42
	MinElectionPoolLen    = 4
	MaxEleSealerCount     = 42
	MinEleSealerCount     = 1
	ElectionBlockCount    = 5 // 10
	BalanceReqForElection = 1 // 100000

	BFTSealersWorkDelay = 5 // When new valid election block mined in the lamp chain, new sealers will start to work after specific number of blocks
)
