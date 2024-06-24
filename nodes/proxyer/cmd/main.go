//go:build linux
// +build linux

package main

import (
	"fmt"
	"net/netip"

	"github.com/lysShub/anton-planet-accelerator/bvvd"
	"github.com/lysShub/anton-planet-accelerator/nodes/proxyer"
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/rawsock/test"
	"github.com/stretchr/testify/require"
)

func main() {
	fmt.Println(debug.Debug())

	config := proxyer.Config{
		MaxRecvBuff: 2048,

		Forwards: []struct {
			Faddr netip.AddrPort
			LocID bvvd.LocID
		}{
			{
				Faddr: netip.MustParseAddrPort("45.150.236.6:19986"),
				LocID: bvvd.Moscow,
			},
		},
	}

	// forward := netip.MustParseAddrPort("45.150.236.6:19986") // 东京

	var t = test.T()
	p, err := proxyer.New(":19986", &config)
	require.NoError(t, err)

	err = p.Serve()
	require.NoError(t, err)
}
