package fudp

import (
	"fmt"
	"sync"
)

type ringInt struct {
	data       []int
	head, size int

	m          *sync.RWMutex
	putTrigger *sync.Cond
}

func newRingInt() *ringInt {
	r := &ringInt{
		data: make([]int, 64),
		m:    &sync.RWMutex{},
	}
	r.putTrigger = sync.NewCond(r.m)
	return r
}

func (r *ringInt) Get() int {
	r.m.RLock()
	defer r.m.RUnlock()

	for r.size == 0 {
		r.putTrigger.Wait()
	}

	i := (len(r.data) + r.head - r.size) % len(r.data)
	r.size--
	return r.data[i]
}

func (r *ringInt) Put(v int) {
	r.m.Lock()
	defer r.m.Unlock()

	if r.size == len(r.data) {
		r.grow()
	}

	r.data[r.head] = v
	r.head = (r.head + 1) % len(r.data)
	r.size++

	r.putTrigger.Signal()
}

func (r *ringInt) grow() {
	tdata := make([]int, len(r.data)*2)
	if r.head < r.size {
		n := copy(tdata, r.data[len(r.data)-(r.size-r.head):])
		copy(tdata[n:], r.data[:r.head])
	} else {
		copy(tdata, r.data[r.head-r.size:r.head])
	}
	r.head = r.size
}

type RingBuffer struct {
	buff       []byte // 存的是fudpPack
	head, size int    // for buff, head is the offset that next write to data
	headIdx    uint32 // expect next block index

	// const
	blockSize int // fec block大小, mtu*8 + fuHeaderLen
	groupSize int // fec 分组大小

	m          *sync.RWMutex
	putTrigger *sync.Cond
}

func NewRingBuffer(blockSize, groupSize int) *RingBuffer {
	return nil
}

func (r *RingBuffer) Put(b fudpPack) {
	r.m.Lock()

	bi := b.BlockIdx()
	bn := len(b)

	i := r.calcOffset(bi)
	if bi >= r.headIdx {
		// chech capacity
		delta := int(bi-r.headIdx+1) * r.blockSize
		for r.size+delta > len(r.buff) {
			r.grow()
		}

		n := copy(r.buff[i:], b)
		if n < bn {
			r.head = copy(r.buff[0:], b[n:])
		} else {
			r.head = (i + n) % len(r.buff)
		}

		r.size += delta
		r.headIdx = bi + 1
	} else {
		if !r.blockNull(i) {
			fmt.Println("repeated block data") // statistic
		}

		n := copy(r.buff[i:], b)
		if n < bn {
			r.head = copy(r.buff[0:], b[n:])
		}
	}

	r.m.Unlock()
	r.putTrigger.Signal()
}

func (r *RingBuffer) Read(b []byte) (int, error) {
	r.m.Lock()
	defer r.m.Unlock()

	for r.size == 0 {
		r.putTrigger.Wait()
	}

	return 0, nil
}

func (r *RingBuffer) grow() {
}

func (r *RingBuffer) calcOffset(idx uint32) int {
	return (r.head + (int(idx)-int(r.headIdx))*r.blockSize) % len(r.buff)
}

func (r *RingBuffer) blockNull(i int) bool {
	i = (i + r.blockSize - 4) % len(r.buff)
	return r.buff[i] == byte((r.blockSize-fudpHdrLen)/8)
}

func (r *RingBuffer) nullBlock(i int) {
	i = (i + r.blockSize - 4) % len(r.buff)
	r.buff[i] = 0
}
