//go:build linux
// +build linux

package gateway

import (
	"fmt"
	"log/slog"
	"math/rand"
	"net/netip"
	"sync/atomic"
	"time"

	"github.com/lysShub/anton-planet-accelerator/bvvd"
	"github.com/lysShub/anton-planet-accelerator/conn"
	"github.com/lysShub/anton-planet-accelerator/nodes"
	"github.com/lysShub/anton-planet-accelerator/nodes/internal"
	"github.com/lysShub/anton-planet-accelerator/nodes/internal/checksum"
	"github.com/lysShub/anton-planet-accelerator/nodes/internal/msg"
	"github.com/lysShub/anton-planet-accelerator/nodes/internal/stats"
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/rawsock/test"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

type Gateway struct {
	config *Config
	start  atomic.Bool

	conn conn.Conn
	cs   *Clients

	sender conn.Conn
	fs     *Forwards

	speed *stats.LinkSpeed

	closeErr errorx.CloseErr
}

func New(addr string, config *Config) (*Gateway, error) {
	var p = &Gateway{
		config: config.init(),
		cs:     NewClients(),
		fs:     NewForwards(),
		speed:  stats.NewLinkSpeed(time.Second),
	}

	err := internal.DisableOffload(config.logger)
	if err != nil {
		return nil, err
	}

	p.conn, err = conn.Bind(nodes.GatewayNetwork, addr)
	if err != nil {
		return nil, p.close(err)
	}

	p.sender, err = conn.Bind(nodes.ForwardNetwork, "")
	if err != nil {
		return nil, p.close(err)
	}

	return p, nil
}

func (p *Gateway) close(cause error) error {
	cause = errors.WithStack(cause)
	if cause != nil {
		p.config.logger.Error(cause.Error(), errorx.Trace(cause))
	} else {
		p.config.logger.Info("close")
	}
	return p.closeErr.Close(func() (errs []error) {
		errs = append(errs, cause)
		if p.speed != nil {
			errs = append(errs, p.speed.Close())
		}
		if p.sender != nil {
			errs = append(errs, p.sender.Close())
		}
		if p.conn != nil {
			errs = append(errs, p.conn.Close())
		}
		return errs
	})
}

func (p *Gateway) Serve() (err error) {
	if p.start.Swap(true) {
		return errors.Errorf("gateway started")
	}
	p.config.logger.Info("start", slog.String("listen", p.conn.LocalAddr().String()), slog.Bool("debug", debug.Debug()))

	go p.donwlinkService()
	return p.close(p.uplinkService())
}

func (p *Gateway) AddForward(faddr netip.AddrPort, fid bvvd.ForwardID, loc bvvd.Location) error {
	if !p.start.Load() {
		return errors.Errorf("gateway not start")
	}

	if err := p.fs.Add(faddr, fid, loc); err != nil {
		return err
	}
	p.config.logger.Info("add forward", slog.String("forward", faddr.String()), slog.String("LocID", loc.String()))
	return nil
}

func (p *Gateway) Speed() (up, down string) {
	up1, down1 := p.speed.Speed()

	var human = func(s float64) string {
		const (
			K = 1024
			M = 1024 * K
			G = 1024 * M
		)

		if s < K {
			return fmt.Sprintf("%.2f B/s", s)
		} else if s < M {
			return fmt.Sprintf("%.2f KB/s", s/K)
		} else if s < G {
			return fmt.Sprintf("%.2f MB/s", s/M)
		} else {
			return fmt.Sprintf("%.2f GB/s", s/G)
		}
	}

	return human(up1), human(down1)
}

func (p *Gateway) uplinkService() (_ error) {
	var (
		pkt = packet.Make(p.config.MaxRecvBuff)
	)

	for {
		caddr, err := p.conn.ReadFromAddrPort(pkt.Sets(64, 0xffff))
		if err != nil {
			return p.close(err)
		} else if pkt.Data() == 0 {
			continue
		}
		p.speed.Uplink(pkt.Data() + 20 + 8)

		hdr := bvvd.Bvvd(pkt.Bytes())

		switch kind := hdr.Kind(); kind {
		case bvvd.PingGateway:
			if err := p.conn.WriteToAddrPort(pkt, caddr); err != nil {
				return p.close(err)
			}
		case bvvd.PingForward:
			hdr.SetClient(caddr)
			if hdr.ForwardID().Valid() == nil {
				// boardcast forward
				var fs []*Forward
				{
					var msg msg.Message
					if err := msg.Decode(pkt); err != nil {
						p.config.logger.Warn(err.Error(), errorx.Trace(err))
						continue
					}
					loc, ok := msg.Payload.(bvvd.Location)
					if !ok {
						p.config.logger.Warn("PingForward payload", slog.Any("payload", msg.Payload))
						continue
					}
					msg.Encode(pkt.SetData(0))
					fs = p.fs.GetByLocation(loc)
				}

				h, d := pkt.Head(), pkt.Data()
				for _, f := range fs {
					if err = p.sender.WriteToAddrPort(pkt.Sets(h, d), f.Addr()); err != nil {
						return p.close(err)
					}
				}
			} else {
				f, err := p.fs.GetByForward(hdr.ForwardID())
				if err != nil {
					p.config.logger.Warn(err.Error(), errorx.Trace(err))
					continue
				}

				if err = p.sender.WriteToAddrPort(pkt, f.Addr()); err != nil {
					return p.close(err)
				}
			}
		case bvvd.PackLossClientUplink:
			var msg msg.Message
			if err := msg.Decode(pkt); err != nil {
				p.config.logger.Error(err.Error(), errorx.Trace(err))
				continue
			}
			msg.Payload = p.cs.Client(caddr).UplinkPL()

			if err := msg.Encode(pkt.SetData(0)); err != nil {
				p.config.logger.Error(err.Error(), errorx.Trace(err))
				continue
			}

			if err = p.conn.WriteToAddrPort(pkt, caddr); err != nil {
				return p.close(err)
			}
		case bvvd.PackLossGatewayDownlink, bvvd.PackLossGatewayUplink:
			f, err := p.fs.GetByForward(hdr.ForwardID())
			if err != nil {
				p.config.logger.Warn(err.Error(), errorx.Trace(err))
				continue
			}

			var msg msg.Message
			if err := msg.Decode(pkt); err != nil {
				p.config.logger.Warn("can't get forward", errorx.Trace(nil))
				continue
			}
			if kind == bvvd.PackLossGatewayDownlink {
				msg.Payload = f.DownlinkPL()
			} else {
				msg.Payload = f.UplinkPL()
			}
			if err := msg.Encode(pkt.SetData(0)); err != nil {
				p.config.logger.Warn("can't get forward", errorx.Trace(nil))
				continue
			}

			if err = p.conn.WriteToAddrPort(pkt, caddr); err != nil {
				return p.close(err)
			}

		case bvvd.Data:
			p.cs.Client(caddr).UplinkID(int(hdr.DataID()))
			if debug.Debug() {
				ok := checksum.ValidChecksum(pkt.DetachN(bvvd.Size), hdr.Proto(), hdr.Server())
				pkt.AttachN(bvvd.Size)
				require.True(test.T(), ok)
			}

			f, err := p.fs.GetByForward(hdr.ForwardID())
			if err != nil {
				p.config.logger.Warn(err.Error(), errorx.Trace(err))
				continue
			}

			hdr.SetClient(caddr)
			hdr.SetDataID(f.UplinkID())
			if debug.Debug() && rand.Int()%100 == 99 {
				continue // PackLossGatewayUplink
			}

			if err = p.sender.WriteToAddrPort(pkt, f.Addr()); err != nil {
				return p.close(err)
			}

			// query gateway --> forward pack loss
			if hdr.DataID() == 0xff {
				var msg = msg.Message{MsgID: 1} // todo: 多个forward时还是要设置有效ID
				msg.Kind = bvvd.PackLossGatewayUplink
				msg.ForwardID = hdr.ForwardID()
				msg.Client = hdr.Client()

				if err := msg.Encode(pkt.SetData(0)); err != nil {
					return p.close(err)
				}

				if err = p.sender.WriteToAddrPort(pkt, f.Addr()); err != nil {
					return p.close(err)
				}
			}
		default:
			p.config.logger.Warn("unknown kind from client", slog.String("kind", kind.String()), slog.String("client", caddr.String()))
		}
	}
}

func (p *Gateway) donwlinkService() (_ error) {
	var (
		pkt = packet.Make(p.config.MaxRecvBuff)
	)

	for {
		faddr, err := p.sender.ReadFromAddrPort(pkt.Sets(64, 0xffff))
		if err != nil {
			return p.close(err)
		} else if pkt.Data() == 0 {
			continue
		}
		p.speed.Downlink(pkt.Data() + 20 + 8)

		hdr := bvvd.Bvvd(pkt.Bytes())

		switch kind := hdr.Kind(); kind {
		case bvvd.Data:
			f, err := p.fs.GetByForward(hdr.ForwardID())
			if err != nil {
				p.config.logger.Warn(err.Error(), errorx.Trace(err))
				continue
			}
			f.DownlinkID(hdr.DataID())

			caddr := hdr.Client()
			hdr.SetDataID(p.cs.Client(caddr).DownlinkID())
			if debug.Debug() && rand.Int()%100 == 99 {
				continue // PackLossClientDownlink
			}

			if err = p.conn.WriteToAddrPort(pkt, caddr); err != nil {
				return p.close(err)
			}
		case bvvd.PackLossGatewayUplink:
			f, err := p.fs.GetByForward(hdr.ForwardID())
			if err != nil {
				p.config.logger.Warn(err.Error(), errorx.Trace(err))
				continue
			}

			var msg msg.Message
			if err := msg.Decode(pkt); err != nil {
				p.config.logger.Error(err.Error(), errorx.Trace(err))
				continue
			}

			if pl, ok := msg.Payload.(stats.PL); ok {
				f.SetUplinkPL(pl)
			} else {
				p.config.logger.Warn(fmt.Sprintf("unknown type %T", msg.Payload), errorx.Trace(nil))
			}
		case bvvd.PingForward:
			if err = p.conn.WriteToAddrPort(pkt, hdr.Client()); err != nil {
				return p.close(err)
			}
		default:
			p.config.logger.Warn("invalid kind from forward", slog.String("kind", kind.String()), slog.String("forward", faddr.String()))
			continue
		}
	}
}
