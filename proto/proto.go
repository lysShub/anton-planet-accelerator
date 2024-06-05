package proto

import (
	"fmt"
	"net/netip"
	"strconv"
	"syscall"

	"github.com/lysShub/netkit/packet"
	"github.com/pkg/errors"
)

type Header struct {
	Kind   Kind
	Proto  uint8
	Client netip.Addr
	Server netip.Addr
}

func (h *Header) Valid() bool {
	return h != nil && h.Server.Is4() && h.Client.Is4() &&
		(h.Proto == syscall.IPPROTO_UDP || h.Proto == syscall.IPPROTO_TCP) &&
		h.Kind.Valid()
}
func (h Header) String() string {
	return fmt.Sprintf(
		"{Server:%s, Client:%s, Proto:%d,  Kind:%s}",
		h.Server.String(), h.Client.String(), h.Proto, h.Kind.String(),
	)
}

//go:generate stringer -output proto_gen.go -type=Kind
type Kind uint8

func (k Kind) Valid() bool {
	return _kind_start < k && k < _kind_end
}

const (
	HeaderSize = 10

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
	to.Attach(h.Client.AsSlice()...)
	to.Attach(h.Proto)
	to.Attach(byte(h.Kind))
	return nil
}

func (h *Header) Decode(from *packet.Packet) error {
	b := from.Bytes()
	if len(b) < HeaderSize {
		return errors.Errorf("too short %d", len(b))
	}

	h.Kind = Kind(b[0])
	h.Proto = b[1]
	h.Client = netip.AddrFrom4([4]byte(b[2:6]))
	h.Server = netip.AddrFrom4([4]byte(b[6:10]))
	if !h.Valid() {
		return errors.Errorf("invalid header %s", h.String())
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
