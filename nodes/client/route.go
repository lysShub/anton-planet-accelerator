package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"
	"sync"

	"github.com/jftuga/geodist"
	"github.com/lysShub/anton-planet-accelerator/bvvd"
	"github.com/pkg/errors"
)

type routeCache struct {
	fixModeProxyer netip.AddrPort

	loction Loction
	mu      sync.RWMutex
	locs    map[bvvd.LocID]netip.AddrPort // loc:paddr
	routes  map[netip.Addr]netip.AddrPort // sadd:paddr
}

type Loction interface {
	Loction(addr netip.Addr) (geodist.Coord, error)
}

func newFixModeRoute(paddr netip.AddrPort) *routeCache {
	return &routeCache{fixModeProxyer: paddr}
}

func newAutoModeRoute(loc Loction) *routeCache {
	return &routeCache{
		loction: loc,
		locs:    map[bvvd.LocID]netip.AddrPort{},
		routes:  map[netip.Addr]netip.AddrPort{},
	}
}

func (r *routeCache) Proxyer(saddr netip.Addr) (paddr netip.AddrPort, loc bvvd.LocID, err error) {
	if r.fixModeProxyer.IsValid() {
		return r.fixModeProxyer, 0, nil
	}

	r.mu.RLock()
	paddr = r.routes[saddr]
	r.mu.RUnlock()
	if paddr.IsValid() {
		return paddr, 0, nil
	}

	coord, err := r.loction.Loction(saddr)
	if err != nil {
		return netip.AddrPort{}, 0, err
	}
	rec, offset := bvvd.Locs.Match(coord)
	if offset > 2000 {
		return netip.AddrPort{}, 0, errors.Errorf("not proxyer server to %s", saddr.String())
	}

	r.mu.RLock()
	paddr = r.locs[rec.ID]
	r.mu.RUnlock()

	if !paddr.IsValid() {
		return netip.AddrPort{}, rec.ID, nil
	}
	return paddr, 0, nil
}

func (r *routeCache) AddRecord(saddr netip.Addr, loc bvvd.LocID, paddr netip.AddrPort) {
	if r.fixModeProxyer.IsValid() {
		panic("fix route mode not need")
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	r.locs[loc] = paddr
	r.routes[saddr] = paddr
}

type temp struct {
	mu    sync.RWMutex
	cache map[netip.Addr]geodist.Coord
}

var T = &temp{cache: map[netip.Addr]geodist.Coord{}}

func (t *temp) Loction(addr netip.Addr) (geodist.Coord, error) {
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
