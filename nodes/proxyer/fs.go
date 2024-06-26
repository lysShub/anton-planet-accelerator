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
	mu sync.RWMutex
	fs map[bvvd.ForwardID]*Forward
}

func NewForwards() *Forwards {
	return &Forwards{
		fs: map[bvvd.ForwardID]*Forward{},
	}
}

func (f *Forwards) GetByForward(fid bvvd.ForwardID) (*Forward, error) {
	f.mu.RLock()
	fw, has := f.fs[fid]
	f.mu.RUnlock()
	if !has {
		return nil, errors.Errorf("not forward %d record", fid)
	}
	return fw, nil
}

// GetByLoc get forwards that is on loc position
func (f *Forwards) GetByLocation(loc bvvd.Location) ([]*Forward, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var fs []*Forward
	for _, f := range f.fs {
		if f.location == loc {
			fs = append(fs, f)
		}
	}
	if len(fs) == 0 {
		return nil, errors.Errorf("not forward %s record", loc.String())
	}
	return fs, nil
}

func (f *Forwards) Add(loc bvvd.Location, id bvvd.ForwardID, faddr netip.AddrPort) error {
	fw, err := newForward(loc, id, faddr)
	if err != nil {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if fw, has := f.fs[id]; has {
		return errors.Errorf("forward id:%d addr:%s location:%s existed", id, fw.addr.String(), fw.location.String())
	}

	f.fs[id] = fw
	return nil
}

type Forward struct {
	addr     netip.AddrPort
	id       bvvd.ForwardID
	location bvvd.Location

	uplinkID atomic.Uint32 // proxyer-->forward inc id

	uplinkPL atomic.Uintptr // proxyer-->forward pl

	donwlinkPL *nodes.PLStats // forward-->proxyer pl
}

func newForward(loc bvvd.Location, id bvvd.ForwardID, faddr netip.AddrPort) (*Forward, error) {
	if !loc.Valid() {
		return nil, errors.Errorf("invalid forward location %s", loc.String())
	} else if !id.Vaid() {
		return nil, errors.Errorf("invalid forward id %d", id)
	} else if !faddr.IsValid() {
		return nil, errors.Errorf("invalid forward address %s", faddr.String())
	}

	return &Forward{
		addr:       faddr,
		id:         id,
		location:   loc,
		donwlinkPL: nodes.NewPLStats(bvvd.MaxID),
	}, nil
}

func (f *Forward) Addr() netip.AddrPort {
	return f.addr
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
