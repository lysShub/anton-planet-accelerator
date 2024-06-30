package game

import (
	"net/netip"

	"github.com/lysShub/netkit/packet"
	"gvisor.dev/gvisor/pkg/tcpip"
)

type Game interface {
	Start()
	Capture(pkt *packet.Packet) (Info, error)
	Close() error
}

type Info struct {
	Proto    tcpip.TransportProtocolNumber // 协议 udp/tcp
	Server   netip.Addr                    // 目标地址
	PlayData bool                          // 对局数据, 要求低延迟
}
