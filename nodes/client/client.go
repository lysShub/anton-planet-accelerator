//go:build windows
// +build windows

package client

import (
	"log/slog"
	"net"
	"net/netip"
	"os"
	"slices"
	"syscall"
	"time"

	"github.com/jftuga/geodist"
	accelerator "github.com/lysShub/anton-planet-accelerator"
	"github.com/lysShub/anton-planet-accelerator/nodes"
	proto "github.com/lysShub/anton-planet-accelerator/proto"
	"github.com/lysShub/divert-go"
	"github.com/lysShub/fatun"
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/errorx"
	mapping "github.com/lysShub/netkit/mapping/process"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/netkit/pcap"
	"github.com/lysShub/rawsock/test"
	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Client struct {
	config *Config
	laddr  netip.AddrPort

	conn *net.UDPConn

	mapping mapping.Mapping

	capture *divert.Handle
	inbound divert.Address

	msgRecver chan msg

	pcap *pcap.Pcap

	route *Route

	closeErr errorx.CloseErr
}

type msg struct {
	header proto.Header
	data   []byte
}

func New(config *Config) (*Client, error) {
	var c = &Client{
		config:    config.init(),
		msgRecver: make(chan msg, 8),
	}
	var err error

	c.conn, err = net.ListenUDP("udp4", nil)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	c.laddr = netip.AddrPortFrom(test.LocIP(), uint16(c.conn.LocalAddr().(*net.UDPAddr).Port))

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

	c.route = NewRoute()
	return c, nil
}

func (c *Client) close(cause error) error {
	cause = errors.WithStack(cause)
	if cause != nil {
		c.config.logger.Error(cause.Error(), errorx.Trace(cause))
	} else {
		c.config.logger.Info("close")
	}
	return c.closeErr.Close(func() (errs []error) {
		errs = append(errs, cause)
		if c.conn != nil {
			errs = append(errs, c.capture.Close())
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

// AddProxyer add a proxyer
func (c *Client) AddProxyer(proxyer netip.AddrPort, proxyLocation geodist.Coord) {
	c.route.AddProxyer(proxyer, proxyLocation)
}

type NetworkStats struct {
	PingProxyer       time.Duration
	PingForward       time.Duration
	PacketLossProxyer proto.PL
	PacketLossForward proto.PL
}

func (c *Client) NetworkStats(timeout time.Duration) (*NetworkStats, error) {
	next, err := c.route.CurrentNext()
	if err != nil {
		return nil, err
	}
	for _, kind := range []proto.Kind{proto.PingProxyer, proto.PacketLossProxyer, proto.PingForward, proto.PacketLossForward} {
		var pkt = packet.Make(proto.HeaderSize)
		var hdr = proto.Header{
			Kind:   kind,
			Proto:  syscall.IPPROTO_TCP,
			Client: netip.IPv4Unspecified(),
			Server: netip.IPv4Unspecified(),
		}
		if err := hdr.Encode(pkt); err != nil {
			return nil, c.close(err)
		}
		if _, err := c.conn.WriteToUDPAddrPort(pkt.Bytes(), next); err != nil {
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
				case proto.PacketLossProxyer:
					err := stats.PacketLossProxyer.Decode(msg.data)
					if err != nil {
						return &stats, err
					}
				case proto.PacketLossForward:
					err := stats.PacketLossForward.Decode(msg.data)
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
	return &stats, nil
}

func (c *Client) captureService() (_ error) {
	var (
		addr divert.Address
		ip   = packet.Make(0, c.config.MaxRecvBuff)
		hdr  = proto.Header{Kind: proto.Data, Client: netip.IPv4Unspecified()}
	)

	for {
		n, err := c.capture.Recv(ip.Sets(0, 0xffff).Bytes(), &addr)
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
					pass = true // todo: logger
				} else {
					return c.close(err)
				}
			} else {
				pass = name != accelerator.Warthunder
			}
		}
		if pass {
			if _, err = c.capture.Send(ip.SetHead(0).Bytes(), &addr); err != nil {
				return c.close(err)
			}
			continue
		}

		if s.Proto == header.TCPProtocolNumber {
			fatun.UpdateTcpMssOption(ip.Bytes(), -c.config.TcpMssDelta)
		}
		if c.pcap != nil {
			head := ip.Head()
			c.pcap.WriteIP(ip.SetHead(0).Bytes())
			ip.SetHead(head)
		}

		next, err := c.route.Next(s.Dst.Addr())
		if err != nil {
			c.config.logger.Error(err.Error(), errorx.Trace(err))
			continue
		}

		nodes.ChecksumClient(ip, s.Proto, s.Dst.Addr())
		hdr.Proto = uint8(s.Proto)
		hdr.Server = s.Dst.Addr()
		hdr.Encode(ip)
		if _, err = c.conn.WriteToUDPAddrPort(ip.Bytes(), next); err != nil {
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
		n, _, err := c.conn.ReadFromUDPAddrPort(pkt.Sets(64, 0xffff).Bytes())
		if err != nil {
			return c.close(err)
		}
		pkt.SetData(n)

		if err := hdr.Decode(pkt); err != nil {
			c.config.logger.Error(err.Error(), errorx.Trace(err))
			continue
		}

		if hdr.Kind != proto.Data {
			select {
			case c.msgRecver <- msg{header: *hdr, data: slices.Clone(pkt.Bytes())}:
			default:
				c.config.logger.Warn("msgRecver filled")
			}
			continue
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

// todo: optimzie
func defaultAdapter() (*net.Interface, error) {
	conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.IP{8, 8, 8, 8}, Port: 53})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer conn.Close()
	laddr := netip.MustParseAddrPort(conn.LocalAddr().String()).Addr().As4()

	ifs, err := net.Interfaces()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	for _, i := range ifs {
		if i.Flags&net.FlagRunning == 0 {
			continue
		}

		addrs, err := i.Addrs()
		if err != nil {
			return nil, errors.WithStack(err)
		}
		for _, addr := range addrs {
			if e, ok := addr.(*net.IPNet); ok && e.IP.To4() != nil {
				if [4]byte(e.IP.To4()) == laddr {
					return &i, nil
				}
			}
		}
	}

	return nil, errors.Errorf("not found default adapter")
}
