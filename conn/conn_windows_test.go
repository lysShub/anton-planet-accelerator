//go:build windows
// +build windows

package conn

import (
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/lysShub/netkit/packet"
	"github.com/stretchr/testify/require"
)

func TestClient(t *testing.T) {
	dst := netip.MustParseAddrPort("8.137.91.200:19987")
	conn, err := Bind("tcp", "")
	require.NoError(t, err)
	defer conn.Close()

	for i := 0; i < 64; i++ {
		var b = packet.Make(64, 0, 8).Append([]byte("hello")...)
		err = conn.WriteToAddrPort(b, dst)
		require.NoError(t, err)

		fmt.Println("send", i)
		time.Sleep(time.Second)
	}

}
