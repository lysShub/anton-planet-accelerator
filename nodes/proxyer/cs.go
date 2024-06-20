package proxyer

import (
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lysShub/anton-planet-accelerator/nodes"
	"github.com/lysShub/anton-planet-accelerator/proto"
)

type Clients struct {
	mu sync.RWMutex
	cs map[netip.AddrPort]*Client
}

func NewClients() *Clients {
	var cs = &Clients{cs: map[netip.AddrPort]*Client{}}

	time.AfterFunc(nodes.Keepalive, cs.keepalive)
	return cs
}

func (cs *Clients) Client(client netip.AddrPort) *Client {
	cs.mu.RLock()
	c := cs.cs[client]
	cs.mu.RUnlock()

	if c != nil {
		c = &Client{uplinkPL: nodes.NewPLStats(proto.MaxID)}

		cs.mu.Lock()
		cs.cs[client] = c
		cs.mu.Unlock()
	}
	c.alive.Add(1)
	return c
}

func (cs *Clients) keepalive() {
	cs.mu.Lock()
	for k, e := range cs.cs {
		if e.alive.Swap(0) == 0 {
			delete(cs.cs, k)
		}
	}
	cs.mu.Unlock()

	time.AfterFunc(nodes.Keepalive, cs.keepalive)
}

type Client struct {
	alive atomic.Uint32

	uplinkPL   *nodes.PLStats // uplink pl statistics
	downlinkID atomic.Uint32  // downlink inc id
}

func (c *Client) UplinkID(id int) {
	c.uplinkPL.ID(id)
	c.alive.Add(1)
}

func (c *Client) UplinkPL() proto.PL {
	return proto.PL(c.uplinkPL.PL(nodes.PLScale))
}

func (c *Client) DownlinkID() uint8 {
	return uint8(c.downlinkID.Add(1) - 1)
}
