package bvvd

import (
	"fmt"
	"net/netip"
	"syscall"

	"github.com/lysShub/netkit/packet"
	"github.com/pkg/errors"
)

type Bvvd []byte

func (b Bvvd) Kind() Kind {
	return Kind(b[0])
}
func (b Bvvd) SetKind(kind Kind) {
	b[0] = byte(kind)
}
func (b Bvvd) Proto() uint8 {
	return b[1]
}
func (b Bvvd) SetProto(proto uint8) {
	b[1] = proto
}
func (b Bvvd) DataID() uint8 {
	return b[2]
}
func (b Bvvd) SetDataID(id uint8) {
	b[2] = id
}
func (b Bvvd) Client() netip.AddrPort {
	return netip.AddrPortFrom(
		netip.AddrFrom4([4]byte(b[5:])),
		uint16(b[3])+uint16(b[4])<<8,
	)
}
func (b Bvvd) SetClient(client netip.AddrPort) {
	if !client.Addr().Is4() {
		panic("only support ipv4")
	}
	copy(b[5:], client.Addr().AsSlice())
	b[3] = byte(client.Port())
	b[4] = byte(client.Port() >> 8)
}
func (b Bvvd) Server() netip.Addr {
	return netip.AddrFrom4([4]byte(b[9:]))
}
func (b Bvvd) SetServer(server netip.Addr) {
	if !server.Is4() {
		panic("only support ipv4")
	}
	copy(b[9:], server.AsSlice())
}

type Fields struct {
	Kind   Kind
	Proto  uint8
	DataID uint8
	Client netip.AddrPort
	Server netip.Addr
}

const MaxID = 0xff

func (h *Fields) Valid() bool {
	return h != nil && h.Server.Is4() && h.Client.Addr().Is4() &&
		(h.Proto == syscall.IPPROTO_UDP || h.Proto == syscall.IPPROTO_TCP) &&
		h.Kind.Valid()
}
func (h Fields) String() string {
	return fmt.Sprintf(
		"{Server:%s, Client:%s, Proto:%d,  Kind:%s}",
		h.Server.String(), h.Client.String(), h.Proto, h.Kind.String(),
	)
}

func (h *Fields) Encode(to *packet.Packet) error {
	if !h.Valid() {
		return errors.Errorf("invalid header %#v", h)
	}

	to.Attach(h.Server.AsSlice()...)
	to.Attach(h.Client.Addr().AsSlice()...)
	to.Attach(byte(h.Client.Port()), byte(h.Client.Port()>>8))
	to.Attach(h.DataID)
	to.Attach(h.Proto)
	to.Attach(byte(h.Kind))
	return nil
}

func (h *Fields) Decode(from *packet.Packet) error {
	b := from.Bytes()
	if len(b) < Size {
		return errors.Errorf("too short %d", len(b))
	}

	h.Kind = Kind(b[0])
	h.Proto = b[1]
	h.DataID = b[2]
	h.Client = netip.AddrPortFrom(
		netip.AddrFrom4([4]byte(b[5:9])),
		uint16(b[3])+uint16(b[4])<<8,
	)
	h.Server = netip.AddrFrom4([4]byte(b[9:13]))
	if !h.Valid() {
		return errors.Errorf("invalid header %s", h.String())
	}

	from.DetachN(Size)
	return nil
}

//go:generate stringer -output bvvd_gen.go -type=Kind
type Kind uint8

func (k Kind) Valid() bool {
	return _kind_start < k && k < _kind_end
}

const (
	Size = 13

	_kind_start Kind = iota
	Data
	PingProxyer             // rtt client  <--> proxyer
	PingForward             // rtt client  <--> forward
	PackLossClientUplink    // pl  client  ---> proxyer
	PackLossProxyerUplink   // pl  proxyer ---> forward
	PackLossProxyerDownlink // pl  forward ---> proxyer
	_kind_end
)
