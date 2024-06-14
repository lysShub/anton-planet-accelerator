package conn

import (
	"net"
	"net/netip"

	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/netkit/route"
	"github.com/pkg/errors"
)

// datagram connect, refer net.UDPConn
type Conn interface {
	Read(*packet.Packet) (err error)
	Write(*packet.Packet) (err error)

	ReadFromAddrPort(*packet.Packet) (netip.AddrPort, error)
	WriteToAddrPort(*packet.Packet, netip.AddrPort) error

	LocalAddr() netip.AddrPort
	RemoteAddr() netip.AddrPort

	Close() error
}

func Dial(network string, laddr, raddr string) (Conn, error) {
	remAddr, err := resolveAddr(raddr)
	if err != nil {
		return nil, err
	} else if remAddr.Addr().IsUnspecified() || remAddr.Port() == 0 {
		return nil, errors.Errorf("unknown remote address %s", remAddr.String())
	}
	locAddr, err := resolveAddr(laddr)
	if err != nil {
		return nil, err
	} else if locAddr.Addr().IsUnspecified() {
		table, err := route.GetTable()
		if err != nil {
			return nil, errors.WithStack(err)
		}
		entry := table.Match(netip.AddrFrom4(remAddr.Addr().As4()))
		if !entry.Valid() {
			return nil, errors.Errorf("network %s is unreachable", remAddr.Addr().String())
		}
		locAddr = netip.AddrPortFrom(entry.Addr, locAddr.Port())
	}

	switch network {
	case "udp", "udp4":
		return dialUDP(locAddr, remAddr)
	case "tcp", "tcp4":
		return dialTCP(locAddr, remAddr)
	default:
		return nil, errors.Errorf("not support network %s", network)
	}
}

func Listen(network string, laddr string) (Conn, error) {
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
		return listenUDP(addr)
	case "tcp", "tcp4":
		return listenTCP(addr)
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
	}

	a := udpAddr.AddrPort()
	if !a.Addr().Is4() {
		return netip.AddrPort{}, errors.Errorf("only support ipv4 %s", udpAddr.String())
	}
	return a, nil
}
