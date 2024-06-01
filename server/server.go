package server

import (
	"log/slog"
	"net"
	"time"

	wrap "github.com/lysShub/anton-planet-accelerator/wrap/server"
	"github.com/lysShub/fatun"
	"github.com/lysShub/fatun/conn"
	"github.com/lysShub/fatun/conn/udp"
	links "github.com/lysShub/fatun/links/maps"
	"github.com/lysShub/netkit/errorx"
	"github.com/pkg/errors"
)

type Server struct {
	config *Config
	server *fatun.Server
}

func New(addr string, config *Config) (*Server, error) {
	var a = &Server{config: config.init()}

	laddr, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		return nil, err
	}
	if len(laddr.IP) == 0 || laddr.IP.IsUnspecified() {
		s, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: []byte{8, 8, 8, 8}, Port: 53})
		if err != nil {
			return nil, errors.WithStack(err)
		}
		defer s.Close()
		laddr.IP = s.LocalAddr().(*net.UDPAddr).IP.To4()
	}

	u, err := udp.Listen(laddr, a.config.MaxRecvBuff)
	if err != nil {
		return nil, err
	}
	rawListener, err := conn.NewListen[conn.Default](u, &conn.Config{MaxRecvBuff: a.config.MaxRecvBuff})
	if err != nil {
		return nil, err
	}
	l, err := wrap.WrapListener(rawListener, a.config.Wrap())
	if err != nil {
		return nil, err
	}

	a.server, err = fatun.NewServer[conn.Default](func(s *fatun.Server) {
		s.MaxRecvBuff = config.MaxRecvBuff
		s.Logger = a.config.logger
		s.Listener = l
		s.Links = wrap.WrapLinks(
			links.NewLinkManager(time.Second*30, rawListener.Addr().Addr()), s.Logger,
		)
	})
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (s *Server) Serve() error {
	s.config.logger.Info("start", slog.String("listen", s.server.Listener.Addr().String()))
	err := s.server.Serve()
	if err != nil {
		s.config.logger.Error(err.Error(), errorx.Trace(err))
	} else {
		s.config.logger.Info("server closed")
	}
	return err
}

func (s *Server) Close() error {
	panic("")
}
