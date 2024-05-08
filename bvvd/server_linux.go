package bvvd

import (
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"sync"

	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Server struct {
	laddr netip.Addr

	client netip.Addr

	conn *net.IPConn

	recvs  map[recvkey]*net.IPConn
	recvMu sync.RWMutex
}

func NewServer(laddr netip.Addr) (*Server, error) {
	var s = &Server{laddr: laddr}

	var err error
	if s.conn, err = net.ListenIP("ip4:ipip", &net.IPAddr{IP: laddr.AsSlice()}); err != nil {
		return nil, err
	}

	go s.uplinkService()
	return s, nil
}

func (s *Server) close(cause error) error {
	return cause
}

func (s *Server) uplinkService() error {
	var ip = make(header.IPv4, 1536)
	for {
		n, addr, err := s.conn.ReadFromIP(ip)
		if err != nil {
			return s.close(errors.WithStack(err))
		}
		ip = ip[:n]

		// 校验client
		src := netip.AddrFrom4([4]byte(addr.IP.To4()))
		if !s.client.IsValid() {
			s.client = src
			fmt.Println("recved")
		} else {
			if s.client != src {
				fmt.Println("other client", src.String())
				continue
			}
		}

		server := netip.AddrFrom4(ip.DestinationAddress().As4())
		s.recvMu.RLock()
		rcv, has := s.recvs[recvkey{server, ip.Protocol()}]
		s.recvMu.RUnlock()
		if !has {
			if rcv, err = net.DialIP(
				"ip4:"+strconv.Itoa(int(ip.Protocol())),
				&net.IPAddr{IP: s.laddr.AsSlice()}, &net.IPAddr{IP: server.AsSlice()},
			); err != nil {
				panic(err)
			}
			s.recvMu.Lock()
			s.recvs[recvkey{server, ip.Protocol()}] = rcv
			s.recvMu.Unlock()
			go s.recvService(rcv)
		}

		_, err = rcv.WriteToIP(ip.Payload(), &net.IPAddr{IP: server.AsSlice()})
		if err != nil {
			panic(err)
		}
	}
}

func (s *Server) recvService(conn *net.IPConn) {
	var ip = make([]byte, 1536)

	for {
		n, err := conn.Read(ip[:cap(ip)])
		if err != nil {
			panic(err)
		}

		if _, err := s.conn.WriteToIP(ip[:n], &net.IPAddr{IP: s.client.AsSlice()}); err != nil {
			panic(err)
		}
	}
}

type recvkey struct {
	server netip.Addr
	proto  uint8
}
