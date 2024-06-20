//go:build linux
// +build linux

package forward

import (
	"log/slog"
	"net/netip"
	"sync"
	"time"

	"github.com/lysShub/anton-planet-accelerator/conn"
	"github.com/lysShub/anton-planet-accelerator/nodes"
	"github.com/lysShub/anton-planet-accelerator/proto"
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/rawsock/test"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Forward struct {
	config *Config

	conn conn.Conn
	ps   *Proxyers

	linkMu sync.RWMutex
	links  map[link]*Link

	closeErr errorx.CloseErr
}

func New(addr string, config *Config) (*Forward, error) {
	var f = &Forward{
		config: config.init(),
		ps:     NewProxyers(),
		links:  map[link]*Link{},
	}
	err := nodes.DisableOffload(config.logger)
	if err != nil {
		return nil, err
	}

	f.conn, err = conn.Bind(nodes.ForwardNetwork, addr)
	if err != nil {
		return nil, f.close(err)
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
		pkt = packet.Make(f.config.MaxRecvBuffSize)
		hdr = proto.Header{}
	)

	for {
		paddr, err := f.conn.ReadFromAddrPort(pkt.Sets(64, 0xffff))
		if err != nil {
			return f.close(err)
		} else if pkt.Data() == 0 {
			continue
		}

		head := pkt.Head()
		if err := hdr.Decode(pkt); err != nil {
			f.config.logger.Error(err.Error(), errorx.Trace(err))
			continue
		}

		switch hdr.Kind {
		case proto.PingForward:
			err := f.conn.WriteToAddrPort(pkt.SetHead(head), paddr)
			if err != nil {
				return f.close(err)
			}
		case proto.PackLossProxyerUplink:
			pkt.SetHead(head).Append(
				f.ps.Proxyer(paddr).UplinkPL().Encode()...,
			)
			err := f.conn.WriteToAddrPort(pkt, paddr)
			if err != nil {
				return f.close(err)
			}
		case proto.Data:
			if debug.Debug() {
				ok := nodes.ValidChecksum(pkt, hdr.Proto, hdr.Server)
				require.True(test.T(), ok)
			}
			f.ps.Proxyer(paddr).UplinkID(hdr.ID)

			t := header.TCP(pkt.Bytes()) // only get port, tcp/udp is same
			link := link{header: hdr, processPort: t.SourcePort(), serverPort: t.DestinationPort()}
			link.header.ID = 0

			f.linkMu.RLock()
			raw, has := f.links[link]
			f.linkMu.RUnlock()
			if !has {
				raw, err = f.addLink(link, paddr)
				if err != nil {
					return f.close(err)
				}
			}

			if err = raw.Send(pkt); err != nil {
				f.delLink(raw.Link())
			}
		default:
			f.config.logger.Warn("invalid header", slog.String("header", hdr.String()), slog.String("proxyer", paddr.String()))
		}
	}
}

func (f *Forward) addLink(link link, paddr netip.AddrPort) (*Link, error) {
	raw, err := NewLink(link, paddr)
	if err != nil {
		return nil, f.close(err)
	}
	f.config.logger.Info("new link", slog.String("link", link.String()), slog.String("local", raw.LocalAddr().String()))

	f.linkMu.Lock()
	f.links[link] = raw
	f.linkMu.Unlock()
	go f.sendService(raw)

	time.AfterFunc(durtion, func() { f.delLink(link) })
	return raw, nil
}

const durtion = time.Minute

func (f *Forward) delLink(link link) error {
	f.linkMu.RLock()
	l := f.links[link]
	f.linkMu.RUnlock()
	if l != nil {
		if l.Alived() {
			time.AfterFunc(durtion, func() { f.delLink(link) })
		} else {
			f.linkMu.Lock()
			delete(f.links, link)
			f.linkMu.Unlock()

			f.config.logger.Info("delect link", slog.String("link", link.String()))
			return l.Close()
		}
	}
	return nil
}

func (f *Forward) sendService(link *Link) (_ error) {
	var (
		pkt = packet.Make(f.config.MaxRecvBuffSize)
		hdr = proto.Header{}
	)

	for {
		if err := link.Recv(pkt.Sets(64, 0xffff)); err != nil {
			return f.delLink(link.Link())
		}

		if err := hdr.Decode(pkt); err != nil {
			f.config.logger.Warn(err.Error(), errorx.Trace(err))
			continue
		}
		hdr.ID = f.ps.Proxyer(link.Proxyer()).DownlinkID()
		hdr.Encode(pkt) // todo: optimize

		err := f.conn.WriteToAddrPort(pkt, link.Proxyer())
		if err != nil {
			return f.close(err)
		}
	}
}
