package client

import (
	stderr "errors"
	"net/netip"
	"sync"
	"sync/atomic"

	"github.com/lysShub/netkit/errorx"
	"github.com/pkg/errors"
)

type route struct {
	inited       atomic.Bool
	fixRouteMode bool

	// fixRoute 或者 autoRout 中的非PlayData, 都发送到default
	defaultGateway netip.AddrPort
	defaultForward netip.AddrPort

	mu     sync.RWMutex
	routes map[netip.Addr]entry

	routeProbe RouteProbe
	inflightMu sync.RWMutex
	inflight   map[netip.Addr]result
}

func newRoute(fixRoute bool) *route {
	return &route{
		fixRouteMode: fixRoute,

		routes:   map[netip.Addr]entry{},
		inflight: map[netip.Addr]result{},
	}
}

type entry struct {
	gateway netip.AddrPort
	forward netip.AddrPort
}

type RouteProbe interface {
	RouteProbe(saddr netip.Addr) (gaddr, faddr netip.AddrPort, err error)
}

func (r *route) Init(probe RouteProbe, defaultGateway, defaultForward netip.AddrPort) {
	if !r.inited.Swap(true) {
		r.defaultGateway = defaultGateway
		r.defaultForward = defaultForward

		r.routeProbe = probe
	}
}

func (r *route) Match(saddr netip.Addr, probe bool) (gaddr, faddr netip.AddrPort, err error) {
	if !r.inited.Load() {
		return netip.AddrPort{}, netip.AddrPort{}, errors.New("route not init")
	}
	if !probe || r.fixRouteMode {
		return r.defaultGateway, r.defaultForward, nil
	}

	r.mu.RLock()
	e, has := r.routes[saddr]
	r.mu.RUnlock()
	if has {
		return e.gateway, e.forward, nil
	}
	return r.probe(saddr)
}

func (r *route) probe(saddr netip.Addr) (gaddr, faddr netip.AddrPort, err error) {
	r.inflightMu.RLock()
	rest, has := r.inflight[saddr]
	r.inflightMu.RUnlock()
	if !has {
		r.inflightMu.Lock()
		r.inflight[saddr] = result{}
		r.inflightMu.Unlock()

		go r.probeRoute(saddr)
		err = errorx.WrapTemp(ErrRouteProbe)
	} else if rest.done {
		r.inflightMu.Lock()
		delete(r.inflight, saddr)
		r.inflightMu.Unlock()

		if rest.err != nil {
			err = errors.WithMessage(rest.err, saddr.String())
		} else {
			r.mu.RLock()
			e := r.routes[saddr]
			r.mu.RUnlock()

			gaddr, faddr = e.gateway, e.forward
		}
	} else {
		err = errorx.WrapTemp(ErrRouteProbing)
	}

	return gaddr, faddr, err
}

type result struct {
	done bool
	err  error
}

func (r *route) probeRoute(saddr netip.Addr) {
	gaddr, fid, err := r.routeProbe.RouteProbe(saddr)
	if err == nil {
		r.mu.Lock()
		r.routes[saddr] = entry{gaddr, fid}
		r.mu.Unlock()
	}

	// todo: inflight 可能会溢出，因为会优先查询routes, inflight可能永远无法删除已经完成的

	r.inflightMu.Lock()
	r.inflight[saddr] = result{err: err, done: true}
	r.inflightMu.Unlock()
}

var ErrRouteProbe = stderr.New("start route probe")
var ErrRouteProbing = stderr.New("route probing")
