package fudp

const HdrSize = 4

type Upack []byte

/*
    结构:
	Upack{
		PAYLOAD: {
			data(nB): 最多可以负载BLOCK-Size*8
		}

		TAIL: {
			Group-Hash(1B)    : group id, 表明所属组
			Shard-Idx(1B)     : FEC组中的shard序号
			Shard-Flag(1B)    : 标志属性, 非0
		}
	}
*/

type Flag uint8

const (
	_ Flag = iota
	Data
	DataGroupTail
)

func (f Upack) Flag() Flag {
	_ = f[2]
	return Flag(f[len(f)-1])
}

func (f Upack) SetFlag(flag Flag) {
	_ = f[2]
	f[len(f)-1] = byte(flag)
}

func (f Upack) ShardIdx() uint8 {
	_ = f[2]
	return f[len(f)-2]
}

func (f Upack) SetGROUPIdx(n uint8) {
	_ = f[2]
	f[len(f)-2] = n
}

func (f Upack) GroupHash() uint8 {
	_ = f[2]
	return f[len(f)-3]
}

func (f Upack) SetGroupHash(u uint8) {
	_ = f[2]
	f[len(f)-3] = u
}
