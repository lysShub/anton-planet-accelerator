//go:build windows
// +build windows

package client

import (
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
	start  atomic.Bool

	game   game.Game
	inject inject.Inject

	conn       conn.Conn
	uplinkId   atomic.Uint32
	downlinkPL *nodes.PLStats

	route *route

	msgMgr *messageManager

	pcap *pcap.Pcap

	closeErr errorx.CloseErr
}

func New(config *Config) (*Client, error) {
	var c = &Client{
		config:     config.init(),
		downlinkPL: nodes.NewPLStats(bvvd.MaxID),
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

	return c, nil
}

func (c *Client) close(cause error) error {
	if cause != nil {
		c.config.logger.Error(cause.Error(), errorx.Trace(cause))
	} else {
		c.config.logger.Info("close")
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

func (c *Client) Start() error {
	if c.start.Swap(true) {
		return errors.Errorf("client started")
	}
	go c.downlinkServic()
	go c.uplinkService()

	if c.config.FixRoute {
		var start = time.Now()
		paddr, loc, err := c.routeProbe(c.config.LocID)
		if err != nil {
			c.config.logger.Error(err.Error(), errorx.Trace(err))
			return err
		}

		c.route = newFixModeRoute(paddr, loc)
		c.config.logger.Info("start",
			slog.String("addr", c.laddr.String()),
			slog.String("network", nodes.ProxyerNetwork),
			slog.String("mode", "fix"),
			slog.String("proxyer", paddr.String()),
			slog.String("location", loc.String()),
			slog.String("rtt", time.Since(start).String()),
			slog.Bool("debug", debug.Debug()),
		)
	} else {
		c.route = newAutoModeRoute(T)
		c.config.logger.Info("start",
			slog.String("addr", c.laddr.String()),
			slog.String("network", nodes.ProxyerNetwork),
			slog.String("mode", "auto"),
			slog.Bool("debug", debug.Debug()),
		)
	}

	return nil
}

func (c *Client) routeProbe(loc bvvd.LocID) (netip.AddrPort, bvvd.LocID, error) {
	var msgIds []uint32

	var msg = nodes.Message{}
	msg.Kind = bvvd.PingForward
	msg.LocID = loc
	msg.LocID.SetID(0)
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
	}, time.Second*3)
	if !ok {
		return netip.AddrPort{}, 0, errors.Errorf("ping fowrard %s timeout", loc.Loc().String())
	}
	return c.config.Proxyers[idx], m.LocID, nil
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
			bvvd.PingProxyer, bvvd.PingForward, bvvd.PackLossClientUplink,
			bvvd.PackLossProxyerUplink, bvvd.PackLossProxyerDownlink,
		}
	)

	// todo: 获取标准性的地址
	paddr, loc, err := c.route.Match(netip.IPv4Unspecified())
	if err != nil {
		return nil, err
	}

	var ids []uint32
	msg := nodes.Message{}
	msg.LocID = loc
	for _, kind := range kinds {
		msg.Kind = kind
		id, err := c.messageRequest(paddr, msg)
		if err != nil {
			return nil, err
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
			stats.PackLossClientUplink = msg.Raw.(nodes.PL)
		case bvvd.PackLossProxyerUplink:
			stats.PackLossProxyerUplink = msg.Raw.(nodes.PL)
		case bvvd.PackLossProxyerDownlink:
			stats.PackLossProxyerDownlink = msg.Raw.(nodes.PL)
		default:
		}
	}

	stats.PackLossClientDownlink = nodes.PL(c.downlinkPL.PL(nodes.PLScale))
	if stats.PackLossClientDownlink == 0 {
		stats.PackLossClientDownlink = math.SmallestNonzeroFloat64
	}
	return stats, err
}

func (c *Client) uplinkService() (_ error) {
	var (
		pkt = packet.Make(0, c.config.MaxRecvBuff)
		hdr = bvvd.Fields{
			Kind:   bvvd.Data,
			LocID:  c.config.LocID,
			Client: netip.AddrPortFrom(netip.IPv4Unspecified(), 0),
		}
		head = 64
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

		var paddr netip.AddrPort
		if c.config.FixRoute || !info.PlayData {
			paddr, hdr.LocID = c.route.Default()
		} else {
			paddr, hdr.LocID, err = c.route.Match(hdr.Server)
			if errorx.Temporary(err) {
				continue // route probing
			} else if err != nil {
				return c.close(err)
			}
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
