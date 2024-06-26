package game

import (
	"net/netip"

	"github.com/lysShub/netkit/packet"
)

type Game interface {
	Start()
	Capture(pkt *packet.Packet) (Info, error)
	Close() error
}

type Info struct {
	Proto    uint8      // 协议 udp/tcp
	Server   netip.Addr // 目标地址
	PlayData bool       // 对局数据
}
