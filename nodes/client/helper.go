package client

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/netip"
	"strings"
	"time"
	"unicode/utf8"
	"unsafe"

	"github.com/jftuga/geodist"
	"github.com/lysShub/anton-planet-accelerator/nodes"
	"github.com/pkg/errors"
)

type NetworkStats struct {
	PingProxyer             time.Duration
	PingForward             time.Duration
	PackLossClientUplink    nodes.PL
	PackLossClientDownlink  nodes.PL
	PackLossProxyerUplink   nodes.PL
	PackLossProxyerDownlink nodes.PL
}

func (n *NetworkStats) init() *NetworkStats {
	n.PingProxyer = InvalidRtt
	n.PingForward = InvalidRtt
	n.PackLossClientUplink = 1.0
	n.PackLossClientDownlink = 1.0
	n.PackLossProxyerUplink = 1.0
	n.PackLossProxyerDownlink = 1.0
	return n
}

const InvalidRtt = time.Hour

func (n *NetworkStats) String() string {
	var s = &strings.Builder{}

	p2 := n.PingForward
	if p2 < InvalidRtt && n.PingProxyer < InvalidRtt && p2 > n.PingProxyer {
		p2 -= n.PingProxyer
	}
	var elems = []string{
		"ping", n.strdur(n.PingProxyer), n.strdur(p2),
		"pl↑", n.PackLossClientUplink.String(), n.PackLossProxyerUplink.String(),
		"pl↓", n.PackLossClientDownlink.String(), n.PackLossProxyerDownlink.String(),
	}

	const size = 6
	for i, e := range elems {
		s.WriteString(e)

		n := size - utf8.RuneCount(unsafe.Slice(unsafe.StringData(e), len(e)))
		for i := 0; i < n; i++ {
			s.WriteByte(' ')
		}

		if (i+1)%3 == 0 {
			s.WriteByte('\n')
		}
	}
	return s.String()
}

func (*NetworkStats) strdur(dur time.Duration) string {
	if dur >= time.Hour {
		return "--.-"
	}
	ss := dur.Seconds() * 1000

	s1 := int(math.Round(ss))
	s2 := int((ss - float64(s1)) * 10)
	if s2 < 0 {
		s2 = 0
	}
	return fmt.Sprintf("%d.%d", s1, s2)
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
	if ret.Status != "success" && ret.Query != ip.String() {
		return geodist.Coord{}, errors.Errorf("invalid response %#v", ret)
	}
	return geodist.Coord{Lat: ret.Lat, Lon: ret.Lon}, errors.Errorf("not found %#v", ret)
}
