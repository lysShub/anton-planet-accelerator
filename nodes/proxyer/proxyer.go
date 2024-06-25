//go:build linux
// +build linux

package proxyer

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
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/rawsock/test"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

type Proxyer struct {
	config *Config
	start  atomic.Bool

	conn conn.Conn
	cs   *Clients

	sender  conn.Conn
	fs      *Forwards
	msgbuff *nodes.Heap[nodes.Message]

	closeErr errorx.CloseErr
}

func New(addr string, config *Config) (*Proxyer, error) {
	var p = &Proxyer{
		config:  config.init(),
		cs:      NewClients(),
		fs:      NewForwards(),
		msgbuff: nodes.NewHeap[nodes.Message](4),
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

func (p *Proxyer) Serve() (err error) {
	if p.start.Swap(true) {
		return errors.Errorf("proxyer started")
	}
	p.config.logger.Info("start", slog.String("listen", p.conn.LocalAddr().String()), slog.Bool("debug", debug.Debug()))

	go p.donwlinkService()
	return p.close(p.uplinkService())
}

func (p *Proxyer) AddForward(faddr netip.AddrPort) error {
	if !p.start.Load() {
		return errors.Errorf("proxyer not start")
	}

	// get forward locatinon by PingForward
	var pkt = packet.Make()
	var msg = nodes.Message{MsgID: rand.Uint32()}
	msg.Kind = bvvd.PingForward
	if err := msg.Encode(pkt); err != nil {
		return err
	}
	if err := p.sender.WriteToAddrPort(pkt, faddr); err != nil {
		return err
	}

	msg, pop := p.msgbuff.PopByDeadline(
		func(e nodes.Message) (pop bool) { return e.MsgID == msg.MsgID },
		time.Now().Add(time.Second*3),
	)
	if !pop {
		return errors.Errorf("add forward %s timeout", faddr.String())
	}
	loc, err := p.fs.Add(msg.LocID, faddr)
	if err != nil {
		return err
	}
	p.config.logger.Info("add forward success", slog.String("forward", faddr.String()), slog.String("LocID", loc.String()))
	return nil
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
			if hdr.LocID().ID() == 0 {
				// boardcast forward
				fs, err := p.fs.GetByLoc(hdr.LocID())
				if err != nil {
					p.config.logger.Warn(err.Error(), errorx.Trace(err))
					continue
				}

				for _, f := range fs {
					if err = p.sender.WriteToAddrPort(pkt, f.Addr()); err != nil {
						return p.close(err)
					}
				}
			} else {
				f, err := p.fs.GetByLocID(hdr.LocID())
				if err != nil {
					p.config.logger.Warn(err.Error(), errorx.Trace(err))
					continue
				}

				if err = p.sender.WriteToAddrPort(pkt, f.Addr()); err != nil {
					return p.close(err)
				}
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
			f, err := p.fs.GetByLocID(hdr.LocID())
			if err != nil {
				p.config.logger.Warn(err.Error(), errorx.Trace(err))
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

			f, err := p.fs.GetByLocID(hdr.LocID())
			if err != nil {
				p.config.logger.Warn(err.Error(), errorx.Trace(err))
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

			// query proxyer --> forward pack loss
			if hdr.DataID() == 0xff {
				var msg = nodes.Message{MsgID: 1} // todo: 多个forward时还是要设置有效ID
				msg.Kind = bvvd.PackLossProxyerUplink
				msg.LocID = hdr.LocID()
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
			f, err := p.fs.GetByLocID(hdr.LocID())
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
		case bvvd.PackLossProxyerUplink:
			f, err := p.fs.GetByLocID(hdr.LocID())
			if err != nil {
				p.config.logger.Warn(err.Error(), errorx.Trace(err))
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
			if hdr.Client().Addr().IsUnspecified() {
				// is proxyer add forward PingForard, not set Client
				var msg nodes.Message
				if err := msg.Decode(pkt); err != nil {
					p.config.logger.Warn(err.Error(), errorx.Trace(nil))
					continue
				}
				p.msgbuff.MustPut(msg)
			} else {
				f, err := p.fs.GetByFaddr(faddr)
				if err != nil {
					p.config.logger.Warn(err.Error(), errorx.Trace(err))
					continue
				}
				// indicate forward, client will send Data with this LocID, if this link is fastest.
				hdr.SetLocID(f.LocID())

				if err = p.conn.WriteToAddrPort(pkt, hdr.Client()); err != nil {
					return p.close(err)
				}
			}
		default:
			p.config.logger.Warn("invalid kind from forward", slog.String("kind", kind.String()), slog.String("forward", faddr.String()))
			continue
		}
	}
}
