package proxy

import "net/netip"

// udp packet
type upack struct {
	laddr, raddr netip.AddrPort
	data         []byte
}

type ch [2](chan *upack)

func newCh(cap int) ch {
	if cap < 1 {
		panic("")
	}

	c := ch{make(chan *upack, cap), make(chan *upack, cap)}
	for i := 0; i < cap; i++ {
		c[1] <- &upack{data: make([]byte, 65535)}
	}
	return c
}

func (c *ch) push(u *upack) {
	t := <-c[1]

	*t, *u = *u, *t

	c[0] <- t
}

func (c *ch) pope(u *upack) {
	t := <-c[0]

	*u, *t = *t, *u

	t.data = t.data[:cap(t.data)]
	c[1] <- t
}
