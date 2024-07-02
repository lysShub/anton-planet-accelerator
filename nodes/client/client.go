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
	"sync/atomic"
	"syscall"
	"time"

	"github.com/lysShub/anton-planet-accelerator/bvvd"
	"github.com/lysShub/anton-planet-accelerator/conn"
	"github.com/lysShub/anton-planet-accelerator/nodes"
	"github.com/lysShub/anton-planet-accelerator/nodes/client/game"
	"github.com/lysShub/anton-planet-accelerator/nodes/client/inject"
	"github.com/lysShub/anton-planet-accelerator/nodes/internal"
	"github.com/lysShub/anton-planet-accelerator/nodes/internal/checksum"
	"github.com/lysShub/anton-planet-accelerator/nodes/internal/heap"
	"github.com/lysShub/anton-planet-accelerator/nodes/internal/msg"
	"github.com/lysShub/anton-planet-accelerator/nodes/internal/stats"
	"github.com/lysShub/fatun"
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/netkit/pcap"
	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Client struct {
	config *Config
	laddr  netip.AddrPort

	game   game.Game
	inject inject.Inject

	conn       conn.Conn
	uplinkId   atomic.Uint32
	downlinkPL *stats.PLStats

	route   *route
	trunk   *trunkRouteRecorder
	msgbuff *heap.Heap[message]

	pcap *pcap.Pcap

	closeErr errorx.CloseErr
}

type message struct {
	msg   *msg.Message
	gaddr netip.AddrPort
	time  time.Time
}

func New(config *Config) (*Client, error) {
	var c = &Client{
		config:     config.init(),
		downlinkPL: stats.NewPLStats(bvvd.MaxID),
		route:      newRoute(config.FixRoute),
		msgbuff:    heap.NewHeap[message](16),
	}
	var err error

	if c.game, err = game.New(config.Name); err != nil {
		return nil, c.close(err)
	}

	if c.inject, err = inject.New(); err != nil {
		return nil, c.close(err)
	}

	c.conn, err = conn.Bind(nodes.GatewayNetwork, "")
	if err != nil {
		return nil, c.close(err)
	}
	c.laddr = c.conn.LocalAddr()

	if config.PcapPath != "" {
		c.pcap, err = pcap.File(config.PcapPath)
		if err != nil {
			return nil, c.close(err)
		}
	}

	return c, c.start()
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
		if c.game != nil {
			errs = append(errs, c.game.Close())
		}
		if c.inject != nil {
			errs = append(errs, c.inject.Close())
		}
		return errs
	})
}

func (c *Client) start() error {
	go c.uplinkService()
	go c.downlinkServic()

	// todo: use ping server (缓存各个游戏各节点server ip)
	var start = time.Now()
	var gaddr, faddr netip.AddrPort
	if err := c.boardcastPingForward(func(m message) bool {
		gaddr = m.gaddr
		faddr = m.msg.Bvvd().Forward()
		return true
	}, time.Second*3); err != nil {
		return err
	}

	c.trunk = newTrunkRouteRecorder(time.Second*3, gaddr, faddr)
	c.route.Init(c, gaddr, faddr)
	c.game.Start()
	c.config.logger.Info("start",
		slog.String("addr", c.laddr.String()),
		slog.String("network", nodes.GatewayNetwork),
		slog.String("mode", "fix"),
		slog.String("gateway", gaddr.String()),
		slog.String("forward", faddr.String()),
		slog.String("location", c.config.Location.String()),
		slog.String("rtt", time.Since(start).String()),
		slog.Bool("debug", debug.Debug()),
	)

	return nil
}

func (c *Client) RouteProbe(saddr netip.Addr) (gaddr, faddr netip.AddrPort, err error) {
	start := time.Now()

	if err := c.boardcastPingServer(saddr, func(msg message) (pop bool) {
		gaddr = msg.gaddr
		faddr = msg.msg.Bvvd().Forward()
		return true
	}, time.Second*3); err != nil {
		return netip.AddrPort{}, netip.AddrPort{}, err
	}

	c.config.logger.Info("route probe server",
		slog.String("gateway", gaddr.String()),
		slog.String("forward", faddr.String()), // todo: 屏蔽
		slog.String("server", saddr.String()),
		slog.Duration("rtt", time.Since(start)),
	)
	return gaddr, faddr, nil
}

