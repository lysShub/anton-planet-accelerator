//go:build windows
// +build windows

package tcp

import (
	"fmt"
	"net/netip"

	"github.com/lysShub/divert-go"
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/netkit/route"
	"github.com/lysShub/rawsock/test"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

var _ = divert.MustLoad(divert.DLL)

type ipConn struct {
	laddr netip.AddrPort
	tcp   windows.Handle

	raw      *divert.Handle
	outbound divert.Address

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

	c.tcp, c.laddr, err = bindLocal(laddr)
	if err != nil {
		return nil, c.close(err)
	}

	var filter = fmt.Sprintf(
		"inbound and ip and tcp and localAddr=%s and localPort=%d",
		c.laddr.Addr().String(), c.laddr.Port(),
	)
	c.raw, err = divert.Open(filter, divert.Network, 0, 0)
	if err != nil {
		return nil, c.close(err)
	}
	c.outbound.SetOutbound(true)

	return c, nil
}

func (c *ipConn) ReadFromAddr(b *packet.Packet) (netip.Addr, error) {
	n, err := c.raw.Recv(b.Bytes(), nil)
	if err != nil {
		return netip.Addr{}, err
	} else if n == 0 {
		return c.ReadFromAddr(b)
	}
	b.SetData(n)

	ip := header.IPv4(b.Bytes())
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
	if debug.Debug() {
		table, err := route.GetTable()
		require.NoError(test.T(), err)
		if !table.Match(to).Next.IsValid() {
			panic("not support loouback")
		}
	}

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
		SrcAddr:        tcpip.AddrFrom4(c.laddr.Addr().As4()),
		DstAddr:        tcpip.AddrFrom4(to.As4()),
		Options:        nil,
	})
	ip.SetChecksum(^ip.CalculateChecksum())
	if debug.Debug() {
		test.ValidIP(test.T(), ip)
	}

	_, err := c.raw.Send(ip, &c.outbound)
	return err
}

func (c *ipConn) AddrPort() netip.AddrPort { return c.laddr }
func (c *ipConn) Close() error             { return c.raw.Close() }
func (c *ipConn) close(cause error) error {
	return c.closeErr.Close(func() (errs []error) {
		errs = append(errs, cause)
		if c.raw != nil {
			errs = append(errs, c.raw.Close())
		}
		if c.tcp != 0 {
			errs = append(errs, errors.WithStack(windows.Close(c.tcp)))
		}
		return errs
	})
}

func bindLocal(laddr netip.AddrPort) (windows.Handle, netip.AddrPort, error) {
	var (
		sa windows.Sockaddr
		af int = windows.AF_INET
		st int = windows.SOCK_STREAM
		po int = windows.IPPROTO_TCP
	)

	if laddr.Addr().Is4() {
		sa = &windows.SockaddrInet4{Addr: laddr.Addr().As4(), Port: int(laddr.Port())}
	} else {
		sa = &windows.SockaddrInet6{Addr: laddr.Addr().As16(), Port: int(laddr.Port())}
		af = windows.AF_INET6
	}

	fd, err := windows.Socket(af, st, po)
	if err != nil {
		return windows.InvalidHandle, netip.AddrPort{}, errors.WithStack(err)
	}

	if err := windows.Bind(fd, sa); err != nil {
		return windows.InvalidHandle, netip.AddrPort{}, errors.WithStack(err)
	}

	if laddr.Port() == 0 {
		rsa, err := windows.Getsockname(fd)
		if err != nil {
			return windows.InvalidHandle, netip.AddrPort{}, errors.WithStack(err)
		}
		switch sa := rsa.(type) {
		case *windows.SockaddrInet4:
			return fd, netip.AddrPortFrom(laddr.Addr(), uint16(sa.Port)), nil
		case *windows.SockaddrInet6:
			return fd, netip.AddrPortFrom(laddr.Addr(), uint16(sa.Port)), nil
		default:
		}
	}
	return fd, laddr, nil
}
