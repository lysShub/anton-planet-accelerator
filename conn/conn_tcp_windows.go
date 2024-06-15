//go:build windows
// +build windows

package conn

import (
	"encoding/hex"
	"fmt"
	"net"
	"net/netip"

	"github.com/lysShub/divert-go"
	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/packet"
	"github.com/pkg/errors"
	"golang.org/x/sys/windows"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

var _ = divert.MustLoad(divert.DLL)

type tcpConn struct {
	tcp          windows.Handle
	laddr, raddr netip.AddrPort

	handle   *divert.Handle
	outbound divert.Address

	closeErr errorx.CloseErr
}

func dialTCP(laddr, raddr netip.AddrPort) (Conn, error) {
	var c = &tcpConn{raddr: raddr}
	var err error

	c.tcp, c.laddr, err = bindLocal(laddr)
	if err != nil {
		return nil, c.close(err)
	}
	if raddr.Addr().IsPrivate() || raddr.Addr().IsLoopback() {
		return nil, errors.Errorf("not support loopback %s-->%s", laddr.String(), raddr.String())
	}

	var filter = fmt.Sprintf(
		"inbound and ip and tcp and localAddr=%s and localPort=%d and remoteAddr=%s and remotePort=%d",
		c.laddr.Addr().String(), c.laddr.Port(), c.raddr.Addr().String(), c.raddr.Port(),
	)
	c.handle, err = divert.Open(filter, divert.Network, 0, 0)
	if err != nil {
		return nil, c.close(err)
	}
	c.outbound = divert.Address{}
	c.outbound.SetOutbound(true)

	return c, nil
}

func listenTCP(laddr netip.AddrPort) (Conn, error) {
	var c = &tcpConn{}
	var err error

	c.tcp, c.laddr, err = bindLocal(laddr)
	if err != nil {
		return nil, c.close(err)
	}

	var filter = fmt.Sprintf(
		"inbound and ip and tcp and localAddr=%s and localPort=%d",
		laddr.Addr().String(), laddr.Port(),
	)
	c.handle, err = divert.Open(filter, divert.Network, 0, 0)
	if err != nil {
		return nil, c.close(err)
	}
	c.outbound = divert.Address{}
	c.outbound.SetOutbound(true)

	return c, nil
}

func (c *tcpConn) close(cause error) error {
	return c.closeErr.Close(func() (errs []error) {
		errs = append(errs, cause)
		if c.handle != nil {
			errs = append(errs, c.handle.Close())
		}
		if c.tcp != 0 {
			errs = append(errs, errors.WithStack(windows.Close(c.tcp)))
		}
		return errs
	})
}

func (c *tcpConn) Read(b *packet.Packet) (err error) {
	_, err = c.ReadFromAddrPort(b)
	return err
}

func (c *tcpConn) Write(b *packet.Packet) (err error) {
	return c.writeTo(b, c.raddr)
}

func (c *tcpConn) ReadFromAddrPort(b *packet.Packet) (src netip.AddrPort, err error) {
	n, err := c.handle.Recv(b.Bytes(), nil)
	if err != nil {
		return netip.AddrPort{}, err
	} else if n < header.IPv4MinimumSize+header.TCPMinimumSize {
		return netip.AddrPort{}, errors.Errorf("invalid ip packet %s", hex.EncodeToString(b.Bytes()))
	}
	b.SetData(n)

	ip := header.IPv4(b.Bytes())
	if int(ip.TotalLength()) > len(ip) {
		return netip.AddrPort{}, errorx.ShortBuff(int(ip.TotalLength()), len(ip))
	}
	tcp := header.TCP(ip.Payload())
	b.DetachN(int(ip.HeaderLength()) + int(tcp.DataOffset()))

	if c.connect() {
		src = c.raddr
	} else {
		src = netip.AddrPortFrom(netip.AddrFrom4(ip.SourceAddress().As4()), tcp.SourcePort())
	}
	return src, nil
}

func (c *tcpConn) WriteToAddrPort(b *packet.Packet, to netip.AddrPort) error {
	if c.connect() {
		return errors.WithStack(net.ErrWriteToConnected)
	}
	return c.writeTo(b, to)
}
func (c *tcpConn) connect() bool { return c.raddr.IsValid() }
func (c *tcpConn) writeTo(b *packet.Packet, to netip.AddrPort) error {
	if !to.IsValid() {
		return errors.WithStack(net.InvalidAddrError(to.String()))
	}
	attachTcpHdr(b, c.laddr, to)
	attachIPv4Hdr(b, c.laddr.Addr(), to.Addr())

	_, err := c.handle.Send(b.Bytes(), &c.outbound)
	return err
}

func (c *tcpConn) Close() error               { return c.close(nil) }
func (c *tcpConn) LocalAddr() netip.AddrPort  { return c.laddr }
func (c *tcpConn) RemoteAddr() netip.AddrPort { return c.raddr }

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
