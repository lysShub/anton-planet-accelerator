package proxy

import (
	"sync"
	"sync/atomic"
)

type Chan struct {
	buff      []**Upack
	head, len atomic.Int32
	size      int32

	m                  *sync.Mutex
	itrigger, otrigger *sync.Cond
}

func NewChan(cap int) *Chan {
	c := &Chan{
		buff: make([]**Upack, cap),
		size: int32(cap),
		m:    &sync.Mutex{},
	}
	c.itrigger = sync.NewCond(c.m)
	c.otrigger = sync.NewCond(c.m)

	for i := 0; i < cap; i++ {
		t := &Upack{}
		c.buff[i] = &t
	}
	return c
}

func (c *Chan) Push(u *Upack) *Upack {
	for c.len.CompareAndSwap(c.size, c.size) {
		c.m.Lock()
		c.otrigger.Wait()
		c.m.Unlock()
	}

	// TODO: len, head need update sync?
	c.len.Add(1)
	h := c.head.Add(1)
	h = h % c.size

	*c.buff[h], u = u, *c.buff[h]
	c.itrigger.Signal()
	return u
}

func (c *Chan) Pope(u *Upack) *Upack {
	for c.len.CompareAndSwap(0, 0) {
		c.m.Lock()
		c.itrigger.Wait()
		c.m.Unlock()
	}

	c.len.Add(-1)
	h := c.head.Load() % c.size
	h = (c.size + h - 1) % c.size

	u, *c.buff[h] = *c.buff[h], u
	c.otrigger.Signal()
	return u
}

func (c *Chan) Len() int {
	return int(c.len.Load())
}
