package client

import (
	"encoding/json"
	stderr "errors"
	"fmt"
	"net/http"
	"net/netip"
	"sync"
	"sync/atomic"

	"github.com/jftuga/geodist"
	"github.com/lysShub/anton-planet-accelerator/bvvd"
	"github.com/lysShub/netkit/errorx"
	"github.com/pkg/errors"
)

type route struct {
	inited       atomic.Bool
	fixRouteMode bool

	// fixRoute 或者 autoRout 中的非PlayData, 都发送到default
	defaultProxyer netip.AddrPort
	defaultForward bvvd.ForwardID

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
	paddr   netip.AddrPort
	forward bvvd.ForwardID
}

type RouteProbe interface {
	RouteProbe(saddr netip.Addr) (paddr netip.AddrPort, forward bvvd.ForwardID, err error)
}

func (r *route) Init(probe RouteProbe, defaultProxyer netip.AddrPort, defaultForward bvvd.ForwardID) {
	if !r.inited.Swap(true) {
		r.defaultProxyer = defaultProxyer
		r.defaultForward = defaultForward

		r.routeProbe = probe
	}
}

func (r *route) Match(saddr netip.Addr, probe bool) (paddr netip.AddrPort, forward bvvd.ForwardID, err error) {
	if !r.inited.Load() {
		return netip.AddrPort{}, 0, errors.New("route not init")
	}
	if !probe || r.fixRouteMode {
		return r.defaultProxyer, r.defaultForward, nil
	}

	r.mu.RLock()
	e, has := r.routes[saddr]
	r.mu.RUnlock()
	if has {
		return e.paddr, e.forward, nil
	}
	return r.probe(saddr)
}

func (r *route) probe(saddr netip.Addr) (paddr netip.AddrPort, forward bvvd.ForwardID, err error) {
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

			paddr, forward = e.paddr, e.forward
		}
	} else {
		err = errorx.WrapTemp(ErrRouteProbing)
	}

	return paddr, forward, err
}

type result struct {
	done bool
	err  error
}

func (r *route) probeRoute(saddr netip.Addr) {
	paddr, fid, err := r.routeProbe.RouteProbe(saddr)
	if err == nil {
		r.mu.Lock()
		r.routes[saddr] = entry{paddr, fid}
		r.mu.Unlock()
	}

	// todo: inflight 可能会溢出，因为会优先查询routes, inflight可能永远无法删除已经完成的

	r.inflightMu.Lock()
	r.inflight[saddr] = result{err: err, done: true}
	r.inflightMu.Unlock()
}

var ErrRouteProbe = stderr.New("start route probe")
var ErrRouteProbing = stderr.New("route probing")

// todo: temp, should from admin
func IPCoord(addr netip.Addr) (geodist.Coord, error) {
	if !addr.Is4() {
		return geodist.Coord{}, errors.New("only support ipv4")
	}

	url := fmt.Sprintf(`http://ip-api.com/json/%s?fields=status,country,lat,lon,query`, addr.String())

	resp, err := http.Get(url)
	if err != nil {
		return geodist.Coord{}, errors.WithStack(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return geodist.Coord{}, errors.Errorf("http code %d", resp.StatusCode)
	}

	var ret = struct {
		Status  string
		Country string
		Lat     float64
		Lon     float64
		Query   string
	}{}
	err = json.NewDecoder(resp.Body).Decode(&ret)
	if err != nil {
		return geodist.Coord{}, err
	}
	if ret.Status != "success" && ret.Query != addr.String() {
		return geodist.Coord{}, errors.Errorf("invalid response %#v", ret)
	}

	return geodist.Coord{Lat: ret.Lat, Lon: ret.Lon}, nil
}
