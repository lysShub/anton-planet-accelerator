package pinger

import (
	"fmt"
	"log/slog"
	"math/rand"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

func Test_Pinger(t *testing.T) {
	var ch = make(chan Info, 16)
	var p, err = NewPinger(ch, slog.Default())
	require.NoError(t, err)

	var (
		google = netip.MustParseAddr("8.8.8.8")
		cf     = netip.MustParseAddr("1.1.1.1")
	)

	start := time.Now()
	require.NoError(t, p.Ping(Info{Addr: cf}))
	require.NoError(t, p.Ping(Info{Addr: google}))

	info1 := <-ch
	fmt.Println(time.Since(start))
	info2 := <-ch
	fmt.Println(time.Since(start))

	fmt.Println(info1.RTT, info2.RTT)
}

func TestXxxx(t *testing.T) {

	go func() {
		time.Sleep(time.Second)
		conn, err := icmp.ListenPacket("ip4:icmp", "192.168.43.35")
		require.NoError(t, err)

		msg, err := (&icmp.Message{
			Type: ipv4.ICMPTypeEcho,
			Code: 0,
			Body: &icmp.Echo{
				ID:   rand.Int(),
				Seq:  rand.Int(),
				Data: []byte("hello"),
			},
		}).Marshal(nil)
		require.NoError(t, err)

		_, err = conn.WriteTo(msg, &net.IPAddr{IP: net.IP{110, 242, 68, 66}})
		require.NoError(t, err)

		var b = make([]byte, 1536)
		for {
			n, raddr, err := conn.ReadFrom(b)
			require.NoError(t, err)

			fmt.Println(1, raddr.String(), string(b[:n]))
		}
	}()

	go func() {
		conn, err := icmp.ListenPacket("ip4:icmp", "192.168.43.35")
		require.NoError(t, err)

		var b = make([]byte, 1536)
		for {
			n, raddr, err := conn.ReadFrom(b)
			require.NoError(t, err)

			fmt.Println(2, raddr.String(), string(b[:n]))
		}
	}()

	time.Sleep(time.Minute)
}
