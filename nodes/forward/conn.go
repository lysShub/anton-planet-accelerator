//go:build linux
// +build linux

package forward

import (
	"fmt"
	"io"
	"net"
	"net/netip"
	"syscall"
	"unsafe"

	"github.com/lysShub/anton-planet-accelerator/nodes"
	"github.com/lysShub/anton-planet-accelerator/proto"
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/rawsock/test"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/bpf"
	"golang.org/x/sys/unix"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Raw struct {
	raw       *net.IPConn
	l         listener
	transport func(*packet.Packet) header.Transport

	header                  proto.Header
	proxyer                 netip.AddrPort
	laddr                   netip.AddrPort
	processPort, serverPort uint16

	closeErr errorx.CloseErr
}

func NewRaw(hdr proto.Header, proxyer netip.AddrPort, firsPacket *packet.Packet) (*Raw, error) {
	var (
		r   = &Raw{}
		err error
	)

	switch hdr.Proto {
	case syscall.IPPROTO_TCP:
		r.l, err = net.ListenTCP("tcp4", nil)
		r.transport = func(p *packet.Packet) header.Transport { return header.TCP(p.Bytes()) }
	case syscall.IPPROTO_UDP:
		r.l, err = wrapUDPLister(net.ListenUDP("udp4", nil))
		r.transport = func(p *packet.Packet) header.Transport { return header.UDP(p.Bytes()) }
	default:
		panic("")
	}
	if err != nil {
		return nil, r.close(errors.WithStack(err))
	}
	if err := bpfFilterAll(r.l); err != nil {
		return nil, r.close(err)
	}
	r.header = hdr
	r.proxyer = proxyer
	r.laddr = netip.MustParseAddrPort(r.l.Addr().String())
	r.processPort = r.transport(firsPacket).SourcePort()
	r.serverPort = r.transport(firsPacket).DestinationPort()

	network := "ip4:" + r.LocalAddr().Network()
	r.raw, err = net.DialIP(network, nil, &net.IPAddr{IP: hdr.Server.AsSlice()})
	if err != nil {
		return nil, r.close(errors.WithStack(err))
	}
	r.laddr = netip.AddrPortFrom(netip.MustParseAddr(r.raw.LocalAddr().String()), r.laddr.Port())
	if err := bpfFilterPort(r.raw, r.serverPort, r.laddr.Port()); err != nil {
		return nil, r.close(err)
	}

	return r, nil
}

func (t *Raw) close(cause error) error {
	cause = errors.WithStack(cause)
	return t.closeErr.Close(func() (errs []error) {
		errs = append(errs, cause)
		if t.raw != nil {
			errs = append(errs, t.raw.Close())
		}
		if t.l != nil {
			errs = append(errs, t.l.Close())
		}
		return errs
	})
}
func (r *Raw) Recv(pkt *packet.Packet) error {
	n, _, err := r.raw.ReadFrom(pkt.Bytes())
	if err != nil {
		return r.close(err)
	}
	pkt.SetData(n)

	hdr := header.TCP(pkt.Bytes())
	if debug.Debug() {
		require.Equal(test.T(), r.laddr.Port(), hdr.SourcePort())
	}
	hdr.SetDestinationPort(r.processPort)

	r.header.Encode(pkt)
	return nil
}
func (r *Raw) Send(pkt *packet.Packet) error {
	fmt.Printf("recv %#v\n\n", pkt.Bytes())

	nodes.ChecksumForward(pkt, r.header.Proto, r.laddr)
	if debug.Debug() {
		// todo
	}

	_, err := r.raw.Write(pkt.Bytes())
	return errors.WithStack(err)
}
func (r *Raw) Header() proto.Header    { return r.header }
func (r *Raw) Proxyer() netip.AddrPort { return r.proxyer }
func (r *Raw) LocalAddr() net.Addr     { return r.l.Addr() }
func (r *Raw) RemoteAddrPort() netip.AddrPort {
	return netip.AddrPortFrom(r.header.Server, r.serverPort)
}
func (r *Raw) Close() error { return r.close(nil) }

type udpLister struct {
	*net.UDPConn
}

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
	return setBpf(raw, []bpf.Instruction{bpf.RetConstant{Val: 0}})
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
	return setBpf(raw, ins)
}

func setBpf(raw raw, ins []bpf.Instruction) error {
	var prog *unix.SockFprog
	if rawIns, err := bpf.Assemble(ins); err != nil {
		return errors.WithStack(err)
	} else {
		prog = &unix.SockFprog{
			Len:    uint16(len(rawIns)),
			Filter: (*unix.SockFilter)(unsafe.Pointer(&rawIns[0])),
		}
	}

	rawconn, err := raw.SyscallConn()
	if err != nil {
		return errors.WithStack(err)
	}

	var e error
	if err := rawconn.Control(func(fd uintptr) {
		e = unix.SetsockoptSockFprog(int(fd), unix.SOL_SOCKET, unix.SO_ATTACH_FILTER, prog)
	}); err != nil {
		return errors.WithStack(err)
	} else if e != nil {
		return errors.WithStack(e)
	}
	return nil
}