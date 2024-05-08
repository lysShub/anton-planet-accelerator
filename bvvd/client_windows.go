//go:build windows
// +build windows

package bvvd

import (
	"encoding/hex"
	"fmt"
	"net"
	"net/netip"
	"syscall"

	"github.com/lysShub/divert-go"
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/mapping/process"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/rawsock/test"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

const warthunder = "curl.exe"

type Client struct {
	laddr, server netip.Addr

	capture *divert.Handle
	procs   process.Mapping

	conn *divert.Handle
}

func NewClient(server netip.Addr) (*Client, error) {
	var c = &Client{laddr: localAddr(), server: server}

	var err error
	if c.procs, err = process.New(); err != nil {
		return nil, c.close(err)
	}

	if c.capture, err = divert.Open(
		"outbound and !loopback and ip and (tcp or udp)", divert.Network, 0, 0,
	); err != nil {
		return nil, c.close(err)
	}

	if c.conn, err = divert.Open(
		"inbound and (tcp or udp) and remoteAddr="+c.server.String(), divert.Network, 0, 0,
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

func localAddr() netip.Addr {
	c, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: []byte{8, 8, 8, 8}, Port: 53})
	if err != nil {
		panic(err)
	}
	defer c.Close()
	return netip.MustParseAddrPort(c.LocalAddr().String()).Addr()
}

func (c *Client) close(cause error) error {
	panic(cause)
}

func (c *Client) outboundService() error {
	// return nil
	if !c.server.Is4() {
		panic("only support ipv4")
	}

	var (
		ip   = packet.Make(36, 1500)
		addr divert.Address
	)
	for {
		n, err := c.capture.Recv(ip.Sets(36, 0xffff).Bytes(), &addr)
		if err != nil {
			return c.close(err)
		} else if n == 0 {
			continue
		}
		ip.SetData(n)

		var hdr = header.IPv4(ip.SetData(n).Bytes())
		if hdr.TotalLength() != uint16(len(hdr)) {
			panic("")
		}
		var t header.Transport
		switch hdr.Protocol() {
		case syscall.IPPROTO_TCP:
			t = header.TCP(hdr.Payload())
		case syscall.IPPROTO_UDP:
			t = header.UDP(hdr.Payload())
		default:
			fmt.Println("not support proto", hdr.Protocol())
			continue
		}
		laddr := netip.AddrPortFrom(
			netip.AddrFrom4(hdr.SourceAddress().As4()), t.SourcePort(),
		)

		name, err := c.procs.Name(laddr, hdr.Protocol())
		if err != nil {
			if errorx.Temporary(err) {
				// fmt.Println(err.Error(), t.DestinationPort(), hdr.DestinationAddress())
				continue
			}
			return c.close(err)
		}
		if name == warthunder {
			fmt.Println("captured")

			test.CalcChecksum(ip.Bytes())
			if debug.Debug() {
				test.ValidIP(test.T(), ip.Bytes())
			}

			if err := clinetEncode(ip, c.server); err != nil {
				return c.close(err)
			}

		}

		_, err = c.capture.Send(ip.Bytes(), &addr)
		if err != nil {
			return c.close(err)
		}
	}
}

func (c *Client) inboundService() error {
	var (
		ip   = packet.Make(0, 1536)
		addr divert.Address
	)

	for {
		n, err := c.conn.Recv(ip.Sets(0, 0xffff).Bytes(), &addr)
		if err != nil {
			return c.close(err)
		} else if n == 0 {
			continue
		}

		hdr := header.IPv4(ip.SetData(n).Bytes())
		if hdr.TotalLength() != uint16(n) {
			fmt.Println("truncate ip：", hex.EncodeToString(ip.Bytes()))
			continue
		} else if hdr.Protocol() == 4 {
			if _, err := clientDecode(ip); err != nil {
				return c.close(err)
			}

			hdr := header.IPv4(ip.Bytes())
			if hdr.TotalLength() != uint16(len(hdr)) {
				fmt.Println("truncate inner ip：", hex.EncodeToString(ip.Bytes()))
				continue
			}

			// set dst ip address
			old := hdr.DestinationAddress().As4()
			hdr.SetDestinationAddressWithChecksumUpdate(tcpip.AddrFrom4(c.laddr.As4()))

			// update transport checksum
			var t header.Transport
			switch proto := hdr.TransportProtocol(); proto {
			case header.TCPProtocolNumber:
				t = header.TCP(hdr.Payload())
			case header.UDPProtocolNumber:
				t = header.UDP(hdr.Payload())
			default:
				fmt.Println("not support inner protocol", proto)
				continue
			}
			sum := ^checksum.Checksum(old[:], t.Checksum())
			sum = checksum.Checksum(c.laddr.AsSlice(), sum)
			t.SetChecksum(^sum)

			if debug.Debug() {
				test.ValidIP(test.T(), hdr)
			}
		}

		if _, err := c.conn.Send(ip.Bytes(), &addr); err != nil {
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

	ip.AttachN(header.IPv4MinimumSize)
	copy(ip.Bytes(), origin[:header.IPv4MinimumSize])
	ipip := header.IPv4(ip.Bytes())
	ipip.SetDestinationAddress(tcpip.AddrFrom4(dst.As4()))
	ipip.SetHeaderLength(header.IPv4MinimumSize)
	ipip.SetTotalLength(ipip.TotalLength() + header.IPv4MinimumSize)
	ipip[protocol] = IPPROTO_IPIP
	if debug.Debug() {
		require.Equal(test.T(), uint8(IPPROTO_IPIP), ipip.Protocol())
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
