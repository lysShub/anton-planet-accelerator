package bvvd

//go:generate stringer -output bvvd_gen.go -type=Kind

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
func (b Bvvd) ForwardID() ForwardID {
	return ForwardID(b[3])
}
func (b Bvvd) SetForwardID(loc ForwardID) {
	b[3] = byte(loc)
}
func (b Bvvd) Client() netip.AddrPort {
	return netip.AddrPortFrom(
		netip.AddrFrom4([4]byte(b[6:])),
		uint16(b[4])+uint16(b[5])<<8,
	)
}
func (b Bvvd) SetClient(client netip.AddrPort) {
	if !client.Addr().Is4() {
		panic("only support ipv4")
	}
	copy(b[6:], client.Addr().AsSlice())
	b[4] = byte(client.Port())
	b[5] = byte(client.Port() >> 8)
}
func (b Bvvd) Server() netip.Addr {
	return netip.AddrFrom4([4]byte(b[10:]))
}
func (b Bvvd) SetServer(server netip.Addr) {
	if !server.Is4() {
		panic("only support ipv4")
	}
	copy(b[10:], server.AsSlice())
}

type Fields struct {
	Kind      Kind           // kind
	Proto     uint8          // opt, tcp or udp
	DataID    uint8          // opt, use for PL statistics
	ForwardID ForwardID      // opt, forward id
	Client    netip.AddrPort // opt, client addr, set by gateway
	Server    netip.Addr     // opt, destination ip
}

const MaxID = 0xff
const Size = 14

func (h Fields) Valid() error {
	switch h.Kind {
	case Data:
		if h.Proto != syscall.IPPROTO_TCP && h.Proto != syscall.IPPROTO_UDP {
			return errors.Errorf("proto %d", h.Proto)
		}
		if err := h.ForwardID.Valid(); err != nil {
			return err
		}
		if !h.Server.IsValid() {
			return errors.Errorf("server invalid")
		}
	case PingGateway:
	case PingForward:
		if err := h.ForwardID.Valid(); err != nil {
			return err
		}
	case PingServer:
		if err := h.ForwardID.Valid(); err != nil {
			return err
		}
		if !h.Server.IsValid() || h.Server.IsUnspecified() {
			return errors.Errorf("server %s", h.Server.String())
		}
	case PackLossClientUplink:
	case PackLossGatewayUplink:
	case PackLossGatewayDownlink:
	default:
		return h.Kind.Valid()
	}

	return nil
}

func (h Fields) String() string {
	return fmt.Sprintf(
		"{Server:%s, Client:%s, Proto:%d,  Kind:%s}",
		h.Server.String(), h.Client.String(), h.Proto, h.Kind.String(),
	)
}

func (h *Fields) Encode(to *packet.Packet) error {
	if err := h.Valid(); err != nil {
		return err
	}

	if h.Server.IsValid() {
		to.Attach(h.Server.AsSlice()...)
	} else {
		to.Attach(0, 0, 0, 0)
	}
	if h.Client.Addr().IsValid() {
		to.Attach(h.Client.Addr().AsSlice()...)
	} else {
		to.Attach(0, 0, 0, 0)
	}
	to.Attach(byte(h.Client.Port()), byte(h.Client.Port()>>8))
	to.Attach(byte(h.ForwardID))
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
	h.ForwardID = ForwardID(b[3])
	h.Client = netip.AddrPortFrom(
		netip.AddrFrom4([4]byte(b[6:])),
		uint16(b[4])+uint16(b[5])<<8,
	)
	h.Server = netip.AddrFrom4([4]byte(b[10:]))

	from.DetachN(Size)
	return h.Valid()
}

type Kind uint8

func (k Kind) Valid() error {
	if 0 < k && k < _kind_end {
		return nil
	}
	return errors.Errorf("kind %s", k.String())
}

const (
	_ Kind = iota

	// payload transmit data
	// 必须要设置ForwardID
	Data

	// rtt client  <--> gateway
	PingGateway

	// rtt client  <--> forward
	PingForward

	// rtt client <--> server
	PingServer

	// pl  gateway ---> forward
	PackLossGatewayUplink

	// pl  forward ---> gateway
	PackLossGatewayDownlink

	// pl  client  ---> gateway
	PackLossClientUplink

	_kind_end
)

type ForwardID uint8

func (f ForwardID) Valid() error {
	if f.valid() {
		return errors.Errorf("forward id %d", f)
	}
	return nil
}

func (f ForwardID) valid() bool { return f != 0 }
