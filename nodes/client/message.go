package client

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"math/rand"
	"net/netip"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"
	"unsafe"

	"github.com/lysShub/anton-planet-accelerator/bvvd"
	"github.com/lysShub/anton-planet-accelerator/nodes"
	"github.com/lysShub/netkit/packet"
	"github.com/pkg/errors"
)

type messageManager struct {
	id   atomic.Uint32
	buff *nodes.Heap[nodes.Message]
}

func newMessageManager() *messageManager {
	var m = &messageManager{
		buff: nodes.NewHeap[nodes.Message](16),
	}
	m.id.Store(rand.Uint32())
	return m
}

func (m *messageManager) ID() uint32 {
	id := m.id.Add(1) - 1
	if id == 0 {
		return m.ID()
	}
	return id
}

func (m *messageManager) Put(paddr netip.AddrPort, hdr bvvd.Fields, payload *packet.Packet) error {
	var msg = nodes.Message{
		Kind:  hdr.Kind,
		LocID: hdr.LocID,
		Peer:  paddr,
	}

	if payload.Data() < 4 {
		return errors.Errorf("recv invalid message payload %s", hex.EncodeToString(payload.Bytes()))
	}
	msg.ID = binary.BigEndian.Uint32(payload.Bytes())
	payload.DetachN(4)

	switch hdr.Kind {
	case bvvd.PingProxyer, bvvd.PingForward:
	case bvvd.PackLossClientUplink, bvvd.PackLossProxyerUplink, bvvd.PackLossProxyerDownlink:
		var pl nodes.PL
		if err := pl.Decode(payload); err != nil {
			return err
		}
		msg.Raw = pl
	default:
		return errors.Errorf("unknown kind %s", hdr.Kind.String())
	}

	m.buff.MustPut(msg)
	return nil
}

func (m *messageManager) PopByID(id uint32) nodes.Message {
	return m.buff.PopBy(func(e nodes.Message) (pop bool) { return e.ID == id })
}

func (m *messageManager) PopBy(fn func(nodes.Message) (pop bool), timeout time.Duration) (nodes.Message, bool) {
	return m.buff.PopByDeadline(fn, time.Now().Add(timeout))
}

type NetworkStates struct {
	PingProxyer             time.Duration
	PingForward             time.Duration
	PackLossClientUplink    nodes.PL
	PackLossClientDownlink  nodes.PL
	PackLossProxyerUplink   nodes.PL
	PackLossProxyerDownlink nodes.PL
}

const InvalidRtt = time.Hour

func (n *NetworkStates) String() string {
	var s = &strings.Builder{}

	p2 := n.PingForward
	if p2 < InvalidRtt && n.PingProxyer < InvalidRtt && p2 > n.PingProxyer {
		p2 -= n.PingProxyer
	}
	var elems = []string{
		"ping", n.strdur(n.PingProxyer), n.strdur(p2),
		"pl↑", n.PackLossClientUplink.String(), n.PackLossProxyerUplink.String(),
		"pl↓", n.PackLossClientDownlink.String(), n.PackLossProxyerDownlink.String(),
	}

	const size = 6
	for i, e := range elems {
		s.WriteString(e)

		n := size - utf8.RuneCount(unsafe.Slice(unsafe.StringData(e), len(e)))
		for i := 0; i < n; i++ {
			s.WriteByte(' ')
		}

		if (i+1)%3 == 0 {
			s.WriteByte('\n')
		}
	}
	return s.String()
}

func (*NetworkStates) strdur(dur time.Duration) string {
	if dur <= 0 || time.Minute <= dur {
		return "--.-"
	}
	ss := dur.Seconds() * 1000

	s1 := int(math.Round(ss))
	s2 := int((ss - float64(s1)) * 10)
	if s2 < 0 {
		s2 = 0
	}
	return fmt.Sprintf("%d.%d", s1, s2)
}