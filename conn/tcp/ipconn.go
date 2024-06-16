package tcp

import (
	"net/netip"

	"github.com/lysShub/netkit/packet"
)

// IPConn a bind loacal addr, protocol and port ip connect
type IPConn interface {

	// ReadFromAddr read ip payload
	ReadFromAddr(b *packet.Packet) (netip.Addr, error)

	// WriteToAddr write ip payload to remote
	WriteToAddr(b *packet.Packet, to netip.Addr) error

	AddrPort() netip.AddrPort

	Close() error
}
