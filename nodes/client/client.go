//go:build windows
// +build windows

package client

import (
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"net/netip"
	"slices"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf8"
	"unsafe"

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
	"golang.org/x/net/context"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Client struct {
	config *Config
	laddr  netip.AddrPort

	mapping mapping.Mapping

	capture *divert.Handle
	inbound divert.Address

	conn        conn.Conn
	uplinkId    atomic.Uint32
	downlinkPL  *nodes.PLStats
	statsRecver chan msg

	proxyers []netip.AddrPort

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
		config:      config.init(),
		downlinkPL:  nodes.NewPLStats(proto.MaxID),
		proxyers:    proxyers,
		statsRecver: make(chan msg, 8),
	}
	var err error

	c.conn, err = conn.Bind(nodes.ProxyerNetwork, "")
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
	PingProxyer             time.Duration
	PingForward             time.Duration
	PackLossClientUplink    proto.PL
	PackLossClientDownlink  proto.PL
	PackLossProxyerUplink   proto.PL
	PackLossProxyerDownlink proto.PL
}

func (n *NetworkStats) String() string {
	var s = &strings.Builder{}

	p2 := time.Duration(0)
	if n.PingForward > n.PingProxyer {
		p2 = n.PingForward - n.PingProxyer
	}
	var elems = []string{
		"ping", n.strdur(n.PingProxyer), n.strdur(p2),
		"pl↑", n.PackLossClientUplink.String(), n.PackLossProxyerUplink.String(),
		"pl↓", n.PackLossClientDownlink.String(), n.PackLossProxyerDownlink.String(),
	}

	const size = 6
	for i, e := range elems {
		s.WriteString(e)

		n := size - utf8.RuneCount(unsafe.Slice(unsafe.StringData(e), len(e)))
		for i := 0; i < n; i++ {
			s.WriteByte(' ')
		}

		if (i+1)%3 == 0 {
			s.WriteByte('\n')
		}
	}
	return s.String()
}

func (*NetworkStats) strdur(dur time.Duration) string {
	ss := dur.Seconds() * 1000

	s1 := int(math.Round(ss))
	s2 := int((ss - float64(s1)) * 10)
	if s2 < 0 {
		s2 = 0
	}
	return fmt.Sprintf("%d.%d", s1, s2)
}

func (c *Client) NetworkStats(timeout time.Duration) (stats *NetworkStats, err error) {
	kinds := []proto.Kind{
		proto.PingProxyer, proto.PingForward, proto.PackLossClientUplink,
		proto.PackLossProxyerUplink, proto.PackLossProxyerDownlink,
	}

	select {
	case <-c.statsRecver:
	default:
		break
	}

	for _, kind := range kinds {
		var pkt = packet.Make(64 + proto.HeaderSize)
		var hdr = proto.Header{
			Kind:   kind,
			Proto:  syscall.IPPROTO_TCP,
			ID:     0,
			Client: netip.AddrPortFrom(netip.IPv4Unspecified(), 0),
			Server: netip.IPv4Unspecified(),
		}
		if err := hdr.Encode(pkt); err != nil {
			return nil, c.close(err)
		}
		if err := c.conn.WriteToAddrPort(pkt, c.proxyers[0]); err != nil {
			return nil, c.close(err)
		}
	}
	start := time.Now()

	stats = &NetworkStats{}
	var timer = time.After(timeout)
	for i := 0; i < len(kinds); {
		select {
		case <-timer:
			err = context.DeadlineExceeded
			goto next
		case msg := <-c.statsRecver:
			switch msg.header.Kind {
			case proto.PingProxyer:
				stats.PingProxyer = time.Since(start)
			case proto.PingForward:
				stats.PingForward = time.Since(start)
			case proto.PackLossClientUplink:
				err := stats.PackLossClientUplink.Decode(msg.data)
				if err != nil {
					return stats, err
				}
			case proto.PackLossProxyerUplink:
				err := stats.PackLossProxyerUplink.Decode(msg.data)
				if err != nil {
					return stats, err
				}
			case proto.PackLossProxyerDownlink:
				err := stats.PackLossProxyerDownlink.Decode(msg.data)
				if err != nil {
					return stats, err
				}
			default:
			}
			i++
		}
	}
next:

	stats.PackLossClientDownlink = proto.PL(c.downlinkPL.PL(nodes.PLScale))
	return stats, err
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
					// if debug.Debug() {
					// 	c.config.logger.Warn("not mapping", slog.String("session", s.String()))
					// }
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
			head1 := ip.Head()
			c.pcap.WriteIP(ip.SetHead(head).Bytes())
			ip.SetHead(head1)
		}

		nodes.ChecksumClient(ip, s.Proto, s.Dst.Addr())
		hdr.Proto = uint8(s.Proto)
		hdr.Server = s.Dst.Addr()
		hdr.ID = uint8(c.uplinkId.Add(1))
		hdr.Encode(ip)

		if debug.Debug() && rand.Int()%100 == 99 {
			continue // PackLossClientUplink
		}

		if err = c.conn.WriteToAddrPort(ip, c.proxyers[0]); err != nil {
			return c.close(err)
		}
	}
}

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
		} else if pkt.Data() == 0 {
			continue
		}

		if err := hdr.Decode(pkt); err != nil {
			c.config.logger.Error(err.Error(), errorx.Trace(err))
			continue
		}

		if hdr.Kind != proto.Data {
			c.handleMsg(msg{
				proxyer: paddr, header: *hdr,
				data: slices.Clone(pkt.Bytes()),
			})
			continue
		}

		c.downlinkPL.ID(int(hdr.ID))

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
	case c.statsRecver <- msg:
	default:
		c.config.logger.Warn("msgRecver filled")
	}
}
