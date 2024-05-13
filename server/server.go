//go:build linux
// +build linux

package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/lysShub/fatcp"
	"github.com/lysShub/fatcp/links"
	"github.com/lysShub/fatcp/links/maps"
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/rawsock/helper"
	rawtcp "github.com/lysShub/rawsock/tcp"
	"github.com/lysShub/rawsock/test"
	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Server struct {
	logger  *slog.Logger
	addr    netip.AddrPort
	addrsum uint16

	listener *fatcp.Listener[*fatcp.Peer]
	links    links.LinksManager

	tcpSnder *net.IPConn
	udpSnder *net.IPConn

	srvCtx   context.Context
	cancel   context.CancelFunc
	closeErr atomic.Pointer[error]
}

type Option func(*Server)

func NewServer(addr string, opts ...Option) (*Server, error) {
	var s = &Server{
		logger: slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}

	if addr, err := resolve(addr, true); err != nil {
		return nil, err
	} else {
		s.addr = addr
		s.addrsum = checksum.Checksum(addr.Addr().AsSlice(), 0)
	}

	if rawl, err := rawtcp.Listen(s.addr); err != nil {
		return nil, err
	} else {
		s.listener, err = fatcp.NewListener[*fatcp.Peer](rawl, &fatcp.Config{})
		if err != nil {
			return nil, err
		}
	}
	s.links = maps.NewLinkManager(time.Second*30, s.listener.Addr().Addr())

	var err error
	s.tcpSnder, err = net.ListenIP("ip4:tcp", &net.IPAddr{IP: s.addr.Addr().AsSlice()})
	if err != nil {
		return nil, errors.WithStack(err)
	} else {
		err = filterLocalPorts(s.tcpSnder, s.addr.Port())
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}
	s.udpSnder, err = net.ListenIP("ip4:udp", &net.IPAddr{IP: s.addr.Addr().AsSlice()})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	s.srvCtx, s.cancel = context.WithCancel(context.Background())
	go s.recvService(s.tcpSnder)
	go s.recvService(s.udpSnder)
	return s, nil
}

func (s *Server) close(cause error) error {
	if s.closeErr.CompareAndSwap(nil, &net.ErrClosed) {
		if s.listener != nil {
			if err := s.listener.Close(); err != nil {
				cause = err
			}
		}
		if s.links != nil {
			if err := s.links.Close(); err != nil {
				cause = err
			}
		}
		if s.tcpSnder != nil {
			if err := s.tcpSnder.Close(); err != nil {
				cause = err
			}
		}
		if s.udpSnder != nil {
			if err := s.udpSnder.Close(); err != nil {
				cause = err
			}
		}
		if s.cancel != nil {
			s.cancel()
		}

		if cause != nil {
			s.closeErr.Store(&cause)
			s.logger.Error(cause.Error(), errorx.Trace(cause))
		}
		return cause
	}
	return *s.closeErr.Load()
}

func (s *Server) Serve() error {
	for {
		conn, err := s.listener.AcceptCtx(s.srvCtx)
		if err != nil {
			if errorx.Temporary(err) {
				s.logger.Warn(err.Error(), errorx.Trace(err))
				continue
			}
			return s.close(err)
		}

		fmt.Println("accept", conn.RemoteAddr())
		go s.serveConn(&links.Conn{Conn: conn})
	}
}

