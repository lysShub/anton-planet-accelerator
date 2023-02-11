package fudp

import (
	"unsafe"
)

type Dpack []byte

const DHdrSize = 5

/*
	Data packet, Type is Data
	{
		HEAD: {
			Type(1B)          :
			DF-Flag(1b) 	  : 分片标志, 仓库IPv4 DF, 0表示分片结束
			HEAD-Flag(1b) 	  : FEC分组开始标志, 其后GROUP-Len哥Block属于同一个组, 可以用于恢复, 1表示分片开始
			GROUP-Len(6b +1)  : FEC分组数据Block个数, 表示一个组中有GROUP-Len+1个Block
			BLOCK-Size(1B x8) : FEC分组中, 每个block的data大小(不包括Hdr), 实际大小x8, 不足用0填充
			Block-Idx(2B)     : Block序号, 用于流
		}

		PAYLOAD: {
			data(nB): 最多可以负载BLOCK-Size*8
		}
	}
*/

func (f Dpack) flush() Dpack {
	bs := f.BLOCKSize()

	if bs+DHdrSize < len(f) {
		return f[:bs+DHdrSize]
	} else if bs+DHdrSize > len(f) {
		t := make([]byte, bs+DHdrSize)
		copy(t, f)
		return Dpack(t)
	} else {
		return f
	}
}

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

func (f Dpack) BLOCKSize() int {
	_ = f[2]
	return int(f[2]) * 8
}

func (f Dpack) SetBLOCKSize(s int) Dpack {
	_ = f[2]
	f[2] = uint8(s / 8)

	return f.flush()
}

func (f Dpack) BLOCKIdx() uint16 {
	_ = f[4]
	return *(*uint16)(unsafe.Pointer(&f[3]))
}

func (f Dpack) SetBLOCKIdx(idx uint16) {
	_ = f[4]
	*(*uint16)(unsafe.Pointer(&f[3])) = idx
}
