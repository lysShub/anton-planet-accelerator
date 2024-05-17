//go:build windows
// +build windows

package client

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"sync/atomic"
	"time"

	"github.com/lysShub/anton-planet-accelerator/common"
	"github.com/lysShub/anton-planet-accelerator/common/control"
	"github.com/lysShub/divert-go"
	"github.com/lysShub/fatcp"
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

const warthunder = "aces.exe"

type Client struct {
	laddr    netip.Addr
	buffSize int
	logger   *slog.Logger

	config *fatcp.Config
	conn   *fatcp.Conn[*fatcp.Peer]

	capture *divert.Handle
	inbound divert.Address
	mapping process.Mapping

	pcap *pcap.Pcap
	ctr  *control.Client

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

func NewClient(server string, opts ...Option) (*Client, error) {
	var c = &Client{
		laddr:    defaultAddr(),
		buffSize: 1536,
		logger:   slog.New(slog.NewJSONHandler(os.Stdout, nil)),
		config:   &fatcp.Config{},
	}
	var err error

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	c.conn, err = fatcp.DialCtx[*fatcp.Peer](ctx, server, c.config)
	if err != nil {
		return nil, c.close(err)
	}

	tcp, err := c.conn.BuiltinTCP(ctx)
	if err != nil {
		return nil, c.close(err)
	}
	c.ctr = control.NewClient(tcp)
	go func() { c.close(c.ctr.Serve()) }()

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
		ip       = packet.Make(c.buffSize)
		addr     divert.Address
		overhead = fatcp.Overhead[*fatcp.Peer]()
	)

	for {
		n, err := c.capture.RecvCtx(c.srvCtx, ip.Sets(overhead, 0xffff).Bytes(), &addr)
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
		dst := netip.AddrFrom4(hdr.SourceAddress().As4())

		pass := dst.IsMulticast()
		if !pass {
			name, err := c.mapping.Name(netip.AddrPortFrom(dst, t.SourcePort()), hdr.Protocol())
			if err != nil {
				if errorx.Temporary(err) {
					// c.logger.Warn(err.Error(), errorx.Trace(nil), slog.Int("dstport", int(t.SourcePort())))
					pass = true
				} else {
					return c.close(err)
				}
			} else {
				pass = name != warthunder
			}
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

		pkt := common.ChecksumClient(ip)
		id := &fatcp.Peer{
			Remote: netip.AddrFrom4(hdr.DestinationAddress().As4()),
			Proto:  hdr.TransportProtocol(),
		}
		if id.Proto == header.TCPProtocolNumber {
			UpdateMSS(pkt.Bytes(), -overhead)
		}
		err = c.conn.Send(c.srvCtx, pkt, id)
		if err != nil {
			return c.close(err)
		}
	}
}
func (c *Client) downlinkServic() error {
	var pkt = packet.Make(0, c.buffSize)
	var overhead = fatcp.Overhead[*fatcp.Peer]()

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

		if peer.Proto == header.TCPProtocolNumber {
			UpdateMSS(pkt.Bytes(), -overhead)
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

func (c *Client) Ping() (time.Duration, error) { return c.ctr.Ping() }

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

func UpdateMSS(hdr header.TCP, delta int) error {
	n := int(hdr.DataOffset())
	if n > header.TCPMinimumSize && delta != 0 {
		oldSum := ^hdr.Checksum()
		for i := header.TCPMinimumSize; i < n; {
			kind := hdr[i]
			switch kind {
			case header.TCPOptionMSS:
				/* {kind} {length} {max seg size} */
				if i+4 <= n && hdr[i+1] == 4 {
					old := binary.BigEndian.Uint16(hdr[i+2:])
					new := int(old) + delta
					if new <= 0 {
						return errors.Errorf("updated mss is invalid %d", new)
					}

					if (i+2)%2 == 0 {
						binary.BigEndian.PutUint16(hdr[i+2:], uint16(new))
						sum := checksum.Combine(checksum.Combine(oldSum, ^old), uint16(new))
						hdr.SetChecksum(^sum)
					} else if i+5 <= n {
						sum := checksum.Combine(oldSum, ^checksum.Checksum(hdr[i+1:i+5], 0))

						binary.BigEndian.PutUint16(hdr[i+2:], uint16(new))

						sum = checksum.Combine(sum, checksum.Checksum(hdr[i+1:i+5], 0))
						hdr.SetChecksum(^sum)
					}
					return nil
				} else {
					return errors.Errorf("invalid tcp packet: %s", hex.EncodeToString(hdr[:n]))
				}
			case header.TCPOptionNOP:
				i += 1
			case header.TCPOptionEOL:
				return nil // not mss opt
			default:
				if i+1 < n {
					i += int(hdr[i+1])
				} else {
					return errors.Errorf("invalid tcp packet: %s", hex.EncodeToString(hdr[:n]))
				}
			}
		}
	}
	return nil
}
