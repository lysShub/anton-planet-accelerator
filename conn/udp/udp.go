package udp

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

func Bind(laddr netip.AddrPort) (*udpConn, error) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: laddr.Addr().AsSlice(), Port: int(laddr.Port())})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &udpConn{conn}, nil
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

func (c *udpConn) LocalAddr() netip.AddrPort {
	return netip.MustParseAddrPort(c.conn.LocalAddr().String())
}
