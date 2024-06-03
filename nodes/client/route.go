package client

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
	proxyerMu sync.RWMutex
	proxyers  map[netip.AddrPort]geodist.Coord // proxyer-address:proxy-localtion

	// todo: ttl
	cacheMu sync.RWMutex
	cache   map[netip.Addr]netip.AddrPort // server-address:proxyer-address
}

func NewRoute() *Route {
	return &Route{
		proxyers: map[netip.AddrPort]geodist.Coord{},

		cache: map[netip.Addr]netip.AddrPort{},
	}
}

func (r *Route) Next(dst netip.Addr) (proxyer netip.AddrPort, err error) {
	r.cacheMu.RLock()
	v, has := r.cache[dst]
	r.cacheMu.RUnlock()
	if !has {
		proxyer, err = r.queryProxyer(dst)
		if err != nil {
			return netip.AddrPort{}, err
		}

		r.cacheMu.Lock()
		defer r.cacheMu.Unlock()
		r.cache[dst] = proxyer
		return proxyer, nil
	} else {
		return v, nil
	}
}

func (r *Route) AddProxyer(proxyer netip.AddrPort, proxyLocation geodist.Coord) {
	r.proxyerMu.Lock()
	defer r.proxyerMu.Unlock()

	r.proxyers[proxyer] = proxyLocation
}

func (r *Route) queryProxyer(server netip.Addr) (proxyer netip.AddrPort, err error) {
	r.proxyerMu.RLock()
	n := len(r.proxyers)
	r.proxyerMu.RUnlock()
	if n == 0 {
		return netip.AddrPort{}, errors.New("not proxyer server")
	}

	loc, err := IP2Localtion(server)
	if err != nil {
		return netip.AddrPort{}, err
	}

	r.proxyerMu.RLock()
	defer r.proxyerMu.RUnlock()

	offset := math.MaxFloat64
	for paddr, ploc := range r.proxyers {
		_, d, err := geodist.VincentyDistance(loc, ploc)
		if err == nil && d < offset {
			proxyer, offset = paddr, d
		}
	}
	if !proxyer.IsValid() {
		return netip.AddrPort{}, errors.Errorf("can't get address %s nearby forward", server.String())
	}

	return proxyer, nil
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
