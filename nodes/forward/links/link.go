//go:build linux
// +build linux

package links

import (
	"net"
	"net/netip"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/lysShub/anton-planet-accelerator/bvvd"
	"github.com/lysShub/anton-planet-accelerator/nodes"
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/rawsock/test"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Link struct {
	links *Links
	lis   listener // occupy local port

	raw   *net.IPConn
	alive atomic.Uint32

	ep     Endpoint
	paddr  netip.AddrPort
	laddr  netip.AddrPort
	header bvvd.Fields

	closeErr errorx.CloseErr
}

func newLink(links *Links, link Endpoint, paddr netip.AddrPort) (*Link, error) {
	var (
		l = &Link{
			links: links,
			ep:    link,
			paddr: paddr,
			header: bvvd.Fields{
				Kind:   bvvd.Data,
				Proto:  link.proto,
				Client: link.client,
				Server: link.server.Addr(),
			},
		}
		err error
	)

	switch link.proto {
	case syscall.IPPROTO_TCP:
		l.lis, err = net.ListenTCP("tcp4", nil)
	case syscall.IPPROTO_UDP:
		l.lis, err = wrapUDPLister(net.ListenUDP("udp4", nil))
	default:
		return nil, errors.Errorf("unknown protocol %d", link.proto)
	}
	if err != nil {
		return nil, l.close(errors.WithStack(err))
	}

	if err := bpfFilterAll(l.lis); err != nil {
		return nil, l.close(err)
	}
	locPort := netip.MustParseAddrPort(l.lis.Addr().String()).Port()

	network := "ip4:" + l.LocalAddr().Network()
	l.raw, err = net.DialIP(network, nil, &net.IPAddr{IP: link.server.Addr().AsSlice()})
	if err != nil {
		return nil, l.close(errors.WithStack(err))
	}
	l.laddr = netip.AddrPortFrom(netip.MustParseAddr(l.raw.LocalAddr().String()), locPort)
	if err := bpfFilterPort(l.raw, l.ep.server.Port(), locPort); err != nil {
		return nil, l.close(err)
	}

	time.AfterFunc(nodes.Keepalive, l.keepalive)
	return l, nil
}

func (l *Link) close(cause error) error {
	cause = errors.WithStack(cause)
	return l.closeErr.Close(func() (errs []error) {
		errs = append(errs, cause)
		if l.raw != nil {
			errs = append(errs, l.raw.Close())
		}
		if l.lis != nil {
			errs = append(errs, l.lis.Close())
		}

		l.links.del(l.ep)
		return errs
	})
}

func (l *Link) keepalive() {
	if l.alive.Swap(0) == 0 {
		l.close(nil)
	} else {
		time.AfterFunc(nodes.Keepalive, l.keepalive)
	}
}

func (l *Link) Recv(pkt *packet.Packet) error {
	n, _, err := l.raw.ReadFrom(pkt.Bytes())
	if err != nil {
		return l.close(err)
	}
	pkt.SetData(n)
	l.alive.Add(1)

	hdr := header.TCP(pkt.Bytes())
	if debug.Debug() {
		require.Equal(test.T(), l.laddr.Port(), hdr.DestinationPort())
	}
	hdr.SetDestinationPort(l.ep.processPort)

	return l.header.Encode(pkt)
}

func (l *Link) Send(pkt *packet.Packet) error {
	nodes.ChecksumForward(pkt, l.ep.proto, l.laddr)
	if debug.Debug() {
		sum := header.PseudoHeaderChecksum(
			tcpip.TransportProtocolNumber(l.ep.proto),
			tcpip.AddrFrom4(l.laddr.Addr().As4()),
			tcpip.AddrFrom4(netip.MustParseAddr(l.raw.RemoteAddr().String()).As4()),
			uint16(pkt.Data()),
		)
		sum = checksum.Checksum(pkt.Bytes(), sum)
		require.Equal(test.T(), uint16(0xffff), sum)
	}

	l.alive.Add(1)
	_, err := l.raw.Write(pkt.Bytes())
	if err != nil {
		return l.close(errors.WithStack(err))
	}
	return nil
}
func (l *Link) Endpoint() Endpoint      { return l.ep }
func (l *Link) Proxyer() netip.AddrPort { return l.paddr }
func (l *Link) LocalAddr() net.Addr     { return l.lis.Addr() }
