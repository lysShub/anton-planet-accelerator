package fudp

import "unsafe"

const DHdrSize = 4

type Upack []byte

/*
structure:

	{
		HEAD: {
			DATA-Flag(1b) 	  : data-packet flag, 0使能
			TAIL-Flag(1b) 	  : FEC组中, 最后一个block标志, 是parity-block, 1使能
			GROUP-Idx(6b)     : FEC组中的block序号
			Group-Hash(2B)    : Block序号, 用于流
		}

		PAYLOAD: {
			data(nB): 最多可以负载BLOCK-Size*8
		}
	}
*/

type Type uint8

const (
	Data Type = 0
	Othe Type = 1
)

func (f Upack) Type() Type {
	_ = f[0]
	return Type(f[0] >> 7)
}

func (f Upack) TAILFlag() bool {
	_ = f[1]
	return f[1]&0b01000000 != 0
}

func (f Upack) SetTAILFlag(tail bool) {
	_ = f[0]
	if tail {
		f[0] = f[0] | 0b01000000
	} else {
		f[0] = f[0] & 0b10111111
	}
}

func (f Upack) GROUPIdx() uint8 {
	_ = f[1]
	return f[1]&0b00111111 + 1
}

func (f Upack) SetGROUPIdx(n uint8) {
	_ = f[1]
	n--
	f[1] = f[1] & (0b00111111 & n)
}

func (f Upack) GroupHash() uint16 {
	_ = f[3]
	return *(*uint16)(unsafe.Pointer(&f[2]))
}

func (f Upack) SetGroupHash(u uint16) {
	_ = f[3]
	*(*uint16)(unsafe.Pointer(&f[2])) = u
}
