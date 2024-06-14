package conn

import (
	"net/netip"
	"sync"
	"syscall"

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

	Close() error
}

func Dial(network string, laddr, raddr netip.AddrPort) (Conn, error) {
	if !laddr.Addr().Is4() || !raddr.Addr().Is4() {
		return nil, errors.Errorf("only support ipv4 %s-->%s", laddr.String(), raddr.String())
	}
	if laddr.IsValid() {
		locAddr, err := defaultLocal(laddr.Addr(), raddr.Addr())
		if err != nil {
			return nil, err
		}
		laddr = netip.AddrPortFrom(locAddr, laddr.Port())
	}

	switch network {
	case "udp", "udp4":
		return dialUDP(laddr, raddr)
	case "tcp", "tcp4":
		return dialTCP(laddr, raddr)
	default:
		return nil, errors.Errorf("not support network %s", network)
	}
}

func Listen(network string, laddr netip.AddrPort) (Conn, error) {
	if !laddr.Addr().Is4() {
		return nil, errors.Errorf("only support ipv4 %s", laddr.String())
	}
	if laddr.IsValid() {
		locAddr, err := defaultLocal(laddr.Addr(), netip.AddrFrom4([4]byte{8, 8, 8, 8}))
		if err != nil {
			return nil, err
		}
		laddr = netip.AddrPortFrom(locAddr, laddr.Port())
	}

	switch network {
	case "udp", "udp4":
		return listenUDP(laddr)
	case "tcp", "tcp4":
		return listenTCP(laddr)
	default:
		return nil, errors.Errorf("not support network %s", network)
	}
}

func defaultLocal(laddr, raddr netip.Addr) (netip.Addr, error) {
	if !laddr.IsUnspecified() {
		return laddr, nil
	}

	table, err := route.GetTable()
	if err != nil {
		return netip.Addr{}, errors.WithStack(err)
	}
	entry := table.Match(raddr)
	if !entry.Valid() {
		err = errors.WithMessagef(
			syscall.ENETUNREACH,
			"%s -> %s", laddr.String(), raddr.String(),
		)
		return netip.Addr{}, errors.WithStack(err)
	}

	if laddr.IsUnspecified() {
		laddr = entry.Addr
	} else {
		if laddr != entry.Addr {
			err = errors.WithMessagef(
				syscall.EADDRNOTAVAIL, laddr.String(),
			)
			return netip.Addr{}, errors.WithStack(err)
		}
	}

	addr2if.Store(laddr, entry.Interface)
	return laddr, nil
}

var addr2if = sync.Map{}
