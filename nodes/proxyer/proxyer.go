//go:build linux
// +build linux

package proxyer

import (
	"log/slog"
	"math/rand"
	"net/netip"

	"github.com/lysShub/anton-planet-accelerator/bvvd"
	"github.com/lysShub/anton-planet-accelerator/conn"
	"github.com/lysShub/anton-planet-accelerator/nodes"
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/rawsock/test"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

type Proxyer struct {
	config *Config

	conn conn.Conn
	cs   *Clients

	sender  conn.Conn
	forward netip.AddrPort // todo: 临时的
	fs      *Forwards

	closeErr errorx.CloseErr
}

func New(addr string, forward netip.AddrPort, config *Config) (*Proxyer, error) {
	var p = &Proxyer{
		config:  config.init(),
		cs:      NewClients(),
		forward: forward,
		fs:      NewForwards(),
	}
	p.fs.Add(forward) // todo: temporary

	err := nodes.DisableOffload(config.logger)
	if err != nil {
		return nil, err
	}

	p.conn, err = conn.Bind(nodes.ProxyerNetwork, addr)
	if err != nil {
		return nil, p.close(err)
	}

	p.sender, err = conn.Bind(nodes.ForwardNetwork, "")
	if err != nil {
		return nil, p.close(err)
	}

	return p, nil
}

func (p *Proxyer) close(cause error) error {
	cause = errors.WithStack(cause)
	if cause != nil {
		p.config.logger.Error(cause.Error(), errorx.Trace(cause))
	} else {
		p.config.logger.Info("close")
	}
	return p.closeErr.Close(func() (errs []error) {
		errs = append(errs, cause)
		if p.conn != nil {
			errs = append(errs, p.conn.Close())
		}
		if p.sender != nil {
			errs = append(errs, p.sender.Close())
		}
		return errs
	})
}

func (p *Proxyer) Serve() error {
	p.config.logger.Info("start", slog.String("listen", p.conn.LocalAddr().String()), slog.Bool("debug", debug.Debug()))
	go p.donwlinkService()
	return p.close(p.uplinkService())
}

func (p *Proxyer) uplinkService() (_ error) {
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

		hdr := bvvd.Bvvd(pkt.Bytes())

		switch kind := hdr.Kind(); kind {
		case bvvd.PingProxyer:
			if err := p.conn.WriteToAddrPort(pkt, caddr); err != nil {
				return p.close(err)
			}
		case bvvd.PackLossClientUplink:
			pl := p.cs.Client(caddr).UplinkPL()
			pkt.Append(pl.Encode()...)

			if err = p.conn.WriteToAddrPort(pkt, caddr); err != nil {
				return p.close(err)
			}
		case bvvd.PackLossProxyerDownlink:
			f := p.fs.Get(p.forward)
			if f == nil {
				p.config.logger.Warn("can't get forward")
				continue
			}
			pkt.Append(f.DownlinkPL().Encode()...)

			if err = p.conn.WriteToAddrPort(pkt, caddr); err != nil {
				return p.close(err)
			}
		case bvvd.PackLossProxyerUplink:
			f := p.fs.Get(p.forward)
			if f == nil {
				p.config.logger.Warn("can't get forward")
				continue
			}
			pkt.Append(f.UplinkPL().Encode()...)

			if err = p.conn.WriteToAddrPort(pkt, caddr); err != nil {
				return p.close(err)
			}
		case bvvd.PingForward:
			hdr.SetClient(caddr)
			err = p.sender.WriteToAddrPort(pkt, p.forward)
			if err != nil {
				return p.close(err)
			}
		case bvvd.Data:
			p.cs.Client(caddr).UplinkID(int(hdr.DataID()))
			if debug.Debug() {
				ok := nodes.ValidChecksum(pkt.DetachN(bvvd.Size), hdr.Proto(), hdr.Server())
				pkt.AttachN(bvvd.Size)
				require.True(test.T(), ok)
			}

			f := p.fs.Get(p.forward)
			if f == nil {
				p.config.logger.Warn("can't get forward", slog.String("forwar", p.forward.String()))
				continue
			}

			hdr.SetClient(caddr)
			hdr.SetDataID(f.UplinkID())
			if debug.Debug() && rand.Int()%100 == 99 {
				continue // PackLossProxyerUplink
			}

			if err = p.sender.WriteToAddrPort(pkt, f.Addr()); err != nil {
				return p.close(err)
			}

			// query proxyer-->forward pack loss
			if hdr.DataID() == 0xff {
				hdr.SetDataID(0)
				hdr.SetKind(bvvd.PackLossProxyerUplink)

				if err = p.sender.WriteToAddrPort(pkt, f.Addr()); err != nil {
					return p.close(err)
				}
			}
		default:
			p.config.logger.Warn("unknown kind from client", slog.String("kind", kind.String()), slog.String("client", caddr.String()))
		}
	}
}

func (p *Proxyer) donwlinkService() (_ error) {
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

		hdr := bvvd.Bvvd(pkt.Bytes())

		switch kind := hdr.Kind(); kind {
		case bvvd.Data:
			f := p.fs.Get(faddr)
			if f == nil {
				p.config.logger.Warn("can't get forward", slog.String("forwar", p.forward.String()))
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
		case bvvd.PackLossProxyerUplink:
			f := p.fs.Get(faddr)
			if f == nil {
				p.config.logger.Warn("can't get forward", slog.String("forwar", p.forward.String()), errorx.Trace(nil))
				continue
			}

			var pl nodes.PL
			if err := pl.Decode(pkt.Bytes()); err != nil {
				p.config.logger.Warn(err.Error(), errorx.Trace(nil))
			} else {
				f.SetUplinkPL(pl)
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
