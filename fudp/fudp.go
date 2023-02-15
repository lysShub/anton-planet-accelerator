package fudp

import (
	"fmt"
	"net"

	"github.com/klauspost/reedsolomon"
	"github.com/tmthrgd/go-memset"
)

// udp conn use fec

const MTU = 1450 // 不包括head

type Fudp struct {
	// config
	groupLen int
	rawConn  net.Conn
	fecEnc   reedsolomon.Encoder

	setFec func()

	// write helper
	groupHash  uint16
	groupIdx   uint8
	blockBuff  Upack
	parityBuff Upack

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
			f.groupLen = p
		}
	}
	return f
}

func (f *Fudp) ghash() (hash uint16, gidx uint8, tail bool) {
	hash = f.groupHash
	gidx = f.groupIdx
	f.groupIdx = (f.groupIdx + 1) % uint8(f.groupLen)
	if f.groupIdx == 0 {
		tail = true
		f.groupHash += 1
	}

	return hash, gidx, tail
}

func (f *Fudp) Write(b []byte) (n int, err error) {
	if len(b) > MTU {
		return 0, fmt.Errorf("message %d too long than %d", len(b), MTU)
	}

	memset.Memset(f.blockBuff[DHdrSize+n:], 0)
	n = copy(f.blockBuff[DHdrSize:], b[0:])
	ghash, gidx, tail := f.ghash()
	f.blockBuff.SetTAILFlag(tail)
	f.blockBuff.SetGroupHash(ghash)
	f.blockBuff.SetGROUPIdx(gidx)

	// fec
	f.fecEnc.EncodeIdx(f.blockBuff[DHdrSize:], int(gidx), [][]byte{f.parityBuff[DHdrSize:]})

	// send
	if _, err = f.rawConn.Write(f.blockBuff[:DHdrSize+n]); err != nil {
		return 0, err
	}

	if tail {
		k := f.k()
		if _, err = f.rawConn.Write(f.parityBuff[:k]); err != nil {
			return 0, err
		}
		memset.Memset(f.parityBuff[DHdrSize:], 0)
	}

	return 0, nil
}

func (f *Fudp) k() int {
	i := len(f.parityBuff) - 1
	for ; i >= 0; i-- {
		if f.parityBuff[i] != 0 {
			break
		}
	}
	return i + 1
}

func (s *Fudp) Read(b []byte) (n int, err error) {

	return
}
