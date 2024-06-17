package tcp

import (
	"math/rand"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/packet"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

// PseudoTCP
// 1. 握手不用等待，大概发就行了
// 2. 没有重传、流控
// 3. 忽略 Close
type PseudoTCP struct {
	conn         *TCPConn
	raddr        netip.Addr
	dial         bool
	lport, rport uint16
	pseudo1      uint16

	mu          sync.RWMutex
	cached      *packet.Packet
	established bool
	isn, ian    uint32

	magic  uint32 // use for keepalive
	sndNxt atomic.Uint32
	rcvNxt atomic.Uint32
}

func NewPseudoTCP(remote netip.AddrPort, conn *TCPConn, dial bool) *PseudoTCP {
	var p = &PseudoTCP{
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
	p.sndNxt.Store(p.isn)

	time.AfterFunc(keepalive, p.keepalive)
	return p
}

func (p *PseudoTCP) Send(pkt *packet.Packet) error {
	if pkt == nil || pkt.Data() == 0 {
		return nil
	}

	if p.handshaked() {
		return p.send(pkt, header.TCPFlagPsh|header.TCPFlagAck, 0, 0)
	} else {
		p.mu.Lock()
		defer p.mu.Unlock()
		if p.cached == nil {
			p.cached = pkt.Clone()
		}

		if p.dial {
			if p.ian == 0 {
				return p.send(packet.Make(64), header.TCPFlagSyn, p.isn, 0)
			} else {
				return p.send(packet.Make(64), header.TCPFlagAck, p.isn+1, p.ian+1)
			}
		} else {
			if p.ian != 0 {
				return p.send(packet.Make(64), header.TCPFlagSyn|header.TCPFlagAck, p.isn, p.ian+1)
			}
		}
	}
	return nil
}

func (p *PseudoTCP) send(pkt *packet.Packet, flags header.TCPFlags, seq, ack uint32) error {
	if seq == 0 {
		seq = p.sndNxt.Load()
	}
	if ack == 0 {
		ack = p.rcvNxt.Load()
	}
	if !flags.Contains(header.TCPFlagAck) {
		ack = 0
	}

	payload := uint32(pkt.Data())
	hdr := header.TCP(pkt.AttachN(header.TCPMinimumSize).Bytes())
	hdr.Encode(&header.TCPFields{
		SrcPort:       p.lport,
		DstPort:       p.rport,
		SeqNum:        seq,
		AckNum:        ack,
		DataOffset:    header.TCPMinimumSize,
		Flags:         flags,
		WindowSize:    0xffff, // todo: calc it
		Checksum:      0,
		UrgentPointer: 0,
	})
	sum := checksum.Combine(p.pseudo1, uint16(len(hdr)))
	sum = checksum.Checksum(hdr, sum)
	hdr.SetChecksum(^sum)

	if err := p.conn.raw.WriteToAddr(pkt, p.raddr); err != nil {
		return err
	}

	if payload+seq > p.sndNxt.Load() {
		p.sndNxt.Store(seq + payload)
	}
	if ack > p.rcvNxt.Load() {
		p.rcvNxt.Store(ack)
	}
	return nil
}

func (p *PseudoTCP) Recv(tcp *packet.Packet) error {
	if tcp == nil || tcp.Data() < header.TCPMinimumSize {
		return nil
	}

	if p.handshaked() {
		return p.recv(tcp)
	} else {
		hdr := header.TCP(tcp.Bytes())
		defer tcp.SetData(0)
		p.mu.Lock()
		defer p.mu.Unlock()

		if p.dial {
			if hdr.Flags() == header.TCPFlagSyn|header.TCPFlagAck {
				if p.ian == 0 {
					p.ian = hdr.SequenceNumber()
				}
				if err := p.send(packet.Make(64), header.TCPFlagAck, p.isn+1, p.ian+1); err != nil {
					return err
				}

				p.established = true
				defer func() { p.cached = nil }()
				return p.send(p.cached, header.TCPFlagPsh|header.TCPFlagAck, 0, 0)
			}
		} else {
			switch hdr.Flags() {
			case header.TCPFlagSyn:
				if p.ian == 0 {
					p.ian = hdr.SequenceNumber()
				}
				return p.send(packet.Make(64), header.TCPFlagSyn|header.TCPFlagAck, p.isn, p.ian+1)
			case header.TCPFlagAck:
				if hdr.AckNumber() == p.isn+1 {
					p.established = true
				} else {
					if debug.Debug() {
						println("server recv invalid ack", p.isn, hdr.AckNumber())
					}
				}
			default:
			}
		}
	}
	return nil
}

func (p *PseudoTCP) recv(tcp *packet.Packet) error {
	hdr := header.TCP(tcp.Bytes())
	tcp.DetachN(int(hdr.DataOffset()))

	nxt := hdr.SequenceNumber() + uint32(len(hdr.Payload()))
	if nxt > p.rcvNxt.Load() {
		p.rcvNxt.Store(nxt)
	}
	return nil
}

func (p *PseudoTCP) handshaked() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.established
}

const keepalive time.Duration = time.Second * 15

func (p *PseudoTCP) keepalive() {
	newMagic := (p.sndNxt.Load() &^ p.rcvNxt.Load())
	if p.magic == newMagic {
		// todo: rst
		p.conn.eps.del(netip.AddrPortFrom(p.raddr, p.rport))
	} else {
		p.magic = newMagic
		time.AfterFunc(keepalive, p.keepalive)
	}
}
