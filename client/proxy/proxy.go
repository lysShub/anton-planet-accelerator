package proxy

import (
	"anton/client/divert"
	"anton/ctx"
	"fmt"
	"net/netip"
	"sync"
)

/*
	处理逻辑:
		代理启动时，监听进程的socket事件, 便且获取进程已经创建udp conn table。socket事件需要一直监听，监听到
	对应的事件后要更新代理。获取udp conn table只在代理启动时进行，会将所有的udp conn都加入代理。

		我们通过laddr确定一个udp conn, 因为udp是无连接的, 所以不能对raddr有要求。代理同样，raddr是可选项。
*/

type Proxy struct {
	Sniff
	pid int // WarThunder process ID

	m *sync.RWMutex
}

// NewProxy creates a new proxy, blocked until process is started.
func NewProxy(ctx ctx.Ctx) (*Proxy, error) {
	// pid, err := util.GetWarThunderPid(ctx)
	// if err != nil {
	// 	return nil, err
	// }

	//
	pid := 6060

	p := &Proxy{
		pid: pid,
		m:   &sync.RWMutex{},
	}

	go p.listenSocketEvent(ctx)

	return &Proxy{}, nil
}

// listenSocketEvent listens the process's socket BIND/LISTEN event.
func (p *Proxy) listenSocketEvent(ctx ctx.Ctx) {
	var hs [2]divert.Handle

	// listen BIND/CLOSE event
	go func(pid int) {
		var f = fmt.Sprintf("udp and (event=BIND or event=CLOSE) and processId=%d", pid)

		var err error
		hs[0], err = divert.Open(f, divert.LAYER_SOCKET, 123, divert.FLAG_SNIFF|divert.FLAG_READ_ONLY)
		if err != nil {
			ctx.Exception(err)
			return
		}

		var b = []byte{}
		var addr divert.ADDRESS
		for {
			_, addr, err = hs[0].Recv(b)
			if err != nil {
				ctx.Exception(err)
				return
			}

			if addr.Header.Event == divert.EVENT_SOCKET_BIND {
				s := addr.Socket()
				laddr := netip.AddrPortFrom(s.LocalAddr(), s.LocalPort)
				raddr := netip.AddrPortFrom(s.RemoteAddr(), s.RemotePort)
				if err = p.addProxyFilterConnected(laddr, raddr); err != nil {
					ctx.Exception(err)
					return
				}
			} else {
				s := addr.Socket()
				laddr := netip.AddrPortFrom(s.LocalAddr(), s.LocalPort)
				raddr := netip.AddrPortFrom(s.RemoteAddr(), s.RemotePort)
				p.deleteProxyFilter(laddr, raddr)
			}
		}
	}(p.pid)

	// listen LISTEN event
	go func(pid int) {
		var f = fmt.Sprintf("udp and event=LISTEN and processId=%d", pid)

		var err error
		hs[1], err = divert.Open(f, divert.LAYER_SOCKET, 122, divert.FLAG_SNIFF|divert.FLAG_READ_ONLY)
		if err != nil {
			ctx.Exception(err)
			return
		}

		var b = []byte{}
		var addr divert.ADDRESS
		for {
			_, addr, err = hs[1].Recv(b)
			if err != nil {
				ctx.Exception(err)
				return
			}

			s := addr.Socket()
			laddr := netip.AddrPortFrom(s.LocalAddr(), s.LocalPort)
			if err = p.addProxyFilter(laddr); err != nil {
				ctx.Exception(err)
				return
			}
		}
	}(p.pid)

	go func() {
		select {
		case <-ctx.Done():
			if err := hs[0].Shutdown(divert.SHUTDOWN_RECV); err != nil {
				ctx.Exception(err)
			}
			if err := hs[1].Shutdown(divert.SHUTDOWN_RECV); err != nil {
				ctx.Exception(err)
			}
			return
		default:
		}
	}()
}

type Sniff struct {
	m *sync.RWMutex

	s map[int]*sniffer
}

type sniffer struct {
	connected    bool
	laddr, raddr netip.AddrPort
}

func (s *Sniff) addProxyFilterConnected(laddr, raddr netip.AddrPort) error {
	fmt.Println("add proxy:", laddr, raddr)
	return nil
}

func (s *Sniff) addProxyFilter(laddr netip.AddrPort) error {
	fmt.Println("add proxy:", laddr)
	return nil
}

func (s *Sniff) deleteProxyFilter(laddr, raddr netip.AddrPort) error {
	return nil
}

func (s *Sniff) Read(p []byte) (n int, err error) {

	return
}

func (s *Sniff) Write(p []byte) (n int, err error) {

	return
}
