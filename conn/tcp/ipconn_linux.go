//go:build linux
// +build linux

package tcp

import (
	"net"
	"net/netip"
	"syscall"
	"unsafe"

	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/rawsock/test"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/bpf"
	"golang.org/x/sys/unix"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type ipConn struct {
	laddr    netip.AddrPort
	tcp      *net.TCPListener
	raw      *net.IPConn
	closeErr errorx.CloseErr
}

func BindIPConn(laddr netip.AddrPort, proto tcpip.TransportProtocolNumber) (IPConn, error) {
	if !laddr.Addr().Is4() {
		return nil, errors.Errorf("only support ipv4")
	} else if proto != header.TCPProtocolNumber {
		return nil, errors.New("only support tcp")
	}
	var c = &ipConn{}
	var err error

	c.tcp, c.laddr, err = listenLocal(laddr)
	if err != nil {
		return nil, c.close(err)
	}

	c.raw, err = net.ListenIP("ip4:tcp", &net.IPAddr{IP: c.laddr.Addr().AsSlice()})
	if err != nil {
		return nil, c.close(err)
	}
	if err := SetRawBPF(c.raw, []bpf.Instruction{
		// load ipv4 header length to regX
		bpf.LoadMemShift{Off: 0},

		// filter dst port
		bpf.LoadIndirect{Off: 2, Size: 2},
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: uint32(c.laddr.Port()), SkipTrue: 1},
		bpf.RetConstant{Val: 0},
		bpf.RetConstant{Val: 0xffff},
	}); err != nil {
		return nil, c.close(err)
	}

	return c, nil
}

func (c *ipConn) ReadFromAddr(b *packet.Packet) (netip.Addr, error) {
	n, err := c.raw.Read(b.Bytes())
	if err != nil {
		return netip.Addr{}, errors.WithStack(err)
	}

	ip := header.IPv4(b.SetData(n).Bytes())
	if len(ip) != int(ip.TotalLength()) {
		return netip.Addr{}, errorx.ShortBuff(int(ip.TotalLength()), len(ip))
	}
	if debug.Debug() {
		require.Equal(test.T(), c.laddr.Addr().As4(), ip.DestinationAddress().As4())
		require.Equal(test.T(), uint8(header.TCPProtocolNumber), ip.Protocol())
	}

	b.DetachN(int(ip.HeaderLength()))
	if debug.Debug() {
		require.Equal(test.T(), c.laddr.Port(), header.TCP(b.Bytes()).DestinationPort())
	}
	return netip.AddrFrom4(ip.SourceAddress().As4()), nil
}

func (c *ipConn) WriteToAddr(b *packet.Packet, to netip.Addr) error {
	_, err := c.raw.WriteToIP(b.Bytes(), &net.IPAddr{IP: to.AsSlice()})
	return err
}

func (c *ipConn) AddrPort() netip.AddrPort { return c.laddr }
func (c *ipConn) Close() error             { return c.close(nil) }

func (c *ipConn) close(cause error) error {
	return c.closeErr.Close(func() (errs []error) {
		errs = append(errs, cause)
		if c.raw != nil {
			errs = append(errs, errors.WithStack(c.raw.Close()))
		}
		if c.tcp != nil {
			errs = append(errs, errors.WithStack(c.tcp.Close()))
		}
		return errs
	})
}

func SetRawBPF(
	conn interface {
		SyscallConn() (syscall.RawConn, error)
	},
	ins []bpf.Instruction,
) error {
	raw, err := conn.SyscallConn()
	if err != nil {
		return errors.WithStack(err)
	}

	var e error
	if err := raw.Control(func(fd uintptr) {
		e = SetBPF(fd, ins)
	}); err != nil {
		return err
	}
	return e
}

func SetBPF(fd uintptr, ins []bpf.Instruction) error {
	// drain buffered packet
	// https://natanyellin.com/posts/ebpf-filtering-done-right/
	err := setBPF(fd, []bpf.Instruction{bpf.RetConstant{Val: 0}})
	if err != nil {
		return err
	}
	var b = make([]byte, 1)
	for {
		n, _, _ := unix.Recvfrom(int(fd), b, unix.MSG_DONTWAIT)
		if n < 0 {
			break
		}
	}

	err = setBPF(fd, ins)
	return err
}

func setBPF(fd uintptr, ins []bpf.Instruction) error {
	var prog *unix.SockFprog
	if rawIns, err := bpf.Assemble(ins); err != nil {
		return err
	} else {
		prog = &unix.SockFprog{
			Len:    uint16(len(rawIns)),
			Filter: (*unix.SockFilter)(unsafe.Pointer(&rawIns[0])),
		}
	}

	err := unix.SetsockoptSockFprog(
		int(fd), unix.SOL_SOCKET, unix.SO_ATTACH_FILTER, prog,
	)
	return err
}

func listenLocal(laddr netip.AddrPort) (*net.TCPListener, netip.AddrPort, error) {
	l, err := net.ListenTCP("tcp", &net.TCPAddr{IP: laddr.Addr().AsSlice(), Port: int(laddr.Port())})
	if err != nil {
		return nil, netip.AddrPort{}, err
	}

	err = SetRawBPF(l, []bpf.Instruction{bpf.RetConstant{Val: 0}})
	if err != nil {
		l.Close()
		return nil, netip.AddrPort{}, err
	}

	addr := netip.MustParseAddrPort(l.Addr().String())
	return l, netip.AddrPortFrom(laddr.Addr(), addr.Port()), nil
}
