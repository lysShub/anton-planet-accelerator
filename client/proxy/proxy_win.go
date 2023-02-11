package proxy

import (
	"fmt"
	"net"
	"net/netip"
	"sync"

	"github.com/lysShub/warthunder/client/divert"
	"github.com/lysShub/warthunder/context"
	"github.com/lysShub/warthunder/helper"
	"github.com/lysShub/warthunder/util"
)

type proxy struct {
	pid       uint32
	proxyConn net.Conn

	connedUDPTable  *acceptTable
	unconnUDPTable  *acceptTable
	acceptPidCancel context.CancelFunc

	proxyTable *proxyTable

	m *sync.RWMutex
}

func newProxy(ctx context.Ctx, pid uint32, proxyConn net.Conn) {
	var p = &proxy{
		pid:       pid,
		proxyConn: proxyConn,

		connedUDPTable:  newAcceptTable(),
		unconnUDPTable:  newAcceptTable(),
		acceptPidCancel: nil,

		proxyTable: newProxyTable(),

		m: &sync.RWMutex{},
	}

	go p.acceptPid(ctx, pid)

	ut, err := util.GetUDPTableByPid(pid)
	if err != nil {
		ctx.Fatal(err)
		return
	}
	if len(ut) > 0 {
		for _, u := range ut {
			if u.Connected() {
				go p.acceptConned(ctx, u.Addr())
			} else {
				go p.acceptUnconn(ctx, u.Addr())
			}
		}
	}
}

func (p *proxy) acceptConned(ctx context.Ctx, laddr netip.AddrPort) {
	// just listen a udp packet
	if p.connedUDPTable.has(laddr) {
		return
	}

	var f = fmt.Sprintf("udp and outbound and localAddr=%s and localPort=%d", laddr.Addr().String(), laddr.Port())
	h, err := divert.Open(f, divert.LAYER_NETWORK, 11, divert.FLAG_READ_ONLY)
	if err != nil {
		ctx.Fatal(err)
		return
	}
	ctx, cancel := context.WithCancel(ctx)
	p.connedUDPTable.add(laddr, func() {
		h.Close()
		cancel()
	})
	defer func() { p.connedUDPTable.del(laddr) }()

	var b []byte = make([]byte, 65535)
	var n int
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, _, err = h.Recv(b)
		if err != nil {
			ctx.Fatal(err)
			return
		}
		if n > 0 {
			// TODO: parse ip packet
			var s sock
			p.addProxy(ctx, s)

			return
		}
	}
}

func (p *proxy) acceptUnconn(ctx context.Ctx, laddr netip.AddrPort) {
	// listen a udp packet, and send a udp packet to remote
	if p.unconnUDPTable.has(laddr) {
		return
	}

	var f = fmt.Sprintf("udp and outbound and localAddr=%s and localPort=%d", laddr.Addr().String(), laddr.Port())
	// TODO: 优先级问题
	h, err := divert.Open(f, divert.LAYER_NETWORK, 11, divert.FLAG_READ_ONLY)
	if err != nil {
		ctx.Fatal(err)
		return
	}
	ctx, cancel := context.WithCancel(ctx)
	p.unconnUDPTable.add(laddr, func() {
		h.Close()
		cancel()
	})
	defer func() { p.unconnUDPTable.del(laddr) }()

	var b []byte = make([]byte, 65535)
	var n int
	for {
		n, _, err = h.Recv(b)
		if err != nil {
			ctx.Fatal(err)
			return
		}
		if n > 0 {
			// TODO: parse ip packet
			var s sock
			p.addProxy(ctx, s)

			// TODO: send a udp packet to remote
			var u helper.Ipack
			p.proxyConn.Write(u)

			return
		}
	}
}

func (p *proxy) acceptPid(ctx context.Ctx, pid uint32) {
	// listen new udp-conn event by pid

	var f = fmt.Sprintf("pid=%d and udp and outbound", pid)
	h, err := divert.Open(f, divert.LAYER_FLOW, 11, divert.FLAG_READ_ONLY)
	if err != nil {
		ctx.Fatal(err)
		return
	}
	ctx, cancel := context.WithCancel(ctx)
	p.acceptPidCancel = func() {
		h.Close()
		cancel()
	}

	var b []byte = []byte{}
	var addr divert.Address
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_, addr, err = h.Recv(b)
		if err != nil {
			ctx.Fatal(err)
			return
		}

		fl := addr.Flow()
		s := sock{
			proto: fl.Protocol,
			laddr: fl.LocalAddr(),
			raddr: fl.RemoteAddr(),
		}
		switch addr.Header.Event {
		case divert.EVENT_FLOW_ESTABLISHED:
			p.addProxy(ctx, s)
		case divert.EVENT_FLOW_DELETED:
			p.delProxy(ctx, s)
		default:
			panic("unknown event")
		}

	}
}

