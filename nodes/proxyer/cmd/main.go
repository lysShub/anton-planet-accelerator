//go:build linux
// +build linux

package main

import (
	"fmt"
	"net/netip"

	"github.com/lysShub/anton-planet-accelerator/nodes/proxyer"
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/rawsock/test"
	"github.com/stretchr/testify/require"
)

func main() {
	fmt.Println(debug.Debug())

	config := proxyer.Config{
		MaxRecvBuff: 1536,
	}

	forward := netip.MustParseAddrPort("45.150.236.6:19986")

	var t = test.T()
	p, err := proxyer.New(":19986", forward, &config)
	require.NoError(t, err)

	err = p.Serve()
	require.NoError(t, err)
}
