package bvvd

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/txthinking/socks5"
)

// func ListenAndServe(addr string) error {
// 	opts := []socks5.Option{
// 		socks5.WithAuthMethods([]socks5.Authenticator{&socks5.NoAuthAuthenticator{}}),
// 		socks5.WithRule(&ipValidRule{}),
// 	}

// 	s := socks5.NewServer(opts...)

// 	return s.ListenAndServe("tcp", addr)
// }

// type ipValidRule struct {
// }

// func (r *ipValidRule) Allow(ctx context.Context, req *socks5.Request) (context.Context, bool) {
// 	fmt.Println("client", req.RemoteAddr)
// 	return ctx, true
// }

func ListenAndServe(addr string) error {
	s, err := socks5.NewClassicServer(addr, global(), "", "", 30, 30)
	if err != nil {
		return err
	}

	return s.ListenAndServe(&ipValidHandler{})
}

type ipValidHandler struct {
	handle socks5.DefaultHandle
}

func (h *ipValidHandler) TCPHandle(s *socks5.Server, conn *net.TCPConn, req *socks5.Request) error {
	fmt.Println("from", conn.RemoteAddr())

	return h.handle.TCPHandle(s, conn, req)
}

func (h *ipValidHandler) UDPHandle(s *socks5.Server, addr *net.UDPAddr, req *socks5.Datagram) error {

	fmt.Println("from", addr.String())

	return h.handle.UDPHandle(s, addr, req)
}

func global() string {
	resp, err := http.Get("http://icanhazip.com")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	return strings.TrimSpace(string(data))
}
