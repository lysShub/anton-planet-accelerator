//go:build linux
// +build linux

package proxyer

import (
	"fmt"
	"log/slog"
	"math/rand"

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

	sender conn.Conn
	fs     *Forwards

	closeErr errorx.CloseErr
}

func New(addr string, config *Config) (*Proxyer, error) {
	var p = &Proxyer{
		config: config.init(),
		cs:     NewClients(),
		fs:     NewForwards(),
	}
	for _, f := range config.Forwards {
		p.fs.Add(f.LocID, config.Forwards[0].Faddr)
	}

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
		case bvvd.PingForward:
			hdr.SetClient(caddr)
			f := p.fs.Get(hdr.LocID())
			if f == nil {
				p.config.logger.Warn("can't get forward", errorx.Trace(nil))
				continue
			}

			err = p.sender.WriteToAddrPort(pkt, f.Addr())
			if err != nil {
				return p.close(err)
			}
		case bvvd.PackLossClientUplink:
			var msg nodes.Message
			if err := msg.Decode(pkt); err != nil {
				p.config.logger.Error(err.Error(), errorx.Trace(err))
				continue
			}
			msg.Raw = p.cs.Client(caddr).UplinkPL()

			if err := msg.Encode(pkt.SetData(0)); err != nil {
				p.config.logger.Error(err.Error(), errorx.Trace(err))
				continue
			}

			if err = p.conn.WriteToAddrPort(pkt, caddr); err != nil {
				return p.close(err)
			}
		case bvvd.PackLossProxyerDownlink, bvvd.PackLossProxyerUplink:
			f := p.fs.Get(hdr.LocID())
			if f == nil {
				p.config.logger.Warn("can't get forward", errorx.Trace(nil))
				continue
			}

			var msg nodes.Message
			if err := msg.Decode(pkt); err != nil {
				p.config.logger.Warn("can't get forward", errorx.Trace(nil))
				continue
			}
			if kind == bvvd.PackLossProxyerDownlink {
				msg.Raw = f.DownlinkPL()
			} else {
				msg.Raw = f.UplinkPL()
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
				ok := nodes.ValidChecksum(pkt.DetachN(bvvd.Size), hdr.Proto(), hdr.Server())
				pkt.AttachN(bvvd.Size)
				require.True(test.T(), ok)
			}

			f := p.fs.Get(hdr.LocID())
			if f == nil {
				p.config.logger.Warn("can't get forward", errorx.Trace(nil))
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
				if err := (&nodes.Message{
					MsgID: 1, // todo: 多个forward时还是要设置有效ID
					Kind:  bvvd.PackLossProxyerUplink,
					LocID: hdr.LocID(),
				}).Encode(pkt.SetData(0)); err != nil {
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
			f := p.fs.Get(hdr.LocID())
			if f == nil {
				p.config.logger.Warn("can't get forward", errorx.Trace(nil))
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
			f := p.fs.Get(hdr.LocID())
			if f == nil {
				p.config.logger.Warn("can't get forward", errorx.Trace(nil))
				continue
			}

			var msg nodes.Message
			if err := msg.Decode(pkt); err != nil {
				p.config.logger.Error(err.Error(), errorx.Trace(err))
				continue
			}

			if pl, ok := msg.Raw.(nodes.PL); ok {
				f.SetUplinkPL(pl)
			} else {
				p.config.logger.Warn(fmt.Sprintf("unknown type %T", msg.Raw), errorx.Trace(nil))
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
