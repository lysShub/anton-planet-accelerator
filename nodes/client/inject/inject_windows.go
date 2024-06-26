//go:build windows
// +build windows

package inject

import (
	"net"
	"net/netip"

	"github.com/lysShub/divert-go"
	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type inject struct {
	handle  *divert.Handle
	inbound divert.Address
}

func New() (Inject, error) {
	var i = &inject{}
	var err error

	i.handle, err = divert.Open("false", divert.Network, 0, divert.WriteOnly)
	if err != nil {
		return nil, err
	}

	if ifi, err := defaultAdapter(); err != nil {
		i.handle.Close()
		return nil, err
	} else {
		i.inbound.SetOutbound(false)
		i.inbound.Network().IfIdx = uint32(ifi.Index)
	}

	return i, nil
}

func (i *inject) Inject(ip header.IPv4) error {
	_, err := i.handle.Send(ip, &i.inbound)
	return err
}

func (i *inject) Close() error {
	return i.handle.Close()
}

// todo: optimzie
func defaultAdapter() (*net.Interface, error) {
	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.IP{8, 8, 8, 8}, Port: 53})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer conn.Close()
	laddr := netip.MustParseAddrPort(conn.LocalAddr().String()).Addr().As4()

	ifs, err := net.Interfaces()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	for _, i := range ifs {
		if i.Flags&net.FlagRunning == 0 {
			continue
		}

		addrs, err := i.Addrs()
		if err != nil {
			return nil, errors.WithStack(err)
		}
		for _, addr := range addrs {
			if e, ok := addr.(*net.IPNet); ok && e.IP.To4() != nil {
				if [4]byte(e.IP.To4()) == laddr {
					return &i, nil
				}
			}
		}
	}

	return nil, errors.Errorf("not found default adapter")
}
