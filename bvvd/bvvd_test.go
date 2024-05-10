package bvvd_test

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	srv "github.com/things-go/go-socks5"
	cli "github.com/txthinking/socks5"
)

func TestXxx(t *testing.T) {
	go TestServer(t)
	time.Sleep(time.Second)

	TestClinet(t)
}

func TestServer(t *testing.T) {

	opts := []srv.Option{
		srv.WithAuthMethods([]srv.Authenticator{&srv.NoAuthAuthenticator{}}),
		srv.WithRule(&ipValidRule{}),
	}

	s := srv.NewServer(opts...)
	log.Fatal(s.ListenAndServe("tcp", ":8080"))

	{

	}
}

type ipValidRule struct {
}

func (r *ipValidRule) Allow(ctx context.Context, req *srv.Request) (context.Context, bool) {
	fmt.Println("clinet", req.RemoteAddr)
	return ctx, true
}

func TestClinet(t *testing.T) {
	c, err := cli.NewClient(":8080", "", "", 30, 30)
	require.NoError(t, err)

	conn, err := c.Dial("tcp", "8.8.8.8:334")

	fmt.Println(conn, err)

	// c.Request()
}
