package client

import (
	"net/netip"
	"sync"
)

type route struct {
	mu     sync.RWMutex
	cache  map[netip.Addr]netip.AddrPort // server-addr:proxyer-addr
	active netip.AddrPort
}

func NewRoute(defaultProxyer netip.AddrPort) *route {
	var r = &route{
		cache:  map[netip.Addr]netip.AddrPort{},
		active: defaultProxyer,
	}
	return r
}

func (r *route) Next(server netip.Addr) (proxyer netip.AddrPort) {
	r.mu.RLock()
	proxyer = r.cache[server]
	r.mu.RUnlock()
	r.active = proxyer
	return proxyer
}

func (r *route) Add(server netip.Addr, proxyer netip.AddrPort) {
	r.mu.RLock()
	_, has := r.cache[server]
	r.mu.RUnlock()
	if !has {
		r.mu.Lock()
		r.cache[server] = proxyer
		r.mu.Unlock()
	}
}

func (r *route) ActiveProxyer() netip.AddrPort {
	return r.active
}
