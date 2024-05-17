package common_test

import (
	"math/rand"
	"net/netip"
	"testing"

	"github.com/lysShub/anton-planet-accelerator/common"
	"github.com/lysShub/fatcp/links"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/rawsock/test"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func Test_Checksum(t *testing.T) {

	var randAddr = func() netip.AddrPort {
		return netip.AddrPortFrom(test.RandIP(), test.RandPort())
	}
	var (
		process = randAddr()
		local   = randAddr()
		server  = randAddr()
		link    = links.Downlink{Server: server, Proto: header.TCPProtocolNumber, Local: local}
		raw     = BuildRawTCP(t, process, server, []byte("hello"))

		pkt = packet.Make(20, 0, len(raw)).Append(raw)
		tcp = common.ChecksumClient(pkt)
		ip  = common.ChecksumServer(tcp, link)
	)

	{
		hdr := header.IPv4(ip.Bytes())
		// require.Equal(t, local.Addr().As4(), hdr.SourceAddress().As4())
		require.Equal(t, server.Addr().As4(), hdr.DestinationAddress().As4())

		tcp := header.TCP(hdr[hdr.HeaderLength():])
		require.Equal(t, local.Port(), tcp.SourcePort())
		require.Equal(t, server.Port(), tcp.DestinationPort())
		psum1 := header.PseudoHeaderChecksum(
			hdr.TransportProtocol(),
			tcpip.AddrFrom4(local.Addr().As4()),
			tcpip.AddrFrom4(server.Addr().As4()),
			0,
		)
		test.ValidTCP(t, tcp, psum1)
	}
}

func BuildRawTCP(t require.TestingT, laddr, raddr netip.AddrPort, tcpPayload []byte) header.IPv4 {
	var ip = make(header.IPv4, header.IPv4MinimumSize+header.TCPMinimumSize+len(tcpPayload))
	ip.Encode(&header.IPv4Fields{
		TOS:            0,
		TotalLength:    uint16(len(ip)),
		ID:             uint16(rand.Uint32()),
		Flags:          0,
		FragmentOffset: 0,
		TTL:            64,
		Protocol:       uint8(header.TCPProtocolNumber),
		Checksum:       0,
		SrcAddr:        tcpip.AddrFrom4(laddr.Addr().As4()),
		DstAddr:        tcpip.AddrFrom4(raddr.Addr().As4()),
		Options:        nil,
	})
	ip.SetChecksum(^checksum.Checksum(ip[:header.IPv4MinimumSize], 0))

	tcp := header.TCP(ip.Payload())
	tcp.Encode(&header.TCPFields{
		SrcPort:    uint16(laddr.Port()),
		DstPort:    uint16(raddr.Port()),
		SeqNum:     rand.Uint32(),
		AckNum:     rand.Uint32(),
		DataOffset: header.TCPMinimumSize,
		Flags:      header.TCPFlagSyn,
		WindowSize: uint16(rand.Uint32()),
		Checksum:   0,
	})
	copy(tcp.Payload(), tcpPayload)

	sum := header.PseudoHeaderChecksum(
		header.TCPProtocolNumber,
		test.Address(laddr.Addr()), test.Address(raddr.Addr()),
		uint16(len(tcp)),
	)
	sum = checksum.Checksum(tcp, sum)
	tcp.SetChecksum(^sum)

	test.ValidIP(t, ip)
	return ip
}
