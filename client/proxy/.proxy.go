package proxy

import (
	"fmt"
	"github.com/lysShub/warthunder/client/divert"
	"github.com/lysShub/warthunder/ctx"
	"github.com/lysShub/warthunder/fudp"
	"net/netip"
	"sync"
)

/*
	处理逻辑:
		代理启动时，监听进程的socket事件, 便且获取进程已经创建udp conn table。socket事件需要一直监听，监听到
	对应的事件后要更新代理。获取udp conn table只在代理启动时进行，会将所有的udp conn都加入代理。

		我们通过laddr确定一个udp-conn。

		TODO: 一个代理udp-conn对应一个fudp-conn虚拟连接
*/

type Proxy struct {
	ch ch

	s map[*sniffer]struct{}

	pid int // WarThunder process ID

	m *sync.RWMutex
}

// NewProxy creates a new proxy, blocked until process is started.
func NewProxy(ctx ctx.Ctx, pid int) (*Proxy, error) {
	pid = 6060

	p := &Proxy{
		pid: pid,
		m:   &sync.RWMutex{},
	}

	go p.listenSocketEvent(ctx)

	return &Proxy{}, nil
}

// listenSocketEvent listens the process's socket BIND/LISTEN event.
func (p *Proxy) listenSocketEvent(ctx ctx.Ctx) {
	var hs [2]divert.Handle

	// listen BIND/CLOSE event
	go func(pid int) {
		var f = fmt.Sprintf("udp and (event=BIND or event=CLOSE) and processId=%d", pid)

		var err error
		hs[0], err = divert.Open(f, divert.LAYER_SOCKET, 123, divert.FLAG_SNIFF|divert.FLAG_READ_ONLY)
		if err != nil {
			ctx.Exception(err)
			return
		}

		var b = []byte{}
		var addr divert.Address
		for {
			_, addr, err = hs[0].Recv(b)
			if err != nil {
				ctx.Exception(err)
				return
			}

			if addr.Header.Event == divert.EVENT_SOCKET_BIND {
				s := addr.Socket()
				laddr := netip.AddrPortFrom(s.LocalAddr(), s.LocalPort)
				p.addProxyFilter(ctx, laddr)
			} else {
				s := addr.Socket()
				laddr := netip.AddrPortFrom(s.LocalAddr(), s.LocalPort)
				p.deleteProxyFilter(laddr)
			}
		}
	}(p.pid)

	// listen LISTEN event
	go func(pid int) {
		var f = fmt.Sprintf("udp and event=LISTEN and processId=%d", pid)

		var err error
		hs[1], err = divert.Open(f, divert.LAYER_SOCKET, 122, divert.FLAG_SNIFF|divert.FLAG_READ_ONLY)
		if err != nil {
			ctx.Exception(err)
			return
		}

		var b = []byte{}
		var addr divert.Address
		for {
			_, addr, err = hs[1].Recv(b)
			if err != nil {
				ctx.Exception(err)
				return
			}

			s := addr.Socket()
			laddr := netip.AddrPortFrom(s.LocalAddr(), s.LocalPort)
			p.addProxyFilter(ctx, laddr)
		}
	}(p.pid)

	go func() {
		select {
		case <-ctx.Done():
			if err := hs[0].Shutdown(divert.SHUTDOWN_RECV); err != nil {
				ctx.Exception(err)
			}
			if err := hs[1].Shutdown(divert.SHUTDOWN_RECV); err != nil {
				ctx.Exception(err)
			}
			return
		default:
		}
	}()
}

type sniffer struct {
	laddr netip.AddrPort
	h     divert.Handle

	ch ch
}

func newSniffer(laddr netip.AddrPort, ch ch, ctx ctx.Ctx) *sniffer {
	return &sniffer{
		laddr: laddr,
		ch:    ch,
	}
}

// TODO: need a base data proxer
func (s *sniffer) do(ctx ctx.Ctx) {
	var f = fmt.Sprintf("udp and outbound and localAddr=%s and localPort=%d", s.laddr.Addr(), s.laddr.Port())
	// TODO: pass close/shutdown
	var err error

	s.h, err = divert.Open(f, divert.LAYER_NETWORK, 1, divert.FLAG_SNIFF|divert.FLAG_RECV_ONLY)
	if err != nil {
		ctx.Exception(err)
		return
	}
	defer s.h.Close()

	var n int
	var addr divert.Address
	var u = fudp.NewUpack()
	for {
		n, addr, err = s.h.Recv(u.Data[:cap(u.Data)])
		if err != nil {
			ctx.Exception(err)
			return
		}

		// TODO: parse ip header
		// u.CAddr,u.SAddr =
		u.Data = u.Data[:n]
		s.ch.push(u)
		if false {
			fmt.Println(addr)
		}

		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}

func (s *sniffer) shutdown() {
	s.h.Shutdown(divert.SHUTDOWN_RECV)
}

func (s *sniffer) close() error {
	return s.h.Close()
}

func (p *Proxy) addProxyFilter(ctx ctx.Ctx, laddr netip.AddrPort) {
	if p.hasFilter(laddr) {
		return
	}

	sr := newSniffer(laddr, p.ch, ctx)
	p.m.Lock()
	p.s[sr] = struct{}{}
	p.m.Unlock()

	go sr.do(ctx)
}

func (p *Proxy) hasFilter(laddr netip.AddrPort) bool {
	p.m.RLock()
	defer p.m.RUnlock()

	for sr := range p.s {
		if sr.laddr == laddr {
			return true
		}
	}

	return false
}

func (p *Proxy) deleteProxyFilter(laddr netip.AddrPort) {
	p.m.Lock()
	defer p.m.Unlock()

	for sr := range p.s {
		if sr.laddr == laddr {
			sr.shutdown()
			delete(p.s, sr)
			return
		}
	}
}

func (p *Proxy) Read(u *fudp.Upack) {
	p.ch.pope(u)
}
