package proxy

import (
	"fmt"
	"net"
	"runtime"
	"sync"

	"github.com/pkg/errors"

	"github.com/lysShub/warthunder/client/divert"
	"github.com/lysShub/warthunder/context"
)

// TDOD: 对于代理启动后再建立的连接, 处理逻辑是：监听到Flow Established事件后, 启动相应的Network代理。
// 这个过程中可能会漏掉数据包, 及在Network代理前, 已经发送了一些数据包。

type proxy struct {
	pid          int
	parentFilter string
	proxyConn    net.Conn

	udpTable map[sock]context.CancelFunc
	tcpTable map[sock]context.CancelFunc

	m *sync.RWMutex
}

func newProxy(ctx context.Ctx, pid int, filter string, proxyConn net.Conn) *proxy {
	p := &proxy{
		pid:          pid,
		parentFilter: filter,
		proxyConn:    proxyConn,
		udpTable:     map[sock]context.CancelFunc{},
		tcpTable:     map[sock]context.CancelFunc{},
		m:            &sync.RWMutex{},
	}

	// outbound
	go p.listenFlow(ctx)
	runtime.Gosched()
	go p.proxyOther(ctx)

	return p
}

func (p *proxy) listenFlow(ctx context.Ctx) {
	var f = fmt.Sprintf("pid=%d and out and %s", p.pid, p.parentFilter)

	h, err := divert.Open(f, divert.LAYER_FLOW, 11, divert.FLAG_READ_ONLY)
	if err != nil {
		ctx.Fatal(err)
		return
	}
	defer h.Close()

	var b []byte
	var addr divert.Address
	for {
		_, addr, err = h.Recv(b)
		if err != nil {
			ctx.Fatal(err)
			return
		}

		fh := addr.Flow()
		switch addr.Header.Event {
		case divert.EVENT_FLOW_ESTABLISHED:
			err = p.addProxy(ctx, sock{
				proto: fh.Protocol,
				laddr: fh.LocalAddr(),
				raddr: fh.RemoteAddr(),
			})
		case divert.EVENT_FLOW_DELETED:
			err = p.delProxy(sock{
				proto: fh.Protocol,
				laddr: fh.LocalAddr(),
				raddr: fh.RemoteAddr(),
			})

		default:
			ctx.Fatal(fmt.Errorf("undefined event %d", addr.Header.Event))
			return
		}
		if err != nil {
			ctx.Fatal(err)
			return
		}

		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}

func (p *proxy) proxyOther(ctx context.Ctx) {
	var f = fmt.Sprintf("pid=%d and outbound and !(%s)", p.pid, p.parentFilter)

	// TODO: 最低优先级
	h, err := divert.Open(f, divert.LAYER_NETWORK, 111, divert.FLAG_READ_ONLY)
	if err != nil {
		ctx.Fatal(err)
		return
	}
	defer h.Close()

	// TODO: 获取mtu
	var b []byte = make([]byte, 1500)
	var addr divert.Address
	for {
		_, addr, err = h.Recv(b)
		if err != nil {
			ctx.Fatal(err)
			return
		}

		switch addr.Header.Event {
		case divert.EVENT_NETWORK_PACKET:
			// TODO: 解析IP包
			var s sock
			if err = p.addProxy(ctx, s); err != nil {
				ctx.Fatal(err)
				return
			}
			var up *Upack
			if _, err = p.proxyConn.Write(up.Marshal()); err != nil {
				ctx.Fatal(errors.Wrap(err, "write to proxy conn"))
				return
			}
		default:
			ctx.Fatal(fmt.Errorf("undefined network event %d", addr.Header.Event))
			return
		}

		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}

func (p *proxy) addProxy(ctx context.Ctx, s sock) error {
	if p.proxyed(s) {
		return nil
	}
	switch s.proto {
	case divert.IPPROTO_UDP, divert.IPPROTO_TCP:
	default:
		return fmt.Errorf("unsupport transport layer protocol %d", s.proto)
	}

	var f = fmt.Sprintf("outbound and tcp and localPort=%d and localAddr=%s and remotePort=%d and remoteAddr=%s ", s.laddr.Port(), s.laddr.Addr().String(), s.raddr.Port(), s.raddr.Addr().String())

	h, err := divert.Open(f, divert.LAYER_NETWORK, 111, divert.FLAG_READ_ONLY)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	p.m.Lock()
	p.tcpTable[s] = func() {
		h.Shutdown(divert.WINDIVERT_SHUTDOWN_BOTH)
		cancel()
	}
	p.m.Unlock()
	go p.proxy(ctx, h, f)

	return nil
}

func (p *proxy) delProxy(s sock) error {
	if !p.proxyed(s) {
		return nil
	}

	p.m.Lock()
	defer p.m.Unlock()

	switch s.proto {
	case divert.IPPROTO_UDP:
		p.udpTable[s]()
		delete(p.udpTable, s)
	case divert.IPPROTO_TCP:
		p.tcpTable[s]()
		delete(p.tcpTable, s)
	default:
	}

	return nil
}

func (p *proxy) proxyed(s sock) (has bool) {
	p.m.RLock()
	defer p.m.RUnlock()

	switch s.proto {
	case divert.IPPROTO_UDP:
		_, has = p.udpTable[s]
	case divert.IPPROTO_TCP:
		_, has = p.tcpTable[s]
	default:
		has = false
	}
	return
}

// proxy proxy a socket
func (p *proxy) proxy(ctx context.Ctx, h divert.Handle, filter string) {
	defer h.Close()

	var b []byte = make([]byte, 1500)
	var addr divert.Address
	var err error
	for {
		_, addr, err = h.Recv(b)
		if err != nil {
			ctx.Fatal(err)
			return
		}

		switch addr.Header.Event {
		case divert.EVENT_NETWORK_PACKET:

			// TODO: 解析IP包
			var up *Upack
			if _, err = p.proxyConn.Write(up.Marshal()); err != nil {
				ctx.Fatal(errors.Wrap(err, "write to proxy conn"))
				return
			}
		default:
			ctx.Fatal(fmt.Errorf("undefined network event %d", addr.Header.Event))
			return
		}

		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}

func (p *proxy) Read(up *Upack) (err error) {
	return nil
}

func (p *proxy) Write(up *Upack) (err error) {
	return nil
}

func (p *proxy) Close() (err error) {
	return nil
}
