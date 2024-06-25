package client

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"
	"unsafe"

	"github.com/lysShub/anton-planet-accelerator/nodes"
	"github.com/lysShub/netkit/packet"
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

func (m *messageManager) Put(pkt *packet.Packet) error {
	var msg = nodes.Message{}
	if err := msg.Decode(pkt); err != nil {
		return err
	}

	m.buff.MustPut(msg)
	return nil
}

func (m *messageManager) PopByID(id uint32) nodes.Message {
	return m.buff.PopBy(func(e nodes.Message) (pop bool) { return e.MsgID == id })
}

func (m *messageManager) PopBy(fn func(nodes.Message) (pop bool), timeout time.Duration) (smg nodes.Message, ok bool) {
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
