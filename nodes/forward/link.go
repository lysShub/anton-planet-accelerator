//go:build linux
// +build linux

package forward

import (
	"io"
	"net"
	"net/netip"
	"syscall"

	"github.com/lysShub/anton-planet-accelerator/nodes"
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/packet"
	rawbpf "github.com/lysShub/rawsock/helper/bpf"
	"github.com/lysShub/rawsock/test"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/bpf"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Link struct {
	raw *net.IPConn
	l   listener

	link    link
	proxyer netip.AddrPort
	laddr   netip.AddrPort

	closeErr errorx.CloseErr
}

func NewLink(link link, proxyer netip.AddrPort) (*Link, error) {
	var (
		r = &Link{
			link:    link,
			proxyer: proxyer,
		}
		err error
	)

	switch link.header.Proto {
	case syscall.IPPROTO_TCP:
		r.l, err = net.ListenTCP("tcp4", nil)
	case syscall.IPPROTO_UDP:
		r.l, err = wrapUDPLister(net.ListenUDP("udp4", nil))
	default:
		panic("")
	}
	if err != nil {
		return nil, r.close(errors.WithStack(err))
	}
	if err := bpfFilterAll(r.l); err != nil {
		return nil, r.close(err)
	}
	locPort := netip.MustParseAddrPort(r.l.Addr().String()).Port()

	network := "ip4:" + r.LocalAddr().Network()
	r.raw, err = net.DialIP(network, nil, &net.IPAddr{IP: link.header.Server.AsSlice()})
	if err != nil {
		return nil, r.close(errors.WithStack(err))
	}
	r.laddr = netip.AddrPortFrom(netip.MustParseAddr(r.raw.LocalAddr().String()), locPort)
	if err := bpfFilterPort(r.raw, r.link.serverPort, locPort); err != nil {
		return nil, r.close(err)
	}
	return r, nil
}

func (t *Link) close(cause error) error {
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
func (r *Link) Recv(pkt *packet.Packet) error {
	n, _, err := r.raw.ReadFrom(pkt.Bytes())
	if err != nil {
		return r.close(err)
	}
	pkt.SetData(n)

	hdr := header.TCP(pkt.Bytes())
	if debug.Debug() {
		require.Equal(test.T(), r.laddr.Port(), hdr.DestinationPort())
	}
	hdr.SetDestinationPort(r.link.processPort)

	r.link.header.Encode(pkt)
	return nil
}
func (r *Link) Send(pkt *packet.Packet) error {
	nodes.ChecksumForward(pkt, r.link.header.Proto, r.laddr)
	if debug.Debug() {
		// todo
	}

	_, err := r.raw.Write(pkt.Bytes())
	return errors.WithStack(err)
}
func (r *Link) Link() link              { return r.link }
func (r *Link) Proxyer() netip.AddrPort { return r.proxyer }
func (r *Link) LocalAddr() net.Addr     { return r.l.Addr() }
func (r *Link) Close() error            { return r.close(nil) }

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
	rawconn, err := raw.SyscallConn()
	if err != nil {
		return err
	}
	return rawbpf.SetRawBPF(rawconn, ins)
}
