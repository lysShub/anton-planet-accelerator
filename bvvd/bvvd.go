package bvvd

//go:generate stringer -output bvvd_gen.go -type=Kind

import (
	"fmt"
	"net/netip"

	"github.com/lysShub/netkit/packet"
	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Bvvd []byte

func (b Bvvd) Kind() Kind {
	return kindproto(b[0]).kind()
}
func (b Bvvd) SetKind(kind Kind) {
	e := kindproto(b[0])
	e.SetKind(kind)
	b[0] = byte(e)
}

func (b Bvvd) Proto() tcpip.TransportProtocolNumber {
	return kindproto(b[0]).Proto()
}
func (b Bvvd) SetProto(proto tcpip.TransportProtocolNumber) {
	e := kindproto(b[0])
	e.SetProto(proto)
	b[0] = byte(e)
}

func (b Bvvd) DataID() uint8 {
	return b[1]
}
func (b Bvvd) SetDataID(id uint8) {
	b[1] = id
}

func (b Bvvd) Client() netip.AddrPort {
	return netip.AddrPortFrom(
		netip.AddrFrom4([4]byte(b[2:])),
		uint16(b[6])+uint16(b[7])<<8,
	)
}
func (b Bvvd) SetClient(caddr netip.AddrPort) {
	if !caddr.Addr().Is4() {
		panic("only support ipv4")
	}
	copy(b[2:], caddr.Addr().AsSlice())
	b[6] = byte(caddr.Port())
	b[7] = byte(caddr.Port() >> 8)
}

func (b Bvvd) Forward() netip.AddrPort {
	return netip.AddrPortFrom(
		netip.AddrFrom4([4]byte(b[8:])),
		uint16(b[12])+uint16(b[13])<<8,
	)
}
func (b Bvvd) SetForward(faddr netip.AddrPort) {
	if !faddr.Addr().Is4() {
		panic("only support ipv4")
	}
	copy(b[8:], faddr.Addr().AsSlice())
	b[12] = byte(faddr.Port())
	b[13] = byte(faddr.Port() >> 8)
}

func (b Bvvd) Server() netip.Addr {
	return netip.AddrFrom4([4]byte(b[14:]))
}
func (b Bvvd) SetServer(server netip.Addr) {
	if !server.Is4() {
		panic("only support ipv4")
	}
	copy(b[14:], server.AsSlice())
}

type Fields struct {
	Kind    Kind                          // kind
	Proto   tcpip.TransportProtocolNumber // opt, tcp or udp
	DataID  uint8                         // opt, use for PL statistics
	Client  netip.AddrPort                // opt, client addr, set by gateway
	Forward netip.AddrPort                // opt, forward
	Server  netip.Addr                    // opt, destination ip
}

const MaxID = 0xff
const Size = 18

func (h Fields) Valid() error {
	switch h.Kind {
	case Data:
		if h.Proto != header.TCPProtocolNumber && h.Proto != header.UDPProtocolNumber {
			return errors.Errorf("proto %d", h.Proto)
		}
		if !h.Forward.IsValid() {
			return errors.New("forward invalid")
		}
		if !h.Server.IsValid() {
			return errors.Errorf("server invalid")
		}
	case PingGateway:
	case PingForward:
	case PingServer:
		if !h.Forward.IsValid() {
			return errors.New("forward invalid")
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

	to.Attach(byte(h.Forward.Port()), byte(h.Forward.Port()>>8))
	if h.Forward.IsValid() {
		to.Attach(h.Forward.Addr().AsSlice()...)
	} else {
		to.Attach(0, 0, 0, 0)
	}

	to.Attach(byte(h.Client.Port()), byte(h.Client.Port()>>8))
	if h.Client.Addr().IsValid() {
		to.Attach(h.Client.Addr().AsSlice()...)
	} else {
		to.Attach(0, 0, 0, 0)
	}

	to.Attach(h.DataID)

	var e kindproto
	e.SetKind(h.Kind)
	e.SetProto(h.Proto)
	to.Attach(byte(e))
	return nil
}

func (h *Fields) Decode(from *packet.Packet) error {
	b := from.Bytes()
	if len(b) < Size {
		return errors.Errorf("too short %d", len(b))
	}

	h.Kind = kindproto(b[0]).kind()
	h.Proto = kindproto(b[0]).Proto()
	h.DataID = b[1]
	h.Client = netip.AddrPortFrom(
		netip.AddrFrom4([4]byte(b[2:])),
		uint16(b[6])+uint16(b[7])<<8,
	)
	h.Forward = netip.AddrPortFrom(
		netip.AddrFrom4([4]byte(b[8:])),
		uint16(b[12])+uint16(b[13])<<8,
	)
	h.Server = netip.AddrFrom4([4]byte(b[14:]))

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

type kindproto byte

func (b kindproto) kind() Kind {
	return Kind(b & 0b00001111)
}
func (b *kindproto) SetKind(k Kind) {
	(*b) = (*b)&0b11110000 + (kindproto(k) & 0b00001111)
}
func (b kindproto) Proto() tcpip.TransportProtocolNumber {
	switch b >> 4 {
	case 1:
		return header.TCPProtocolNumber
	case 2:
		return header.UDPProtocolNumber
	default:
		return 0
	}
}
func (b *kindproto) SetProto(p tcpip.TransportProtocolNumber) {
	var e kindproto
	switch p {
	case header.TCPProtocolNumber:
		e = 1 << 4
	case header.UDPProtocolNumber:
		e = 2 << 4
	default:
	}
	(*b) = e + (*b)&0b00001111
}
