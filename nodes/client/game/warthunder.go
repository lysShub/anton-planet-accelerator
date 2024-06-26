//go:build windows
// +build windows

package game

import (
	"net/netip"

	accelerator "github.com/lysShub/anton-planet-accelerator"
	"github.com/lysShub/divert-go"
	"github.com/lysShub/netkit/errorx"
	mapping "github.com/lysShub/netkit/mapping/process"
	"github.com/lysShub/netkit/packet"
	"golang.org/x/sys/windows"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

const Warthunder = "aces.exe"

type warthunder struct {
	handle *divert.Handle
	addr   divert.Address

	mapping mapping.Mapping

	closeErr errorx.CloseErr
}

func newWarthundr() (Game, error) {
	var g = &warthunder{}
	var err error

	var filter = "outbound and !loopback and ip and (tcp or udp)"
	g.handle, err = divert.Open(filter, divert.Network, 0, 0)
	if err != nil {
		return nil, g.close(err)
	}

	if g.mapping, err = mapping.New(); err != nil {
		return nil, g.close(err)
	}
	return g, nil
}

func (w *warthunder) close(cause error) error {
	return w.closeErr.Close(func() (errs []error) {
		errs = append(errs, cause)
		if w.mapping != nil {
			errs = append(errs, w.mapping.Close())
		}
		if w.handle != nil {
			errs = append(errs, w.handle.Close())
		}
		return errs
	})
}

func (w *warthunder) Capture(pkt *packet.Packet) (Info, error) {
	for {
		n, err := w.handle.Recv(pkt.Bytes(), &w.addr)
		if err != nil {
			return Info{}, w.close(err)
		} else if n < header.IPv4MinimumSize {
			continue
		}
		hdr := header.IPv4(pkt.Bytes())

		pass := netip.AddrFrom4(hdr.DestinationAddress().As4()).IsMulticast()
		if !pass {
			src := netip.AddrPortFrom(
				netip.AddrFrom4(hdr.SourceAddress().As4()),
				uint16(header.TCP(hdr.Payload()).SourcePort()), // tcp/udp is  same
			)

			name, err := w.mapping.Name(src, hdr.Protocol())
			if err != nil {
				if errorx.Temporary(err) {
					pass = false
				} else {
					return Info{}, w.close(err)
				}
			} else {
				pass = name != accelerator.Warthunder
			}
		}
		if pass {
			if _, err = w.handle.Send(pkt.Bytes(), &w.addr); err != nil {
				return Info{}, w.close(err)
			}
			continue
		}

		playData := hdr.Protocol() == windows.IPPROTO_UDP
		if playData {
			udp := header.UDP(hdr.Payload())

			// https://support.gaijin.net/hc/en-us/articles/200070211--War-Thunder-game-ports
			playData = 20000 <= udp.DestinationPort() && udp.DestinationPort() <= 30000
		}

		pkt.DetachN(int(hdr.HeaderLength()))
		return Info{
			Proto:    hdr.Protocol(),
			Server:   netip.AddrFrom4(hdr.DestinationAddress().As4()),
			PlayData: playData,
		}, nil
	}
}

func (w *warthunder) Close() error { return w.close(nil) }
