package proxyer

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/netip"
	"sync"

	"github.com/jftuga/geodist"
	"github.com/pkg/errors"
)

type Route struct {
	nextMu sync.RWMutex
	nexts  map[geodist.Coord]netip.AddrPort // localtion:addr

	// todo: ttl
	cacheMu sync.RWMutex
	cache   map[netip.Addr]cachekey // dstination-address:forward-address
}

type cachekey struct {
	addr netip.AddrPort
	dist float64
}

func NewRoute() *Route {
	return &Route{
		nexts: map[geodist.Coord]netip.AddrPort{},

		cache: map[netip.Addr]cachekey{},
	}
}

func (r *Route) Next(dst netip.Addr) (next netip.AddrPort, dist float64, err error) {
	r.cacheMu.RLock()
	v, has := r.cache[dst]
	r.cacheMu.RUnlock()
	if !has {
		next, dist, err = r.queryForward(dst)
		if err != nil {
			return netip.AddrPort{}, 0, err
		}

		r.cacheMu.Lock()
		defer r.cacheMu.Unlock()
		r.cache[dst] = cachekey{next, dist}
		return next, dist, nil
	} else {
		return v.addr, v.dist, nil
	}
}

func (r *Route) AddForward(addr netip.AddrPort, location geodist.Coord) {
	r.nextMu.Lock()
	defer r.nextMu.Unlock()

	r.nexts[location] = addr
}

func (r *Route) queryForward(ip netip.Addr) (next netip.AddrPort, dist float64, err error) {
	loc, err := IP2Localtion(ip)
	if err != nil {
		return netip.AddrPort{}, 0, err
	}

	r.nextMu.RLock()
	defer r.nextMu.RUnlock()

	dist = math.MaxFloat64
	for k, e := range r.nexts {
		_, d, err := geodist.VincentyDistance(loc, k)
		if err == nil && d < dist {
			next, dist = e, d
		}
	}
	if !next.IsValid() {
		return netip.AddrPort{}, 0, errors.Errorf("can't get address %s nearby forward", ip.String())
	}

	return next, dist, nil
}

func IP2Localtion(ip netip.Addr) (geodist.Coord, error) {
	if !ip.Is4() {
		return geodist.Coord{}, errors.New("only support ipv4")
	}

	url := fmt.Sprintf(`http://ip-api.com/json/%s?fields=status,country,lat,lon,query`, ip.String())

	resp, err := http.Get(url)
	if err != nil {
		return geodist.Coord{}, errors.WithStack(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return geodist.Coord{}, errors.Errorf("http code %d", resp.StatusCode)
	}

	type Resp struct {
		Status  string
		Country string
		Lat     float64
		Lon     float64
		Query   string
	}

	var ret Resp
	err = json.NewDecoder(resp.Body).Decode(&ret)
	if err != nil {
		return geodist.Coord{}, err
	}
	if ret.Status != "success" && ret.Query != ip.String() {
		return geodist.Coord{}, errors.Errorf("invalid response %#v", ret)
	}
	return geodist.Coord{Lat: ret.Lat, Lon: ret.Lon}, nil
}
