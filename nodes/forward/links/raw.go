//go:build linux
// +build linux

package links

import (
	"io"
	"net"
	"syscall"

	"github.com/lysShub/anton-planet-accelerator/conn/tcp"
	"golang.org/x/net/bpf"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type udpLister struct {
	*net.UDPConn
}

var _ listener = (*udpLister)(nil)

func wrapUDPLister(conn *net.UDPConn, err error) (listener, error) {
	return &udpLister{UDPConn: conn}, err
}

func (u udpLister) Addr() net.Addr {
	return u.UDPConn.LocalAddr()
}

type listener interface {
	Addr() net.Addr
	raw
	io.Closer
}

type raw interface {
	SyscallConn() (syscall.RawConn, error)
}

func bpfFilterAll(raw raw) error {
	return tcp.SetRawBPF(raw, []bpf.Instruction{bpf.RetConstant{Val: 0}})
}

func bpfFilterPort(raw raw, srcPort, dstPort uint16) error {
	const SrcPortOffset = header.TCPSrcPortOffset // tcp/udp is same
	const DstPortOffset = header.TCPDstPortOffset

	var ins = []bpf.Instruction{
		// store IPv4HdrLen regX
		bpf.LoadMemShift{Off: 0},

		bpf.LoadIndirect{Off: SrcPortOffset, Size: 2},
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: uint32(srcPort), SkipTrue: 1},
		bpf.RetConstant{Val: 0},

		bpf.LoadIndirect{Off: DstPortOffset, Size: 2},
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: uint32(dstPort), SkipTrue: 1},
		bpf.RetConstant{Val: 0},

		bpf.RetConstant{Val: 0xffff},
	}

	return tcp.SetRawBPF(raw, ins)
}
