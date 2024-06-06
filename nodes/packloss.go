package nodes

import (
	"sync"
)

type PLStats struct {
	mu       sync.RWMutex
	deltaSum int
	count    int
	lastId   int
	lastPl   float64
}

func (p *PLStats) PL() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.count == 0 {
		return 0
	} else if p.count < 64 {
		return p.lastPl
	}

	dropedCount := p.count - 1 - p.deltaSum
	if dropedCount < 0 {
		dropedCount = -dropedCount
	}
	pl := float64(dropedCount) / float64(p.count+dropedCount)

	p.deltaSum = 0
	p.count = 0
	p.lastId = 0
	p.lastPl = pl
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
		if id == 0 {
			p.count--
		} else {
			p.deltaSum += id - p.lastId
		}
		p.lastId = id
	}
	p.count++

	return p.count
}
