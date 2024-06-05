package nodes

import (
	"sync"
)

type PLStats struct {
	mu       sync.RWMutex
	delteSum int
	count    int
	last     int
}

func (p *PLStats) PL() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()

	n := p.count - 1 - p.delteSum
	if n < 0 {
		n = -n
	}
	pl := float64(n) / float64(p.count)

	p.delteSum = 0
	p.count = 0
	p.last = 0
	return pl / 2
}

func (p *PLStats) Pack(id int) int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.count == 0 {
		p.last = id
	} else {
		if id == 0 {
			p.count--
		} else {
			p.delteSum += id - p.last
		}
		p.last = id
	}
	p.count++

	return p.count
}
