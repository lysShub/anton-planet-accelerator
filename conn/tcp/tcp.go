package tcp

import (
	"math/rand"
	"net/netip"
	"sync"
	"time"

	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/packet"
	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

// TCPConn 约定先调用Write的作为握手发起方（不会有server主动发送数据的情况，因为Conn只提供WriteTo）。
type TCPConn struct {
	laddr netip.AddrPort
	raw   IPConn

	eps *eps
}

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
	t := c.eps.getep(to)
	if t == nil {
		t = newPseudoTCP(to, c, true)
		c.eps.setep(to, t)
		if err := t.send_sync(); err != nil {
			return err
		}
	}

	return t.send(b, header.TCPFlagPsh)
}

func (c *TCPConn) ReadFromAddrPort(b *packet.Packet) (netip.AddrPort, error) {
	head, data := b.Head(), b.Data()

	rip, err := c.raw.ReadFromAddr(b)
	if err != nil {
		return netip.AddrPort{}, err
	} else if b.Data() < header.TCPMinimumSize {
		return netip.AddrPort{}, errorx.WrapTemp(errors.Errorf("tcp packet %#v", b.Bytes()))
	}

	println("收到")

	tcp := header.TCP(b.Bytes())
	raddr := netip.AddrPortFrom(rip, tcp.SourcePort())

	t := c.eps.getep(raddr)
	if t == nil {
		t = newPseudoTCP(raddr, c, false)
		c.eps.setep(raddr, t)
	}
	t.recv(b)

	if b.Data() == 0 {
		if tcp.Flags() == header.TCPFlagSyn {
			if err := t.recv_sync(); err != nil {
				return netip.AddrPort{}, err
			}
		}
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

func (e *eps) getep(raddr netip.AddrPort) *pseudoTCP {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.eps[raddr]
}
func (e *eps) setep(raddr netip.AddrPort, ep *pseudoTCP) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.eps[raddr] = ep
}
func (e *eps) delep(raddr netip.AddrPort) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.eps, raddr)
}

// pseudoTCP
// 1. 握手不用等待，大概发就行了
// 2. 没有重传、流控
// 3. 忽略 Close
type pseudoTCP struct {
	conn         *TCPConn
	raddr        netip.Addr
	dial         bool
	lport, rport uint16
	pseudo1      uint16

	mu     sync.RWMutex
	alive  bool
	sndNxt uint32
	rcvNxt uint32
}

func newPseudoTCP(remote netip.AddrPort, conn *TCPConn, dial bool) *pseudoTCP {
	var p = &pseudoTCP{
		conn:  conn,
		raddr: remote.Addr(),
		dial:  dial,
		lport: conn.LocalAddr().Port(),
		rport: remote.Port(),
		pseudo1: header.PseudoHeaderChecksum(
			header.TCPProtocolNumber,
			tcpip.AddrFrom4(conn.LocalAddr().Addr().As4()),
			tcpip.AddrFrom4(remote.Addr().As4()),
			0,
		),

		sndNxt: rand.Uint32(), // isn
	}

	time.AfterFunc(keepalive, p.keepalive)
	return p
}

func (p *pseudoTCP) send(pkt *packet.Packet, flags header.TCPFlags) error {
	payload := pkt.Data()
	nxt := uint32(0)
	if flags.Contains(header.TCPFlagAck) {
		nxt = p.rcvNxt
	}

	hdr := header.TCP(pkt.AttachN(header.TCPMinimumSize).Bytes())
	hdr.Encode(&header.TCPFields{
		SrcPort:       p.lport,
		DstPort:       p.rport,
		SeqNum:        p.sndNxt,
		AckNum:        nxt,
		DataOffset:    header.TCPMinimumSize,
		Flags:         flags,
		WindowSize:    2048,
		Checksum:      0,
		UrgentPointer: 0,
	})
	sum := checksum.Combine(p.pseudo1, uint16(len(hdr)))
	sum = checksum.Checksum(hdr, sum)
	hdr.SetChecksum(^sum)

	if err := p.conn.raw.WriteToAddr(pkt, p.raddr); err != nil {
		return err
	}

	if payload > 0 {
		p.mu.Lock()
		p.sndNxt += uint32(payload)
		p.alive = true
		p.mu.Unlock()
	}
	return nil
}

func (t *pseudoTCP) recv(tcp *packet.Packet) error {
	hdr := header.TCP(tcp.Bytes())

	nxt := hdr.SequenceNumber() + uint32(len(hdr.Payload()))
	t.mu.Lock()
	if nxt > t.rcvNxt {
		t.alive = true
		t.rcvNxt = nxt
	}
	t.mu.Unlock()

	tcp.DetachN(int(hdr.DataOffset()))
	return nil
}

func (p *pseudoTCP) send_sync() error {
	if !p.dial {
		panic("")
	}
	return p.send(packet.Make(64, 0), header.TCPFlagSyn)
}

func (p *pseudoTCP) recv_sync() error {
	if p.dial {
		panic("")
	}
	return p.send(packet.Make(64, 0), header.TCPFlagSyn|header.TCPFlagAck)
}

const keepalive time.Duration = time.Second * 15

func (p *pseudoTCP) keepalive() {
	p.mu.RLock()
	alive := p.alive
	p.mu.RUnlock()

	if !alive {
		p.conn.eps.delep(netip.AddrPortFrom(p.raddr, p.rport))
	} else {
		p.mu.Lock()
		p.alive = false
		p.mu.Unlock()
		time.AfterFunc(keepalive, p.keepalive)
	}
}