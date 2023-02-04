package fudp

import (
	"sync"
)

// 相较于bak, 此处的策略是完全不需要等待然后skip
//  表现为读取阻塞写入不阻塞

type RecvHeap struct {
	// const
	blockSize int // fec block大小, mtu*8 + fuHeaderLen
	groupSize int // fec 分组大小

	// buff store fudpPack data, is aligned by blockIdx
	buff         []byte // 存的是fudpPack
	head, size   int    // for buff, head is the offset that next write to data
	nextBlockIdx uint32 // expect next recv block index

	// record has loss data group, init valuse is groupSize
	groupLack []int

	// blockIdxs is record can read block indexs, must be len(blockIdxs) == len(buff)/blockSize.
	// buff[getI()] is the first block can read block's first byte.
	// make sure use getI() and putI() consistent with actual logic.
	blockIdxs                   []int
	blockIdxsSize, blockIdxsHdr int

	m *sync.RWMutex
}

func NewRingBuffer(groups int) *RecvHeap {
	return nil
}

func (h *RecvHeap) Put(p fudpPack) {

}

func (h *RecvHeap) Get(b []byte) (n int) {

	return 0
}