func (s *Server) serveConn(conn *links.Conn) (_ error) {
	var pkt = packet.Make(0, 1536)

	// todo: handle tcp
	_, err := conn.BuiltinTCP(s.srvCtx)
	if err != nil {
		s.logger.Error(err.Error(), errorx.Trace(err), slog.String("clinet", conn.RemoteAddr().String()))
		return
	}

	for {
		peer, err := conn.Recv(s.srvCtx, pkt.Sets(0, 0xffff))
		if err != nil {
			if errorx.Temporary(err) {
				s.logger.Warn(err.Error(), errorx.Trace(err))
			} else {
				s.logger.Error(err.Error(), errorx.Trace(err), slog.String("clinet", conn.RemoteAddr().String()))
				return nil
			}
		}

		var t header.Transport
		switch peer.Proto {
		case syscall.IPPROTO_TCP:
			t = header.TCP(pkt.Bytes())
		case syscall.IPPROTO_UDP:
			t = header.UDP(pkt.Bytes())
		default:
			continue
		}

		// get/alloc local port
		link := links.Uplink{
			Process: netip.AddrPortFrom(conn.LocalAddr().Addr(), t.SourcePort()),
			Proto:   peer.Proto,
			Server:  netip.AddrPortFrom(peer.Remote, t.DestinationPort()),
		}
		port, has := s.links.Uplink(link)
		if !has {
			port, err = s.links.Add(link, conn)
			if err != nil {
				s.logger.Warn(err.Error(), errorx.Trace(err))
				continue
			}
			fmt.Println("add session", link.Server.String())
		}

		// update src-port to local port
		t.SetSourcePort(port)
		sum := checksum.Combine(t.Checksum(), port)
		sum = checksum.Combine(sum, s.addrsum)
		t.SetChecksum(^sum)
		if debug.Debug() {
			psum := header.PseudoHeaderChecksum(
				tcpip.TransportProtocolNumber(peer.Proto),
				tcpip.AddrFrom4(s.listener.Addr().Addr().As4()),
				tcpip.AddrFrom4(link.Server.Addr().As4()),
				0,
			)
			test.ValidTCP(test.T(), pkt.Bytes(), psum)
		}

		switch peer.Proto {
		case syscall.IPPROTO_TCP:
			_, err = s.tcpSnder.WriteToIP(pkt.Bytes(), &net.IPAddr{IP: peer.Remote.AsSlice()})
		case syscall.IPPROTO_UDP:
			_, err = s.udpSnder.WriteToIP(pkt.Bytes(), &net.IPAddr{IP: peer.Remote.AsSlice()})
		default:
		}
		if err != nil {
			return s.close(errors.WithStack(err))
		}
	}
}

var Overhead = 36

func (s *Server) recvService(raw *net.IPConn) error {
	var ip = packet.Make(1536)

	for {
		n, err := raw.Read(ip.Sets(Overhead, 0xffff).Bytes())
		if err != nil {
			if errorx.Temporary(err) {
				s.logger.Warn(err.Error(), errorx.Trace(nil))
			}
			return s.close(err)
		} else if n < header.IPv4MinimumSize {
			continue
		}
		ip.SetData(n)
		if _, err := helper.IntegrityCheck(ip.Bytes()); err != nil {
			s.logger.Warn(err.Error(), errorx.Trace(nil))
			continue
		}

		link, err := links.StripIP(ip)
		if err != nil {
			s.logger.Warn(err.Error(), errorx.Trace(err))
			continue
		}
		down, has := s.links.Downlink(link)
		if !has {
			// don't log, has too many other process's packet
		} else {
			p := &fatcp.Peer{Proto: link.Proto, Remote: link.Server.Addr()}
			err = down.Donwlink(s.srvCtx, ip, p)
			if err != nil {
				s.logger.Warn(err.Error(), errorx.Trace(err))
			}
		}
	}
}

func resolve(addr string, local bool) (netip.AddrPort, error) {
	if taddr, err := net.ResolveTCPAddr("tcp4", addr); err != nil {
		return netip.AddrPort{}, errors.WithStack(err)
	} else {
		if taddr.Port == 0 {
			taddr.Port = 443
		}
		if len(taddr.IP) == 0 || taddr.IP.IsUnspecified() {
			if local {
				s, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: []byte{8, 8, 8, 8}, Port: 53})
				if err != nil {
					return netip.AddrPort{}, errors.WithStack(err)
				}
				defer s.Close()
				taddr.IP = s.LocalAddr().(*net.UDPAddr).IP
			} else {
				return netip.AddrPort{}, errors.Errorf("require ip or domain")
			}
		}
		return netip.MustParseAddrPort(taddr.String()), nil
	}
}
