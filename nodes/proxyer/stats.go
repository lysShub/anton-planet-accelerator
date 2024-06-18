package proxyer

import (
	"net/netip"
	"sync"
	"sync/atomic"

	"github.com/lysShub/anton-planet-accelerator/nodes"
	"github.com/lysShub/anton-planet-accelerator/proto"
)

type StatsSet struct {
	mu    sync.RWMutex
	conns map[netip.AddrPort]*stats
}

func NewStatsSet() *StatsSet {
	return &StatsSet{conns: map[netip.AddrPort]*stats{}}
}

func (ss *StatsSet) Stats(caddr netip.AddrPort) *stats {
	ss.mu.RLock()
	s := ss.conns[caddr]
	ss.mu.RUnlock()

	if s == nil {
		s = &stats{pl: nodes.NewPLStats(proto.MaxID)}
		ss.mu.Lock()
		ss.conns[caddr] = s
		ss.mu.Unlock()

		// todo: keepalive
	}
	return s
}

type stats struct {
	pl *nodes.PLStats // uplink pl statistics
	id atomic.Uint32  // downlink inc id
}

func (s *stats) Uplink(id int) { s.pl.ID(id) }

func (s *stats) UplinkPL() proto.PL {
	return proto.PL(s.pl.PL(nodes.PLScale))
}

func (s *stats) Downlink() uint8 {
	return uint8(s.id.Add(1))
}
