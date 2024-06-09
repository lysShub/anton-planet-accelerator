//go:build linux
// +build linux

package proxyer_test

import (
	"fmt"
	"net/netip"
	"testing"

	"github.com/lysShub/anton-planet-accelerator/nodes/proxyer"
	"github.com/lysShub/netkit/debug"
	"github.com/stretchr/testify/require"
)

// go test -race -v -tags "debug" -run TestXxxx

func TestXxxx(t *testing.T) {
	fmt.Println(debug.Debug())

	config := proxyer.Config{
		MaxRecvBuff: 1536,
	}

	forward := netip.MustParseAddrPort("45.150.236.6:19986")

	p, err := proxyer.New(":19986", forward, &config)
	require.NoError(t, err)

	err = p.Serve()
	require.NoError(t, err)
}

func TestXxxx2(t *testing.T) {
	fmt.Println(debug.Debug())

	config := proxyer.Config{
		MaxRecvBuff: 1536,
	}

	forward := netip.MustParseAddrPort("8.222.83.62:19986")

	p, err := proxyer.New(":19987", forward, &config)
	require.NoError(t, err)

	err = p.Serve()
	require.NoError(t, err)
}
