package nodes

import (
	"math"
	"sync"

	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/rawsock/test"
	"github.com/stretchr/testify/require"
)

/*

	丢包统计： 记录这段时间最大值与最小值，同时记录id个数
	去 重： 需要一个数组记录，取余然后数组值加1，如果大于则已经收到了

*/

// LoopIds 将回环递增id展开，比如有回环id:
//
//	0 1 2 3 4 0 1 2 3
//
// 展开为：
//
//	0 1 2 3 4 5 6 7 8
//
// 允许回环id有小尺度乱序
type LoopIds struct {
	maxid     int
	dimension int

	idx, last int
}

func NewLoopIds(maxId int) *LoopIds {
	if maxId < 3 {
		panic(maxId)
	}
	return &LoopIds{
		maxid:     maxId,
		dimension: maxId / 3,
		last:      -1,
	}
}

func (i *LoopIds) delta(a1, a2 int) (d int, nearby bool) {
	if debug.Debug() {
		require.LessOrEqual(test.T(), a1, i.maxid)
		require.LessOrEqual(test.T(), a2, i.maxid)
	}

	if a1 > a2 {
		// 254 1
		// 5   4

		if d = a1 - a2; d < i.dimension {
			return -d, true
		} else if d = (i.maxid - a1) + a2 + 1; d < i.dimension {
			return d, true
		} else {
			d = a1 - a2
			return min(d, i.maxid-d), false
		}
	} else {
		//  4 5
		//  1 254
		if d = a2 - a1; d < i.dimension {
			return d, true
		} else if d = a1 + (i.maxid - a2) + 1; d < i.dimension {
			return -d, true
		} else {
			d = a1 - a2
			return min(d, i.maxid-d), false
		}
	}
}

func (i *LoopIds) Expand(id int) int {
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

// 要求id不重复
type PLStats struct {
	mu sync.RWMutex

	li           *LoopIds
	maxId, minId int
	count        int
}

func NewPLStats(maxId int) *PLStats {
	return &PLStats{
		li:    NewLoopIds(maxId),
		minId: math.MaxInt,
	}
}

func (p *PLStats) ID(id int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	i := p.li.Expand(id)
	if i < 0 {
		if debug.Debug() {
			println("loose id", id)
		}
		return
	}

	p.maxId = max(p.maxId, i)
	p.minId = min(p.minId, i)
	p.count++
}

func (p *PLStats) PL(limit int) float64 {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.count < limit || p.count < 2 {
		return 0
	}

	n := p.maxId - p.minId + 1
	if n < p.count {
		return 0 // repeat id
	}
	pl := float64(n-p.count) / float64(n)
	return pl
}

type PLStats2 struct {
	mu sync.RWMutex

	li           *LoopIds
	maxId, minId int
	count        int

	dups [dimension]int
}

const dimension = 32

func NewPLStats2(maxId int) *PLStats2 {
	var ps = &PLStats2{li: NewLoopIds(maxId)}
	for i := range ps.dups {
		ps.dups[i] = -1
	}
	return ps
}

func (p *PLStats2) ID(id int) (recved bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	i := p.li.Expand(id)
	if i < 0 {
		if debug.Debug() {
			println("loose id", id)
		}
		return
	}

	if i <= p.dups[i%dimension] {
		return true
	}

	p.maxId = max(p.maxId, i)
	p.minId = min(p.minId, i)
	p.count++
	p.dups[i%dimension] = i
	return false
}

func (p *PLStats2) PL(limit int) float64 {
	return 0
}
