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
	"github.com/lysShub/netkit/eth"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/netkit/route"
	"github.com/mdlayher/arp"
	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

// tcp datagram connect, refer net.UDPConn
type tcpConn2 struct {
	tcp          *net.TCPListener
	laddr, raddr netip.AddrPort

	raw *eth.ETHConn
	to  net.HardwareAddr

	closeErr errorx.CloseErr
}

func listenTCP(laddr netip.AddrPort) (Conn, error) {
	var c = &tcpConn2{}
	var err error

	c.tcp, c.laddr, err = listenTcpLocal(laddr)
	if err != nil {
		return nil, c.close(err)
	}

	c.raw, c.to, err = newEthConn(c.laddr)
	if err != nil {
		return nil, c.close(errors.WithStack(err))
	}
	err = SetRawBPF(c.raw, FilterIPv4AndLocal(c.laddr, header.TCPProtocolNumber))
	if err != nil {
		return nil, c.close(err)
	}

	return c, nil
}

func (c *tcpConn2) close(cauae error) error {
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

func (c *tcpConn2) Read(b *packet.Packet) (err error) {
	_, err = c.ReadFromAddrPort(b)
	return err
}

func (c *tcpConn2) Write(b *packet.Packet) (err error) {
	attachTcpHdr(b, c.laddr, c.raddr)
	_, err = c.raw.Write(b.Bytes())
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (c *tcpConn2) ReadFromAddrPort(b *packet.Packet) (src netip.AddrPort, err error) {
	n, _, err := c.raw.ReadFromETH(b.Bytes())
	if err != nil {
		return netip.AddrPort{}, err
	} else if n < header.IPv4MinimumSize+header.TCPMinimumSize {
		return netip.AddrPort{}, errors.Errorf("invalid tcp packet %s", hex.EncodeToString(b.Bytes()))
	}
	if debug.Debug() && n == b.Data() {
		slog.Warn("too short warning", errorx.Trace(nil))
	}
	b.SetData(n)

	ip := header.IPv4(b.Bytes())
	tcp := header.TCP(ip.Payload())
	b.DetachN(int(ip.HeaderLength()) + int(tcp.DataOffset()))

	if c.connect() {
		src = c.raddr
	} else {
		src = netip.AddrPortFrom(
			netip.AddrFrom4(ip.SourceAddress().As4()),
			tcp.SourcePort(),
		)
	}
	return src, nil
}
func (c *tcpConn2) connect() bool { return c.raddr.IsValid() }

func (c *tcpConn2) WriteToAddrPort(b *packet.Packet, to netip.AddrPort) (err error) {
	if !to.IsValid() {
		return errors.WithStack(net.InvalidAddrError(to.String()))
	}
	attachTcpHdr(b, c.laddr, to)
	attachIPv4Hdr(b, c.laddr.Addr(), to.Addr())

	_, err = c.raw.WriteToETH(b.Bytes(), c.to)
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (c *tcpConn2) Close() error               { return c.close(nil) }
func (c *tcpConn2) LocalAddr() netip.AddrPort  { return c.laddr }
func (c *tcpConn2) RemoteAddr() netip.AddrPort { return c.raddr }

func newEthConn(laddr netip.AddrPort) (raw *eth.ETHConn, to net.HardwareAddr, err error) {
	table, err := route.GetTable()
	if err != nil {
		return nil, nil, err
	}

	var entry route.Entry
	for _, e := range table {
		if e.Addr == laddr.Addr() {
			entry = e
			break
		}
	}

	i, err := net.InterfaceByIndex(int(entry.Interface))
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}
	raw, err = eth.Listen("eth:ip4", i)
	if err != nil {
		return nil, nil, err
	}

	if !entry.Next.IsValid() {
		// On-link
		to = i.HardwareAddr
	} else {
		c, err := arp.Dial(i)
		if err != nil {
			return nil, nil, errors.WithStack(err)
		}
		to, err = c.Resolve(entry.Next)
		if err != nil {
			return nil, nil, errors.WithStack(err)
		}
	}
	return raw, to, nil
}
