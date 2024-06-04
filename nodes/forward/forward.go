//go:build linux
// +build linux

package forward

import (
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"sync"

	"github.com/lysShub/anton-planet-accelerator/proto"
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/packet"
	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Forward struct {
	config *Config

	conn *net.UDPConn

	linkMu sync.RWMutex
	links  map[link]*Link

	closeErr errorx.CloseErr
}

type link struct {
	header      proto.Header
	processPort uint16
	serverPort  uint16
}

func (l link) String() string {
	return fmt.Sprintf(
		"{ID:%d, Proto:%d,ProcessPort:%d, Server:%s}",
		l.header.ID, l.header.Proto, l.processPort,
		netip.AddrPortFrom(l.header.Server, l.serverPort),
	)
}

func New(addr string, config *Config) (*Forward, error) {
	var f = &Forward{config: config.init(), links: map[link]*Link{}}

	laddr, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	f.conn, err = net.ListenUDP("udp4", laddr)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return f, nil
}

func (f *Forward) close(cause error) error {
	return f.closeErr.Close(func() (errs []error) {
		errs = append(errs, cause)
		if f.conn != nil {
			errs = append(errs, f.conn.Close())
		}

		f.linkMu.Lock()
		for _, e := range f.links {
			errs = append(errs, e.Close())
		}
		clear(f.links)
		f.linkMu.Unlock()
		return errs
	})
}

func (f *Forward) Serve() error {
	f.config.logger.Info("start", slog.String("listen", f.conn.LocalAddr().String()), slog.Bool("debug", debug.Debug()))
	return f.recvService()
}

func (f *Forward) recvService() (err error) {
	var (
		pkt  = packet.Make(f.config.MaxRecvBuffSize)
		hdr  = proto.Header{}
		head = 64
	)

	for {
		n, paddr, err := f.conn.ReadFromUDPAddrPort(pkt.Sets(head, 0xffff).Bytes())
		if err != nil {
			return f.close(err)
		}
		pkt.SetData(n)

		if err := hdr.Decode(pkt); err != nil {
			f.config.logger.Error(err.Error(), errorx.Trace(err))
			continue
		}

		switch hdr.Kind {
		case proto.PingForward:
			pkt.SetHead(head)
			_, err := f.conn.WriteToUDPAddrPort(pkt.Bytes(), paddr)
			if err != nil {
				return f.close(err)
			}
		case proto.PacketLossForward:
			var pl proto.PL = 0.11 // todo:

			pkt.SetHead(head)
			pkt.Append(pl.Encode()...)
			_, err = f.conn.WriteToUDPAddrPort(pkt.Bytes(), paddr)
			if err != nil {
				return f.close(err)
			}
		case proto.Data:
			t := header.TCP(pkt.Bytes()) // only get port, tcp/udp is same
			link := link{header: hdr, processPort: t.SourcePort(), serverPort: t.DestinationPort()}

			f.linkMu.RLock()
			raw, has := f.links[link]
			f.linkMu.RUnlock()
			if !has {
				raw, err = NewLink(link, paddr)
				if err != nil {
					return f.close(err)
				}
				f.config.logger.Info("new link", slog.String("link", link.String()), slog.String("local", raw.LocalAddr().String()))

				f.linkMu.Lock()
				f.links[link] = raw
				f.linkMu.Unlock()
				go f.sendService(raw)
			}

			if err = raw.Send(pkt); err != nil {
				f.deleteRaw(raw)
			}
		default:
			f.config.logger.Warn("invalid header", slog.String("header", hdr.String()), slog.String("proxyer", paddr.String()))
		}
	}
}

func (f *Forward) sendService(raw *Link) (_ error) {
	var (
		pkt = packet.Make(f.config.MaxRecvBuffSize)
	)

	for {
		if err := raw.Recv(pkt.Sets(64, 0xffff)); err != nil {
			return f.deleteRaw(raw)
		}

		_, err := f.conn.WriteToUDPAddrPort(pkt.Bytes(), raw.Proxyer())
		if err != nil {
			return f.close(err)
		}
	}
}

func (f *Forward) deleteRaw(raw *Link) error {
	f.config.logger.Info("delect link", slog.String("link", raw.link.String()))

	f.linkMu.Lock()
	delete(f.links, raw.Link())
	f.linkMu.Unlock()
	return raw.Close()
}
