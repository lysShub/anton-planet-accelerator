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

func (p *Gateway) AddForward(faddr netip.AddrPort) error {
	if !p.start.Load() {
		return errors.Errorf("gateway not start")
	}

	coord, err := internal.IPCoord(faddr.Addr())
	if err != nil {
		return err
	}
	loc, offset := bvvd.Locations.Match(coord)
	if offset > 500 {
		return errors.Errorf("forward %s offset location %s %fkm", faddr.Addr(), loc, offset)
	}

	if err := p.fs.Add(faddr, loc); err != nil {
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
		} else if pkt.Data() < bvvd.Size {
			continue
		}
		p.speed.Uplink(pkt.Data() + 20 + 8)

		hdr := bvvd.Bvvd(pkt.Bytes())
		hdr.SetClient(caddr)
		if hdr.Kind() != bvvd.Data && pkt.Data() < msg.MinSize {
			p.config.logger.Warn("too small", slog.Int("size", pkt.Data()), slog.String("client", caddr.String()), slog.String("kind", hdr.Kind().String()))
			continue
		}

		switch kind := hdr.Kind(); kind {
		case bvvd.PingGateway:
			if err := p.conn.WriteToAddrPort(pkt, caddr); err != nil {
				return p.close(err)
			}
		case bvvd.PingForward, bvvd.PingServer:
			var faddrs []netip.AddrPort
			if hdr.Forward().Addr().IsUnspecified() {
				faddrs = p.fs.Forwards() // boardcast
			} else {
				faddrs = []netip.AddrPort{hdr.Forward()}
			}
			for _, faddr := range faddrs {
				hdr.SetForward(faddr)
				if err = p.conn.WriteToAddrPort(pkt, faddr); err != nil {
					return p.close(err)
				}
			}
		case bvvd.PackLossClientUplink:
			pl := p.cs.Client(caddr).UplinkPL()
			if err := msg.Message(pkt.Bytes()).SetPayload(&pl); err != nil {
				p.config.logger.Warn(err.Error(), errorx.Trace(err))
				continue
			}

			if err = p.conn.WriteToAddrPort(pkt, caddr); err != nil {
				return p.close(err)
			}
		case bvvd.PackLossGatewayUplink:
			if err := p.sender.WriteToAddrPort(pkt, hdr.Forward()); err != nil {
				return p.close(err)
			}
		case bvvd.PackLossGatewayDownlink:
			f, err := p.fs.Get(hdr.Forward())
			if err != nil {
				p.config.logger.Warn(err.Error(), errorx.Trace(err))
				continue
			}
			pl := f.DownlinkPL()

			if err := msg.Message(pkt.Bytes()).SetPayload(&pl); err != nil {
				p.config.logger.Warn(err.Error(), errorx.Trace(err))
				continue
			}

			if err = p.conn.WriteToAddrPort(pkt, caddr); err != nil {
				return p.close(err)
			}
		case bvvd.Data:
			// todo: 也许应该保留clinet data id, 现在PackLossGateway是共用的，可能会不准确
			p.cs.Client(caddr).UplinkID(int(hdr.DataID()))

			if debug.Debug() {
				ok := checksum.ValidChecksum(pkt.DetachN(bvvd.Size), uint8(hdr.Proto()), hdr.Server())
				pkt.AttachN(bvvd.Size)
				require.True(test.T(), ok)
			}

			f, err := p.fs.Get(hdr.Forward())
			if err != nil {
				p.config.logger.Warn(err.Error(), errorx.Trace(err))
				continue
			}

			hdr.SetDataID(f.UplinkID())
			if debug.Debug() && rand.Int()%100 == 99 {
				continue // PackLossGatewayUplink
			}

			if err = p.sender.WriteToAddrPort(pkt, f.Addr()); err != nil {
				return p.close(err)
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
			f, err := p.fs.Get(hdr.Forward())
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
			if err = p.conn.WriteToAddrPort(pkt, hdr.Client()); err != nil {
				return p.close(err)
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
