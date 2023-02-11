package fudp

type Type uint8

const (
	Data Type = 0
	Ping Type = 1
	Plos Type = 2 // pack per loss
)

type Fpack []byte

func (f Fpack) Type() Type {
	_ = f[0]
	return Type(f[0])
}
