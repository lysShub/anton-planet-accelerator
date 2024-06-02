//go:build windows
// +build windows

package client

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"syscall"
	"time"

	accelerator "github.com/lysShub/anton-planet-accelerator"
	proto "github.com/lysShub/anton-planet-accelerator/proto"
	"github.com/lysShub/divert-go"
	"github.com/lysShub/fatun"
	"github.com/lysShub/netkit/errorx"
	mapping "github.com/lysShub/netkit/mapping/process"
	"github.com/lysShub/netkit/packet"
	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	stdsum "gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Client struct {
	config *Config
	id     proto.ID

	conn *net.UDPConn

	mapping mapping.Mapping

	capture *divert.Handle
	inbound divert.Address

	msgch chan msg

	closeErr errorx.CloseErr
}

type msg struct {
	header proto.Header
	data   []byte
}

func New(proxyer string, id proto.ID, config *Config) (*Client, error) {
	var c = &Client{
		config: config, id: id,
		msgch: make(chan msg, 8),
	}
	var err error

	raddr, err := net.ResolveUDPAddr("udp4", proxyer)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	c.conn, err = net.DialUDP("udp4", nil, raddr)
	if err != nil {
		return nil, errors.WithStack(err)
	}

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

	return c, nil
}

func (c *Client) close(cause error) error {
	cause = errors.WithStack(cause)
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
	go c.captureService()
	go c.injectServic()
}

func (c *Client) PingProxyer(ctx context.Context) (time.Duration, error) {
	var pkt = packet.Make(proto.HeaderSize)

	var hdr = proto.Header{
		Server: netip.IPv4Unspecified(),
		Proto:  syscall.IPPROTO_TCP,
		ID:     c.id,
		Kind:   proto.PingProxyer,
	}
	if err := hdr.Encode(pkt); err != nil {
		return 0, c.close(err)
	}
	start := time.Now()
	if _, err := c.conn.Write(pkt.Bytes()); err != nil {
		return 0, c.close(err)
	}

	if _, err := c.readMsg(ctx, proto.PingProxyer); err != nil {
		return 0, err
	}
	return time.Since(start), nil
}

func (c *Client) readMsg(ctx context.Context, kind proto.Kind) (msg, error) {
	for {
		select {
		case <-ctx.Done():
			return msg{}, errors.WithStack(ctx.Err())
		case msg := <-c.msgch:
			if msg.header.Kind == kind {
				return msg, nil
			} else {
				c.msgch <- msg
			}
		}
	}
}

func (c *Client) PacketLossProxyer(ctx context.Context) (float64, error) {
	var pkt = packet.Make(proto.HeaderSize)

	var hdr = proto.Header{
		Server: netip.IPv4Unspecified(),
		Proto:  syscall.IPPROTO_TCP,
		ID:     c.id,
		Kind:   proto.PacketLossProxyer,
	}
	if err := hdr.Encode(pkt); err != nil {
		return 0, c.close(err)
	}
	if _, err := c.conn.Write(pkt.Bytes()); err != nil {
		return 0, c.close(err)
	}

	msg, err := c.readMsg(ctx, proto.PacketLossProxyer)
	if err != nil {
		return 0, err
	}

	pl, err := strconv.ParseFloat(string(msg.data), 64)
	if err != nil {
		return 0, errors.WithStack(err)
	}
	return pl, nil
}

func (c *Client) captureService() (_ error) {
	var (
		addr divert.Address
		ip   = packet.Make(0, c.config.MaxRecvBuff)
		hdr  = proto.Header{ID: c.id, Kind: proto.Data}
	)

	for {
		n, err := c.capture.Recv(ip.SetData(0xffff).Bytes(), &addr)
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

		var t header.Transport
		if s.Proto == header.TCPProtocolNumber {
			fatun.UpdateTcpMssOption(ip.Bytes(), -c.config.TcpMssDelta)
			t = header.TCP(ip.Bytes())
		} else {
			t = header.UDP(ip.Bytes())
		}
		srcPort := t.SourcePort()
		t.SetSourcePort(0)
		t.SetChecksum(0)
		sum := header.PseudoHeaderChecksum(
			s.Proto,
			ip4zero,
			tcpip.AddrFrom4(s.Dst.Addr().As4()),
			uint16(ip.Data()),
		)
		t.SetChecksum(^checksum.Checksum(ip.Bytes(), sum))
		t.SetSourcePort(srcPort)

		hdr.Proto = uint8(s.Proto)
		hdr.Server = s.Dst.Addr()
		hdr.Encode(ip)
		if _, err = c.conn.Write(ip.Bytes()); err != nil {
			return c.close(err)
		}
	}
}

var ip4zero = tcpip.AddrFrom4([4]byte{})

func (c *Client) injectServic() (_ error) {
	var (
		laddr = netip.MustParseAddrPort(c.conn.LocalAddr().String()).Addr()
		pkt   = packet.Make(0, c.config.MaxRecvBuff)
		hdr   = &proto.Header{}
	)

	for {
		n, err := c.conn.Read(pkt.Sets(64, 0xffff).Bytes())
		if err != nil {
			return c.close(err)
		}
		pkt.SetData(n)

		if err := hdr.Decode(pkt); err != nil {
			fmt.Println(err.Error())
			continue
		} else if hdr.ID != c.id {
			fmt.Println("未知id")
			continue
		}

		if hdr.Kind != proto.Data {
			select {
			case c.msgch <- msg{header: *hdr, data: pkt.Bytes()}:
			default:
				fmt.Println("c.msgch 溢出")
			}
			continue
		}

		ip := header.IPv4(pkt.AppendN(header.IPv4MinimumSize).Bytes())
		ip.Encode(&header.IPv4Fields{
			TotalLength: uint16(pkt.Data()),
			TTL:         64,
			Protocol:    hdr.Proto,
			SrcAddr:     tcpip.AddrFrom4(hdr.Server.As4()),
			DstAddr:     tcpip.AddrFrom4(laddr.As4()),
		})
		rechecksum(ip)

		_, err = c.capture.Send(ip, &c.inbound)
		if err != nil {
			return c.close(err)
		}
	}
}

func rechecksum(ip header.IPv4) {
	ip.SetChecksum(0)
	ip.SetChecksum(^ip.CalculateChecksum())

	psum := header.PseudoHeaderChecksum(
		ip.TransportProtocol(),
		ip.SourceAddress(),
		ip.DestinationAddress(),
		ip.PayloadLength(),
	)
	switch proto := ip.TransportProtocol(); proto {
	case header.TCPProtocolNumber:
		tcp := header.TCP(ip.Payload())
		tcp.SetChecksum(0)
		tcp.SetChecksum(^stdsum.Checksum(tcp, psum))
	case header.UDPProtocolNumber:
		udp := header.UDP(ip.Payload())
		udp.SetChecksum(0)
		udp.SetChecksum(^stdsum.Checksum(udp, psum))
	default:
		panic(fmt.Sprintf("not support protocol %d", proto))
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
