//go:build linux
// +build linux

package server

import (
	"context"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lysShub/fatcp"
	"github.com/lysShub/fatcp/ports"
	"github.com/lysShub/netkit/packet"
	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type linkManager struct {
	addr     netip.Addr
	ap       *ports.Adapter
	duration time.Duration

	uplinkMap map[uplink]*port
	ttl       *Heap[ttlkey]
	uplinkMu  sync.RWMutex

	downlinkMap map[downlink]*link
	donwlinkMu  sync.RWMutex
}

type uplink struct {
	Process netip.AddrPort
	Proto   tcpip.TransportProtocolNumber
	Server  netip.AddrPort
}

type downlink struct {
	Server netip.AddrPort
	Proto  tcpip.TransportProtocolNumber
	Local  netip.AddrPort
}

func NewLinkManager(ttl time.Duration, addr netip.Addr) *linkManager {
	var m = &linkManager{
		addr:     addr,
		ap:       ports.NewAdapter(addr),
		duration: ttl,

		uplinkMap: map[uplink]*port{},
		ttl:       NewHeap[ttlkey](16),

		downlinkMap: map[downlink]*link{},
	}

	return m
}

type ttlkey struct {
	s uplink
	t time.Time
}

func (t ttlkey) valid() bool {
	return t.s.Process.IsValid() && t.s.Server.IsValid() && t.t != time.Time{}
}

type link struct {
	conn *Conn
	port uint16 // client port
}

func (l *link) Donwlink(ctx context.Context, pkt *packet.Packet, peer fatcp.Peer) error {
	switch peer.Proto {
	case header.TCPProtocolNumber:
		header.TCP(pkt.Bytes()).SetDestinationPort(l.port)
	case header.UDPProtocolNumber:
		header.UDP(pkt.Bytes()).SetDestinationPort(l.port)
	default:
		return errors.Errorf("not support protocol %d", peer.Proto)
	}
	return l.conn.Send(ctx, pkt, peer)
}

type port atomic.Uint64

func NewPort(p uint16) *port {
	var a = &atomic.Uint64{}
	a.Store(uint64(p) << 48)
	return (*port)(a)
}
func (p *port) p() *atomic.Uint64 { return (*atomic.Uint64)(p) }
func (p *port) Idle() bool {
	d := p.p().Load()
	const flags uint64 = 0xffff000000000000

	p.p().Store(d & flags)
	return d&(^flags) == 0
}
func (p *port) Port() uint16 { return uint16(p.p().Add(1) >> 48) }

func (t *linkManager) cleanup() {
	var (
		links  []uplink
		lports []uint16
	)
	t.uplinkMu.Lock()
	for i := 0; i < t.ttl.Size(); i++ {
		i := t.ttl.Pop()
		if i.valid() && time.Since(i.t) > t.duration {
			p := t.uplinkMap[i.s]
			if p.Idle() {
				links = append(links, i.s)
				lports = append(lports, p.Port())
				delete(t.uplinkMap, i.s)
			} else {
				t.ttl.Put(ttlkey{i.s, time.Now()})
			}
		} else {
			t.ttl.Put(ttlkey{i.s, time.Now()})
			break
		}
	}
	t.uplinkMu.Unlock()
	if len(links) == 0 {
		return
	}

	var conns []*Conn
	t.donwlinkMu.Lock()
	for i, e := range links {
		s := downlink{Server: e.Server, Proto: e.Proto, Local: netip.AddrPortFrom(t.addr, lports[i])}
		conns = append(conns, t.downlinkMap[s].conn)
		delete(t.downlinkMap, s)
	}
	t.donwlinkMu.Unlock()

	for i, e := range links {
		t.ap.DelPort(e.Proto, lports[i], e.Server)
	}
	for _, e := range conns {
		if e != nil {
			e.Dec()
		}
	}
}

func (t *linkManager) Add(s uplink, c *Conn) (localPort uint16, err error) {
	t.cleanup()

	localPort, err = t.ap.GetPort(s.Proto, s.Server)
	if err != nil {
		return 0, err
	}

	t.uplinkMu.Lock()
	t.uplinkMap[s] = NewPort(localPort)
	t.ttl.Put(ttlkey{s: s, t: time.Now()})
	t.uplinkMu.Unlock()

	t.donwlinkMu.Lock()
	t.downlinkMap[downlink{
		Server: s.Server,
		Proto:  s.Proto,
		Local:  netip.AddrPortFrom(t.addr, localPort),
	}] = &link{
		conn: c,
		port: s.Process.Port(),
	}
	t.donwlinkMu.Unlock()

	return localPort, nil
}

// Uplink get uplink packet local port
func (t *linkManager) Uplink(s uplink) (localPort uint16, has bool) {
	t.uplinkMu.RLock()
	defer t.uplinkMu.RUnlock()
	p, has := t.uplinkMap[s]
	if !has {
		return 0, false
	}
	return p.Port(), true
}

// Downlink get donwlink packet proxyer and client port
func (t *linkManager) Downlink(s downlink) (p *link, has bool) {
	t.donwlinkMu.RLock()
	defer t.donwlinkMu.RUnlock()

	key, has := t.downlinkMap[s]
	if !has {
		return nil, false
	}
	return key, true
}

func (t *linkManager) Close() error {
	return t.ap.Close()
}
