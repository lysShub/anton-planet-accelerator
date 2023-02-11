package fudp

import (
	"net"

	"github.com/klauspost/reedsolomon"
)

// TODO: 放弃DF, 放弃DF后idx亦可抛弃, 只保留groupId即可

// 相较于bak, 此处的策略是完全不需要等待然后skip
//  表现为读取阻塞写入不阻塞

type FecBuff struct {
	// buff可以存储n个group, 而且是对齐的
	// 读取时会将其copy到buff中
	// 读取不是有序的, 优先读取最前的数据包
	//
	// 策略：
	// 在尝试读取下下个组时才会检查是否能恢复（如果有丢包的话）

	// const
	packSize int
	groupLen int
	rawConn  net.Conn
	fecEnc   reedsolomon.Encoder

	buff   []byte
	head   int // 记录最前面的group
	expIdx uint16
}

type block struct {
	b Dpack
	n int
}

type group struct {
	bs   []block
	sidx int // 组中第一个block的idx
	own  int // 记录本组中收到多少个Dpack
}

func (g *group) in(idx int) bool {
	return g.sidx <= idx && idx < g.sidx+len(g.bs)
}

func (g *group) put(b Dpack) {

}

func (g *group) used() bool {
	return false
}

func NewRingBuffer(blockSize, groupLen, n int) *FecBuff {
	b := blockSize + DHdrSize

	return &FecBuff{
		packSize: b,
		groupLen: groupLen,

		buff: make([]byte, b*groupLen*n),
	}
}

func (r *FecBuff) Put(b Dpack) {

}

func (r *FecBuff) Get(b []byte) (n int, err error) {
	return 0, nil
}
