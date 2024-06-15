package conn

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_resolveAddr(t *testing.T) {
	{
		addr, err := resolveAddr("")
		require.NoError(t, err)
		require.Equal(t, netip.AddrPortFrom(netip.IPv4Unspecified(), 0), addr)
	}

	{
		addr, err := resolveAddr(":")
		require.NoError(t, err)
		require.Equal(t, netip.AddrPortFrom(netip.IPv4Unspecified(), 0), addr)
	}

	{
		addr, err := resolveAddr(":1234")
		require.NoError(t, err)
		require.Equal(t, netip.AddrPortFrom(netip.IPv4Unspecified(), 1234), addr)
	}

	{
		addr, err := resolveAddr("1.1.1.1:")
		require.NoError(t, err)
		require.Equal(t, netip.AddrPortFrom(netip.AddrFrom4([4]byte{1, 1, 1, 1}), 0), addr)
	}

	{
		addr, err := resolveAddr("1.1.1.1:1234")
		require.NoError(t, err)
		require.Equal(t, netip.AddrPortFrom(netip.AddrFrom4([4]byte{1, 1, 1, 1}), 1234), addr)
	}

	{
		addr, err := resolveAddr("baidu.com:1234")
		require.NoError(t, err)
		require.Equal(t, uint16(1234), addr.Port())
	}
}
