//go:build linux
// +build linux

package main

import (
	"fmt"
	"net/netip"
	"time"

	"github.com/lysShub/anton-planet-accelerator/nodes/proxyer"
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/rawsock/test"
	"github.com/stretchr/testify/require"
)

func main() {
	fmt.Println(debug.Debug())

	config := proxyer.Config{
		MaxRecvBuff: 2048,
	}

	// faddr := netip.MustParseAddrPort("45.150.236.6:19986") // 东京
	faddr := netip.MustParseAddrPort("45.131.69.50:19986") // 莫斯科

	var t = test.T()
	p, err := proxyer.New(":19986", &config)
	require.NoError(t, err)

	go func() {
		time.Sleep(time.Second)
		require.NoError(t, p.AddForward(faddr))
	}()

	err = p.Serve()
	require.NoError(t, err)
}
