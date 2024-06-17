package nodes

import (
	"sync"
)

type PLStats struct {
	dimension int

	mu       sync.RWMutex
	deltaSum int
	count    int
	lastId   int
}

func NewPLStats() *PLStats {
	return NewPLStatsWithDimension(64)
}

func NewPLStatsWithDimension(dimension int) *PLStats {
	if dimension < 0 {
		dimension = -dimension
	}
	if dimension < 1 {
		dimension = 1
	}
	return &PLStats{dimension: dimension}
}

func (p *PLStats) PL() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.deltaSum < 0 {
		p.deltaSum = -p.deltaSum
	}
	if p.deltaSum < p.dimension {
		return 0
	}

	//  125  4 3
	// 12345 4 5
	dropedCount := p.deltaSum - (p.count - 1)
	if dropedCount < 0 {
		dropedCount = -dropedCount
	}

	pl := float64(dropedCount) / float64(p.count+dropedCount)

	p.deltaSum = 0
	p.count = 0
	p.lastId = 0
	return pl
}

func (p *PLStats) ID(id int) int {
	if id < 0 {
		panic("require >= 0")
	}
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.count == 0 {
		p.lastId = id
	} else {
		n := id - p.lastId
		if -p.dimension < n && n < p.dimension {
			p.deltaSum += n
		} else {
			p.deltaSum += 1
			// if debug.Debug() {
			// 	println("loose id", n)
			// }
		}
		p.lastId = id
	}
	p.count++

	return p.count
}
