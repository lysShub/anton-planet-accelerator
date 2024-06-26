package client

import (
	"encoding/json"
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

	// auto route mode
	mu     sync.RWMutex
	routes map[netip.Addr]entry
}

type entry struct {
	paddr   netip.AddrPort
	forward bvvd.ForwardID
}

func (e entry) valid() bool {
	return e.paddr.IsValid() && e.forward.Vaid()
}

type Location interface {
	Location(addr netip.Addr) (geodist.Coord, error)
}

func newRoute(fixRoute bool) *route {
	return &route{
		fixRouteMode: fixRoute,
	}
}

func (r *route) Init(defaultProxyer netip.AddrPort, defaultForward bvvd.ForwardID) {
	if !r.inited.Swap(true) {
		r.defaultProxyer = defaultProxyer
		r.defaultForward = defaultForward
	}
}

func (r *route) Match(saddr netip.Addr) (paddr netip.AddrPort, forward bvvd.ForwardID, err error) {
	if !r.inited.Load() {
		return netip.AddrPort{}, 0, errorx.WrapTemp(errors.New("route not init"))
	}
	if r.fixRouteMode {
		return r.defaultProxyer, r.defaultForward, nil
	}

	r.mu.RLock()
	e := r.routes[saddr]
	r.mu.RUnlock()
	if e.valid() {
		return e.paddr, e.forward, nil
	}

	err = errors.New("not record")
	return
}

func (r *route) AddRecord(saddr netip.Addr, proxyer netip.AddrPort, forward bvvd.ForwardID) error {
	if r.defaultProxyer.IsValid() {
		return errors.New("fix route mode not need")
	} else if !saddr.IsValid() {
		return errors.Errorf("server address %s invalid", saddr.String())
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	e := entry{proxyer, forward}
	if !e.valid() {
		return errors.Errorf("entry %s %d invalid", proxyer.String(), forward)
	}

	r.routes[saddr] = e
	return nil
}

type temp struct {
	mu    sync.RWMutex
	cache map[netip.Addr]geodist.Coord
}

var T = &temp{cache: map[netip.Addr]geodist.Coord{}}

func (t *temp) Location(addr netip.Addr) (geodist.Coord, error) {
	if !addr.Is4() {
		return geodist.Coord{}, errors.New("only support ipv4")
	}
	t.mu.RLock()
	coord, has := t.cache[addr]
	t.mu.RUnlock()
	if has {
		return coord, nil
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
	coord = geodist.Coord{Lat: ret.Lat, Lon: ret.Lon}

	t.mu.Lock()
	t.cache[addr] = coord
	t.mu.Unlock()
	return coord, nil
}
