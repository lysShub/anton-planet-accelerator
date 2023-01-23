package proxy

import (
	"net"
	"net/netip"

	"github.com/lysShub/warthunder/context"
)

type Upack struct {
	sockIdx uint16

	sock

	data []byte
}

func (u *Upack) Marshal() []byte {

	return nil
}

// sock 5-tuple define a network connection
type sock struct {
	laddr, raddr netip.AddrPort
}

// NewProxy 代理一个应用的UDP出网流量
func NewProxy(ctx context.Ctx, pid int, proxyConn net.Conn) {
	// return newProxy(ctx, pid, filter, proxyConn)
	return
}
