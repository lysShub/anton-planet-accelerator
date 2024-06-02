package proto

import (
	"encoding/binary"
	"net/netip"
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

type ID uint16
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
	PlProxyer
	PlForward
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