func (p *proxy) addProxy(ctx context.Ctx, s sock) {
	if p.proxyTable.has(s) {
		return
	}

	// TODO: protocol
	var f = fmt.Sprintf("udp and outbound and localAddr=%s and localPort=%d and remoteAddr=%s and remotePort=%d", s.laddr.Addr().String(), s.laddr.Port(), s.raddr.Addr().String(), s.raddr.Port())

	h, err := divert.Open(f, divert.LAYER_NETWORK, 11, divert.FLAG_READ_ONLY)
	if err != nil {
		ctx.Fatal(err)
		return
	}
	ctx, cancel := context.WithCancel(ctx)
	p.m.Lock()
	p.proxyTable.add(s, func() {
		h.Close()
		cancel()
	})
	p.m.Unlock()
	go p.proxy(ctx, h, s)
}

func (p *proxy) delProxy(ctx context.Ctx, s sock) {
	p.m.Lock()
	defer p.m.Unlock()

	p.connedUDPTable.del(s.laddr)
	p.proxyTable.del(s)
}

func (p *proxy) proxy(ctx context.Ctx, h divert.Handle, s sock) {
	defer func() { p.proxyTable.del(s) }()

	var da []byte = make([]byte, 65535)
	var err error
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_, _, err = h.Recv(da)
		if err != nil {
			ctx.Fatal(err)
			return
		}

		// TODO: parse ip packet
		var s sock
		p.addProxy(ctx, s)

		// TODO: send a udp packet to remote
		var u helper.Ipack
		p.proxyConn.Write(u)

	}
}

type acceptTable struct {
	t map[netip.AddrPort]context.CancelFunc
	m *sync.RWMutex
}

func newAcceptTable() *acceptTable {
	return &acceptTable{
		t: make(map[netip.AddrPort]context.CancelFunc),
		m: &sync.RWMutex{},
	}
}

func (t *acceptTable) has(addr netip.AddrPort) bool {
	t.m.RLock()
	defer t.m.RUnlock()

	_, ok := t.t[addr]
	return ok
}

func (t *acceptTable) add(addr netip.AddrPort, cancel context.CancelFunc) bool {
	t.m.Lock()
	defer t.m.Unlock()

	if _, ok := t.t[addr]; !ok {
		t.t[addr] = cancel

		return true
	}
	return false
}

func (t *acceptTable) del(addr netip.AddrPort) bool {
	t.m.Lock()
	defer t.m.Unlock()

	if cancel, ok := t.t[addr]; ok {
		if cancel != nil {
			cancel()
		}
		delete(t.t, addr)

		return true
	}

	return false
}

func (t *acceptTable) delAll() {
	t.m.Lock()
	defer t.m.Unlock()

	for _, cancel := range t.t {
		if cancel != nil {
			cancel()
		}
	}
	t.t = make(map[netip.AddrPort]context.CancelFunc)
}

type proxyTable struct {
	t map[sock]context.CancelFunc
	m *sync.RWMutex
}

func newProxyTable() *proxyTable {
	return &proxyTable{
		t: make(map[sock]context.CancelFunc),
		m: &sync.RWMutex{},
	}
}

func (t *proxyTable) has(s sock) bool {
	t.m.RLock()
	defer t.m.RUnlock()

	_, ok := t.t[s]
	return ok
}

func (t *proxyTable) add(s sock, cancel context.CancelFunc) bool {
	t.m.Lock()
	defer t.m.Unlock()

	if _, ok := t.t[s]; !ok {
		t.t[s] = cancel

		return true
	}
	return false
}

func (t *proxyTable) del(s sock) bool {
	t.m.Lock()
	defer t.m.Unlock()

	if cancel, ok := t.t[s]; ok {
		if cancel != nil {
			cancel()
		}
		delete(t.t, s)

		return true
	}

	return false
}

func (t *proxyTable) delAll() {
	t.m.Lock()
	defer t.m.Unlock()

	for _, cancel := range t.t {
		if cancel != nil {
			cancel()
		}
	}
	t.t = make(map[sock]context.CancelFunc)
}
