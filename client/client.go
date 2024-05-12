//go:build windows
// +build windows

package client

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"sync/atomic"
	"time"

	"github.com/lysShub/divert-go"
	"github.com/lysShub/fatcp"
	sconn "github.com/lysShub/fatcp"
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/mapping/process"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/netkit/pcap"
	"github.com/lysShub/rawsock/test"
	"github.com/pkg/errors"
	"golang.org/x/sys/windows"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

const warthunder = "curl.exe"

type Client struct {
	laddr    netip.Addr
	buffSize int
	logger   *slog.Logger

	config *fatcp.Config
	conn   *fatcp.Conn

	capture *divert.Handle
	inbound divert.Address
	mapping process.Mapping

	pcap *pcap.Pcap

	srvCtx   context.Context
	cancel   context.CancelFunc
	closeErr atomic.Pointer[error]
}

type Option func(*Client)

func WithHandshake(handshake fatcp.Handshake) Option {
	return func(c *Client) {
		c.config.Handshake = handshake
	}
}

func WithMaxRecvBuffSize(size int) Option {
	return func(c *Client) {
		c.config.MaxRecvBuffSize = size
	}
}

func NewClient(server string, opts ...Option) (*Client, error) {
	var c = &Client{
		laddr:    defaultAddr(),
		buffSize: 1536,
		logger:   slog.New(slog.NewJSONHandler(os.Stdout, nil)),
		config:   &sconn.Config{},
	}
	var err error

	c.conn, err = fatcp.Dial(server, c.config)
	if err != nil {
		return nil, c.close(err)
	}

	var filter = "outbound and !loopback and ip and (tcp or udp)"
	c.capture, err = divert.Open(filter, divert.Network, 0, 0)
	if err != nil {
		return nil, c.close(err)
	}
	if ifidx, err := ifIdxByAddr(netip.IPv4Unspecified()); err != nil {
		return nil, err
	} else {
		c.inbound.SetOutbound(false)
		c.inbound.Network().IfIdx = ifidx
	}
	if c.mapping, err = process.New(); err != nil {
		return nil, c.close(err)
	}

	c.pcap, err = pcap.File(fmt.Sprintf("clinet-%d.pcap", time.Now().Unix()))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	c.srvCtx, c.cancel = context.WithCancel(context.Background())
	go c.uplinkService()
	go c.downlinkServic()
	return c, nil
}

func (c *Client) close(cause error) error {
	if c.closeErr.CompareAndSwap(nil, &os.ErrClosed) {
		if c.cancel != nil {
			c.cancel()
		}
		if c.conn != nil {
			if err := c.conn.Close(); err != nil {
				cause = err
			}
		}
		if c.capture != nil {
			if err := c.capture.Close(); err != nil {
				cause = err
			}
		}
		if c.mapping != nil {
			if err := c.mapping.Close(); err != nil {
				cause = err
			}
		}

		if cause != nil {
			c.closeErr.Store(&cause)
			c.logger.Error(cause.Error(), errorx.Trace(cause))
		}
		return cause
	}
	return *c.closeErr.Load()
}

func (c *Client) uplinkService() error {
	var (
		ip   = packet.Make(c.buffSize)
		addr divert.Address
	)

	for {
		n, err := c.capture.RecvCtx(c.srvCtx, ip.Sets(fatcp.Overhead, 0xffff).Bytes(), &addr)
		if err != nil {
			return c.close(err)
		} else if n == 0 {
			continue
		}

		hdr := header.IPv4(ip.SetData(n).Bytes())
		var t header.Transport
		switch hdr.Protocol() {
		case windows.IPPROTO_UDP:
			t = header.UDP(hdr.Payload())
		case windows.IPPROTO_TCP:
			t = header.TCP(hdr.Payload())
		default:
			return c.close(errors.Errorf("capture not support protocol %d", hdr.Protocol()))
		}

		pass := false
		name, err := c.mapping.Name(netip.AddrPortFrom(
			netip.AddrFrom4(hdr.SourceAddress().As4()), t.SourcePort(),
		), hdr.Protocol())
		if err != nil {
			if errorx.Temporary(err) {
				c.logger.Warn(err.Error(), errorx.Trace(nil))
				pass = true
			} else {
				return c.close(err)
			}
		} else {
			pass = name != warthunder
		}
		if pass {
			if _, err = c.capture.Send(hdr, &addr); err != nil {
				return c.close(err)
			}
			continue
		}

		if err := c.pcap.WritePacket(ip); err != nil {
			return c.close(err)
		}

		t.SetChecksum(0)
		srcPort := t.SourcePort()
		t.SetSourcePort(0)
		sum := header.PseudoHeaderChecksum(
			hdr.TransportProtocol(),
			tcpip.AddrFrom4([4]byte{}),
			hdr.DestinationAddress(),
			uint16(len(hdr.Payload())),
		)
		t.SetChecksum(checksum.Checksum(hdr.Payload(), sum))
		t.SetSourcePort(srcPort)

		pkt := ip.SetHead(ip.Head() + int(hdr.HeaderLength()))
		id := sconn.Peer{
			Remote: netip.AddrFrom4(hdr.DestinationAddress().As4()),
			Proto:  hdr.TransportProtocol(),
		}
		err = c.conn.Send(c.srvCtx, pkt, id)
		if err != nil {
			return c.close(err)
		}
	}
}
func (c *Client) downlinkServic() error {
	var pkt = packet.Make(0, c.buffSize)
	for {
		peer, err := c.conn.Recv(c.srvCtx, pkt.Sets(0, c.buffSize))
		if err != nil {
			if errorx.Temporary(err) {
				c.logger.Warn(err.Error(), errorx.Trace(err))
				continue
			} else {
				return c.close(err)
			}
		}

		ip := header.IPv4(pkt.AttachN(header.IPv4MinimumSize).Bytes())
		ip.Encode(&header.IPv4Fields{
			TotalLength: uint16(pkt.Data()),
			TTL:         64,
			Protocol:    uint8(peer.Proto),
			SrcAddr:     tcpip.AddrFrom4(peer.Remote.As4()),
			DstAddr:     tcpip.AddrFrom4(c.laddr.As4()),
		})
		test.CalcChecksum(ip)
		if debug.Debug() {
			test.ValidIP(test.T(), ip)
		}

		if err := c.pcap.WritePacket(pkt); err != nil {
			return c.close(err)
		}

		_, err = c.capture.Send(ip, &c.inbound)
		if err != nil {
			return c.close(err)
		}
	}
}

func (c *Client) Close() error {
	return c.close(nil)
}

// todo: remove it
func ifIdxByAddr(laddr netip.Addr) (uint32, error) {
	i, err := ifaceByAddr(laddr)
	if err != nil {
		return 0, err
	}
	return uint32(i.Index), nil
}

// todo: remove it
func defaultAddr() netip.Addr {
	s, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: []byte{8, 8, 8, 8}, Port: 53})
	if err != nil {
		panic(errors.WithStack(err))
	}
	defer s.Close()
	return netip.MustParseAddrPort(s.LocalAddr().String()).Addr()
}

// todo: remove it
func ifaceByAddr(laddr netip.Addr) (*net.Interface, error) {
	if laddr.IsUnspecified() {
		laddr = defaultAddr()
	}

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
				if netip.AddrFrom4([4]byte(e.IP.To4())) == laddr {
					return &i, nil
				}
			}
		}
	}

	return nil, errors.Errorf("not found adapter %s mtu", laddr.String())
}
