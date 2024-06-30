//go:build windows
// +build windows

package game

import (
	"errors"
	"net/netip"
	"sync/atomic"

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
	started atomic.Bool

	handle *divert.Handle
	addr   divert.Address

	mapping mapping.Mapping

	closeErr errorx.CloseErr
}

func newWarthundr() (Game, error) {
	var g = &warthunder{}
	var err error

	var filter = "outbound and !loopback and ip and (tcp or udp)"
	g.handle, err = divert.Open(filter, divert.Network, -1, 0)
	if err != nil {
		return nil, g.close(err)
	}

	if g.mapping, err = mapping.New(); err != nil {
		return nil, g.close(err)
	}
	return g, nil
}

func (w *warthunder) Start() { w.started.Store(true) }

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
	h, d := pkt.Head(), pkt.Data()
	for {
		n, err := w.handle.Recv(pkt.Sets(h, d).Bytes(), &w.addr)
		if err != nil {
			if errors.Is(err, windows.ERROR_INSUFFICIENT_BUFFER) {
				return Info{}, errorx.WrapTemp(err)
			}
			return Info{}, w.close(err)
		} else if n < header.IPv4MinimumSize {
			continue
		}
		pkt.SetData(n)
		hdr := header.IPv4(pkt.Bytes())

		pass := !w.started.Load() || netip.AddrFrom4(hdr.DestinationAddress().As4()).IsMulticast()
		if !pass {
			src := netip.AddrPortFrom(
				netip.AddrFrom4(hdr.SourceAddress().As4()),
				uint16(header.TCP(hdr.Payload()).SourcePort()), // tcp/udp is  same
			)

			name, err := w.mapping.Name(src, hdr.Protocol())
			if err != nil {
				if errorx.Temporary(err) {
					pass = true // todo: maybe drop it
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
			Proto:    hdr.TransportProtocol(),
			Server:   netip.AddrFrom4(hdr.DestinationAddress().As4()),
			PlayData: playData,
		}, nil
	}
}

func (w *warthunder) Close() error { return w.close(nil) }
