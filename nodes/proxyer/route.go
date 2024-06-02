package proxyer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"
	"sync"

	"github.com/jftuga/geodist"
	"github.com/pkg/errors"
)

type Route struct {
	forwardMu     sync.RWMutex
	forwards      map[geodist.Coord]netip.Addr // localtion:addr
	distanceLimit float64

	// todo: ttl
	cacheMu sync.RWMutex
	cache   map[netip.Addr]netip.Addr // dst:next
}

func NewRoute(distLimit float64) *Route {
	return &Route{
		forwards:      map[geodist.Coord]netip.Addr{},
		distanceLimit: distLimit,

		cache: map[netip.Addr]netip.Addr{},
	}
}

func (r *Route) Next(dst netip.Addr) (next netip.Addr, err error) {
	r.cacheMu.RLock()
	next, has := r.cache[dst]
	r.cacheMu.RUnlock()
	if !has {
		next, err = r.queryForward(dst)
		if err != nil {
			return netip.Addr{}, err
		}

		r.cacheMu.Lock()
		r.cache[dst] = next
		r.cacheMu.Unlock()
	}
	return next, nil
}

func (r *Route) AddForward(addr netip.Addr, location geodist.Coord) {
	r.forwardMu.Lock()
	defer r.forwardMu.Unlock()

	r.forwards[location] = addr
}

func (r *Route) queryForward(ip netip.Addr) (forward netip.Addr, err error) {
	loc, err := IP2Localtion(ip)
	if err != nil {
		return netip.Addr{}, err
	}

	r.forwardMu.RLock()
	defer r.forwardMu.RUnlock()

	var addr netip.Addr
	var tmp float64 = r.distanceLimit
	for k, e := range r.forwards {
		_, dist, err := geodist.VincentyDistance(loc, k)
		if err == nil && 0 < dist && dist < tmp {
			addr, tmp = e, dist
		}
	}
	if !addr.IsValid() {
		return netip.Addr{}, errors.Errorf("can't get address %s nearby forward", ip.String())
	}

	return addr, nil
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
