package forward

import (
	"net/netip"
	"sync"
	"sync/atomic"

	"github.com/lysShub/anton-planet-accelerator/bvvd"
	"github.com/lysShub/anton-planet-accelerator/nodes"
)

type Proxyers struct {
	mu sync.RWMutex
	ps map[netip.AddrPort]*Proxyer
	// todo: keepalive
}

func NewProxyers() *Proxyers {
	return &Proxyers{
		ps: map[netip.AddrPort]*Proxyer{},
	}
}

func (ps *Proxyers) Proxyer(paddr netip.AddrPort) *Proxyer {
	ps.mu.RLock()
	p := ps.ps[paddr]
	ps.mu.RUnlock()

	if p == nil {
		p = &Proxyer{uplinkPL: nodes.NewPLStats(bvvd.MaxID)}
		ps.mu.Lock()
		ps.ps[paddr] = p
		ps.mu.Unlock()
	}
	return p
}

type Proxyer struct {
	uplinkPL   *nodes.PLStats
	downlinkID atomic.Uint32
}

func (p *Proxyer) UplinkID(id uint8) {
	p.uplinkPL.ID(int(id))
}

func (p *Proxyer) UplinkPL() nodes.PL {
	return nodes.PL(p.uplinkPL.PL(nodes.PLScale))
}

func (p *Proxyer) DownlinkID() uint8 {
	return uint8(p.downlinkID.Add(1) - 1)
}
