//go:build windows
// +build windows

package bvvd

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/lysShub/divert-go"
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/mapping/process"
	"github.com/lysShub/netkit/route"
	"github.com/lysShub/rawsock/test"
	"github.com/pkg/errors"
	"github.com/txthinking/socks5"
	"golang.org/x/sys/windows"
	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
	"gvisor.dev/gvisor/pkg/waiter"
)

const warthunder = "aces.exe"

// const warthunder = "sokit.exe"

type Client struct {
	local  netip.Addr
	server netip.AddrPort
	mtu    int
	logger *slog.Logger

	client *socks5.Client

	capture     *divert.Handle
	inboundAddr divert.Address
	mapping     process.Mapping

	stack        *stack.Stack
	link         *channel.Endpoint
	udpInboundCh chan *stack.PacketBuffer

	srvCtx   context.Context
	cancel   context.CancelFunc
	closeErr atomic.Pointer[error]
}

func NewClient(server netip.AddrPort) (*Client, error) {
	var s = &Client{server: server, mtu: 1500}
	s.srvCtx, s.cancel = context.WithCancel(context.Background())
	s.logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

	table, err := route.GetTable()
	if err != nil {
		return nil, s.close(err)
	} else if !table[0].Addr.IsValid() {
		return nil, s.close(errors.New("no internet connection"))
	}
	s.local = table[0].Addr
	s.inboundAddr.SetOutbound(false)
	s.inboundAddr.Network().IfIdx = table[0].Interface

	s.client, err = socks5.NewClient(server.String(), "", "", 30, 30)
	if err != nil {
		return nil, s.close(err)
	}

	var filter = fmt.Sprintf("outbound and !loopback and ip and ifIdx=%d and (tcp or udp)", table[0].Interface)
	s.capture, err = divert.Open(filter, divert.Network, 0, 0)
	if err != nil {
		return nil, s.close(err)
	}
	if s.mapping, err = process.New(); err != nil {
		return nil, s.close(err)
	}

	const nicid = 1
	s.stack = stack.New(stack.Options{
		NetworkProtocols:   []stack.NetworkProtocolFactory{ipv4.NewProtocol},
		TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol, udp.NewProtocol},
		HandleLocal:        false,
	})
	s.link = channel.New(16, uint32(s.mtu), "")
	if err := s.stack.CreateNIC(nicid, s.link); err != nil {
		return nil, s.close(errors.New(err.String()))
	}
	if err := s.stack.AddProtocolAddress(nicid, tcpip.ProtocolAddress{
		Protocol:          header.IPv4ProtocolNumber,
		AddressWithPrefix: tcpip.AddrFromSlice(s.local.AsSlice()).WithPrefix(),
	}, stack.AddressProperties{}); err != nil {
		return nil, s.close(errors.New(err.String()))
	}
	s.stack.SetRouteTable([]tcpip.Route{{Destination: header.IPv4EmptySubnet, NIC: nicid}})
	s.stack.SetPromiscuousMode(nicid, true)
	s.stack.SetTransportProtocolHandler(
		tcp.ProtocolNumber, tcp.NewForwarder(s.stack, 0xffff, 4096, s.handleTCPConnect).HandlePacket,
	)
	s.stack.SetTransportProtocolHandler(
		udp.ProtocolNumber, udp.NewForwarder(s.stack, s.handleUDPConnect).HandlePacket, // udp HandlePacket is sync block
	)
	s.udpInboundCh = make(chan *stack.PacketBuffer, 256)

	go s.captrueService()
	go s.inboundServic()
	go s.ouboundServic()
	return s, nil
}

func (c *Client) close(cause error) error {
	if c.closeErr.CompareAndSwap(nil, &os.ErrClosed) {
		if c.cancel != nil {
			c.cancel()
		}
		if c.capture != nil {
			if err := c.capture.Close(); err != nil {
				cause = err
			}
		}
		if c.mapping != nil {
			if err := c.mapping.Close(); err != nil {
				cause = err
			}
		}
		if c.stack != nil {
			c.stack.Close()
		}
		if c.link != nil {
			c.link.Close()
		}

		if cause != nil {
			c.closeErr.Store(&cause)
			c.logger.Error(cause.Error(), errorx.Trace(cause))
		}
		return cause
	}
	return *c.closeErr.Load()
}

