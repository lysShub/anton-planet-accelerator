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
	ID     uint8 // Data id
	Client netip.AddrPort
	Server netip.Addr
}

const MaxID = 0xff

func (h *Header) Valid() bool {
	return h != nil && h.Server.Is4() && h.Client.Addr().Is4() &&
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
	HeaderSize = 13

	_kind_start Kind = iota
	Data
	PingProxyer    // client-proxyer   之间的rtt
	PingForward    // proxyer-forward 之间的rtt
	PackLossUplink // client-proxyer  之间的丢包
	_kind_end
)

func (h *Header) Encode(to *packet.Packet) error {
	if !h.Valid() {
		return errors.Errorf("invalid header %#v", h)
	}

	to.Attach(h.Server.AsSlice()...)
	to.Attach(h.Client.Addr().AsSlice()...)
	to.Attach(byte(h.Client.Port()), byte(h.Client.Port()>>8))
	to.Attach(h.ID)
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
	h.ID = b[2]
	h.Client = netip.AddrPortFrom(
		netip.AddrFrom4([4]byte(b[5:9])),
		uint16(b[3])+uint16(b[4])<<8,
	)
	h.Server = netip.AddrFrom4([4]byte(b[9:13]))
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
func (p PL) String() string {
	if p < 0.0001 {
		return "00.0"
	}

	v := float64(p * 100)
	v1 := int(v)
	v2 := int((v - float64(v1)) * 10)

	return fmt.Sprintf("%02d.%d", v1, v2)
}
