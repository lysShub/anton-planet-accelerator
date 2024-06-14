//go:build linux
// +build linux

package conn

import (
	"fmt"
	"net/netip"
	"testing"

	"github.com/lysShub/netkit/packet"
	"github.com/stretchr/testify/require"
)

func TestServer(t *testing.T) {
	conn, err := Listen("tcp", netip.MustParseAddrPort("0.0.0.0:19987"))
	require.NoError(t, err)
	defer conn.Close()

	var b = packet.Make(1536)
	for {
		raddr, err := conn.ReadFromAddrPort(b.Sets(0, 0xfff))
		require.NoError(t, err)

		fmt.Println("收到", raddr.String())

		if b.Data() == 0 {
			continue
		}
		err = conn.WriteToAddrPort(b, raddr)
		require.NoError(t, err)
	}
}

func TestClient(t *testing.T) {
	conn, err := Dial("tcp", netip.MustParseAddrPort("0.0.0.0:0"), netip.MustParseAddrPort("8.137.91.200:19987"))
	require.NoError(t, err)
	defer conn.Close()

	var b = packet.From([]byte("hello"))
	err = conn.Write(b)
	require.NoError(t, err)

	err = conn.Read(b)
	require.NoError(t, err)
}
