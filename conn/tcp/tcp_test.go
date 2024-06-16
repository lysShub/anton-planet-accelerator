package tcp_test

import (
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/lysShub/anton-planet-accelerator/conn/tcp"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/rawsock/test"
	"github.com/stretchr/testify/require"
)

func TestXxxx(t *testing.T) {
	var (
		caddr = netip.AddrPortFrom(test.LocIP(), 19986)
		saddr = netip.AddrPortFrom(test.LocIP(), 8080)
	)

	go func() {
		time.Sleep(time.Second)
		conn, err := tcp.Bind(caddr)
		require.NoError(t, err)
		defer conn.Close()

		var b = packet.Make(64, 0, 8).Append([]byte("hello")...)

		err = conn.WriteToAddrPort(b, saddr)
		require.NoError(t, err)

		fmt.Println("send")
	}()
	go func() {
		conn, err := tcp.Bind(saddr)
		require.NoError(t, err)
		defer conn.Close()

		var b = packet.Make(64)

		src, err := conn.ReadFromAddrPort(b.Sets(0, 0xffff))
		require.NoError(t, err)

		fmt.Println("recv", src.String())
	}()

	time.Sleep(time.Second * 2)
}
