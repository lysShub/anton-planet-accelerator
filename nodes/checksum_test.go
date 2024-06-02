package nodes_test

import (
	"math/rand"
	"net/netip"
	"syscall"
	"testing"

	"github.com/lysShub/anton-planet-accelerator/nodes"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/rawsock/test"
	"gvisor.dev/gvisor/pkg/tcpip"
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
		pkt     = packet.Make(0, 20)
	)
	header.TCP(pkt.Bytes()).Encode(&header.TCPFields{
		SrcPort:    process.Port(),
		DstPort:    server.Port(),
		SeqNum:     rand.Uint32(),
		AckNum:     rand.Uint32(),
		WindowSize: uint16(rand.Uint32()),
	})

	nodes.ChecksumClient(pkt, header.TCPProtocolNumber, server.Addr())
	nodes.ChecksumForward(pkt, syscall.IPPROTO_TCP, local)

	test.ValidTCP(t, pkt.Bytes(), header.PseudoHeaderChecksum(
		header.TCPProtocolNumber,
		tcpip.AddrFrom4(local.Addr().As4()),
		tcpip.AddrFrom4(server.Addr().As4()),
		0,
	))

}
