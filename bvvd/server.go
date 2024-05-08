package bvvd

import (
	"net/netip"

	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func source(ip header.IPv4) (netip.AddrPort, uint8) {
	addr := netip.AddrFrom4(ip.SourceAddress().As4())

	proto := ip.TransportProtocol()
	switch proto {
	case header.TCPProtocolNumber:
		tcp := header.TCP(ip.Payload())
		return netip.AddrPortFrom(addr, tcp.SourcePort()), uint8(proto)
	case header.UDPProtocolNumber:
		udp := header.UDP(ip.Payload())
		return netip.AddrPortFrom(addr, udp.SourcePort()), uint8(proto)
	case header.ICMPv4ProtocolNumber:
		return netip.AddrPortFrom(addr, 0), uint8(proto)
	default:
		panic("")
	}
}
