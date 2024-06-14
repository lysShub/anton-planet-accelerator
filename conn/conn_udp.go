package conn

import (
	"log/slog"
	"net"
	"net/netip"

	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/packet"
	"github.com/pkg/errors"
)

type udpConn struct {
	conn *net.UDPConn
}

func dialUDP(laddr, raddr netip.AddrPort) (Conn, error) {
	conn, err := net.DialUDP("udp",
		&net.UDPAddr{IP: laddr.Addr().AsSlice(), Port: int(laddr.Port())},
		&net.UDPAddr{IP: raddr.Addr().AsSlice(), Port: int(raddr.Port())},
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &udpConn{conn}, nil
}

func listenUDP(laddr netip.AddrPort) (Conn, error) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: laddr.Addr().AsSlice(), Port: int(laddr.Port())})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &udpConn{conn}, nil
}

func (c *udpConn) Read(b *packet.Packet) (err error) {
	n, err := c.conn.Read(b.Bytes())
	if err != nil {
		return errors.WithStack(err)
	}
	b.SetData(n)
	return nil
}
func (c *udpConn) Write(b *packet.Packet) (err error) {
	_, err = c.conn.Write(b.Bytes())
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}
func (c *udpConn) ReadFromAddrPort(b *packet.Packet) (netip.AddrPort, error) {
	n, addr, err := c.conn.ReadFromUDPAddrPort(b.Bytes())
	if err != nil {
		return netip.AddrPort{}, err
	}
	if debug.Debug() && n == b.Data() {
		slog.Warn("too short warning", errorx.Trace(nil))
	}
	b.SetData(n)
	return addr, nil
}
func (c *udpConn) WriteToAddrPort(b *packet.Packet, dst netip.AddrPort) error {
	_, err := c.conn.WriteToUDPAddrPort(b.Bytes(), dst)
	return err
}
func (c *udpConn) Close() error { return c.conn.Close() }
