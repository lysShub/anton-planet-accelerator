package fudp

// fragment layer
//  split a too large udp(over fragSize limit) into several small ones, just like ip fragment.
//  meanwhile, enable to logic stream link, with increments id; and can't merge fragments (like tcp Nagle),
//  because of we must recover to datagram at receiver side.

/*
	frag package structure:
	{ origin-data : frag-idx(23b) end-flag(1b)}
*/

type frag struct {
	id uint32

	fragSize int
}

func NewFrag(fragSize int) *frag {
	return &frag{}
}

func (f *frag) Wrap(in []byte) (out []byte, idx int) {

	for i := 0; i < len(in); i = i + f.fragSize {

	}

	return nil, 0
}

func (f *frag) inc() (a, b, c byte) {
	a = byte(f.id)
	b = byte(f.id >> 8)
	c = byte(f.id >> 16)
	c = c << 1

	f.id++
	return
}
