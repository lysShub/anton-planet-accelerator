//go:build windows
// +build windows

package client

import (
	"math/rand"
	"net/netip"
	"time"

	"github.com/lysShub/anton-planet-accelerator/bvvd"
	"github.com/lysShub/anton-planet-accelerator/nodes/internal/msg"
	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/packet"
	"github.com/pkg/errors"
)

func (c *Client) boardcastPingForward(fn func(message) bool, timeout time.Duration) (err error) {
	var pkt = packet.Make(msg.MinSize)

	var m = msg.Fields{MsgID: rand.Uint32()}
	m.Kind = bvvd.PingForward
	if err := m.Encode(pkt); err != nil {
		return err
	}

	for _, gaddr := range c.config.Gateways {
		if err := c.conn.WriteToAddrPort(pkt, gaddr); err != nil {
			return c.close(err)
		}
	}

	_, ok := c.msgbuff.PopDeadline(func(msg message) (pop bool) {
		if msg.MsgID() == m.MsgID {
			return fn(msg)
		}
		return false
	}, time.Now().Add(timeout))
	if !ok {
		return errorx.WrapTemp(errors.Errorf("timeout"))
	}
	return nil
}

func (c *Client) boardcastPingServer(saddr netip.Addr, fn func(message) bool, timeout time.Duration) (err error) {
	var pkt = packet.Make(msg.MinSize)

	var m = msg.Fields{MsgID: rand.Uint32()}
	m.Kind = bvvd.PingServer
	m.Server = saddr
	if err := m.Encode(pkt); err != nil {
		return err
	}

	for _, gaddr := range c.config.Gateways {
		if err := c.conn.WriteToAddrPort(pkt, gaddr); err != nil {
			return c.close(err)
		}
	}

	_, ok := c.msgbuff.PopDeadline(func(msg message) (pop bool) {
		if msg.MsgID() == m.MsgID {
			return fn(msg)
		}
		return false
	}, time.Now().Add(timeout))
	if !ok {
		return errorx.WrapTemp(errors.Errorf("timeout"))
	}
	return nil
}
