package proxy

import "warthunder/fudp"

type ch [2](chan *fudp.Upack)

func newCh(cap int) ch {
	if cap < 1 {
		panic("")
	}

	c := ch{make(chan *fudp.Upack, cap), make(chan *fudp.Upack, cap)}
	for i := 0; i < cap; i++ {
		c[1] <- fudp.NewUpack()
	}
	return c
}

func (c *ch) push(u *fudp.Upack) {
	t := <-c[1]

	*t, *u = *u, *t

	c[0] <- t
}

func (c *ch) pope(u *fudp.Upack) {
	t := <-c[0]

	*u, *t = *t, *u

	t.Data = t.Data[:0]
	c[1] <- t
}
