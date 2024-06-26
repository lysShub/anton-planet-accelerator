//go:build linux
// +build linux

package links

import (
	"fmt"
	"net/netip"
	"sync"

	"github.com/lysShub/anton-planet-accelerator/bvvd"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Links struct {
	mu    sync.RWMutex
	links map[Endpoint]*Link
}

func NewLinks() *Links {
	return &Links{links: map[Endpoint]*Link{}}
}

func (ls *Links) Link(ep Endpoint, paddr netip.AddrPort, forwardID bvvd.ForwardID) (l *Link, new bool, err error) {
	ls.mu.RLock()
	l = ls.links[ep]
	ls.mu.RUnlock()

	if l == nil {
		l, err = newLink(ls, ep, paddr, forwardID)
		if err != nil {
			return nil, false, err
		}
		ls.mu.Lock()
		ls.links[ep] = l
		ls.mu.Unlock()
		new = true
	}
	return l, new, nil
}

func (ls *Links) del(ep Endpoint) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	delete(ls.links, ep)
}

func (ls *Links) Close() error {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	for _, e := range ls.links {
		e.close(nil)
	}
	clear(ls.links)
	return nil
}

type Endpoint struct {
	client      netip.AddrPort
	proto       uint8
	processPort uint16
	server      netip.AddrPort
}

func NewEP(hdr bvvd.Bvvd, t header.Transport) Endpoint {
	return Endpoint{
		client:      hdr.Client(),
		proto:       hdr.Proto(),
		processPort: t.SourcePort(),
		server:      netip.AddrPortFrom(hdr.Server(), t.DestinationPort()),
	}
}

func (e Endpoint) String() string {
	return fmt.Sprintf(
		"{Client:%s,Proto:%d,ProcessPort:%d,Server:%s}",
		e.client.String(), e.proto, e.processPort, e.server.String(),
	)
}
