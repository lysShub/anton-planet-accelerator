package fudp

import (
	"net"

	"github.com/klauspost/reedsolomon"
	"github.com/tmthrgd/go-memset"
)

// udp conn use fec

const MTU = 1450

type Fudp struct {
	// config
	groupDataShards int // groupShards = groupDataShards + 1(groupParityShard)
	rawConn         net.Conn
	fecEnc          reedsolomon.Encoder

	setFec func()

	// write helper
	groupHash, groupIdx uint8
	blockBuff           Upack
	parityBuff          Upack
	parityLen           int

	// read helper

}

// NewFudp
func NewFudp(conn net.Conn) *Fudp {
	return &Fudp{}
}

// SetFEC 设置fec的纠错能力p[1, 64].
//
//	p=16表示每组中有16数据包, 有一个校验数据包, 17个数据包
//
// 中丢失一个数据包将能被恢复。
//
//	如果1:1的冗余度不能抵抗信道的丢包率, 数据包将会被重复
//
// 发送。
func (f *Fudp) SetFEC(p int) *Fudp {
	f.setFec = func() {
		if 1 <= p && p <= 64 {
			f.groupDataShards = p
		}
	}
	return f
}

func (f *Fudp) wrefresh() (hash, gidx uint8, tail bool) {
	hash = f.groupHash
	gidx = f.groupIdx
	if gidx == 0 { // group start
		f.parityBuff = f.parityBuff[:cap(f.parityBuff)]
		memset.Memset(f.parityBuff[:f.parityLen+HdrSize], 0)
		f.parityLen = 0
	}

	f.groupIdx = (f.groupIdx + 1) % uint8(f.groupDataShards)
	if f.groupIdx == 0 { // group end
		tail = true
		f.groupHash += 1
	}

	return hash, gidx, tail
}

func (f *Fudp) Write(b []byte) (n int, err error) {
	n = copy(f.blockBuff[0:], b[0:])
	defer func() {
		f.blockBuff = f.blockBuff[:cap(f.blockBuff)]
		memset.Memset(f.blockBuff[:n+HdrSize], 0)
	}()

	ghash, gidx, tail := f.wrefresh()
	f.blockBuff = f.blockBuff[:n+HdrSize]
	f.blockBuff.SetGroupHash(ghash)
	f.blockBuff.SetGROUPIdx(gidx)
	f.blockBuff.SetFlag(Data)

	// fec
	if err = f.fecEnc.EncodeIdx(f.blockBuff, int(gidx), [][]byte{f.parityBuff[:len(f.blockBuff)]}); err != nil {
		return 0, err
	} else {
		f.parityLen = max(f.parityLen, len(f.blockBuff))
	}

	// send
	if _, err = f.rawConn.Write(f.blockBuff[:HdrSize+n]); err != nil {
		return 0, err
	}

	if tail {
		f.parityBuff = f.parityBuff[:f.parityLen+HdrSize]
		f.blockBuff.SetGroupHash(ghash)
		f.blockBuff.SetGROUPIdx(gidx + 1)
		f.parityBuff.SetFlag(DataGroupTail)

		if _, err = f.rawConn.Write(f.parityBuff); err != nil {
			return 0, err
		}
	}

	return 0, nil
}

func max[T int](x, y T) T {
	if x > y {
		return x
	}
	return y
}

func (s *Fudp) Read(b []byte) (n int, err error) {

	return
}
