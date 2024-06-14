package conn

import (
	"net/netip"

	"github.com/lysShub/netkit/packet"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

// todo: state machine
func attachTcpHdr(b *packet.Packet, src, dst netip.AddrPort) {
	hdr := header.TCP(b.AttachN(header.TCPMinimumSize).Bytes())
	hdr.Encode(&header.TCPFields{
		SrcPort:    src.Port(),
		DstPort:    dst.Port(),
		SeqNum:     0,
		AckNum:     0,
		DataOffset: header.TCPMinimumSize,
		Flags:      header.TCPFlagSyn | header.TCPFlagPsh,
		WindowSize: 0xffff,
		Checksum:   0,
	})

	sum := header.PseudoHeaderChecksum(
		header.TCPProtocolNumber,
		tcpip.AddrFrom4(src.Addr().As4()),
		tcpip.AddrFrom4(dst.Addr().As4()),
		uint16(len(hdr)),
	)
	sum = checksum.Checksum(hdr, sum)
	hdr.SetChecksum(^sum)
}

func attachIPv4Hdr(b *packet.Packet, src, dst netip.Addr) {
	ip := header.IPv4(b.AttachN(header.IPv4MinimumSize).Bytes())
	ip.Encode(&header.IPv4Fields{
		TOS:            0,
		TotalLength:    uint16(len(ip)),
		ID:             0,
		Flags:          0,
		FragmentOffset: 0,
		TTL:            64,
		Protocol:       uint8(header.TCPProtocolNumber),
		Checksum:       0,
		SrcAddr:        tcpip.AddrFrom4(src.As4()),
		DstAddr:        tcpip.AddrFrom4(dst.As4()),
		Options:        nil,
	})
	ip.SetChecksum(^ip.CalculateChecksum())
}
