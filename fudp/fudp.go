package fudp

import (
	"errors"
	"net"
	"sort"
	"unsafe"

	"github.com/klauspost/reedsolomon"
	"github.com/tmthrgd/go-memset"
)

// udp conn use fec
/*
	fudp packet structure:
	因为要同时兼容流与数据报, 而且不存在工作模式、没有握手, 所以要把所有的信息都放在header

	{origin-data :  : reliable(1b) : reliable-resend(1) : stream(1b) : mtu(1B, *8) : fec-group-head(1b) : fec-group-len(4b, n:1) : block-end(1b) : block-idx(4B)}

*/

/*
	fudp packet structure:
	{origin-data:  frag-end-block-flag(1b) : fec-start-block-flag(1b) : fec-data-block-len(6b) : fudp-mtu(1B x8) : block-idx(4B) }
*/

type fudpPack []byte

const fudpHdrLen int = 6

func (p fudpPack) Valid() bool {
	return len(p) > fudpHdrLen
}

func (p fudpPack) Payload() []byte {
	return p[:len(p)-fudpHdrLen]
}

func (p fudpPack) IsFragEnd() bool {
	i := len(p) - 1
	return p[i-5]&0b10000000 != 0
}

func (p fudpPack) IsFecStart() bool {
	i := len(p) - 1
	return p[i-5]&0b01000000 != 0
}

func (p fudpPack) FecDataLen() int {
	i := len(p) - 1
	return int(p[i-5] & 0b00111111)
}

func (p fudpPack) Mtu() int {
	i := len(p) - 1
	return int(p[i-4]) * 8
}

func (p fudpPack) BlockIdx() uint32 {
	i := len(p) - 1
	return *(*uint32)(unsafe.Pointer(&p[i-3]))
}

type Fudp struct {
	rawConn net.Conn

	// Fudp's mtu, means the max size of a fudp packet's payload
	mtu   int
	_mtu8 uint8

	fragBuff []byte
	blockIdx uint32

	fecLen      int
	fecDataLen  int
	_fecDataLen uint8
	fecIdx      int
	fecParity   [][]byte
	fecEnc      reedsolomon.Encoder

	recvBuff [][]byte
}

// NewFudp
func NewFudp(conn net.Conn) *Fudp {
	return &Fudp{}
}

func (s *Fudp) SetFEC(dataBlocks int) error {
	return nil
}

func (s *Fudp) SetMTU(mtu int) error {
	return nil
}

func (s *Fudp) Write(b []byte) (n int, err error) {
	for i := 0; i < len(b); i = i + s.mtu {
		n := copy(s.fragBuff[0:s.mtu], b[i:i+s.mtu])
		if err := s.fec(s.fragBuff[:n]); err != nil {
			return 0, err
		}

		s.fragBuff = s.fragBuff[:n+fudpHdrLen]

		j := n + fudpHdrLen - 1
		*(*uint32)(unsafe.Pointer(&s.fragBuff[j-3])) = s.blockIdx
		s.fragBuff[j-4] = s._mtu8
		s.fragBuff[j-5] = s._fecDataLen & 0b00111111
		if n >= len(b) {
			s.fragBuff[j-5] |= 0b10000000
		}
		if s.fecIdx == 0 {
			s.fragBuff[j-5] |= 0b01000000
		}

		_, err := s.rawConn.Write(s.fragBuff)
		if err != nil {
			return 0, err
		}

		s.blockIdx++
		s.fecIdx = (s.fecIdx + 1) % s.fecDataLen

		if s.fecIdx == 0 {
			if _, err := s.Write(s.fecParity[0]); err != nil {
				return 0, err
			}
			memset.Memset(s.fecParity[0], 0)
		}
	}
	return 0, nil
}

func (s *Fudp) fec(b []byte) error {
	return s.fecEnc.EncodeIdx(b, s.fecIdx, s.fecParity)
}

func (s *Fudp) Read(b []byte) (n int, err error) {
	if n, err = s.rawConn.Read(b); err != nil {
		return n, err
	}

	p := fudpPack(b[:n])
	if !p.Valid() {
		return 0, errors.New("invalid fudp packet")
	}

	p.BlockIdx()

	return 0, nil
}

type group struct {
	startIdx       uint32
	blockSize      int
	groupDataLen   int
	blocks         [][]byte
	parity         []byte
	putLen, getLen int
	active         bool
}

func newGroup(blockSize, groupDataLen int, startBlockIdx uint32) *group {
	var g = &group{
		startIdx:  uint32(startBlockIdx),
		blockSize: blockSize,
		parity:    make([]byte, 0, blockSize),
		active:    true,
	}
	for i := 0; i < groupDataLen; i++ {
		g.blocks = append(g.blocks, make([]byte, 0, blockSize))
	}
	return g
}

func (g *group) IsGroup(idx uint32) bool {
	return g.startIdx < idx && idx < g.startIdx+uint32(g.groupDataLen)
}

func (g *group) PutBlock(idx uint32, b []byte) (ok bool) {
	i := idx - g.startIdx

	copy(g.blocks[i], b)

	g.putLen++
	return g.putLen >= g.groupDataLen-1
}

func (g *group) Active() bool {
	return g.active
}

func (g *group) Reset() {}

func (g *group) GetData() []byte {
	return nil
}

type groups struct {
	list []*group
}

func newGroups() *groups {
	return &groups{}
}

func (g *groups) Len() int { return len(g.list) }
func (g *groups) Less(i, j int) bool {
	return (!g.list[i].Active() && g.list[j].Active()) ||
		(g.list[i].Active() && g.list[j].Active() && g.list[i].startIdx < g.list[j].startIdx)
}
func (g *groups) Swap(i, j int) { g.list[i], g.list[j] = g.list[j], g.list[i] }

func (g *groups) getGroup(blockSize, groupDataLen int, idx uint32) *group {
	for _, gu := range g.list {
		if !gu.Active() &&
			gu.blockSize == blockSize &&
			gu.groupDataLen == groupDataLen {

			gu.active = true
			gu.startIdx = idx
			return gu
		}
	}

	gp := newGroup(blockSize, groupDataLen, idx)
	g.list = append(g.list, gp)

	sort.Sort(g)
	return gp
}

func (g *groups) PutPacket(p fudpPack) {
	if p.IsFecStart() {
		gu := g.getGroup(p.Mtu(), p.FecDataLen(), p.BlockIdx())
		gu.PutBlock(p.BlockIdx(), p.Payload())
	}

	idx := p.BlockIdx()
	for i := range g.list {
		if g.list[i].IsGroup(idx) {
			g.list[i].PutBlock(idx, p.Payload())
			return
		}
	}
}

func (g *groups) GetData() []byte {
	for _, g := range g.list {
		if g.Active() {
			return g.GetData()
		}
	}
	return nil
}
