package gateway

import (
	"net/netip"
	"sync"
	"sync/atomic"

	"github.com/lysShub/anton-planet-accelerator/bvvd"
	"github.com/lysShub/anton-planet-accelerator/nodes"
	"github.com/lysShub/anton-planet-accelerator/nodes/internal/stats"
	"github.com/pkg/errors"
)

type Forwards struct {
	mu sync.RWMutex
	fs map[netip.AddrPort]*Forward // faddr
}

func NewForwards() *Forwards {
	return &Forwards{
		fs: map[netip.AddrPort]*Forward{},
	}
}

func (f *Forwards) Get(faddr netip.AddrPort) (*Forward, error) {
	f.mu.RLock()
	fw, has := f.fs[faddr]
	f.mu.RUnlock()
	if !has {
		return nil, errors.Errorf("not forward %s record", faddr.String())
	}
	return fw, nil
}

func (f *Forwards) Forwards() (fs []netip.AddrPort) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	for _, e := range f.fs {
		fs = append(fs, e.faddr)
	}
	return fs
}

func (f *Forwards) Add(faddr netip.AddrPort, loc bvvd.Location) error {
	fw, err := newForward(faddr, loc)
	if err != nil {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if fw, has := f.fs[faddr]; has {
		return errors.Errorf("forward faddr:%s location:%s existed", fw.faddr.String(), fw.loc.String())
	}

	f.fs[faddr] = fw
	return nil
}

type Forward struct {
	faddr netip.AddrPort
	loc   bvvd.Location

	uplinkID atomic.Uint32 // gateway-->forward inc id

	donwlinkPL *stats.PLStats // forward-->gateway pl
}

func newForward(faddr netip.AddrPort, loc bvvd.Location) (*Forward, error) {
	if err := loc.Valid(); err != nil {
		return nil, err
	} else if !faddr.IsValid() {
		return nil, errors.Errorf("invalid forward address %s", faddr.String())
	}

	return &Forward{
		faddr:      faddr,
		loc:        loc,
		donwlinkPL: stats.NewPLStats(bvvd.MaxID),
	}, nil
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

func (f *Forward) DownlinkPL() stats.PL {
	return stats.PL(f.donwlinkPL.PL(nodes.PLScale))
}
