//go:build linux
// +build linux

package forward

import (
	"errors"
	"log/slog"
	"math/rand"
	"net"
	"net/netip"

	"github.com/lysShub/anton-planet-accelerator/bvvd"
	"github.com/lysShub/anton-planet-accelerator/conn"
	"github.com/lysShub/anton-planet-accelerator/nodes"
	"github.com/lysShub/anton-planet-accelerator/nodes/forward/links"
	"github.com/lysShub/anton-planet-accelerator/nodes/internal"
	"github.com/lysShub/anton-planet-accelerator/nodes/internal/checksum"
	"github.com/lysShub/anton-planet-accelerator/nodes/internal/msg"
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/rawsock/test"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Forward struct {
	config *Config
	faddr  netip.AddrPort
	loc    bvvd.Location

	conn conn.Conn
	ps   *Gateways

	links *links.Links

	closeErr errorx.CloseErr
}

func New(addr string, config *Config) (*Forward, error) {
	var f = &Forward{
		config: config.init(),
		faddr:  netip.MustParseAddrPort(""), // todo:
		ps:     NewGateways(),
		links:  links.NewLinks(),
	}
	err := internal.DisableOffload(config.logger)
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
	panic("获取faddr loc")

	f.config.logger.Info("start",
		slog.String("listen", f.conn.LocalAddr().String()),
		slog.String("faddr", f.faddr.String()),
		slog.String("location", f.loc.Hans()),
		slog.Bool("debug", debug.Debug()),
	)
	return f.uplinkService()
}

func (f *Forward) uplinkService() (err error) {
	var (
		pkt = packet.Make(f.config.MaxRecvBuffSize)
	)

	for {
		gaddr, err := f.conn.ReadFromAddrPort(pkt.Sets(64, 0xffff))
		if err != nil {
			return f.close(err)
		} else if pkt.Data() < bvvd.Size {
			continue
		}

		hdr := bvvd.Bvvd(pkt.Bytes())

		switch kind := hdr.Kind(); kind {
		case bvvd.PingForward:
			var msg msg.Message
			if err := msg.Decode(pkt); err != nil {
				f.config.logger.Error(err.Error(), errorx.Trace(err))
				continue
			}
			msg.Forward = f.faddr
			msg.Payload = f.loc
			if err := msg.Encode(pkt.SetData(0)); err != nil {
				f.config.logger.Error(err.Error(), errorx.Trace(err))
				continue
			}

			if err := f.conn.WriteToAddrPort(pkt, gaddr); err != nil {
				return f.close(err)
			}
		case bvvd.PackLossGatewayUplink:
			var msg msg.Message
			if err := msg.Decode(pkt); err != nil {
				f.config.logger.Error(err.Error(), errorx.Trace(err))
				continue
			}
			msg.Payload = f.ps.Gateway(gaddr).UplinkPL()
			if err := msg.Encode(pkt.SetData(0)); err != nil {
				f.config.logger.Error(err.Error(), errorx.Trace(err))
				continue
			}

			if err := f.conn.WriteToAddrPort(pkt, gaddr); err != nil {
				return f.close(err)
			}
		case bvvd.Data:
			f.ps.Gateway(gaddr).UplinkID(hdr.DataID())

			// remove bvvd header
			pkt = pkt.DetachN(bvvd.Size)
			if debug.Debug() {
				require.True(test.T(), checksum.ValidChecksum(pkt, uint8(hdr.Proto()), hdr.Server()))
				require.Equal(test.T(), f.faddr, hdr.Forward())
			}

			// only get port, tcp/udp is same
			ep := links.NewEP(hdr, header.TCP(pkt.Bytes()))

			// read/create corresponding link, the link will self close by keepalive,
			// so if Send/Recv return net.ErrClosed error should ignore.
			link, new, err := f.links.Link(ep, gaddr, f.faddr)
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
	)

	for {
		if err := link.Recv(pkt.Sets(64, 0xffff)); err != nil {
			if errors.Is(err, net.ErrClosed) {
				f.config.logger.Info("del link", slog.String("endpoint", link.Endpoint().String()))
				return nil // close by keepalive
			} else if errorx.Temporary(err) {
				f.config.logger.Warn(err.Error(), errorx.Trace(err))
				continue
			} else {
				return f.close(err)
			}
		}

		id := f.ps.Gateway(link.Gateway()).DownlinkID()
		bvvd.Bvvd(pkt.Bytes()).SetDataID(id)

		if debug.Debug() && rand.Int()%100 == 99 {
			continue // PackLossGatewayDownlink
		}

		if err := f.conn.WriteToAddrPort(pkt, link.Gateway()); err != nil {
			return f.close(err)
		}
	}
}
