package tcp

import (
	"math/rand"
	"net/netip"
	"sync"
	"time"

	"github.com/lysShub/netkit/packet"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

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

	mu          sync.RWMutex
	alive       bool
	acked       bool
	established bool
	isn         uint32
	sndNxt      uint32
	rcvNxt      uint32
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
	}
	for p.isn == 0 {
		p.isn = rand.Uint32()
	}
	p.sndNxt = p.isn

	time.AfterFunc(keepalive, p.keepalive)
	return p
}

func (p *pseudoTCP) Send(pkt *packet.Packet) error {
	if p.handshaked() {
		return p.send(pkt, header.TCPFlagPsh)
	} else {
		// todo: cache this packet
		p.mu.Lock()
		defer p.mu.Unlock()
		if p.dial {
			if p.rcvNxt == 0 {
				return p.send(packet.Make(64), header.TCPFlagSyn)
			} else {
				return p.send(packet.Make(64), header.TCPFlagAck)
			}
		} else {
			if p.rcvNxt != 0 {
				return p.send(packet.Make(64), header.TCPFlagSyn|header.TCPFlagAck)
			}
		}
	}
	return nil
}

func (p *pseudoTCP) send(pkt *packet.Packet, flags header.TCPFlags) error {
	ack := uint32(0)
	if flags.Contains(header.TCPFlagAck) {
		ack = p.rcvNxt
	}

	payload := pkt.Data()
	hdr := header.TCP(pkt.AttachN(header.TCPMinimumSize).Bytes())
	hdr.Encode(&header.TCPFields{
		SrcPort:       p.lport,
		DstPort:       p.rport,
		SeqNum:        p.sndNxt,
		AckNum:        ack,
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
		p.sndNxt += uint32(payload)
		p.alive = true
	}
	return nil
}

func (p *pseudoTCP) Recv(tcp *packet.Packet) error {
	if p.handshaked() {
		return p.recv(tcp)
	} else {
		defer tcp.SetData(0)
		tcp := header.TCP(tcp.Bytes())

		p.mu.Lock()
		defer p.mu.Unlock()
		if p.dial {
			if tcp.Flags() == header.TCPFlagSyn|header.TCPFlagAck {
				if !p.acked {
					p.rcvNxt = tcp.AckNumber() + 1
					p.acked = true
				}
				return p.send(packet.Make(64), header.TCPFlagAck)
			}
		} else {
			switch tcp.Flags() {
			case header.TCPFlagSyn:
				if !p.acked {
					p.rcvNxt = tcp.SequenceNumber() + 1
					p.acked = true
				}
				return p.send(packet.Make(64), header.TCPFlagSyn|header.TCPFlagAck)
			case header.TCPFlagAck:
				if tcp.AckNumber() == p.isn+1 {
					p.established = true
				} else {
					panic("")
				}
			default:
			}
		}
	}
	return nil
}

func (p *pseudoTCP) recv(tcp *packet.Packet) error {
	hdr := header.TCP(tcp.Bytes())
	tcp.DetachN(int(hdr.DataOffset()))

	nxt := hdr.SequenceNumber() + uint32(len(hdr.Payload()))

	if p.mu.TryLock() {
		defer p.mu.Unlock()
	}
	if nxt > p.rcvNxt {
		p.alive = true
		p.rcvNxt = nxt
	}
	return nil
}

func (p *pseudoTCP) handshaked() bool {
	if p.mu.TryRLock() {
		defer p.mu.RUnlock()
	}
	return p.established
}

const keepalive time.Duration = time.Second * 15

func (p *pseudoTCP) keepalive() {
	p.mu.RLock()
	alive := p.alive
	p.mu.RUnlock()

	if !alive {
		p.conn.eps.del(netip.AddrPortFrom(p.raddr, p.rport))
	} else {
		p.mu.Lock()
		p.alive = false
		p.mu.Unlock()
		time.AfterFunc(keepalive, p.keepalive)
	}
}

func (p *pseudoTCP) close(cause error) error {
	panic(cause)
}
