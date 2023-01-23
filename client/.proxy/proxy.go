package proxy

import (
	"net"
	"net/netip"

	"github.com/lysShub/warthunder/client/divert"
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
	proto        divert.Proto
	laddr, raddr netip.AddrPort
}

// Marshal marshal sock {laddr:raddr} to []byte,
// is a part of IP header
func (s *sock) IPMarshal() []byte {
	l4, r4 := s.laddr.Addr().Is4(), s.raddr.Addr().Is4()
	if l4 && r4 {
		bl := s.laddr.Addr().As4()
		br := s.raddr.Addr().As4()
		return append(bl[:], br[:]...)
	} else if !l4 && !r4 {
		bl := s.laddr.Addr().As16()
		br := s.raddr.Addr().As16()
		return append(bl[:], br[:]...)
	} else {
		// TODO: ipv4 on ipv6
		panic("not support")
	}
}

type Io interface {
	Read(p *Upack) (err error)
	Write(p *Upack) (err error)
}

// 尝试代理一个应用的网络
// 代理时，可能已经产生了网络连接, 对于已经建立的TCP连接, 是不能被代理的。
// 对于UDP, 我们会代理其数据包, 即使是已经建立的连接的数据包
func NewProxy(ctx context.Ctx, pid int, filter string, proxyConn net.Conn) {
	// return newProxy(ctx, pid, filter, proxyConn)
	return
}
