package tcp

import (
	"net/netip"
	"sync"

	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/packet"
	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

// TCPConn 约定
//
//	先调用Write的触发握手（不会有server主动发送数据的情况，因为Conn只提供WriteTo）。
//	先收到SYN的接受握手
type TCPConn struct {
	laddr netip.AddrPort
	raw   IPConn

	eps *eps
}

// Bind bind a datagram pseudo-tcp connect, require call ReadFromAddrPort always.
func Bind(laddr netip.AddrPort) (*TCPConn, error) {
	var c = &TCPConn{eps: neweps()}
	var err error

	c.raw, err = BindIPConn(laddr, header.TCPProtocolNumber)
	if err != nil {
		return nil, err
	}

	c.laddr = c.raw.AddrPort()
	return c, nil
}

func (c *TCPConn) WriteToAddrPort(b *packet.Packet, to netip.AddrPort) (err error) {
	t := c.eps.get(to)
	if t == nil {
		t = c.eps.set(to, newPseudoTCP(to, c, true))
	}

	return t.Send(b)
}

func (c *TCPConn) ReadFromAddrPort(b *packet.Packet) (netip.AddrPort, error) {
	head, data := b.Head(), b.Data()

	rip, err := c.raw.ReadFromAddr(b)
	if err != nil {
		return netip.AddrPort{}, err
	} else if b.Data() < header.TCPMinimumSize {
		return netip.AddrPort{}, errorx.WrapTemp(errors.Errorf("tcp packet %#v", b.Bytes()))
	}
	tcp := header.TCP(b.Bytes())
	raddr := netip.AddrPortFrom(rip, tcp.SourcePort())

	t := c.eps.get(raddr)
	if t == nil {
		t = c.eps.set(raddr, newPseudoTCP(raddr, c, false))
	}
	if err := t.Recv(b); err != nil {
		return netip.AddrPort{}, err
	}

	if b.Data() == 0 {
		return c.ReadFromAddrPort(b.Sets(head, data))
	} else {
		return raddr, nil
	}
}

func (c *TCPConn) LocalAddr() netip.AddrPort { return c.laddr }
func (c *TCPConn) Close() error              { return c.raw.Close() }

type eps struct {
	mu  sync.RWMutex
	eps map[netip.AddrPort]*pseudoTCP
}

func neweps() *eps {
	return &eps{
		eps: map[netip.AddrPort]*pseudoTCP{},
	}
}

func (e *eps) get(raddr netip.AddrPort) *pseudoTCP {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.eps[raddr]
}
func (e *eps) set(raddr netip.AddrPort, ep *pseudoTCP) *pseudoTCP {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.eps[raddr] = ep
	return ep
}
func (e *eps) del(raddr netip.AddrPort) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.eps, raddr)
}
