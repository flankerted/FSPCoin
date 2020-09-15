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

package elephanthash

import (
	"errors"
	"fmt"
	"math/big"
	"runtime"
	"time"

	"github.com/contatract/go-contatract/baseparam"
	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/common/math"
	"github.com/contatract/go-contatract/consensus"
	types "github.com/contatract/go-contatract/core/types_elephant"
	"github.com/contatract/go-contatract/core_elephant/state"
	"github.com/contatract/go-contatract/crypto"
	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/params"
)

// Ethash proof-of-work protocol constants.
var (
	FrontierBlockReward    *big.Int = big.NewInt(5e+18) // Block reward in wei for successfully mining a block
	maxUncles                       = 2                 // Maximum number of uncles allowed in a single block
	allowedFutureBlockTime          = 15 * time.Second  // Max time from current time allowed for blocks, before they're considered future blocks
)

// Various error messages to mark blocks invalid. These should be private to
// prevent engine specific errors from being referenced in the remainder of the
// codebase, inherently breaking if the engine is swapped out. Please put common
// error types into the consensus package.
var (
	errLargeBlockTime    = errors.New("timestamp too big")
	errZeroBlockTime     = errors.New("timestamp equals parent's")
	errTooManyUncles     = errors.New("too many uncles")
	errDuplicateUncle    = errors.New("duplicate uncle")
	errUncleIsAncestor   = errors.New("uncle is ancestor")
	errDanglingUncle     = errors.New("uncle's parent is not ancestor")
	errNonceOutOfRange   = errors.New("nonce out of range")
	errInvalidDifficulty = errors.New("non-positive difficulty")
	errInvalidMixDigest  = errors.New("invalid mix digest")
	errInvalidPoW        = errors.New("invalid proof-of-work")
)

// Author implements consensus.Engine, returning the header's coinbase as the
// proof-of-work verified author of the block.
func (elephanthash *Elephanthash) Author(header *types.Header) (common.Address, error) {
	return header.Coinbase, nil
}

func (elephanthash *Elephanthash) VerifyHeaderLampBase(header *types.Header) error {
	lamp := elephanthash.hbftNode.Lamp
	if lamp.LastEleSealersBlockNum() > 0 && int64(lamp.CurrEleSealersBlockNum()) == header.LampBaseNumber.Int64() {
		if hash := lamp.CurrEleSealersBlockHash(); hash.Equal(header.LampBaseHash) {
			return nil
		} else {
			log.Error("verifyHeaderLampBase failed", "hash", hash, "LampBaseHash", header.LampBaseHash)
			return consensus.ErrInvalidHeader
		}
	} else if lamp.LastEleSealersBlockNum() > 0 && int64(lamp.LastEleSealersBlockNum()) == header.LampBaseNumber.Int64() {
		if hash := lamp.LastEleSealersBlockHash(); hash.Equal(header.LampBaseHash) {
			return nil
		} else {
			log.Error("verifyHeaderLampBase failed", "LastEleSealersBlockNum", lamp.LastEleSealersBlockNum, "LampBaseNumber", header.LampBaseNumber.Int64())
			return consensus.ErrInvalidHeader
		}
	} else if lamp.LastEleSealersBlockNum() == 0 && int64(lamp.CurrEleSealersBlockNum()) == header.LampBaseNumber.Int64() &&
		header.LampBaseNumber.Int64() == baseparam.ElectionBlockCount {
		return nil
	} else {
		log.Error("verifyHeaderLampBase failed", "LastEleSealersBlockNum", lamp.LastEleSealersBlockNum(), "CurrEleSealersBlockNum", lamp.CurrEleSealersBlockNum(), "LampBaseNumber", header.LampBaseNumber.Int64())
		return consensus.ErrInvalidHeader
	}
}

