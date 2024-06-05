package nodes

import (
	"sync"
)

type PLStats struct {
	mu       sync.RWMutex
	deltaSum int
	count    int
	last     int
}

func (p *PLStats) PL() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.count == 0 {
		return 0
	}

	dropedCount := p.count - 1 - p.deltaSum
	if dropedCount < 0 {
		dropedCount = -dropedCount
	}
	pl := float64(dropedCount) / float64(p.count+dropedCount)

	p.deltaSum = 0
	p.count = 0
	p.last = 0
	return pl
}

func (p *PLStats) Pack(id int) int {
	if id < 0 {
		panic("require >= 0")
	}
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.count == 0 {
		p.last = id
	} else {
		if id == 0 {
			p.count--
		} else {
			p.deltaSum += id - p.last
		}
		p.last = id
	}
	p.count++

	return p.count
}
