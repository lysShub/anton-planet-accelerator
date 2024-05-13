package server

import (
	"context"
	"net/netip"
	"sync"
	"syscall"

	"github.com/lysShub/fatcp"
	"github.com/lysShub/fatcp/ports"
	"github.com/lysShub/netkit/packet"
	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type linkManager struct {
	ap *ports.Adapter

	uplinks  map[upkey]uint16
	uplinkMu sync.RWMutex

	downlinks  map[downkey]*donwlink
	downlinkMu sync.RWMutex
}

func newLinkManager(addr netip.Addr) *linkManager {
	var m = &linkManager{
		ap:        ports.NewAdapter(addr),
		uplinks:   map[upkey]uint16{},
		downlinks: map[downkey]*donwlink{},
	}
	return m
}

type upkey struct {
	proto tcpip.TransportProtocolNumber

	// destionation server-address
	server netip.AddrPort
}

type downkey struct {
	// local port
	local uint16

	proto tcpip.TransportProtocolNumber

	// destionation server-address
	server netip.AddrPort
}

type donwlink struct {
	conn *fatcp.Conn

	// client port
	port uint16
}

func (d *donwlink) Downlink(ctx context.Context, pkt *packet.Packet, srv fatcp.Peer) error {
	header.UDP(pkt.Bytes()).SetDestinationPort(d.port)
	return d.conn.Send(ctx, pkt, srv)
}

// uplink malloc local port by destination server-address.
func (m *linkManager) uplink(d upkey) (port uint16, has bool) {
	m.uplinkMu.RLock()
	defer m.uplinkMu.RUnlock()

	port, has = m.uplinks[d]
	return port, has
}

func (m *linkManager) downlink(s downkey) (down *donwlink, has bool) {
	m.downlinkMu.RLock()
	defer m.downlinkMu.RUnlock()

	down, has = m.downlinks[s]
	return down, has
}

type session struct {
	src   netip.AddrPort
	proto uint8
	dst   netip.AddrPort
}

func (m *linkManager) add(clientPort uint16, key upkey, conn *fatcp.Conn) (port uint16, err error) {
	port, err = m.ap.GetPort(tcpip.TransportProtocolNumber(key.proto), key.server)
	if err != nil {
		return 0, err
	}

	m.uplinkMu.Lock()
	m.uplinks[upkey{proto: key.proto, server: key.server}] = port
	m.uplinkMu.Unlock()

	m.downlinkMu.Lock()
	m.downlinks[downkey{
		local: port, proto: key.proto, server: key.server,
	}] = &donwlink{
		conn: conn,
		port: clientPort,
	}
	m.downlinkMu.Unlock()

	return port, nil
}

func (s *linkManager) Close() error {
	return s.ap.Close()
}

func getDownkeyAndStripIPHeader(ip *packet.Packet) (downkey, error) {
	hdr := header.IPv4(ip.Bytes())
	switch ver := header.IPVersion(hdr); ver {
	case 4:
	default:
		return downkey{}, errors.Errorf("not support ip version %d", ver)
	}
	switch proto := hdr.Protocol(); proto {
	case syscall.IPPROTO_TCP, syscall.IPPROTO_UDP:
	default:
		return downkey{}, errors.Errorf("not support protocol %d", proto)
	}
	defer ip.SetHead(ip.Head() + int(hdr.HeaderLength()))

	var t = header.UDP(hdr.Payload())
	return downkey{
		local: t.DestinationPort(),
		proto: hdr.TransportProtocol(),
		server: netip.AddrPortFrom(
			netip.AddrFrom4(hdr.SourceAddress().As4()),
			t.SourcePort(),
		),
	}, nil
}
