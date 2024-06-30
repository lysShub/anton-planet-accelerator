package client

import (
	"fmt"
	"math"
	"net/netip"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
	"unsafe"

	"github.com/lysShub/anton-planet-accelerator/nodes/internal/stats"
)

type NetworkStates struct {
	PingGateway             time.Duration
	PingForward             time.Duration
	PackLossClientUplink    stats.PL
	PackLossClientDownlink  stats.PL
	PackLossGatewayUplink   stats.PL
	PackLossGatewayDownlink stats.PL
}

func (n *NetworkStates) String() string {
	var s = &strings.Builder{}

	p2 := time.Duration(0)
	if n.PingGateway > 0 && n.PingForward > n.PingGateway {
		p2 = n.PingForward - n.PingGateway
	}
	var elems = []string{
		"ping", n.strdur(n.PingGateway), n.strdur(p2),
		"pl↑", n.PackLossClientUplink.String(), n.PackLossGatewayUplink.String(),
		"pl↓", n.PackLossClientDownlink.String(), n.PackLossGatewayDownlink.String(),
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

func (*NetworkStates) strdur(dur time.Duration) string {
	if dur <= 0 || time.Minute <= dur {
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

type trunkRouteRecorder struct {
	scale     time.Duration
	initPaddr netip.AddrPort
	initFaddr netip.AddrPort

	sync.RWMutex
	lastWasInit bool

	updateTime time.Time
	updated    bool
	gaddr      netip.AddrPort
	faddr      netip.AddrPort
}

func newTrunkRouteRecorder(scale time.Duration, gaddr, faddr netip.AddrPort) *trunkRouteRecorder {
	return &trunkRouteRecorder{
		scale:     scale,
		initPaddr: gaddr,
		initFaddr: faddr,
	}
}

func (r *trunkRouteRecorder) Trunk() (gaddr, faddr netip.AddrPort, updata bool) {
	r.Lock()
	defer r.Unlock()

	if time.Since(r.updateTime) < r.scale {
		updata = r.updated || r.lastWasInit
		r.updated, r.lastWasInit = false, false
		return r.gaddr, r.faddr, updata
	} else {
		updata = !r.lastWasInit
		r.lastWasInit = true
		return r.initPaddr, r.initFaddr, updata
	}
}

func (r *trunkRouteRecorder) Update(gaddr, faddr netip.AddrPort) {
	r.Lock()
	defer r.Unlock()

	if r.gaddr != gaddr {
		r.gaddr, r.updated = gaddr, true
	}
	if r.faddr != faddr {
		r.faddr, r.updated = faddr, true
	}
	r.updateTime = time.Now()
}
