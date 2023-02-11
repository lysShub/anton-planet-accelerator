package fudp

import (
	"net"

	"github.com/klauspost/reedsolomon"
	"github.com/tmthrgd/go-memset"
)

// udp conn use fec

type Fudp struct {
	// config
	groupLen int
	bockSize int

	rawConn net.Conn
	fecEnc  reedsolomon.Encoder

	setFec func()
	setMTU func()

	// write helper
	blockIdx   uint16
	groupIdx   int
	blockBuff  Dpack
	parityBuff Dpack

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

			f.parityBuff.SetGROUPLen(uint8(p))
			f.blockBuff.SetGROUPLen(uint8(p))
		}
	}
	return f
}

// SetMTU 设置MTU, 将作用于DF
// mtu = DHdrSize + BlockSize
// BlockSize的限制是: 1. 8的整数倍   2. 范围为[8, 2040]
func (f *Fudp) SetMTU(mtu int) {
	f.setMTU = func() {
		bs := mtu - DHdrSize
		if bs < 0 {
			return
		}
		if bs%8 != 0 {
			bs = bs - bs%8 + 8
		}

		f.parityBuff = f.parityBuff.SetBLOCKSize(bs)
		f.blockBuff = f.blockBuff.SetBLOCKSize(bs)
	}
}

func (f *Fudp) gidx() (idx int) {
	idx = f.groupIdx
	f.groupIdx = (f.groupIdx + 1) % f.groupLen
	return
}

func (f *Fudp) bidx() (idx uint16) {
	idx = f.blockIdx
	f.blockIdx++
	return
}

func (f *Fudp) Write(b []byte) (n int, err error) {
	m := len(b)
	for i := 0; i < m; {
		gidx := f.gidx()

		n := copy(f.blockBuff[DHdrSize:], b[i:])
		if m-(i+n) > 0 {
			f.blockBuff.SetDFFlag(true)
		} else {
			f.blockBuff.SetDFFlag(false)
		}
		if gidx == 0 {
			f.blockBuff.SetHEADFlag(true)
		} else {
			f.blockBuff.SetHEADFlag(false)
		}
		f.blockBuff.SetBLOCKIdx(f.bidx())

		// fec
		if n < f.bockSize {
			memset.Memset(f.blockBuff[DHdrSize+n:], 0)
		}
		f.fecEnc.EncodeIdx(f.blockBuff[DHdrSize:], gidx, [][]byte{f.parityBuff[DHdrSize:]})

		// send
		if _, err = f.rawConn.Write(f.blockBuff[:DHdrSize+n]); err != nil {
			return 0, err
		}

		// group tail, send fec-packet
		if gidx == f.groupLen-1 {
			f.parityBuff.SetBLOCKIdx(f.bidx())
			// f.parityBuff.SetDFFlag(false)
			// f.parityBuff.SetHEADFlag(false)

			if _, err = f.rawConn.Write(f.parityBuff); err != nil {
				return 0, err
			}
			memset.Memset(f.parityBuff[DHdrSize:], 0)
		}

		i += n
	}

	return 0, nil
}

func (s *Fudp) Read(b []byte) (n int, err error) {

	return
}