func (c *Client) captrueService() error {
	var ip = make(header.IPv4, 1536)
	var addr divert.Address
	for {
		n, err := c.capture.RecvCtx(c.srvCtx, ip[:cap(ip)], &addr)
		if err != nil {
			return c.close(err)
		} else if n == 0 {
			continue
		}
		ip = ip[:n]

		var src netip.AddrPort
		switch ip.Protocol() {
		case windows.IPPROTO_UDP, windows.IPPROTO_TCP:
			src = netip.AddrPortFrom(
				netip.AddrFrom4(ip.SourceAddress().As4()), header.UDP(ip.Payload()).SourcePort(),
			)
		default:
			return c.close(errors.Errorf("capture not support protocol %d", ip.Protocol()))
		}

		pass := false
		name, err := c.mapping.Name(src, ip.Protocol())
		if err != nil {
			if errorx.Temporary(err) {
				c.logger.Warn(err.Error(), errorx.Trace(nil))
				pass = true
			} else {
				return c.close(err)
			}
		} else {
			pass = name != warthunder
		}
		if pass {
			if _, err = c.capture.Send(ip, &addr); err != nil {
				return c.close(err)
			}
			continue
		}

		test.CalcChecksum(ip)
		var t header.UDP
		switch ip.Protocol() {
		case windows.IPPROTO_UDP, windows.IPPROTO_TCP:
			t = header.UDP(ip.Payload())
		default:
			c.close(errors.Errorf("capture not support protocol %d", ip.Protocol()))
		}
		reversal(ip, t)
		if debug.Debug() {
			test.ValidIP(test.T(), ip)
		}

		if ip.Protocol() == windows.IPPROTO_TCP {
			pkb := stack.NewPacketBuffer(stack.PacketBufferOptions{
				Payload: buffer.MakeWithData(ip),
			})
			c.link.InjectInbound(header.IPv4ProtocolNumber, pkb)
		} else {
			pkb := stack.NewPacketBuffer(stack.PacketBufferOptions{
				Payload: buffer.MakeWithData(ip),
			})

			for loop := true; loop; {
				select {
				case c.udpInboundCh <- pkb:
					loop = false
				case <-c.srvCtx.Done():
					return c.close(c.srvCtx.Err())
				default:
					old := <-c.udpInboundCh
					old.DecRef()
					c.logger.Warn("inboundCh filled", errorx.Trace(nil), slog.String("len", strconv.Itoa(len(c.udpInboundCh))))
				}
			}
		}

	}
}
func (c *Client) inboundServic() error {
	var pkb *stack.PacketBuffer
	for {
		select {
		case pkb = <-c.udpInboundCh:
		case <-c.srvCtx.Done():
			return c.close(c.srvCtx.Err())
		}

		// ip := header.IPv4(pkb.AsSlices()[0])
		// test.CalcChecksum(ip) // TX checksum offload
		// if debug.Debug() {
		// 	test.ValidIP(test.T(), ip)
		// }
		// var t header.UDP
		// switch ip.Protocol() {
		// case windows.IPPROTO_UDP, windows.IPPROTO_TCP:
		// 	t = header.UDP(ip.Payload())
		// default:
		// 	return c.close(errors.Errorf("capture not support protocol %d", ip.Protocol()))
		// }
		// reversal(ip, t)

		c.link.InjectInbound(header.IPv4ProtocolNumber, pkb)
	}
}

func (c *Client) ouboundServic() error {
	var ip = make(header.IPv4, 1536)
	for {
		pkb := c.link.ReadContext(c.srvCtx)
		if pkb == nil {
			return c.close(c.srvCtx.Err())
		}
		ip = ip[:0]
		for _, e := range pkb.AsSlices() {
			ip = append(ip, e...)
		}
		pkb.DecRef()

		switch ip.Protocol() {
		case windows.IPPROTO_UDP, windows.IPPROTO_TCP:
		default:
			return c.close(errors.Errorf("capture not support protocol %d", ip.Protocol()))
		}
		reversal(ip, header.UDP(ip.Payload()))

		if debug.Debug() {
			test.ValidIP(test.T(), ip)
		}
		_, err := c.capture.Send(ip, &c.inboundAddr)
		if err != nil {
			return c.close(err)
		}
	}
}

// reversal src/dst addr-port, only handle port, tcp/udp is the same
func reversal(ip header.IPv4, t header.UDP) {
	saddr, daddr := ip.SourceAddress(), ip.DestinationAddress()
	ip.SetSourceAddress(daddr)
	ip.SetDestinationAddress(saddr)
	sport, dport := t.SourcePort(), t.DestinationPort()
	t.SetSourcePort(dport)
	t.SetDestinationPort(sport)

	if debug.Debug() {
		test.ValidIP(test.T(), ip)
	}
}

func (c *Client) handleTCPConnect(fr *tcp.ForwarderRequest) {
	var wq waiter.Queue
	ep, err := fr.CreateEndpoint(&wq)
	if err != nil {
		fr.Complete(true)
		c.logger.Warn(err.String(), errorx.Trace(nil))
		return
	}
	// defer fr.Complete(false)

	uconn := gonet.NewTCPConn(&wq, ep)

	id := fr.ID()
	dst := net.JoinHostPort(id.RemoteAddress.String(), strconv.Itoa(int(id.RemotePort)))
	start := time.Now()
	pconn, e := c.client.Dial("tcp", dst)
	if e != nil {
		// ep.Close()
		fr.Complete(true)
		c.logger.Warn(e.Error(), errorx.Trace(nil))
		return
	}
	fmt.Println("dial udp", time.Since(start))

	go pipe(uconn, pconn, c.mtu)
	go pipe(pconn, uconn, c.mtu)
}

var ids = map[stack.TransportEndpointID]bool{}

func (c *Client) handleUDPConnect(fr *udp.ForwarderRequest) {

	var wq waiter.Queue
	ep, err := fr.CreateEndpoint(&wq)
	if err != nil {
		c.logger.Warn(err.String(), errorx.Trace(nil))
		return
	}

	uconn := gonet.NewUDPConn(c.stack, &wq, ep)

	id := fr.ID()

	dst := net.JoinHostPort(id.RemoteAddress.String(), strconv.Itoa(int(id.RemotePort)))
	str := fmt.Sprintln("udp ", net.JoinHostPort(id.LocalAddress.String(), strconv.Itoa(int(id.LocalPort))), dst)
	if ids[id] {
		fmt.Println("重复", str)
	} else {
		ids[id] = true
		fmt.Println(str)
	}

	start := time.Now()
	pconn, e := c.client.Dial("udp", dst)
	if e != nil {
		ep.Close()
		c.logger.Warn(e.Error(), errorx.Trace(nil))
		return
	}
	fmt.Println("dial udp", time.Since(start))

	go pipe(uconn, pconn, c.mtu)
	go pipe(pconn, uconn, c.mtu)
}

func pipe(src, dst net.Conn, mtu int) {
	_, _ = io.CopyBuffer(dst, src, make([]byte, mtu))
	src.Close()
	dst.Close()
}

func (c *Client) Close() error {
	return c.close(nil)
}
