//go:build linux
// +build linux

package proxyer

import (
	"log/slog"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"

	"github.com/lysShub/anton-planet-accelerator/nodes"
	"github.com/lysShub/anton-planet-accelerator/proto"
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/packet"
	"github.com/pkg/errors"
)

type Proxyer struct {
	config  *Config
	forward netip.AddrPort

	conn *net.UDPConn

	connStatsMu sync.RWMutex
	connStats   map[netip.AddrPort]*stats

	sender *net.UDPConn

	closeErr errorx.CloseErr
}

type stats struct {
	pl *nodes.PLStats
	id atomic.Uint32
}

func New(addr string, forward netip.AddrPort, config *Config) (*Proxyer, error) {
	var p = &Proxyer{
		config:  config.init(),
		forward: forward,
	}

	laddr, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	p.conn, err = net.ListenUDP("udp4", laddr)
	if err != nil {
		return nil, p.close(err)
	}

	p.sender, err = net.ListenUDP("udp4", nil)
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
		n, caddr, err := p.conn.ReadFromUDPAddrPort(pkt.Sets(64, 0xffff).Bytes())
		if err != nil {
			return p.close(err)
		}
		pkt.SetData(n)

		if err := hdr.Decode(pkt); err != nil {
			p.config.logger.Warn(err.Error(), errorx.Trace(err))
			continue
		}
		hdr.Client = caddr
		if err := hdr.Encode(pkt); err != nil {
			p.config.logger.Warn(err.Error(), errorx.Trace(err))
			continue
		}
		stats := p.statsUp(caddr, hdr.ID)

		switch hdr.Kind {
		case proto.PingProxyer:
			_, err = p.conn.WriteToUDPAddrPort(pkt.Bytes(), caddr)
			if err != nil {
				return p.close(err)
			}
		case proto.PacketLossProxyer:
			pkt.Append(proto.PL(stats.pl.PL()).Encode()...)
			_, err = p.conn.WriteToUDPAddrPort(pkt.Bytes(), caddr)
			if err != nil {
				return p.close(err)
			}
		default:
			_, err = p.sender.WriteToUDPAddrPort(pkt.Bytes(), p.forward)
			if err != nil {
				return p.close(err)
			}
		}
	}
}

func (p *Proxyer) statsUp(caddr netip.AddrPort, id uint8) *stats {
	p.connStatsMu.RLock()
	s, has := p.connStats[caddr]
	p.connStatsMu.RUnlock()
	if !has {
		s = &stats{
			pl: &nodes.PLStats{},
		}
		p.connStatsMu.Lock()
		p.connStats[caddr] = s
		p.connStatsMu.Unlock()
	}
	s.pl.Pack(int(id))
	return s
}

func (p *Proxyer) donwlinkService() (_ error) {
	var (
		pkt = packet.Make(p.config.MaxRecvBuff)
		hdr = &proto.Header{}
	)

	for {
		n, _, err := p.sender.ReadFromUDPAddrPort(pkt.Sets(64, 0xffff).Bytes())
		if err != nil {
			return p.close(err)
		}
		pkt.SetData(n)

		hdr.ID = p.statsDown(hdr.Client)
		if err := hdr.Decode(pkt); err != nil {
			p.config.logger.Warn(err.Error(), errorx.Trace(err))
			continue
		}
		pkt.AttachN(proto.HeaderSize)

		_, err = p.conn.WriteToUDPAddrPort(pkt.Bytes(), hdr.Client)
		if err != nil {
			return p.close(err)
		}
	}
}

func (p *Proxyer) statsDown(caddr netip.AddrPort) uint8 {
	p.connStatsMu.RLock()
	s, has := p.connStats[caddr]
	p.connStatsMu.RUnlock()
	if !has {
		s = &stats{
			pl: &nodes.PLStats{},
		}
		p.connStatsMu.Lock()
		p.connStats[caddr] = s
		p.connStatsMu.Unlock()
	}
	return uint8(s.id.Add(1))
}
