//go:build linux
// +build linux

package forward

import (
	"errors"
	"log/slog"
	"math/rand"
	"net"

	"github.com/lysShub/anton-planet-accelerator/conn"
	"github.com/lysShub/anton-planet-accelerator/nodes"
	"github.com/lysShub/anton-planet-accelerator/nodes/forward/links"
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

	links *links.Links

	closeErr errorx.CloseErr
}

func New(addr string, config *Config) (*Forward, error) {
	var f = &Forward{
		config: config.init(),
		ps:     NewProxyers(),
		links:  links.NewLinks(),
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
		if f.links != nil {
			errs = append(errs, f.links.Close())
		}
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
				require.True(test.T(), nodes.ValidChecksum(pkt, hdr.Proto, hdr.Server))
			}
			f.ps.Proxyer(paddr).UplinkID(hdr.ID)

			// only get port, tcp/udp is same
			ep := links.NewEP(hdr, header.TCP(pkt.Bytes()))

			link, new, err := f.links.Link(ep, paddr)
			if err != nil {
				return f.close(err)
			} else if new {
				f.config.logger.Info("new link", slog.String("endpoint", ep.String()))
				go f.downlinkService(link)
			}

			if err = link.Send(pkt); err != nil {
				if !errors.Is(err, net.ErrClosed) {
					return f.close(err)
				}
			}
		default:
		}
	}
}

func (f *Forward) downlinkService(link *links.Link) (_ error) {
	var (
		pkt = packet.Make(f.config.MaxRecvBuffSize)
		hdr = proto.Header{}
	)

	for {
		if err := link.Recv(pkt.Sets(64, 0xffff)); err != nil {
			if errors.Is(err, net.ErrClosed) {
				f.config.logger.Info("del link", slog.String("endpoint", link.Endpoint().String()))
				return nil
			} else {
				return f.close(err)
			}
		}

		if err := hdr.Decode(pkt); err != nil {
			f.config.logger.Warn(err.Error(), errorx.Trace(err))
			continue
		}
		hdr.ID = f.ps.Proxyer(link.Proxyer()).DownlinkID()
		hdr.Encode(pkt) // todo: optimize

		if debug.Debug() && rand.Int()%100 == 99 {
			continue // PackLossProxyerDownlink
		}

		if err := f.conn.WriteToAddrPort(pkt, link.Proxyer()); err != nil {
			return f.close(err)
		}
	}
}
