//go:build windows
// +build windows

package client

import (
	"log/slog"
	"net/netip"
	"os"
	"slices"
	"sync/atomic"
	"syscall"
	"time"

	accelerator "github.com/lysShub/anton-planet-accelerator"
	"github.com/lysShub/anton-planet-accelerator/conn"
	"github.com/lysShub/anton-planet-accelerator/nodes"
	proto "github.com/lysShub/anton-planet-accelerator/proto"
	"github.com/lysShub/divert-go"
	"github.com/lysShub/fatun"
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/errorx"
	mapping "github.com/lysShub/netkit/mapping/process"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/netkit/pcap"
	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Client struct {
	config *Config
	laddr  netip.AddrPort

	mapping mapping.Mapping

	capture *divert.Handle
	inbound divert.Address

	conn conn.Conn
	id   atomic.Uint32
	pl   *nodes.PLStats

	proxyers        []netip.AddrPort
	routeProbeCache map[netip.Addr]int8 // 有的数据包只有上行，避免这种数据包一直被route probe
	route           *route

	msgRecver chan msg

	pcap *pcap.Pcap

	closeErr errorx.CloseErr
}

type msg struct {
	proxyer netip.AddrPort
	header  proto.Header
	data    []byte
}

func New(proxyers []netip.AddrPort, config *Config) (*Client, error) {
	var c = &Client{
		config:          config.init(),
		pl:              &nodes.PLStats{},
		proxyers:        proxyers,
		routeProbeCache: map[netip.Addr]int8{},
		route:           NewRoute(proxyers[0]),
		msgRecver:       make(chan msg, 8),
	}
	var err error

	c.conn, err = conn.Bind(nodes.Network, "")
	if err != nil {
		return nil, errors.WithStack(err)
	}
	c.laddr = c.conn.LocalAddr()

	if c.mapping, err = mapping.New(); err != nil {
		return nil, c.close(err)
	}

	var filter = "outbound and !loopback and ip and (tcp or udp)"
	c.capture, err = divert.Open(filter, divert.Network, 0, 0)
	if err != nil {
		return nil, c.close(err)
	}
	if ifi, err := defaultAdapter(); err != nil {
		return nil, err
	} else {
		c.inbound.SetOutbound(false)
		c.inbound.Network().IfIdx = uint32(ifi.Index)
	}

	if config.PcapPath != "" {
		c.pcap, err = pcap.File(config.PcapPath)
		if err != nil {
			return nil, c.close(err)
		}
	}

	return c, nil
}

func (c *Client) close(cause error) error {
	if !c.closeErr.Closed() {
		if cause != nil {
			c.config.logger.Error(cause.Error(), errorx.Trace(cause))
		} else {
			c.config.logger.Info("close")
		}
	}

	return c.closeErr.Close(func() (errs []error) {
		errs = append(errs, cause)
		if c.conn != nil {
			errs = append(errs, c.conn.Close())
		}
		if c.mapping != nil {
			errs = append(errs, c.mapping.Close())
		}
		if c.capture != nil {
			errs = append(errs, c.capture.Close())
		}
		return errs
	})
}

func (c *Client) Start() {
	c.config.logger.Info("start", slog.String("addr", c.laddr.String()), slog.Bool("debug", debug.Debug()))
	go c.captureService()
	go c.injectServic()
}

type NetworkStats struct {
	PingProxyer      time.Duration
	PingForward      time.Duration
	PackLossUplink   proto.PL
	PackLossDownlink proto.PL
}

func (c *Client) NetworkStats(timeout time.Duration) (*NetworkStats, error) {
	for _, kind := range []proto.Kind{
		proto.PingProxyer, proto.PingForward, proto.PackLossUplink,
	} {
		var pkt = packet.Make(64 + proto.HeaderSize)
		var hdr = proto.Header{
			Kind:  kind,
			Proto: syscall.IPPROTO_TCP,
			// ID:     uint8(c.id.Add(1)),
			Client: netip.AddrPortFrom(netip.IPv4Unspecified(), 0),
			Server: netip.IPv4Unspecified(),
		}
		if err := hdr.Encode(pkt); err != nil {
			return nil, c.close(err)
		}
		if err := c.conn.WriteToAddrPort(pkt, c.route.ActiveProxyer()); err != nil {
			return nil, c.close(err)
		}
	}
	start := time.Now()

	var stats NetworkStats
	var timer = time.After(timeout)
	for i := 0; i < 4; {
		for {
			select {
			case <-timer:
				return &stats, errors.WithStack(os.ErrDeadlineExceeded)
			case msg := <-c.msgRecver:
				switch msg.header.Kind {
				case proto.PingProxyer:
					stats.PingProxyer = time.Since(start)
				case proto.PingForward:
					stats.PingForward = time.Since(start)
				case proto.PackLossUplink:
					err := stats.PackLossUplink.Decode(msg.data)
					if err != nil {
						return &stats, err
					}
				default:
					c.msgRecver <- msg
					continue
				}
				i++
			}
		}
	}
	stats.PackLossDownlink = proto.PL(c.pl.PL())

	return &stats, nil
}

func (c *Client) captureService() (_ error) {
	var (
		addr divert.Address
		ip   = packet.Make(0, c.config.MaxRecvBuff)
		hdr  = proto.Header{Kind: proto.Data, Client: netip.AddrPortFrom(netip.IPv4Unspecified(), 0)}
		head = 64
	)

	for {
		n, err := c.capture.Recv(ip.Sets(head, 0xffff).Bytes(), &addr)
		if err != nil {
			return c.close(err)
		} else if n == 0 {
			continue
		}
		ip.SetData(n)

		s, err := fatun.StripIP(ip)
		if err != nil {
			return c.close(err)
		}

		pass := s.Dst.Addr().IsMulticast()
		if !pass {
			name, err := c.mapping.Name(s.Src, uint8(s.Proto))
			if err != nil {
				if errorx.Temporary(err) {
					if debug.Debug() {
						c.config.logger.Warn("not mapping", slog.String("session", s.String()))
					}
					pass = false
				} else {
					return c.close(err)
				}
			} else {
				pass = name != accelerator.Warthunder
			}
		}
		if pass {
			if _, err = c.capture.Send(ip.SetHead(head).Bytes(), &addr); err != nil {
				return c.close(err)
			}
			continue
		}

		if s.Proto == header.TCPProtocolNumber {
			fatun.UpdateTcpMssOption(ip.Bytes(), c.config.TcpMssDelta)
		}
		if c.pcap != nil {
			head := ip.Head()
			c.pcap.WriteIP(ip.SetHead(0).Bytes())
			ip.SetHead(head)
		}

		nodes.ChecksumClient(ip, s.Proto, s.Dst.Addr())
		hdr.Proto = uint8(s.Proto)
		hdr.Server = s.Dst.Addr()
		hdr.ID = uint8(c.id.Add(1))
		hdr.Encode(ip)

		next := c.route.Next(hdr.Server)
		if !next.IsValid() {
			next, err = c.routeProbe(ip)
			if err != nil {
				return c.close(err)
			} else if !next.IsValid() {
				continue
			}
		}

		if ip.Data()+20+8 > 1500 {
			println("too-big", ip.Data(), hdr.String())
		}

		if err = c.conn.WriteToAddrPort(ip, next); err != nil {
			return c.close(err)
		}
	}
}

func (c *Client) routeProbe(pkt *packet.Packet) (netip.AddrPort, error) {
	switch len(c.proxyers) {
	case 0:
		return netip.AddrPort{}, errors.New("not proxyer")
	case 1:
		return c.proxyers[0], nil
	default:
		var hdr proto.Header
		hdr.Decode(pkt)
		hdr.ID = 0
		dstPort := header.TCP(pkt.Bytes()).DestinationPort()
		hdr.Encode(pkt)

		if c.routeProbeCache[hdr.Server] > 3 {
			// println("drop probe")
			return netip.AddrPort{}, nil
		}
		c.routeProbeCache[hdr.Server]++

		for _, e := range c.proxyers {

			println("send probe", hdr.Server.String(), hdr.Proto, dstPort, e.String())

			err := c.conn.WriteToAddrPort(pkt, e)
			if err != nil {
				return netip.AddrPort{}, err
			}
		}
		return netip.AddrPort{}, nil
	}
}

type eee struct {
	server  netip.Addr
	proxyer netip.AddrPort
}

var maps = map[eee]bool{}

func (c *Client) injectServic() (_ error) {
	var (
		laddr = tcpip.AddrFrom4(c.laddr.Addr().As4())
		pkt   = packet.Make(0, c.config.MaxRecvBuff)
		hdr   = &proto.Header{}
	)

	for {
		paddr, err := c.conn.ReadFromAddrPort(pkt.Sets(64, 0xffff))
		if err != nil {
			return c.close(err)
		}

		if err := hdr.Decode(pkt); err != nil {
			c.config.logger.Error(err.Error(), errorx.Trace(err))
			continue
		}
		c.pl.ID(int(hdr.ID))

		if hdr.Kind != proto.Data {
			c.handleMsg(msg{
				proxyer: paddr, header: *hdr,
				data: slices.Clone(pkt.Bytes()),
			})
			continue
		}
		{
			e := eee{server: hdr.Server, proxyer: paddr}
			if !maps[e] {
				maps[e] = true
				dstPort := header.TCP(pkt.Bytes()).DestinationPort()
				println("recv probe", e.server.String(), dstPort, hdr.Proto, e.proxyer.String())
			}
		}
		if c.route.Add(hdr.Server, paddr) {
			// c.config.logger.Info("route probe", slog.String("server", hdr.Server.String()), slog.String("proxyer", paddr.String()))
		}
		if hdr.Proto == uint8(header.TCPProtocolNumber) {
			fatun.UpdateTcpMssOption(pkt.Bytes(), c.config.TcpMssDelta)
		}

		ip := header.IPv4(pkt.AttachN(header.IPv4MinimumSize).Bytes())
		ip.Encode(&header.IPv4Fields{
			TotalLength: uint16(pkt.Data()),
			TTL:         64,
			Protocol:    hdr.Proto,
			SrcAddr:     tcpip.AddrFrom4(hdr.Server.As4()),
			DstAddr:     laddr,
		})
		nodes.Rechecksum(ip)

		if c.pcap != nil {
			c.pcap.WriteIP(ip)
		}

		_, err = c.capture.Send(ip, &c.inbound)
		if err != nil {
			return c.close(err)
		}
	}
}

func (c *Client) handleMsg(msg msg) {
	select {
	case c.msgRecver <- msg:
	default:
		c.config.logger.Warn("msgRecver filled")
	}
}