func (c *Client) MatchForward(loc bvvd.Location) (gaddr, faddr netip.AddrPort, err error) {
	start := time.Now()
	type info struct {
		message
		loc    bvvd.Location
		off    float64
		retime time.Duration
	}

	var infos []info
	if err := c.boardcastPingForward(func(m message) bool {
		infos = append(infos, info{message: m})
		return true
	}, time.Second*3); err != nil && !errorx.Temporary(err) {
		return netip.AddrPort{}, netip.AddrPort{}, err
	} else if len(infos) == 0 {
		return netip.AddrPort{}, netip.AddrPort{}, errors.New("not replay")
	}

	for i, msg := range infos {
		coord, err := internal.IPCoord(msg.msg.Bvvd().Forward().Addr())
		if err != nil {
			return netip.AddrPort{}, netip.AddrPort{}, err
		}
		loc, off := bvvd.Locations.Match(coord)

		infos[i].loc = loc
		infos[i].off = off
		// convert coordinate offset to delay by 30km/ms (usaully is slow speed)
		infos[i].retime = infos[i].time.Sub(start) + time.Millisecond*time.Duration(off/30)
	}

	slices.SortFunc(infos, func(a, b info) int {
		if a.retime < b.retime {
			return -1
		} else if a.retime > b.retime {
			return 1
		} else {
			return 0
		}
	})
	if infos[0].loc != loc {
		c.config.logger.Warn("matched forward not same location", slog.String("infos", fmt.Sprintf("%#v", infos)))
	}

	gaddr, faddr = infos[0].gaddr, infos[0].msg.Bvvd().Forward()
	c.config.logger.Info("mathch forward",
		slog.String("gateway", gaddr.String()),
		slog.String("forward", faddr.String()), // todo: 屏蔽
		slog.String("location", loc.String()),
		slog.Duration("rtt", infos[0].time.Sub(start)),
	)
	return gaddr, faddr, nil
}

func (c *Client) NetworkStats(timeout time.Duration) (s *NetworkStates, err error) {
	var (
		start = time.Now()
		kinds = []bvvd.Kind{
			bvvd.PingGateway, bvvd.PingForward,
			bvvd.PackLossClientUplink,
			bvvd.PackLossGatewayUplink, bvvd.PackLossGatewayDownlink,
		}
	)
	gaddr, faddr, _ := c.trunk.Trunk()

	var pkt = packet.Make(msg.MinSize)

	var m = msg.Fields{MsgID: rand.Uint32()}
	m.Kind = kinds[0]
	m.Forward = faddr
	if err := m.Encode(pkt); err != nil {
		return nil, err
	}
	hdr := bvvd.Bvvd(pkt.Bytes())

	for _, kind := range kinds {
		hdr.SetKind(kind)
		if err := c.conn.WriteToAddrPort(pkt, gaddr); err != nil {
			return nil, c.close(err)
		}
	}

	s = &NetworkStates{}
	for i := 0; i < len(kinds); i++ {
		msg, ok := c.msgbuff.PopDeadline(func(msg message) (pop bool) {
			return msg.msg.MsgID() == m.MsgID
		}, time.Now().Add(time.Second*3))
		if !ok {
			err = errorx.WrapTemp(errors.New("timeout"))
			break
		}
		switch msg.msg.Kind() {
		case bvvd.PingGateway:
			s.PingGateway = time.Since(start)
		case bvvd.PingForward:
			s.PingForward = time.Since(start)
		case bvvd.PackLossClientUplink:
			if err := msg.msg.Payload(&s.PackLossClientUplink); err != nil {
				return nil, err
			}
		case bvvd.PackLossGatewayUplink:
			if err := msg.msg.Payload(&s.PackLossGatewayUplink); err != nil {
				return nil, err
			}
		case bvvd.PackLossGatewayDownlink:
			if err := msg.msg.Payload(&s.PackLossGatewayDownlink); err != nil {
				return nil, err
			}
		default:
		}
	}

	s.PackLossClientDownlink = stats.PL(c.downlinkPL.PL(nodes.PLScale))
	if s.PackLossClientDownlink == 0 {
		s.PackLossClientDownlink = math.SmallestNonzeroFloat64
	}
	return s, err
}

