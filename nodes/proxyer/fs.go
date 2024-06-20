package proxyer

import (
	"net/netip"
	"sync"
	"sync/atomic"

	"github.com/lysShub/anton-planet-accelerator/nodes"
	"github.com/lysShub/anton-planet-accelerator/proto"
)

type Forwards struct {
	mu sync.RWMutex
	fs map[netip.AddrPort]*Forward
}

func NewForwards() *Forwards {
	return &Forwards{fs: map[netip.AddrPort]*Forward{}}
}

// todo: 根据地理标签获取
func (f *Forwards) Get(addr netip.AddrPort) *Forward {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.fs[addr]
}

func (f *Forwards) Add(forward netip.AddrPort) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fs[forward] = &Forward{}
}

type Forward struct {
	uplinkID   atomic.Uint32  // proxyer-->forward inc id
	donwlinkPL *nodes.PLStats // forward-->proxyer pl
}

func (f *Forward) UplinkID() uint8 {
	return uint8(f.uplinkID.Add(1) - 1)
}

func (f *Forward) DownlinkID(id uint8) {
	f.donwlinkPL.ID(int(id))
}

func (f *Forward) DownlinkPL() proto.PL {
	return proto.PL(f.donwlinkPL.PL(nodes.PLScale))
}
