package proxyer

import (
	"net/netip"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/lysShub/anton-planet-accelerator/bvvd"
	"github.com/lysShub/anton-planet-accelerator/nodes"
	"github.com/pkg/errors"
)

type Forwards struct {
	mu     sync.RWMutex
	faddrs map[netip.AddrPort]bvvd.LocID // fadd:LocID
	fs     map[bvvd.LocID]*Forward
}

func NewForwards() *Forwards {
	return &Forwards{fs: map[bvvd.LocID]*Forward{}}
}

func (f *Forwards) GetByLocID(loc bvvd.LocID) (*Forward, error) {
	f.mu.RLock()
	fw, has := f.fs[loc]
	f.mu.RUnlock()
	if !has {
		return nil, errors.Errorf("not forward %s record", loc.String())
	}
	return fw, nil
}

// GetByLoc get forwards that is on loc position
func (f *Forwards) GetByLoc(loc bvvd.LocID) ([]*Forward, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var ids []bvvd.LocID
	for _, id := range f.faddrs {
		if id.Overlap(loc) {
			ids = append(ids, id)
		}
	}

	var fws []*Forward
	for _, id := range ids {
		fws = append(fws, f.fs[id])
	}
	if len(fws) == 0 {
		return nil, errors.Errorf("not forward %s record", loc.Loc().String())
	}
	return fws, nil
}

func (f *Forwards) GetByFaddr(faddr netip.AddrPort) (*Forward, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	id, has := f.faddrs[faddr]
	if !has {
		return nil, errors.Errorf("not forward %s record", faddr)
	}

	fw, has := f.fs[id]
	if !has {
		return nil, errors.Errorf("not forward %s record", id.String())
	}
	return fw, nil
}

func (f *Forwards) Add(loc bvvd.LocID, faddr netip.AddrPort) (bvvd.LocID, error) {
	if !faddr.IsValid() {
		return 0, errors.Errorf("invalid forward address %s", faddr.String())
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if _, has := f.faddrs[faddr]; has {
		return 0, errors.Errorf("forward %s existed", faddr.String())
	}
	if loc.SetID(uint8(len(f.faddrs)+1)) != nil {
		return 0, errors.Errorf("add too many forward for loction %s", loc.String())
	}

	f.faddrs[faddr] = loc
	f.fs[loc] = &Forward{
		faddr:      faddr,
		loc:        loc,
		donwlinkPL: nodes.NewPLStats(bvvd.MaxID),
	}
	return loc, nil
}

type Forward struct {
	faddr netip.AddrPort
	loc   bvvd.LocID

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

func (f *Forward) LocID() bvvd.LocID { return f.loc }

func (f *Forward) SetUplinkPL(pl nodes.PL) {
	tmp := *(*uintptr)(unsafe.Pointer(&pl))
	f.uplinkPL.Store(tmp)
}
