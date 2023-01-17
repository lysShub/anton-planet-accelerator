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

type Proxy interface {
	// 代理一个Process的网络数据包
	// 由于divert Network不支持pid, 所以代理的逻辑是: 接收Divert Flow事件, 让操作一个Divert Network。
	// 这样会有两个问题：
	//     1. 接收到Divert Flow到启动Divert Network时, 应用可能已经发送了数据包、尤其对于TCP, 回导致代理错误。
	//     2. 如果应用在代理之前启动、并且创建了socket,  那么是否代理那些socket? (对于TCP, 肯定是不能代理的)
	// 问题1暂时不用解决, 因为这是边界情况
	// 问题2, 只代理新建的tcp连接

	// Read read a packet from the process, and write it to the proxyConn
	Read(p *Upack) (err error)

	// Write read a packet from the proxyConn, and write it to the process
	Write(p *Upack) (err error)
}

func NewProxy(ctx context.Ctx, pid int, filter string, proxyConn net.Conn) Proxy {
	// return newProxy(ctx, pid, filter, proxyConn)

	return nil
}
