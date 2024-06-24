package nodes

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"sync"
	"time"
	"unsafe"

	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/rawsock/test"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

// LoopIds expand the loop by increasing the ID, such as:
//
//	0 1 2 3 4 0 1 2 3
//
// expand to：
//
//	0 1 2 3 4 5 6 7 8
//
// allow loop-id has small scale disorder
type LoopIds struct {
	maxId     int
	dimension int

	idx, last int
}

func NewLoopIds(maxId int) *LoopIds {
	if maxId < 3 {
		panic(maxId)
	}
	return &LoopIds{
		maxId:     maxId,
		dimension: maxId / 3,
		last:      -1,
	}
}

func (i *LoopIds) delta(a1, a2 int) (d int, nearby bool) {
	if debug.Debug() {
		require.LessOrEqual(test.T(), a1, i.maxId)
		require.LessOrEqual(test.T(), a2, i.maxId)
	}

	if a1 > a2 {
		// 254 1
		// 5   4

		if d = a1 - a2; d < i.dimension {
			return -d, true
		} else if d = (i.maxId - a1) + a2 + 1; d < i.dimension {
			return d, true
		} else {
			d = a1 - a2
			return min(d, i.maxId-d), false
		}
	} else {
		//  4 5
		//  1 254
		if d = a2 - a1; d < i.dimension {
			return d, true
		} else if d = a1 + (i.maxId - a2) + 1; d < i.dimension {
			return -d, true
		} else {
			d = a1 - a2
			return min(d, i.maxId-d), false
		}
	}
}

// Expand expand loop-id to index
func (i *LoopIds) Expand(id int) (index int) {
	if i.last < 0 {
		i.last = id
		i.idx = id
		return id
	} else {
		d, nearby := i.delta(i.last, id)
		i.last = id
		i.idx += d
		if nearby {
			return i.idx
		}
		return -1
	}
}
func (i *LoopIds) MaxID() int { return i.maxId }

// Reset avoid int overflow
func (i *LoopIds) Reset() { i.idx, i.last = 0, 0 }

type PLStats struct {
	mu sync.RWMutex
	l  *LoopIds
	s  *stats
}

func NewPLStats(maxId int) *PLStats {
	var p = &PLStats{
		l: NewLoopIds(maxId),
		s: newStats(),
	}
	time.AfterFunc(reset, p.reset)
	return p
}

const reset = time.Hour * 4

func (p *PLStats) reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.l.Reset()
	p.s.init()
	time.AfterFunc(reset, p.reset)
}

func (p *PLStats) ID(id int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	i := p.l.Expand(id)
	if i < 0 {
		if debug.Debug() {
			println("loose id", id)
		}
		return
	}
	p.s.Index(i)
}

func (p *PLStats) PL(limit int) PL {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.s.PL(limit)
}

type PLStats2 struct {
	mu sync.RWMutex

	l    *LoopIds
	s    *stats
	dups [dimension]int
}

const dimension = 32

// NewPLStats2 statistics pl and deduplicate
func NewPLStats2(maxId int) *PLStats2 {
	var p = &PLStats2{
		l: NewLoopIds(maxId),
		s: newStats(),
	}
	for i := range p.dups {
		p.dups[i] = -1
	}
	time.AfterFunc(reset, p.reset)
	return p
}

func (p *PLStats2) reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.l.Reset()
	p.s.init()
	for i := range p.dups {
		p.dups[i] = -1
	}
	time.AfterFunc(reset, p.reset)
}

func (p *PLStats2) ID(id int) (recved bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	i := p.l.Expand(id)
	if i < 0 {
		if debug.Debug() {
			println("loose id", id)
		}
		return
	}

	if i <= p.dups[i%dimension] {
		return true
	}
	p.dups[i%dimension] = i
	p.s.Index(i)
	return false
}

func (p *PLStats2) PL(limit int) PL {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.s.PL(limit)
}

type stats struct {
	maxId, minId int
	count        int
}

func newStats() *stats {
	var p = &stats{}
	p.init()
	return p
}

func (p *stats) init() {
	p.maxId = 0
	p.minId = math.MaxInt
	p.count = 0
}

func (p *stats) Index(i int) {
	p.maxId = max(p.maxId, i)
	p.minId = min(p.minId, i)
	p.count++
}

func (p *stats) PL(limit int) PL {
	if p.count < limit || p.count < 2 {
		return 0
	}
	defer p.init()

	n := p.maxId - p.minId + 1
	if n < p.count {
		return 0 // repeat id
	}
	pl := float64(n-p.count) / float64(n)
	if pl == 0 {
		pl = math.SmallestNonzeroFloat64
	}
	return PL(pl)
}

type PL float64

func (p PL) Encode(to *packet.Packet) error {
	if p == 0 {
		p = math.SmallestNonzeroFloat64
	}
	if err := p.Valid(); err != nil {
		return err
	}
	to.Append(p.bytes()...)
	return nil
}

func (p *PL) Decode(from *packet.Packet) (err error) {
	if from.Data() < 8 {
		return errors.Errorf("too small %d", from.Data())
	}
	b := from.Detach(make([]byte, 8))

	v := binary.BigEndian.Uint64(b)
	*p = *(*PL)(unsafe.Pointer(&v))
	return p.Valid()
}

func (p PL) bytes() []byte {
	return binary.BigEndian.AppendUint64(make([]byte, 0, 8), *(*uint64)(unsafe.Pointer(&p)))
}

func (p PL) Valid() error {
	if p <= 0 || 1 < p {
		return errors.Errorf("invalid pack loss %s", hex.EncodeToString(p.bytes()))
	}
	return nil
}

func (p PL) String() string {
	if p <= 0 || 1 <= p {
		return "--.-"
	}

	v := float64(p * 100)
	v1 := int(math.Round(v))
	v2 := int((v - float64(v1)) * 10)
	if v2 < 0 {
		v2 = 0
	}
	return fmt.Sprintf("%02d.%d", v1, v2)
}
