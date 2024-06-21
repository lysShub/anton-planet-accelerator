//go:build windows
// +build windows

package client

import (
	"log/slog"
	"math/rand"
	"net/netip"
	"slices"
	"sync/atomic"
	"syscall"
	"time"

	accelerator "github.com/lysShub/anton-planet-accelerator"
	"github.com/lysShub/anton-planet-accelerator/bvvd"
	"github.com/lysShub/anton-planet-accelerator/conn"
	"github.com/lysShub/anton-planet-accelerator/nodes"
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
	header  bvvd.Fields
	data    []byte
}

func New(proxyers []netip.AddrPort, config *Config) (*Client, error) {
	var c = &Client{
		config:      config.init(),
		downlinkPL:  nodes.NewPLStats(bvvd.MaxID),
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

func (c *Client) NetworkStats(timeout time.Duration) (stats *NetworkStats, err error) {
	kinds := []bvvd.Kind{
		bvvd.PingProxyer, bvvd.PingForward, bvvd.PackLossClientUplink,
		bvvd.PackLossProxyerUplink, bvvd.PackLossProxyerDownlink,
	}

	select {
	case <-c.statsRecver:
	default:
		break
	}

	for _, kind := range kinds {
		var pkt = packet.Make(64 + bvvd.Size)
		var hdr = bvvd.Fields{
			Kind:   kind,
			Proto:  syscall.IPPROTO_TCP,
			DataID: 0,
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

	stats = (&NetworkStats{}).init()
	var timer = time.After(timeout)
	for i := 0; i < len(kinds); {
		select {
		case <-timer:
			err = context.DeadlineExceeded
			i = len(kinds) // break
		case msg := <-c.statsRecver:
			switch msg.header.Kind {
			case bvvd.PingProxyer:
				stats.PingProxyer = time.Since(start)
			case bvvd.PingForward:
				stats.PingForward = time.Since(start)
			case bvvd.PackLossClientUplink:
				err := stats.PackLossClientUplink.Decode(msg.data)
				if err != nil {
					return stats, err
				}
			case bvvd.PackLossProxyerUplink:
				err := stats.PackLossProxyerUplink.Decode(msg.data)
				if err != nil {
					return stats, err
				}
			case bvvd.PackLossProxyerDownlink:
				err := stats.PackLossProxyerDownlink.Decode(msg.data)
				if err != nil {
					return stats, err
				}
			default:
				return nil, errors.Errorf("unknown net statistics kind %s", msg.header.Kind)
			}
			i++
		}
	}

	stats.PackLossClientDownlink = nodes.PL(c.downlinkPL.PL(nodes.PLScale))
	return stats, err
}

func (c *Client) captureService() (_ error) {
	var (
		addr divert.Address
		ip   = packet.Make(0, c.config.MaxRecvBuff)
		hdr  = bvvd.Fields{Kind: bvvd.Data, Client: netip.AddrPortFrom(netip.IPv4Unspecified(), 0)}
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
		hdr.DataID = uint8(c.uplinkId.Add(1))
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
		hdr   = &bvvd.Fields{}
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

		if hdr.Kind != bvvd.Data {
			c.handleMsg(msg{
				proxyer: paddr, header: *hdr,
				data: slices.Clone(pkt.Bytes()),
			})
			continue
		}

		c.downlinkPL.ID(int(hdr.DataID))

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