// VerifyHeader checks whether a header conforms to the consensus rules of the
// stock Ethereum ethash engine.
func (elephanthash *Elephanthash) VerifyHeader(chain consensus.ChainReaderCtt, header *types.Header) error {
	// If we're running a full engine faking, accept any input as valid
	if elephanthash.config.PowMode == ModeFullFake {
		return nil
	}
	// Short circuit if the header is known, or it's parent not
	number := header.Number.Uint64()
	if chain.GetHeader(header.Hash(), number) != nil {
		return nil
	}
	parent := chain.GetHeader(header.ParentHash, number-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	// Sanity checks passed, do a proper verification
	return elephanthash.verifyHeader(chain, header, parent)
}

// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
// concurrently. The method returns a quit channel to abort the operations and
// a results channel to retrieve the async verifications.
func (elephanthash *Elephanthash) VerifyHeaders(chain consensus.ChainReaderCtt, headers []*types.Header) (chan<- struct{}, <-chan error) {
	// If we're running a full engine faking, accept any input as valid
	if elephanthash.config.PowMode == ModeFullFake || len(headers) == 0 {
		abort, results := make(chan struct{}), make(chan error, len(headers))
		for i := 0; i < len(headers); i++ {
			results <- nil
		}
		return abort, results
	}

	// Spawn as many workers as allowed threads
	workers := runtime.GOMAXPROCS(0)
	if len(headers) < workers {
		workers = len(headers)
	}

	// Create a task channel and spawn the verifiers
	var (
		inputs = make(chan int)
		done   = make(chan int, workers)
		errors = make([]error, len(headers))
		abort  = make(chan struct{})
	)
	for i := 0; i < workers; i++ {
		go func() {
			for index := range inputs {
				errors[index] = elephanthash.verifyHeaderWorker(chain, headers, index)
				done <- index
			}
		}()
	}

	errorsOut := make(chan error, len(headers))
	go func() {
		defer close(inputs)
		var (
			in, out = 0, 0
			checked = make([]bool, len(headers))
			inputs  = inputs
		)
		for {
			select {
			case inputs <- in:
				if in++; in == len(headers) {
					// Reached end of headers. Stop sending to workers.
					inputs = nil
				}
			case index := <-done:
				for checked[index] = true; checked[out]; out++ {
					errorsOut <- errors[out]
					if out == len(headers)-1 {
						return
					}
				}
			case <-abort:
				return
			}
		}
	}()
	return abort, errorsOut
}

func (elephanthash *Elephanthash) verifyHeaderWorker(chain consensus.ChainReaderCtt, headers []*types.Header, index int) error {
	var parent *types.Header
	if index == 0 {
		parent = chain.GetHeader(headers[0].ParentHash, headers[0].Number.Uint64()-1)
	} else if headers[index-1].Hash() == headers[index].ParentHash {
		parent = headers[index-1]
	}
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	if chain.GetHeader(headers[index].Hash(), headers[index].Number.Uint64()) != nil {
		return nil // known block
	}
	return elephanthash.verifyHeader(chain, headers[index], parent)
}

// VerifyBFT verifies that the given block's BFT records conform to the consensus
// rules of the engine.
func (elephanthash *Elephanthash) VerifyBFT(chain consensus.ChainReaderCtt, block *types.Block) error {
	if block.HBFTStageChain() == nil {
		return fmt.Errorf("invalid BFT stage chain: stage is nil")
	}
	stages := block.HBFTStageChain()
	if len(stages) != 2 {
		return fmt.Errorf("invalid BFT stage chain: the count of stages is not valid")
	}
	if parent := chain.GetBlock(block.ParentHash(), block.NumberU64()-1); parent == nil {
		return fmt.Errorf("invalid BFT stage chain: can not find parent block, parentHash: %s, the hash of the block: %s",
			block.ParentHash().TerminalString(), block.Hash().TerminalString())
	} else if parent.NumberU64() > 0 {
		parentStages := parent.HBFTStageChain()
		if len(parentStages) != 2 {
			return fmt.Errorf("invalid BFT stage chain of parent block: the count of stages is not valid")
		}
		if !stages[0].BlockHash.Equal(block.Hash()) {
			return fmt.Errorf("invalid BFT pre-confirm stage's blockHash: %s, the hash of the block: %s",
				stages[0].BlockHash.TerminalString(), block.Hash().TerminalString())
		}
		//if !stages[0].ParentStage.Equal(parentStages[1].HashWithBft()) {
		//	if parentStagesHash := parentStages[1].Hash(); !parentStagesHash.Equal(stages[0].Hash()) {
		//		return fmt.Errorf("invalid BFT stage chain: the parent stage mismatch, have %x, want %x", stages[0].ParentStage, parentStages[1].Hash())
		//	}
		//}
	}
	if !stages[1].ParentStage.Equal(stages[0].Hash()) {
		return fmt.Errorf("invalid BFT stage chain: the stages can not build a chain")
	}
	if stages[0].ReqTimeStamp <= block.Time().Uint64() {
		return fmt.Errorf("invalid BFT stage chain: the time stamp of request mesage is invalid")
	}
	if stages[1].Timestamp <= stages[0].Timestamp || stages[0].Timestamp <= stages[0].ReqTimeStamp {
		return fmt.Errorf("invalid BFT stage chain: the time stamp of stages are invalid")
	}

	sealers, _ := elephanthash.hbftNode.EleBackend.GetRealSealersByNum(block.NumberU64() - 1)
	sealersMap := make(map[common.Address]common.Address)
	if sealers == nil {
		return fmt.Errorf("invalid BFT stage chain: can not get the sealers")
	}

	for i, stage := range stages {
		if len(stage.ValidSigns) <= elephanthash.hbftNode.EleBackend.Get2fRealSealersCnt() {
			return fmt.Errorf("invalid BFT stage chain: the count of valid signatures can not be accepted in stage[%d], have %d, want %d",
				i, len(stage.ValidSigns), elephanthash.hbftNode.EleBackend.Get2fRealSealersCnt()+1)
		}

		msg := stage.MsgHash.Bytes()
		for _, sealer := range sealers {
			sealersMap[sealer] = sealer
		}
		for j, sign := range stage.ValidSigns {
			addr, err := EcRecover(msg, sign)
			if err != nil {
				return fmt.Errorf("invalid BFT stage chain: can not recover the address of the signature[%d]", j)
			}

			if _, ok := sealersMap[addr]; ok {
				delete(sealersMap, addr)
			} else {
				return fmt.Errorf("invalid BFT stage chain: the address %s recovered from the signature[%d] in stage[%d] is not valid",
					addr.String(), j, i)
			}
		}
	}

	return nil
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

// verifyHeader checks whether a header conforms to the consensus rules of the
// stock Ethereum ethash engine.
// See YP section 4.3.4. "Block Header Validity"
func (elephanthash *Elephanthash) verifyHeader(chain consensus.ChainReaderCtt, header, parent *types.Header) error {
	// Ensure that the header's extra-data section is of a reasonable size
	if uint64(len(header.Extra)) > params.MaximumExtraDataSize {
		return fmt.Errorf("extra-data too long: %d > %d", len(header.Extra), params.MaximumExtraDataSize)
	}
	// Verify the header's timestamp
	if header.Time.Cmp(big.NewInt(time.Now().Add(allowedFutureBlockTime).UnixNano()/1e6)) > 0 {
		return consensus.ErrFutureBlock
	}
	if header.Time.Cmp(parent.Time) <= 0 {
		return errZeroBlockTime
	}
	// Verify that lamp base number and hash
	if common.GetConsensusBft() {
		if err := elephanthash.VerifyHeaderLampBase(header); err != nil {
			return err
		}
	}

	//// Verify the block's difficulty based in it's timestamp and parent's difficulty
	//expected := elephanthash.CalcDifficulty(chain, header.Time.Uint64(), parent)
	//
	//if expected.Cmp(header.Difficulty) != 0 {
	//	return fmt.Errorf("invalid difficulty: have %v, want %v", header.Difficulty, expected)
	//}

	// Verify that the gas limit is <= 2^63-1
	cap := uint64(0x7fffffffffffffff)
	if header.GasLimit > cap {
		return fmt.Errorf("invalid gasLimit: have %v, max %v", header.GasLimit, cap)
	}
	// Verify that the gasUsed is <= gasLimit
	if header.GasUsed > header.GasLimit {
		return fmt.Errorf("invalid gasUsed: have %d, gasLimit %d", header.GasUsed, header.GasLimit)
	}

	// Verify that the gas limit remains within allowed bounds
	diff := int64(parent.GasLimit) - int64(header.GasLimit)
	if diff < 0 {
		diff *= -1
	}
	limit := parent.GasLimit / params.GasLimitBoundDivisor

	if uint64(diff) >= limit || header.GasLimit < params.MinGasLimit {
		return fmt.Errorf("invalid gas limit: have %d, want %d += %d", header.GasLimit, parent.GasLimit, limit)
	}
	// Verify that the block number is parent's +1
	if diff := new(big.Int).Sub(header.Number, parent.Number); diff.Cmp(big.NewInt(1)) != 0 {
		return consensus.ErrInvalidNumber
	}

	return nil
}

// CalcDifficulty is the difficulty adjustment algorithm. It returns
// the difficulty that a new block should have when created at time
// given the parent block's time and difficulty.
func (elephanthash *Elephanthash) CalcDifficulty(chain consensus.ChainReaderCtt, time uint64, parent *types.Header) *big.Int {
	return CalcDifficulty(chain.Config(), time, parent)
}

// CalcDifficulty is the difficulty adjustment algorithm. It returns
// the difficulty that a new block should have when created at time
// given the parent block's time and difficulty.
func CalcDifficulty(config *params.ChainConfig, time uint64, parent *types.Header) *big.Int {
	return calcDifficulty(time, parent)
}

// Some weird constants to avoid constant memory allocs for them.
var (
	expDiffPeriod = big.NewInt(100000)
	big1          = big.NewInt(1)
	big2          = big.NewInt(2)
	big9          = big.NewInt(9)
	big10         = big.NewInt(10)
	bigMinus99    = big.NewInt(-99)
	big2999999    = big.NewInt(2999999)
)

// calcDifficulty is the difficulty adjustment algorithm. It returns the
// difficulty that a new block should have when created at time given the parent
// block's time and difficulty. The calculation uses the Frontier rules.
func calcDifficulty(time uint64, parent *types.Header) *big.Int {
	diff := new(big.Int)
	adjust := new(big.Int).Div(parent.Difficulty, params.DifficultyBoundDivisor)
	bigTime := new(big.Int)
	bigParentTime := new(big.Int)

	bigTime.SetUint64(time)
	bigParentTime.Set(parent.Time)

	if bigTime.Sub(bigTime, bigParentTime).Cmp(params.DurationLimit) < 0 {
		diff.Add(parent.Difficulty, adjust)
	} else {
		diff.Sub(parent.Difficulty, adjust)
	}
	if diff.Cmp(params.MinimumDifficulty) < 0 {
		diff.Set(params.MinimumDifficulty)
	}

	periodCount := new(big.Int).Add(parent.Number, big1)
	periodCount.Div(periodCount, expDiffPeriod)
	if periodCount.Cmp(big1) > 0 {
		// diff = diff + 2^(periodCount - 2)
		expDiff := periodCount.Sub(periodCount, big2)
		expDiff.Exp(big2, expDiff, nil)
		diff.Add(diff, expDiff)
		diff = math.BigMax(diff, params.MinimumDifficulty)
	}
	return diff
}

// Prepare implements consensus.Engine, initializing the difficulty field of a
// header to conform to the ethash protocol. The changes are done inline.
func (elephanthash *Elephanthash) Prepare(chain consensus.ChainReaderCtt, header *types.Header) error {
	parent := chain.GetHeader(header.ParentHash, header.Number.Uint64()-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	header.Difficulty = elephanthash.CalcDifficulty(chain, header.Time.Uint64()/1000, parent)
	return nil
}

// Finalize implements consensus.Engine, accumulating the block and uncle rewards,
// setting the final state and assembling the block.
func (elephanthash *Elephanthash) Finalize(chain consensus.ChainReaderCtt, header *types.Header, state *state.StateDB,
	txs []*types.Transaction, inputTxs []*types.ShardingTxBatch, uncles []*types.Header, receipts []*types.Receipt, eBase *common.Address) (*types.Block, error) {
	// Accumulate any block and uncle rewards and commit the final state root
	accumulateRewards(chain.Config(), state, header, uncles)
	header.Root = state.IntermediateRoot(false)

	// Header seems complete, assemble into a block and return
	return types.NewBlock(header, txs, inputTxs, uncles, receipts, nil, eBase), nil
}

// Some weird constants to avoid constant memory allocs for them.
var (
	big8  = big.NewInt(8)
	big32 = big.NewInt(32)
)

// AccumulateRewards credits the coinbase of the given block with the mining
// reward. The total reward consists of the static block reward and rewards for
// included uncles. The coinbase of each uncle block is also rewarded.
func accumulateRewards(config *params.ChainConfig, state *state.StateDB, header *types.Header, uncles []*types.Header) {
	// Select the correct block reward based on chain progression
	blockReward := FrontierBlockReward

	// Accumulate the rewards for the miner and any included uncles
	reward := new(big.Int).Set(blockReward)
	r := new(big.Int)
	for _, uncle := range uncles {
		r.Add(uncle.Number, big8)
		r.Sub(r, header.Number)
		r.Mul(r, blockReward)
		r.Div(r, big8)
		state.AddBalance(uncle.Coinbase, r)

		r.Div(blockReward, big32)
		reward.Add(reward, r)
	}
	state.AddBalance(header.Coinbase, reward)
}
