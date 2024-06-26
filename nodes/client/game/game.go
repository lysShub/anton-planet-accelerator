package game

import (
	"net/netip"

	"github.com/lysShub/netkit/packet"
	"github.com/pkg/errors"
)

type Game interface {
	Capture(pkt *packet.Packet) (Info, error)
	Close() error
}

type Info struct {
	Proto    uint8      // 协议 udp/tcp
	Server   netip.Addr // 目标地址
	PlayData bool       // 对局数据
}

func New(name string) (game Game, err error) {
	switch name {
	case Warthunder:
		return newWarthundr()
	default:
		return nil, errors.Errorf("not support game %s", name)
	}
}
