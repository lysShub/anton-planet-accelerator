package proxyer

import (
	"net/netip"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/lysShub/anton-planet-accelerator/bvvd"
	"github.com/lysShub/anton-planet-accelerator/nodes"
)

type Forwards struct {
	mu sync.RWMutex
	fs map[bvvd.LocID]*Forward
}

func NewForwards() *Forwards {
	return &Forwards{fs: map[bvvd.LocID]*Forward{}}
}

func (f *Forwards) Get(loc bvvd.LocID) *Forward {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.fs[loc]
}

func (f *Forwards) Add(loc bvvd.LocID, faddr netip.AddrPort) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, has := f.fs[loc]; has {
		panic("todo: 现在只能一对一")
	}

	f.fs[loc] = &Forward{
		faddr:      faddr,
		donwlinkPL: nodes.NewPLStats(bvvd.MaxID),
	}
}

type Forward struct {
	faddr netip.AddrPort

	uplinkID atomic.Uint32 // proxyer-->forward inc id

	uplinkPL atomic.Uintptr // proxyer-->forward pl

	donwlinkPL *nodes.PLStats // forward-->proxyer pl
}

func (f *Forward) Addr() netip.AddrPort {
	return f.faddr
}

func (f *Forward) UplinkID() uint8 {
	return uint8(f.uplinkID.Add(1) - 1)
}

func (f *Forward) DownlinkID(id uint8) {
	f.donwlinkPL.ID(int(id))
}

func (f *Forward) DownlinkPL() nodes.PL {
	return nodes.PL(f.donwlinkPL.PL(nodes.PLScale))
}

func (f *Forward) UplinkPL() nodes.PL {
	// todo: will cause PackLossProxyerUplink keep last value, when not data uplink transmit
	tmp := f.uplinkPL.Load()
	return *(*nodes.PL)(unsafe.Pointer(&tmp))
}

func (f *Forward) SetUplinkPL(pl nodes.PL) {
	tmp := *(*uintptr)(unsafe.Pointer(&pl))
	f.uplinkPL.Store(tmp)
}
