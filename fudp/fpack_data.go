package fudp

import (
	"unsafe"
)

type Dpack []byte

const DHdrSize = 4

/*
	Data packet, Type is Data
	{
		HEAD: {
			Type(1B)          :
			DF-Flag(1b) 	  : 分片标志, 仓库IPv4 DF, 0表示分片结束
			HEAD-Flag(1b) 	  : FEC分组开始标志, 其后GROUP-Len哥Block属于同一个组, 可以用于恢复, 1表示分片开始
			GROUP-Len(6b +1)  : FEC分组数据Block个数, 表示一个组中有GROUP-Len+1个Block
			Block-Hash(2B)    : Block序号, 用于流
		}

		PAYLOAD: {
			data(nB): 最多可以负载BLOCK-Size*8
		}
	}
*/

func (f Dpack) DFFlag() bool {
	_ = f[1]
	return f[1]&0b10000000 == 1
}

func (f Dpack) SetDFFlag(df bool) {
	_ = f[1]
	if df {
		f[1] = f[1] | 0b10000000
	} else {
		f[1] = f[1] & 0b01111111
	}
}

func (f Dpack) HEADFlag() bool {
	_ = f[1]
	return f[1]&0b01000000 == 1
}

func (f Dpack) SetHEADFlag(head bool) {
	_ = f[1]
	if head {
		f[1] = f[1] | 0b01000000
	} else {
		f[1] = f[1] & 0b10111111
	}
}

func (f Dpack) GROUPLen() uint8 {
	_ = f[1]
	return f[1]&0b00111111 + 1
}

func (f Dpack) SetGROUPLen(n uint8) {
	_ = f[1]
	n--
	f[1] = f[1] & (0b00111111 & n)
}

func (f Dpack) BLOCKIdx() uint16 {
	_ = f[3]
	return *(*uint16)(unsafe.Pointer(&f[2]))
}

func (f Dpack) SetBLOCKIdx(idx uint16) {
	_ = f[3]
	*(*uint16)(unsafe.Pointer(&f[2])) = idx
}
