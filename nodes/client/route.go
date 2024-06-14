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
	eq := r.active == proxyer
	r.mu.RUnlock()

	if !eq && proxyer.IsValid() {
		r.mu.Lock()
		r.active = proxyer
		r.mu.Unlock()
	}
	return proxyer
}

func (r *route) Add(server netip.Addr, proxyer netip.AddrPort) bool {
	r.mu.RLock()
	_, has := r.cache[server]
	r.mu.RUnlock()
	if !has {
		r.mu.Lock()
		r.cache[server] = proxyer
		r.mu.Unlock()

		// println("route", server.String(), proxyer.String())
	}
	return !has
}

func (r *route) ActiveProxyer() netip.AddrPort {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.active
}
