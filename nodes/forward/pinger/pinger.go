package pinger

import (
	"log/slog"
	"math/rand"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lysShub/anton-planet-accelerator/bvvd"
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/rawsock/test"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/icmp"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

var once atomic.Bool

type Pinger struct {
	log  *slog.Logger
	buff chan<- Info

	cacheMu sync.RWMutex
	cache   map[netip.Addr]time.Duration

	inflightMu sync.RWMutex
	inflight   map[netip.Addr]*key

	conn *icmp.PacketConn

	closeErr errorx.CloseErr
}

type key struct {
	start time.Time
	infos []Info
}

type Info struct {
	Addr netip.Addr
	RTT  time.Duration

	Msg bvvd.Bvvd
}

func NewPinger(replayChan chan<- Info, log *slog.Logger) (*Pinger, error) {
	if !once.CompareAndSwap(false, true) {
		return nil, errors.New("require process singleton")
	}

	var p = &Pinger{
		log:      log,
		buff:     replayChan,
		cache:    map[netip.Addr]time.Duration{},
		inflight: map[netip.Addr]*key{},
	}
	var err error

	p.conn, err = icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return nil, err
	}

	go p.recvService()
	return p, nil
}

func (p *Pinger) close(cause error) error {
	return p.closeErr.Close(func() (errs []error) {
		if cause == nil {
			p.log.Error(cause.Error(), errorx.Trace(cause))
		} else {
			p.log.Info("pinger close")
		}

		errs = append(errs, cause)
		if p.conn != nil {
			errs = append(errs, errors.WithStack(p.conn.Close()))
		}
		return errs
	})
}

func (p *Pinger) Ping(info Info) error {
	if !info.Addr.Is4() {
		return errors.Errorf("only support ipv4 %s", info.Addr.String())
	}

	p.cacheMu.RLock()
	rtt, has := p.cache[info.Addr]
	p.cacheMu.RUnlock()
	if has {
		info.RTT = rtt
		p.buff <- info
		return nil
	}

	p.inflightMu.Lock()
	k, has := p.inflight[info.Addr]
	if has {
		k.infos = append(k.infos, info)
	}
	p.inflightMu.Unlock()
	if has {
		return nil
	}

	return p.send(info)
}

// with 16 byte payload
const size = header.ICMPv4PayloadOffset + 16

func (p *Pinger) send(info Info) error {
	var echo = make(header.ICMPv4, size)
	echo.SetType(header.ICMPv4Echo)
	echo.SetCode(0)
	echo.SetIdent(uint16(rand.Uint32()))
	echo.SetSequence(uint16(rand.Uint32()))
	echo.SetChecksum(^checksum.Checksum(echo, 0))
	if debug.Debug() {
		require.Equal(test.T(), uint16(0xffff), checksum.Checksum(echo, 0))
	}

	_, err := p.conn.WriteTo(echo, &net.IPAddr{IP: info.Addr.AsSlice()})
	if err != nil {
		return err
	}

	p.inflightMu.Lock()
	p.inflight[info.Addr] = &key{
		start: time.Now(),
		infos: []Info{info},
	}
	p.inflightMu.Unlock()
	return nil
}

func (p *Pinger) recvService() (_ error) {
	var b = make([]byte, size+header.IPv4MinimumSize)
	for i := uint8(0); ; i++ {
		_, rip, err := p.conn.ReadFrom(b)
		if err != nil {
			return p.close(err)
		}

		addr := netip.AddrFrom4([4]byte(rip.(*net.IPAddr).IP))

		p.inflightMu.RLock()
		k, has := p.inflight[addr]
		p.inflightMu.RUnlock()
		if has {
			p.inflightMu.Lock()
			delete(p.inflight, addr)
			p.inflightMu.Unlock()

			rtt := time.Since(k.start) + 1
			for _, info := range k.infos {
				info.RTT = rtt
				select {
				case p.buff <- info:
					p.log.Info("ping", slog.String("addr", addr.String()), slog.Duration("rtt", rtt))
				default:
					p.log.Error("pinger replay chan write block")
				}
			}
		}

		// remove not replay record in inflight
		if i == 0xff {
			p.inflightMu.Lock()
			for k, v := range p.inflight {
				if time.Since(v.start) > time.Second*3 {
					delete(p.inflight, k)
					p.log.Warn("ping not replay", slog.String("addr", k.String()))
				}
			}
			p.inflightMu.Unlock()
		}
	}
}

func (p *Pinger) Close() error { return p.close(nil) }
