package forward

import (
	"net/netip"
	"sync"
	"sync/atomic"

	"github.com/lysShub/anton-planet-accelerator/bvvd"
	"github.com/lysShub/anton-planet-accelerator/nodes"
)

type Gateways struct {
	mu sync.RWMutex
	ps map[netip.AddrPort]*Gateway
	// todo: keepalive
}

func NewGateways() *Gateways {
	return &Gateways{
		ps: map[netip.AddrPort]*Gateway{},
	}
}

func (ps *Gateways) Gateway(gaddr netip.AddrPort) *Gateway {
	ps.mu.RLock()
	p := ps.ps[gaddr]
	ps.mu.RUnlock()

	if p == nil {
		p = &Gateway{uplinkPL: nodes.NewPLStats(bvvd.MaxID)}
		ps.mu.Lock()
		ps.ps[gaddr] = p
		ps.mu.Unlock()
	}
	return p
}

type Gateway struct {
	uplinkPL   *nodes.PLStats
	downlinkID atomic.Uint32
}

func (p *Gateway) UplinkID(id uint8) {
	p.uplinkPL.ID(int(id))
}

func (p *Gateway) UplinkPL() nodes.PL {
	return nodes.PL(p.uplinkPL.PL(nodes.PLScale))
}

func (p *Gateway) DownlinkID() uint8 {
	return uint8(p.downlinkID.Add(1) - 1)
}
