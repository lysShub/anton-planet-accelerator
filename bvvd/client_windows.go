//go:build windows
// +build windows

package bvvd

import (
	"encoding/hex"
	"fmt"
	"net/netip"
	"syscall"

	"github.com/lysShub/divert-go"
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/mapping/process"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/netkit/route"
	"github.com/lysShub/rawsock/test"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

const warthunder = "aces.exe"

type Client struct {
	laddr, server netip.Addr

	capture     *divert.Handle
	inboundAddr divert.Address
	procs       process.Mapping

	conn         *divert.Handle
	outboundAddr divert.Address
}

func NewClient(server netip.Addr) (*Client, error) {
	var c = &Client{server: server}

	rows, err := route.GetTable()
	if err != nil {
		return nil, c.close(err)
	} else {
		c.inboundAddr.SetOutbound(false)
		c.inboundAddr.Network().IfIdx = rows[0].Interface
		c.outboundAddr.SetOutbound(true)
		c.laddr = rows[0].Addr
	}
	if c.procs, err = process.New(); err != nil {
		return nil, c.close(err)
	}

	if c.capture, err = divert.Open(
		"outbound and !loopback and ip and (tcp or udp)", divert.Network, 1, 0,
	); err != nil {
		return nil, c.close(err)
	}

	if c.conn, err = divert.Open(
		fmt.Sprintf("inbound and remoteAddr=%s and (tcp or udp)", c.server), divert.Network, 0, 0,
	); err != nil {
		return nil, c.close(err)
	}

	if c.procs, err = process.New(); err != nil {
		return nil, c.close(err)
	}

	go c.outboundService()
	go c.inboundService()
	return c, nil
}

func (c *Client) close(cause error) error {
	return cause
}

func (c *Client) outboundService() error {
	if !c.server.Is4() {
		panic("only support ipv4")
	}
	var ip = packet.Make(36, 1500)

	for {
		n, err := c.capture.Recv(ip.Sets(36, 0xffff).Bytes(), nil)
		if err != nil {
			return c.close(err)
		}

		var hdr = header.IPv4(ip.SetData(n).Bytes())
		if hdr.TotalLength() != uint16(n) {
			fmt.Println("truncate ip：", hex.EncodeToString(hdr))
			continue
		}
		var t header.Transport
		switch hdr.Protocol() {
		case syscall.IPPROTO_TCP:
			t = header.TCP(hdr.Payload())
		case syscall.IPPROTO_UDP:
			t = header.UDP(hdr.Payload())
		default:
			continue
		}
		laddr := netip.AddrPortFrom(
			netip.AddrFrom4(hdr.SourceAddress().As4()), t.SourcePort(),
		)

		name, err := c.procs.Name(laddr, hdr.Protocol())
		if err != nil {
			if errorx.Temporary(err) {
				fmt.Println(err.Error())
				continue
			}
			return c.close(err)
		}
		if name == warthunder {
			if err := clinetEncode(ip, c.server); err != nil {
				return c.close(err)
			}

			_, err = c.conn.Send(ip.Bytes(), &c.outboundAddr)
			if err != nil {
				return c.close(err)
			}
		}
	}
}

func (c *Client) inboundService() error {
	var ipip = packet.Make(0, 1536)

	for {
		n, err := c.conn.Recv(ipip.Sets(0, 0xffff).Bytes(), nil)
		if err != nil {
			return c.close(err)
		}
		if header.IPv4(ipip.SetData(n).Bytes()).TotalLength() != uint16(n) {
			fmt.Println("truncate ip：", hex.EncodeToString(ipip.Bytes()))
			continue
		}

		if _, err := clientDecode(ipip); err != nil {
			return c.close(err)
		}

		ip := header.IPv4(ipip.Bytes())
		if ip.TotalLength() != uint16(len(ip)) {
			fmt.Println("truncate inner ip：", hex.EncodeToString(ipip.Bytes()))
			continue
		}

		// set dst ip address
		old := ip.DestinationAddress().As4()
		ip.SetDestinationAddressWithChecksumUpdate(tcpip.AddrFrom4(c.laddr.As4()))

		// update transport checksum
		var t header.Transport
		switch proto := ip.TransportProtocol(); proto {
		case header.TCPProtocolNumber:
			t = header.TCP(ip.Payload())
		case header.UDPProtocolNumber:
			t = header.UDP(ip.Payload())
		default:
			fmt.Println("not support inner protocol", proto)
			continue
		}
		sum := ^checksum.Checksum(old[:], t.Checksum())
		sum = checksum.Checksum(c.laddr.AsSlice(), sum)
		t.SetChecksum(^sum)

		if debug.Debug() {
			test.ValidIP(test.T(), ip)
		}

		if _, err := c.capture.Send(ip, &c.inboundAddr); err != nil {
			return c.close(err)
		}
	}
}

func clinetEncode(ip *packet.Packet, dst netip.Addr) (err error) {

	origin := header.IPv4(ip.Bytes())
	if ver := header.IPVersion(origin); ver != 4 {
		return errors.Errorf("not support ip version %d", ver)
	}
	if debug.Debug() {
		test.ValidIP(test.T(), ip.Bytes())
	}

	// client是对的， proxyer 的 src-ip 不对

	ip.AppendN(header.IPv4MinimumSize)
	copy(ip.Bytes(), origin[:header.IPv4MinimumSize])
	ipip := header.IPv4(ip.Bytes())
	ipip.SetDestinationAddress(tcpip.AddrFrom4(dst.As4()))
	ipip.SetHeaderLength(header.IPv4MinimumSize)
	ipip.SetTotalLength(ipip.TotalLength() + header.IPv4MinimumSize)
	ipip[protocol] = IPPROTO_IPIP
	if debug.Debug() {
		require.Equal(test.T(), IPPROTO_IPIP, ipip.Protocol())
	}

	ipip.SetChecksum(^ipip.CalculateChecksum())
	return nil
}

const (
	protocol     = 9
	IPPROTO_IPIP = 4
)

func clientDecode(ipip *packet.Packet) (from netip.Addr, err error) {
	origin := header.IPv4(ipip.Bytes())
	if ver := header.IPVersion(origin); ver != 4 {
		return netip.Addr{}, errors.Errorf("not support ip version %d", ver)
	} else if origin.Protocol() != IPPROTO_IPIP {
		return netip.Addr{}, errors.Errorf("not support protocol %d", ver)
	}
	if debug.Debug() {
		require.True(test.T(), header.IPv4(ipip.Bytes()).IsChecksumValid())
	}

	from = netip.AddrFrom4(header.IPv4(ipip.Bytes()).SourceAddress().As4())
	ipip.SetHead(ipip.Head() + int(origin.HeaderLength()))
	if debug.Debug() {
		test.ValidIP(test.T(), ipip.Bytes())
	}
	return from, nil
}
