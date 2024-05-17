//go:build linux
// +build linux

package server

import (
	"context"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"sync/atomic"
	"time"

	"github.com/lysShub/anton-planet-accelerator/common"
	"github.com/lysShub/anton-planet-accelerator/common/control"
	"github.com/lysShub/fatcp"
	"github.com/lysShub/fatcp/links"
	"github.com/lysShub/fatcp/links/maps"
	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/rawsock/helper"
	rawtcp "github.com/lysShub/rawsock/tcp"
	"github.com/pkg/errors"
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
		err = setHdrinclAndBpfFilterLocalPorts(s.tcpSnder, s.addr.Port())
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}
	s.udpSnder, err = net.ListenIP("ip4:udp", &net.IPAddr{IP: s.addr.Addr().AsSlice()})
	if err != nil {
		return nil, errors.WithStack(err)
	} else {
		err = setHdrinclAndBpfFilterLocalPorts(s.tcpSnder)
		if err != nil {
			return nil, errors.WithStack(err)
		}
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

		s.logger.Info("accept", slog.String("client", conn.RemoteAddr().String()))
		go s.serveConn(&links.Conn{Conn: conn})
	}
}

// links.Conn 还应该具备流控, 丢包统计, 超时等功能
func (s *Server) serveConn(conn *links.Conn) (_ error) {
	var (
		client      = conn.RemoteAddr()
		ctx, cancle = context.WithCancelCause(s.srvCtx)
	)
	defer cancle(nil)

	// handshake
	handshakeCtx, handshakeCancel := context.WithTimeout(ctx, time.Second*5)
	defer handshakeCancel()
	tcp, err := conn.BuiltinTCP(handshakeCtx)
	if err != nil {
		s.logger.Error(err.Error(), errorx.Trace(err), slog.String("clinet", client.String()))
		return
	}

	// control
	stop := context.AfterFunc(ctx, func() { tcp.SetDeadline(time.Now()) }) // todo: gonet support bind ctx
	defer stop()
	ctr := control.NewServer(tcp)
	go func() {
		err := ctr.Serve()
		cancle(err)
	}()

	var pkt = packet.Make(0, 1536)
	var t header.Transport
	for {
		peer, err := conn.Recv(ctx, pkt.Sets(0, 0xffff))
		if err != nil {
			if errorx.Temporary(err) {
				s.logger.Warn(err.Error(), errorx.Trace(err))
				continue
			} else {
				s.logger.Error(err.Error(), errorx.Trace(err), slog.String("client", client.String()))
				return nil
			}
		}

		// get/alloc local port
		up := links.Uplink{
			Process: netip.AddrPortFrom(conn.LocalAddr().Addr(), t.SourcePort()),
			Proto:   peer.Proto,
			Server:  netip.AddrPortFrom(peer.Remote, t.DestinationPort()),
		}
		localPort, has := s.links.Uplink(up)
		if !has {
			localPort, err = s.links.Add(up, conn)
			if err != nil {
				s.logger.Warn(err.Error(), errorx.Trace(err))
				continue
			}
			s.logger.Info("add session", slog.String("server", up.Server.String()), slog.String("client", client.String()))
		}

		down := links.Downlink{
			Server: up.Server,
			Proto:  up.Proto,
			Local:  netip.AddrPortFrom(s.addr.Addr(), localPort),
		}
		ip := common.ChecksumServer(pkt, down)

		switch peer.Proto {
		case header.TCPProtocolNumber:
			_, err = s.tcpSnder.Write(ip.Bytes())
		case header.UDPProtocolNumber:
			_, err = s.udpSnder.Write(ip.Bytes())
		default:
		}
		if err != nil {
			return s.close(errors.WithStack(err))
		}
	}
}

func (s *Server) recvService(raw *net.IPConn) error {
	var (
		ip       = packet.Make(1536)
		overhead = fatcp.Overhead[*fatcp.Peer]()
	)

	for {
		n, err := raw.Read(ip.Sets(overhead, 0xffff).Bytes())
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
				// todo: handle downlinker closed: delete links record
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
