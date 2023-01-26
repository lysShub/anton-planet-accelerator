package proxy

import (
	"net"
	"net/netip"

	"github.com/lysShub/warthunder/client/divert"
	"github.com/lysShub/warthunder/context"
)

/*
	数据包中有被代理应用app本地caddr和请求服务器server地址saddr。
	代理客户端proxyClient和代理服务器proxyServer之间只建立一个UDP连接:
		proxyServer收到一个数据包后, 判断是否是新的saddr, 如果是新的, 则proxyServer
	将会和server之间建立一条新的udp连接。同时维护一个表, key为saddr, 值为caddr。
		如果一个应用需要占用n个端口, 那么一台服务器最多代理n/2^16个客户端。
	在session层里, 是无状态的, 由caddr-saddr确定是否是使用新端口。
*/

// type Upack struct {
// 	sock
// 	data []byte
// }

// sock 5-tuple define a network connection
type sock struct {
	proto        divert.Proto
	laddr, raddr netip.AddrPort
}

// NewProxy 代理一个应用的UDP出网流量
func NewProxy(ctx context.Ctx, pid uint32, proxyConn net.Conn) {
	newProxy(ctx, pid, proxyConn)
}
