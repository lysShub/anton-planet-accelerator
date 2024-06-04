package proto

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"strconv"
	"syscall"

	"github.com/lysShub/netkit/packet"
	"github.com/pkg/errors"
)

type Header struct {
	Server netip.Addr
	Proto  uint8
	ID     ID
	Kind   Kind
}

func (h *Header) Valid() bool {
	return h != nil && h.Server.Is4() &&
		(h.Proto == syscall.IPPROTO_UDP || h.Proto == syscall.IPPROTO_TCP) &&
		h.Kind.Valid()
}
func (h Header) String() string {
	return fmt.Sprintf(
		"{Server:%s, Proto:%d, ID:%d, Kind:%s}",
		h.Server.String(), h.Proto, h.ID, h.Kind.String(),
	)
}

type ID uint16

//go:generate stringer -output proto_gen.go -type=Kind
type Kind uint8

func (k Kind) Valid() bool {
	return _kind_start < k && k < _kind_end
}

const (
	HeaderSize = 8

	_kind_start Kind = iota
	Data
	PingProxyer
	PingForward
	PacketLossProxyer
	PacketLossForward
	_kind_end
)

func (h *Header) Encode(to *packet.Packet) error {
	if !h.Valid() {
		return errors.Errorf("invalid header %#v", h)
	}

	to.Attach(h.Server.AsSlice()...)
	to.Attach(h.Proto)
	to.Attach(binary.BigEndian.AppendUint16(nil, uint16(h.ID))...)
	to.Attach(byte(h.Kind))
	return nil
}

func (h *Header) Decode(from *packet.Packet) error {
	b := from.Bytes()
	if len(b) < HeaderSize {
		return errors.Errorf("packet too short %d", len(b))
	}

	h.Kind = Kind(b[0])
	h.ID = ID(binary.BigEndian.Uint16(b[1:3]))
	h.Proto = b[3]
	h.Server = netip.AddrFrom4([4]byte(b[4:8]))
	if !h.Valid() {
		return errors.Errorf("invalid header %#v", h)
	}

	from.DetachN(HeaderSize)
	return nil
}

type PL float64

func (p PL) Encode() (to []byte) {
	if err := p.Valid(); err != nil {
		panic(err)
	}
	return strconv.AppendFloat(nil, float64(p), 'f', 3, 64)
}
func (p *PL) Decode(from []byte) (err error) {
	v, err := strconv.ParseFloat(string(from), 64)
	if err != nil {
		return errors.WithStack(err)
	}
	*p = PL(v)
	return p.Valid()
}
func (p PL) Valid() error {
	if p < 0 || 1 < p {
		return errors.New("invalid pack loss")
	}
	return nil
}
