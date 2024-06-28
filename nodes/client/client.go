//go:build windows
// +build windows

package client

import (
	"log/slog"
	"math"
	"math/rand"
	"net/netip"
	"slices"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/lysShub/anton-planet-accelerator/bvvd"
	"github.com/lysShub/anton-planet-accelerator/conn"
	"github.com/lysShub/anton-planet-accelerator/nodes"
	"github.com/lysShub/anton-planet-accelerator/nodes/client/game"
	"github.com/lysShub/anton-planet-accelerator/nodes/client/inject"
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
	downlinkPL *nodes.PLStats

	route *route
	trunk *trunkRouteRecorder

	msgMgr *messageManager

	pcap *pcap.Pcap

	closeErr errorx.CloseErr
}

func New(config *Config) (*Client, error) {
	var c = &Client{
		config:     config.init(),
		downlinkPL: nodes.NewPLStats(bvvd.MaxID),
		route:      newRoute(config.FixRoute),
		msgMgr:     newMessageManager(),
	}
	var err error

	if c.game, err = game.New(config.Name); err != nil {
		return nil, c.close(err)
	}

	if c.inject, err = inject.New(); err != nil {
		return nil, c.close(err)
	}

	c.conn, err = conn.Bind(nodes.ProxyerNetwork, "")
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

	var start = time.Now()
	paddr, forward, err := c.routeProbe(c.config.Location, time.Second*3)
	if err != nil {
		c.config.logger.Error(err.Error(), errorx.Trace(err))
		return err
	}

	c.trunk = newTrunkRouteRecorder(time.Second*3, paddr, forward)
	c.route.Init(c, paddr, forward)
	c.game.Start()
	c.config.logger.Info("start",
		slog.String("addr", c.laddr.String()),
		slog.String("network", nodes.ProxyerNetwork),
		slog.String("mode", "fix"),
		slog.String("proxyer", paddr.String()),
		slog.Int("forward", int(forward)),
		slog.String("location", c.config.Location.String()),
		slog.String("rtt", time.Since(start).String()),
		slog.Bool("debug", debug.Debug()),
	)

	return nil
}

func (c *Client) routeProbe(loc bvvd.Location, timeout time.Duration) (paddr netip.AddrPort, forward bvvd.ForwardID, err error) {
	var msgIds []uint32

	// start := time.Now()
	var msg = nodes.Message{}
	msg.Kind = bvvd.PingForward
	msg.ForwardID = 0
	msg.Payload = loc
	for _, paddr := range c.config.Proxyers {
		id, err := c.messageRequest(paddr, msg)
		if err != nil {
			return netip.AddrPort{}, 0, err
		}
		msgIds = append(msgIds, id)
	}

	var idx int
	m, ok := c.msgMgr.PopBy(func(m nodes.Message) (pop bool) {
		idx = slices.Index(msgIds, m.MsgID)
		return idx >= 0
	}, timeout)
	if !ok {
		pxys := []string{}
		for _, e := range c.config.Proxyers {
			pxys = append(pxys, e.String())
		}
		return netip.AddrPort{}, 0, errors.Errorf("PingForward location:%s proxyers:%s timeout", loc.String(), strings.Join(pxys, ","))
	}
	// if debug.Debug() {
	// 	go func() {
	// 		var msgs = []nodes.Message{m}
	// 		var idxs = []int{idx}
	// 		var durs = []time.Duration{time.Since(start)}
	// 		for {
	// 			i := 0
	// 			m, ok := c.msgMgr.PopBy(func(m nodes.Message) (pop bool) {
	// 				i = slices.Index(msgIds, m.MsgID)
	// 				return i >= 0
	// 			}, time.Second*5)
	// 			if !ok {
	// 				break
	// 			}
	// 			msgs = append(msgs, m)
	// 			idxs = append(idxs, i)
	// 			durs = append(durs, time.Since(start))
	// 		}
	// 		var es []string
	// 		for i, e := range msgs {
	// 			es = append(es,
	// 				fmt.Sprintf(
	// 					"%s-%d %s",
	// 					c.config.Proxyers[idxs[i]].String(), int(e.ForwardID), durs[i].String(),
	// 				),
	// 			)
	// 		}
	// 		fmt.Println("replay", len(msgs))
	// 		for _, e := range es {
	// 			fmt.Println(e)
	// 		}
	// 	}()
	// }

	return c.config.Proxyers[idx], m.ForwardID, nil
}

func (c *Client) messageRequest(paddr netip.AddrPort, msg nodes.Message) (msgId uint32, err error) {
	var pkt = packet.Make(64 + bvvd.Size)

	msg.MsgID = c.msgMgr.ID()
	if err := msg.Encode(pkt); err != nil {
		return 0, err
	}

	if err := c.conn.WriteToAddrPort(pkt, paddr); err != nil {
		return 0, c.close(err)
	}
	return msg.MsgID, nil
}

