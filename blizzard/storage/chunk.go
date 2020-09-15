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

package storage

import (
	"fmt"
	"math/big"

	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/common/math"
	//"github.com/contatract/go-contatract/node"
)

// 各节点的数据同步设计
// 数据的更新首先要确认的一点就是时间顺序，这点我们可以通过在Wop中增加时间戳（或者当前网络的
// BlockId)来标记，以此达到效果
// 对于Wop操作，节点接收到之后，对比Wop涉及的segment的时间戳和nonce标记，判断是否来自更新的
// 操作，如果是则执行update工作，如果不是，则丢弃
//
// 对于用户下载操作，如何能确保下载到最新的数据，除了逐一segment进行环比，还有什么效率更高的
// 方案，逐一环比的策略就是一次性将所有矿工有关该slice的segment info[]下载回来，整理出最新
// 的方案列表，然后同时请求下载。
//
// 校验的做法：我们可以不设置verifier, 因为那样通讯流程复杂，而且verifier也不易控制
// 我们可以通过magicfn(ChunkId, SelfId, BlockId, BlockHash) 满足一定要求的方式确定某chunk
// 是否可以发起校验交易(难度选择类似POW)，由于同一Chunk各矿工可能正在执行写入同步，这个校验
// 对时间要求并不用过于严格，我们可以规定BlockId在一定时间范围内都可以被接受，比如
// CurrentBlockId - BlockId < 100 这样就给予矿工们充分的同步时间。
// 为了防止矿工为了校验而拒绝tenant的更新请求，我们规定校验时需要将tenant提交的内容作为构建校验
// 内容的一个环节（不能全盘由tenant来制定校验内容，否则也存在刷的可能）
//
// 校验的设计可以设定为:
// 矿工首先BlockId判断自己的某个Chunk是否可以进行校验，如果符合条件，则根据
// 获取当前时间，并根据 SelectMagicFn(BlockId, TP)得到一个历史索引（TP是一个时间参数，要求每个
// 对应BlockId的矿工得到的历史索引是一致的。然后利用这个历史索引找到跟该索引最接近的Wop块
// 然后验证这个Wop块所在的128KB验证块的数据（验证数据的组成是 Data(32B) + 12*Hash(2B) = 56B)
// 存储的目的是为了提供服务，所以读取方面我们也不允许矿工为了节省自己的带宽资源而拒绝服务
// 是否需要建立一个评价体系？（流量证明？)
// 流量证明，可以使用收集访问者的流量签名来进行。传输伊始，访问者需要提供一个流量需求

// 校验的512KB块的索引，广播自己的校验需求给指定Chunk的矿工组的各个节点，让它们构造这个512KB块
// 的merkle树, 可以只取其中的128字节数据块+12个路径hash的方式进行验证。

// 采用访问者令牌的方式提供存储空间的访问权，每个segment都有一个tag, 这个tag是由上层应用者根据
// 业务需求制定的（比如文件id），制定后调用api将tag设定给所有涉及的segment保存，这样tenant（空间租用者）
// 就能够签发对这些数据的访问者令牌了。各位segment的存储矿工只需要判断visitor传递的令牌中是否有tenant
// 签名的tag就能判断是否允许访问了

const (
	MaxChunkIDBytes = 16
	MaxChunkCopys   = 32 // Amount of hashes to be fetched per retrieval request
)

const (
	kSize = 1024
	mSize = kSize * 1024
	gSize = mSize * 1024
	tSize = gSize * 1024
	pSize = tSize * 1024
	eSize = pSize * 1024
	//zSize = eSize * 1024
)

const (
	fileDir = "config/chaindata/gctt/file"
)

const (
	ChunkSize       = gSize // 每个chunk的大小
	ChunkSplitCount = 1024  // 每个chunk切分个数
	// ChunkCopyCount  = 2     //8 // 可以开始使用，也可以继续申领
	// ChunkCopyCountMax        = 2     // MaxChunkCopys 副本最大个数
	// ChunkCopyCountMin        = 2  //4 // 8个减少到4个以下就不能再使用
	// chunkApplyCountMax       = 10 // 开放申请的最大个数
	// chunkPersonApplyCountMax = 5 // 每个人可以申请的最大chunk数
)

const (
	// 无主状态
	chunkStatusNone = 1
	// 认领状态
	chunkStatusGetting = 2
	// 认证状态
	chunkStatusVerifying = 3
	// 认证成功
	chunkStatusVerified = 4
	// 数据状态
	chunkStatusData = 5
)

const (
	SliceSize       = mSize
	SegmentCount    = 32
	SegmentDataSize = SliceSize / SegmentCount
	SliceCount      = ChunkSize / SliceSize
)

type ChunkIdentity [MaxChunkIDBytes]byte

func (a *ChunkIdentity) Equal(b ChunkIdentity) bool {
	if len(a) != len(b) {
		return false
	}

	var i int
	for i = 0; i < MaxChunkIDBytes; i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ChunkIdentity prints as a long hexadecimal number.
func (n ChunkIdentity) String() string {
	return fmt.Sprintf("%x", n[:])
}

func (n *ChunkIdentity) Copy(b []byte) bool {
	if len(b) < MaxChunkIDBytes {
		return false
	}
	for i := 0; i < MaxChunkIDBytes; i++ {
		n[i] = b[i]
	}
	return true
}

func Int2ChunkIdentity(chunkIntId uint64) ChunkIdentity {
	var chunkId ChunkIdentity
	chunkId.Copy(math.Uint64ToBytes(chunkIntId, MaxChunkIDBytes))
	return chunkId
}

func ChunkIdentity2Int(chunkId ChunkIdentity) uint64 {
	return math.BytesToUint64(chunkId[:])
}

type visitorToken struct {
	tag        int
	visitor    common.Address
	permission int

	// Signature values(tenant sign all above)
	V *big.Int `json:"v" gencodec:"required"`
	R *big.Int `json:"r" gencodec:"required"`
	S *big.Int `json:"s" gencodec:"required"`
}
