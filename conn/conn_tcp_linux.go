//go:build linux
// +build linux

package conn

import (
	"encoding/hex"
	"log/slog"
	"net"
	"net/netip"

	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/packet"
	"github.com/pkg/errors"
	"golang.org/x/net/bpf"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

// tcp datagram connect, refer net.UDPConn
type tcpConn struct {
	tcp          *net.TCPListener
	laddr, raddr netip.AddrPort

	raw *net.IPConn

	closeErr errorx.CloseErr
}

func dialTCP(laddr, raddr netip.AddrPort) (Conn, error) {
	var c = &tcpConn{raddr: raddr}
	var err error

	c.tcp, c.laddr, err = listenTcpLocal(laddr)
	if err != nil {
		return nil, c.close(err)
	}

	c.raw, err = net.DialIP("ip4:tcp", &net.IPAddr{IP: c.laddr.Addr().AsSlice()}, &net.IPAddr{IP: c.raddr.Addr().AsSlice()})
	if err != nil {
		return nil, c.close(errors.WithStack(err))
	}
	err = SetRawBPF(c.raw, FilterPorts(c.laddr.Port(), c.raddr.Port()))
	if err != nil {
		return nil, c.close(err)
	}

	return c, nil
}

func listenTCP(laddr netip.AddrPort) (Conn, error) {
	var c = &tcpConn{}
	var err error

	c.tcp, c.laddr, err = listenTcpLocal(laddr)
	if err != nil {
		return nil, c.close(err)
	}

	c.raw, err = net.DialIP("ip4:tcp", &net.IPAddr{IP: c.laddr.Addr().AsSlice()}, &net.IPAddr{IP: c.raddr.Addr().AsSlice()})
	if err != nil {
		return nil, c.close(errors.WithStack(err))
	}
	err = SetRawBPF(c.raw, FilterPorts(c.laddr.Port(), 0))
	if err != nil {
		return nil, c.close(err)
	}

	return c, nil
}

func (c *tcpConn) close(cauae error) error {
	return c.closeErr.Close(func() (errs []error) {
		errs = append(errs, cauae)
		if c.tcp != nil {
			errs = append(errs, errors.WithStack(c.tcp.Close()))
		}
		if c.raw != nil {
			errs = append(errs, errors.WithStack(c.raw.Close()))
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
	n, rip, err := c.raw.ReadFromIP(b.Bytes())
	if err != nil {
		return netip.AddrPort{}, err
	} else if n < header.TCPMinimumSize {
		return netip.AddrPort{}, errors.Errorf("invalid tcp packet %s", hex.EncodeToString(b.Bytes()))
	}
	if debug.Debug() && n == b.Data() {
		slog.Warn("too short warning", errorx.Trace(nil))
	}
	b.SetData(n)

	hdr := header.TCP(b.Bytes())
	if int(hdr.DataOffset()) > n {
		return netip.AddrPort{}, errors.Errorf("invalid tcp packet %s", hex.EncodeToString(b.Bytes()))
	}
	b.DetachN(int(hdr.DataOffset()))

	if c.connect() {
		src = c.raddr
	} else {
		src = netip.AddrPortFrom(netip.AddrFrom4([4]byte(rip.IP.To4())), hdr.SourcePort())
	}
	return src, nil
}
func (c *tcpConn) WriteToAddrPort(b *packet.Packet, to netip.AddrPort) (err error) {
	if c.connect() {
		return errors.WithStack(net.ErrWriteToConnected)
	}
	return c.writeTo(b, to)
}

func (c *tcpConn) writeTo(b *packet.Packet, to netip.AddrPort) (err error) {
	if !to.IsValid() {
		return errors.WithStack(net.InvalidAddrError(to.String()))
	}

	attachTcpHdr(b, c.laddr, to)

	_, err = c.raw.WriteToIP(b.Bytes(), &net.IPAddr{IP: to.Addr().AsSlice()})
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}
func (c *tcpConn) connect() bool { return c.raddr.IsValid() }

func (c *tcpConn) Close() error { return c.close(nil) }

// listenTcpLocal avoid system tcp stack replay RST
func listenTcpLocal(laddr netip.AddrPort) (*net.TCPListener, netip.AddrPort, error) {
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