func (c *Client) NetworkStats(timeout time.Duration) (stats *NetworkStates, err error) {
	var (
		start = time.Now()
		kinds = []bvvd.Kind{
			bvvd.PingProxyer, bvvd.PingForward,
			bvvd.PackLossClientUplink,
			bvvd.PackLossProxyerUplink, bvvd.PackLossProxyerDownlink,
		}
	)
	paddr, fid, _ := c.trunk.Trunk()

	var ids []uint32
	msg := nodes.Message{}
	msg.ForwardID = fid
	for _, kind := range kinds {
		msg.Kind = kind
		id, err := c.messageRequest(paddr, msg)
		if err != nil {
			return nil, c.close(err)
		}
		ids = append(ids, id)
	}

	stats = &NetworkStates{}
	for i := 0; i < len(kinds); i++ {
		msg, ok := c.msgMgr.PopBy(func(m nodes.Message) (pop bool) {
			return slices.Contains(ids, m.MsgID)
		}, time.Second*3)
		if !ok {
			err = errorx.WrapTemp(errors.New("timeout"))
			break
		}
		switch msg.Kind {
		case bvvd.PingProxyer:
			stats.PingProxyer = time.Since(start)
		case bvvd.PingForward:
			stats.PingForward = time.Since(start)
		case bvvd.PackLossClientUplink:
			stats.PackLossClientUplink = msg.Payload.(nodes.PL)
		case bvvd.PackLossProxyerUplink:
			stats.PackLossProxyerUplink = msg.Payload.(nodes.PL)
		case bvvd.PackLossProxyerDownlink:
			stats.PackLossProxyerDownlink = msg.Payload.(nodes.PL)
		default:
		}
	}

	stats.PackLossClientDownlink = nodes.PL(c.downlinkPL.PL(nodes.PLScale))
	if stats.PackLossClientDownlink == 0 {
		stats.PackLossClientDownlink = math.SmallestNonzeroFloat64
	}
	return stats, err
}

func (c *Client) RouteProbe(saddr netip.Addr) (paddr netip.AddrPort, fid bvvd.ForwardID, err error) {
	coord, err := IPCoord(saddr)
	if err != nil {
		return netip.AddrPort{}, 0, err
	}
	loc, offset := bvvd.Locations.Match(coord)

	paddr, fid, err = c.routeProbe(loc, time.Second*3)
	if err != nil {
		return netip.AddrPort{}, 0, err
	}

	c.config.logger.Info("route probe",
		slog.Float64("offset", offset),
		slog.String("server", saddr.String()),
		slog.String("location", loc.String()),
		slog.String("proxyer", paddr.String()),
		slog.Int("forward", int(fid)),
	)
	return paddr, fid, nil
}

func (c *Client) uplinkService() (_ error) {
	var (
		pkt = packet.Make(0, c.config.MaxRecvBuff)
		hdr = bvvd.Fields{
			Kind:   bvvd.Data,
			Client: netip.AddrPortFrom(netip.IPv4Unspecified(), 0),
		}
		head  = 64
		paddr netip.AddrPort
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

		nodes.ChecksumClient(pkt, info.Proto, info.Server)
		hdr.Proto = info.Proto
		hdr.Server = info.Server
		hdr.DataID = uint8(c.uplinkId.Add(1))

		if debug.Debug() && rand.Int()%100 == 99 {
			continue // PackLossClientUplink
		}

		paddr, hdr.ForwardID, err = c.route.Match(hdr.Server, info.PlayData)
		if errorx.Temporary(err) {
			if errors.Is(err, ErrRouteProbe) {
				c.config.logger.Info("start route probe", slog.String("server", hdr.Server.String()))
			}
			continue // route probing
		} else if err != nil {
			return c.close(err)
		}
		if info.PlayData {
			c.trunk.Update(paddr, hdr.ForwardID)
		}

		if err := hdr.Encode(pkt); err != nil {
			return c.close(err)
		}
		if err = c.conn.WriteToAddrPort(pkt, paddr); err != nil {
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
		_, err := c.conn.ReadFromAddrPort(pkt.Sets(64, 0xffff))
		if err != nil {
			return c.close(err)
		} else if pkt.Data() == 0 {
			continue
		}

		hdr := bvvd.Bvvd(pkt.Bytes())

		if hdr.Kind() != bvvd.Data {
			if err = c.msgMgr.Put(pkt); err != nil {
				c.config.logger.Warn(err.Error(), errorx.Trace(err))
			}
			continue
		}

		c.downlinkPL.ID(int(hdr.DataID()))

		pkt.DetachN(bvvd.Size)
		if hdr.Proto() == uint8(header.TCPProtocolNumber) {
			fatun.UpdateTcpMssOption(pkt.Bytes(), c.config.TcpMssDelta)
		}

		ip := header.IPv4(pkt.AttachN(header.IPv4MinimumSize).Bytes())
		ip.Encode(&header.IPv4Fields{
			TotalLength: uint16(pkt.Data()),
			TTL:         64,
			Protocol:    hdr.Proto(),
			SrcAddr:     tcpip.AddrFrom4(hdr.Server().As4()),
			DstAddr:     laddr,
		})
		nodes.Rechecksum(ip)

		if c.pcap != nil {
			c.pcap.WriteIP(ip)
		}

		if err := c.inject.Inject(ip); err != nil {
			return c.close(err)
		}
	}
}
