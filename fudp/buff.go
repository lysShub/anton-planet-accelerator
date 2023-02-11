package fudp

// 相较于bak, 此处的策略是完全不需要等待然后skip
//  表现为读取阻塞写入不阻塞

type RecvHeap struct {
	// const
	bockSize int
	groupLen int

	buff         []byte // 存的是fudpPack
	head, size   int    // for buff, head is the offset that next write to data
	nextBlockIdx uint32 // expect next recv block index
}

func NewRingBuffer(blockSize, groupSize int) *RecvHeap {
	return nil
}

// func (h *RecvHeap) Put(p fudpPack) {

// }

// func (h *RecvHeap) Get(b []byte) (n int) {

// 	return 0
// }
