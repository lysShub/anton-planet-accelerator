package nodes

import (
	"fmt"
	"net/netip"
	"syscall"

	"github.com/lysShub/netkit/packet"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

/*
	约定一种传输层checksum计算方法, 为减少server的计算压力：
	uplink:
		client 使用传输层checksum的标准计算方法, 只是将src-port, PseudoHeader中的src-ip视为0。
		server 则可以根据client的计算约定, 快速求出实际的checksum。

	downlink:
		server 不计算checksum, 在client重新计算。
*/

func ChecksumClient(pkt *packet.Packet, proto tcpip.TransportProtocolNumber, dst netip.Addr) {
	var t header.Transport
	switch proto {
	case header.TCPProtocolNumber:
		t = header.TCP(pkt.Bytes())
	case header.UDPProtocolNumber:
		t = header.UDP(pkt.Bytes())
	default:
		panic(fmt.Sprintf("not support protocole %d", proto))
	}

	srcPort := t.SourcePort()
	t.SetSourcePort(0)
	t.SetChecksum(0)
	sum := header.PseudoHeaderChecksum(
		proto,
		ip4zero,
		tcpip.AddrFrom4(dst.As4()),
		uint16(pkt.Data()),
	)
	t.SetChecksum(^checksum.Checksum(pkt.Bytes(), sum))
	t.SetSourcePort(srcPort)
}

var ip4zero = tcpip.AddrFrom4([4]byte{})

func ChecksumForward(pkt *packet.Packet, proto uint8, loc netip.AddrPort) {
	sum := checksum.Checksum(loc.Addr().AsSlice(), loc.Port())

	var t header.Transport
	switch proto {
	case syscall.IPPROTO_TCP:
		t = header.TCP(pkt.Bytes())
	case syscall.IPPROTO_UDP:
		t = header.UDP(pkt.Bytes())
	default:
		panic(fmt.Sprintf("not support protocole %d", proto))
	}
	t.SetChecksum(^checksum.Combine(sum, ^t.Checksum()))
	t.SetSourcePort(loc.Port())
}