func (c *Client) uplinkService() (_ error) {
	var (
		pkt = packet.Make(0, c.config.MaxRecvBuff)
		hdr = bvvd.Fields{
			Kind:   bvvd.Data,
			Client: netip.AddrPortFrom(netip.IPv4Unspecified(), 0),
		}
		head  = 64
		gaddr netip.AddrPort
	)

	for {
		info, err := c.game.Capture(pkt.Sets(head, 0xffff))
		if err != nil {
			return c.close(err)
		}

		if info.Proto == syscall.IPPROTO_TCP {
			fatun.UpdateTcpMssOption(pkt.Bytes(), c.config.TcpMssDelta)
		}

		if c.pcap != nil {
			head1 := pkt.Head()
			c.pcap.WriteIP(pkt.SetHead(head).Bytes())
			pkt.SetHead(head1)
		}

		checksum.ChecksumClient(pkt, uint8(info.Proto), info.Server)
		hdr.Proto = info.Proto
		hdr.Server = info.Server
		hdr.DataID = uint8(c.uplinkId.Add(1))

		if debug.Debug() && rand.Int()%100 == 99 {
			continue // PackLossClientUplink
		}

		gaddr, hdr.Forward, err = c.route.Match(hdr.Server, info.PlayData)
		if errorx.Temporary(err) {
			if errors.Is(err, ErrRouteProbe) {
				c.config.logger.Info("start route probe", slog.String("server", hdr.Server.String()))
			}
			continue // route probing
		} else if err != nil {
			return c.close(err)
		}
		if info.PlayData {
			c.trunk.Update(gaddr, hdr.Forward)
		}

		if err := hdr.Encode(pkt); err != nil {
			return c.close(err)
		}
		if err = c.conn.WriteToAddrPort(pkt, gaddr); err != nil {
			return c.close(err)
		}
	}
}

func (c *Client) downlinkServic() (_ error) {
	var (
		laddr = tcpip.AddrFrom4(c.laddr.Addr().As4())
		pkt   = packet.Make(0, c.config.MaxRecvBuff)
	)

	for {
		gaddr, err := c.conn.ReadFromAddrPort(pkt.Sets(64, 0xffff))
		if err != nil {
			return c.close(err)
		} else if pkt.Data() == 0 {
			continue
		}

		hdr := bvvd.Bvvd(pkt.Bytes())

		if hdr.Kind() != bvvd.Data {
			if pkt.Data() >= msg.MinSize {
				c.msgbuff.MustPut(message{
					msg:   (*msg.Message)(packet.From(pkt.Bytes())),
					gaddr: gaddr, time: time.Now(),
				})
			} else {
				c.config.logger.Warn("too small", slog.String("kind", hdr.Kind().String()))
			}
			continue
		}

		c.downlinkPL.ID(int(hdr.DataID()))

		pkt.DetachN(bvvd.Size)
		if hdr.Proto() == header.TCPProtocolNumber {
			fatun.UpdateTcpMssOption(pkt.Bytes(), c.config.TcpMssDelta)
		}

		ip := header.IPv4(pkt.AttachN(header.IPv4MinimumSize).Bytes())
		ip.Encode(&header.IPv4Fields{
			TotalLength: uint16(pkt.Data()),
			TTL:         64,
			Protocol:    uint8(hdr.Proto()),
			SrcAddr:     tcpip.AddrFrom4(hdr.Server().As4()),
			DstAddr:     laddr,
		})
		checksum.Rechecksum(ip)

		if c.pcap != nil {
			c.pcap.WriteIP(ip)
		}

		if err := c.inject.Inject(ip); err != nil {
			return c.close(err)
		}
	}
}
