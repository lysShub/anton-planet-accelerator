package proxy

import (
	"context"
	"fmt"
	"net/netip"
	"sync"
	"warthunder/client/divert"
	"warthunder/ctx"
	"warthunder/util"
)

type Upack struct {
	sockIdx uint16 // 单增, 五元组确定一个sock

	proto uint8
	lPort uint16
	lAddr [16]uint8
	rPort uint16
	rAddr [16]uint8

	data []byte
}

type sock struct {
	proto        divert.Proto
	laddr, raddr netip.AddrPort
}

// 代理一个process的传输层数据
type Proxy struct {
	// 原理:
	// NewPorxy 时,
	// 1. 先根据pid, 找到程序已经建立的socket table, 而这个table只有laddr, 所以把它
	// 加入到Network 阻塞代理中, 并且等待其发送数据时, 就可以获取到raddr了
	// 2. 监听所有Flow with pid事件
	//
	// 对于1任务, 如果是unconnected socket, 需要终身捕获。而且需要把读取到的数据包导入正确的数据流中

	pid          int
	parentFilter string
	proxyConn    Io

	sockArr []bool

	// 接收表, 用于代理在作用之前, proces已经建立的连接
	// sock's raddr is null
	// udp unconnected socket 需要终身捕获
	acceptTable map[sock]context.CancelFunc

	// 代理table
	proxyTable map[sock]context.CancelFunc

	m *sync.RWMutex
}

func NewProxy(c context.Context, pid int, filter string, proxyConn Io) *Proxy {
	c1 := ctx.WithFatal(c)

	p := &Proxy{
		pid:          pid,
		parentFilter: filter,
		proxyConn:    proxyConn,
	}
	go func() {
		<-c.Done()

	}()

	//
	uts, err := util.GetUDPTableByPid(pid)
	if err != nil {
		panic(err)
	}
	for _, ut := range uts {
		p.accept(c1, sock{
			proto: divert.IPPROTO_UDP,
			laddr: netip.AddrPortFrom(ut.LocalAddr(), ut.LocalPort()),
		}, ut.Connected())
	}

	return p
}

// func (p *Proxy) acceptUDP(ctx ctx.Ctx, laddr netip.AddrPort, connected bool) {
// 	has := false
// 	p.m.Lock()
// 	if _, has = p.acceptTable[laddr]; !has {
// 		p.acceptTable[laddr] = struct{}{}
// 	}
// 	p.m.Unlock()

// 	if !has {
// 		go p.accept(ctx, laddr, divert.IPPROTO_UDP, connected)
// 	}
// }

// func (p *Proxy) acceptTCP(ctx ctx.Ctx, laddr netip.AddrPort) {
// 	has := false
// 	p.m.Lock()
// 	if _, has = p.acceptTable[laddr]; !has {
// 		p.acceptTable[laddr] = struct{}{}
// 	}
// 	p.m.Unlock()

// 	if !has {
// 		go p.accept(ctx, laddr, tcp, true)
// 	}
// }

func (p *Proxy) accept(c ctx.Ctx, s sock, connected bool) {
	var cancel context.CancelFunc
	switch s.proto {
	case divert.IPPROTO_UDP:
		if !connected {
			// TODO: 尝试ctx.WithTimeout只返回cancel, 这样使用者就可以不用引入官方context包
			_, cancel = context.WithCancel(c)
		}
	case divert.IPPROTO_TCP:
	default:
		panic("not support prto " + s.proto.String())
	}

	p.acceptTable[s] = cancel

	defer func() {
		p.m.Lock()
		delete(p.acceptTable, s)
		p.m.Unlock()
	}()

	var f = fmt.Sprintf("%s.SrcPort = %d and outbound", s.proto, s.laddr.Port())
	// TODO: 比proxy更高优先级
	h, err := divert.Open(f, divert.LAYER_NETWORK, 111, divert.FLAG_READ_ONLY)
	if err != nil {
		c.Fatal(err)
		return
	}
	defer h.Close()

	var d = make([]byte, 65535)
	var u = &Upack{data: make([]byte, 65535)}
	var n int
	var addr divert.Address
	for {
		if n, addr, err = h.Recv(d); err != nil {
			c.Fatal(err)
			return
		} else {
			a := addr.Network()
			if false {
				fmt.Println(a, n)
			}

			// TODO: parse packet
			var raddr netip.AddrPort
			p.addProxy(sock{proto: s.proto, laddr: s.laddr, raddr: raddr})

			if err = p.proxyConn.Write(u); err != nil {
				c.Fatal(err)
				return
			}
		}

		select {
		case <-c.Done():
			return
		default:
			if connected {
				break
			}
		}
	}
}

func (p *Proxy) addProxy(s sock) {
	switch s.proto {
	case divert.IPPROTO_UDP, divert.IPPROTO_TCP:
	default:
		panic("not support proto " + s.proto.String())
	}

	sockIdx := p.getSockIdx()
	var f = fmt.Sprintf("%s.SrcPort=%d and %s.DstPort=%d and outbound", s.proto, s.laddr.Port(), s.proto, s.raddr.Port())

	divert.Open(f, divert.LAYER_NETWORK, 11, divert.FLAG_READ_ONLY)
	fmt.Println(sockIdx)
}

func (p *Proxy) remProxy(laddr, raddr netip.AddrPort) {}

func (p *Proxy) getSockIdx() uint16 {
	p.m.Lock()
	defer p.m.Unlock()

	if len(p.sockArr) < 65536 {
		p.sockArr = append(p.sockArr, true)
		return uint16(len(p.sockArr)) - 1
	} else {
		for i, v := range p.sockArr {
			if !v {
				p.sockArr[i] = true
				return uint16(i)
			}
		}
	}

	panic("too many sockets")
}

func (p *Proxy) putSockIdx(idx uint16) {
	p.m.Lock()
	defer p.m.Unlock()

	if idx < uint16(len(p.sockArr)) {
		p.sockArr[idx] = false
	}
}

func (p *Proxy) Read(b *Upack) error {

	return nil
}

func (p *Proxy) Write(b *Upack) error {
	return nil
}
