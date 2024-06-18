//go:build linux
// +build linux

package proxyer

import (
	"log/slog"
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
	config  *Config
	forward netip.AddrPort

	conn conn.Conn

	sender conn.Conn

	ss *StatsSet

	closeErr errorx.CloseErr
}

func New(addr string, forward netip.AddrPort, config *Config) (*Proxyer, error) {
	var p = &Proxyer{
		config:  config.init(),
		forward: forward,
		ss:      NewStatsSet(),
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
		hdr = &proto.Header{}
	)

	for {
		caddr, err := p.conn.ReadFromAddrPort(pkt.Sets(64, 0xffff))
		if err != nil {
			return p.close(err)
		} else if pkt.Data() == 0 {
			continue
		}

		if err := hdr.Decode(pkt); err != nil {
			p.config.logger.Warn(err.Error(), errorx.Trace(err))
			continue
		}
		if debug.Debug() && hdr.Kind == proto.Data {
			ok := nodes.ValidChecksum(pkt, hdr.Proto, hdr.Server)
			require.True(test.T(), ok)
		}

		switch hdr.Kind {
		case proto.PingProxyer:
			hdr.Encode(pkt)
			err = p.conn.WriteToAddrPort(pkt, caddr)
			if err != nil {
				return p.close(err)
			}
		case proto.PackLossUplink:
			hdr.Encode(pkt)
			pl := p.ss.Stats(caddr).UplinkPL()
			pkt.Append(pl.Encode()...)

			err = p.conn.WriteToAddrPort(pkt, caddr)
			if err != nil {
				return p.close(err)
			}
		default:
			if hdr.Kind == proto.Data {
				p.ss.Stats(caddr).Uplink(int(hdr.ID))
				print(hdr.ID, ", ")
			}

			hdr.Client = caddr
			hdr.ID = 0 // proxyer-forward之间的丢包还没加上
			hdr.Encode(pkt)

			err = p.sender.WriteToAddrPort(pkt, p.forward)
			if err != nil {
				return p.close(err)
			}
		}
	}
}

func (p *Proxyer) donwlinkService() (_ error) {
	var (
		pkt = packet.Make(p.config.MaxRecvBuff)
		hdr = &proto.Header{}
	)

	for {
		_, err := p.sender.ReadFromAddrPort(pkt.Sets(64, 0xffff))
		if err != nil {
			return p.close(err)
		} else if pkt.Data() == 0 {
			continue
		}

		if err := hdr.Decode(pkt); err != nil {
			p.config.logger.Warn(err.Error(), errorx.Trace(err))
			continue
		}
		hdr.ID = uint8(p.ss.Stats(hdr.Client).Downlink())
		hdr.Encode(pkt) // todo: optimize

		err = p.conn.WriteToAddrPort(pkt, hdr.Client)
		if err != nil {
			return p.close(err)
		}
	}
}
