//go:build windows
// +build windows

package conn

import (
	"net/netip"
	"testing"

	"github.com/lysShub/netkit/packet"
	"github.com/stretchr/testify/require"
)

func TestClient(t *testing.T) {

	conn, err := Dial("tcp", netip.MustParseAddrPort("0.0.0.0:0"), netip.MustParseAddrPort("8.137.91.200:19987"))
	require.NoError(t, err)
	defer conn.Close()

	var b = packet.From([]byte("hello"))
	// var b = packet.Make(64, 0, 0)
	err = conn.Write(b)
	require.NoError(t, err)

	err = conn.Read(b)
	require.NoError(t, err)
}
