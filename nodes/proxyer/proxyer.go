//go:build linux
// +build linux

package proxyer

import (
	"log/slog"
	"math/rand"
	"net/netip"

	"github.com/lysShub/anton-planet-accelerator/conn"
	"github.com/lysShub/anton-planet-accelerator/nodes"
	"github.com/lysShub/anton-planet-accelerator/proto"
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
		hdr = &proto.Header{}
	)

	for {
		caddr, err := p.conn.ReadFromAddrPort(pkt.Sets(64, 0xffff))
		if err != nil {
			return p.close(err)
		} else if pkt.Data() == 0 {
			continue
		}

		head := pkt.Head()
		if err := hdr.Decode(pkt); err != nil {
			p.config.logger.Warn(err.Error(), errorx.Trace(err))
			continue
		}

		switch hdr.Kind {
		case proto.PingProxyer:
			err = p.conn.WriteToAddrPort(pkt.SetHead(head), caddr)
			if err != nil {
				return p.close(err)
			}
		case proto.PackLossClientUplink:
			pl := p.cs.Client(caddr).UplinkPL()
			pkt.SetHead(head).Append(pl.Encode()...)

			err = p.conn.WriteToAddrPort(pkt, caddr)
			if err != nil {
				return p.close(err)
			}
		case proto.PackLossProxyerDownlink:
			f := p.fs.Get(p.forward)
			if f == nil {
				p.config.logger.Warn("can't get forward")
				continue
			}
			pkt.SetHead(head).Append(f.DownlinkPL().Encode()...)

			err = p.conn.WriteToAddrPort(pkt, caddr)
			if err != nil {
				return p.close(err)
			}
		case proto.PackLossProxyerUplink:
			f := p.fs.Get(p.forward)
			if f == nil {
				p.config.logger.Warn("can't get forward")
				continue
			}
			pkt.SetHead(head).Append(f.UplinkPL().Encode()...)

			err = p.conn.WriteToAddrPort(pkt, caddr)
			if err != nil {
				return p.close(err)
			}
		case proto.PingForward:
			hdr.Client = caddr
			hdr.Encode(pkt)

			err = p.sender.WriteToAddrPort(pkt, p.forward)
			if err != nil {
				return p.close(err)
			}
		case proto.Data:
			p.cs.Client(caddr).UplinkID(int(hdr.ID))
			if debug.Debug() {
				ok := nodes.ValidChecksum(pkt, hdr.Proto, hdr.Server)
				require.True(test.T(), ok)
			}

			f := p.fs.Get(p.forward)
			if f == nil {
				p.config.logger.Warn("can't get forward", slog.String("forwar", p.forward.String()))
				continue
			}

			hdr.Client = caddr
			hdr.ID = f.UplinkID()
			hdr.Encode(pkt)

			if rand.Int()%100 == 99 {
				continue // PackLossProxyerUplink
			}

			err = p.sender.WriteToAddrPort(pkt, f.Addr())
			if err != nil {
				return p.close(err)
			}

			if hdr.ID == 0xff {
				hdr.Kind = proto.PackLossProxyerUplink
				hdr.Encode(pkt.SetData(0))

				err = p.sender.WriteToAddrPort(pkt, f.Addr())
				if err != nil {
					return p.close(err)
				}
			}
		default:
			panic("")
		}
	}
}

func (p *Proxyer) donwlinkService() (_ error) {
	var (
		pkt = packet.Make(p.config.MaxRecvBuff)
		hdr = &proto.Header{}
	)

	for {
		faddr, err := p.sender.ReadFromAddrPort(pkt.Sets(64, 0xffff))
		if err != nil {
			return p.close(err)
		} else if pkt.Data() == 0 {
			continue
		}

		head := pkt.Head()
		if err := hdr.Decode(pkt); err != nil {
			p.config.logger.Warn(err.Error(), errorx.Trace(err))
			continue
		}

		switch hdr.Kind {
		case proto.Data:
			f := p.fs.Get(faddr)
			if f == nil {
				p.config.logger.Warn("can't get forward", slog.String("forwar", p.forward.String()))
				continue
			}
			f.DownlinkID(hdr.ID)

			hdr.ID = p.cs.Client(hdr.Client).DownlinkID()
			hdr.Encode(pkt)

			if rand.Int()%100 == 99 {
				continue // PackLossClientDownlink
			}

			err = p.conn.WriteToAddrPort(pkt.SetHead(head), hdr.Client)
			if err != nil {
				return p.close(err)
			}
		case proto.PackLossProxyerUplink:
			f := p.fs.Get(faddr)
			if f == nil {
				p.config.logger.Warn("can't get forward", slog.String("forwar", p.forward.String()), errorx.Trace(nil))
				continue
			}

			var pl proto.PL
			if err := pl.Decode(pkt.Bytes()); err != nil {
				p.config.logger.Warn(err.Error(), errorx.Trace(nil))
			} else {
				f.SetUplinkPL(pl)
			}
		case proto.PingForward:
			err = p.conn.WriteToAddrPort(pkt.SetHead(head), hdr.Client)
			if err != nil {
				return p.close(err)
			}
		default:
			p.config.logger.Warn("invalid kind from forward", slog.String("kind", hdr.Kind.String()), slog.String("forward", faddr.String()))
			continue
		}
	}
}
