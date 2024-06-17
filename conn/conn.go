package conn

import (
	"net"
	"net/netip"

	"github.com/lysShub/anton-planet-accelerator/conn/tcp"
	"github.com/lysShub/anton-planet-accelerator/conn/udp"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/netkit/route"
	"github.com/pkg/errors"
)

// datagram connect, refer net.UDPConn
type Conn interface {
	// read transport layer payload, notice maybe return 0 length data
	ReadFromAddrPort(*packet.Packet) (netip.AddrPort, error)

	// write transport layer payload
	WriteToAddrPort(*packet.Packet, netip.AddrPort) error

	LocalAddr() netip.AddrPort

	Close() error
}

func Bind(network string, laddr string) (Conn, error) {
	addr, err := resolveAddr(laddr)
	if err != nil {
		return nil, err
	}
	if addr.Addr().IsUnspecified() {
		table, err := route.GetTable()
		if err != nil {
			return nil, errors.WithStack(err)
		}
		entry := table.Match(netip.AddrFrom4([4]byte{8, 8, 8, 8}))
		if !entry.Valid() {
			return nil, errors.New("not network connection")
		}
		addr = netip.AddrPortFrom(entry.Addr, addr.Port())
	}

	switch network {
	case "udp", "udp4":
		return udp.Bind(addr)
	case "tcp", "tcp4":
		return tcp.Bind(addr)
	default:
		return nil, errors.Errorf("not support network %s", network)
	}
}

func resolveAddr(addr string) (netip.AddrPort, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return netip.AddrPort{}, errors.WithStack(err)
	}
	if len(udpAddr.IP) == 0 {
		udpAddr.IP = netip.IPv4Unspecified().AsSlice()
	} else if udpAddr.IP.To4() != nil {
		udpAddr.IP = udpAddr.IP.To4()
	}

	a := udpAddr.AddrPort()
	if !a.Addr().Is4() {
		return netip.AddrPort{}, errors.Errorf("only support ipv4 %s", udpAddr.String())
	}
	return a, nil
}
