//go:build linux
// +build linux

package main

import (
	"fmt"
	"net/netip"
	"time"

	"github.com/lysShub/anton-planet-accelerator/nodes/gateway"
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/rawsock/test"
	"github.com/stretchr/testify/require"
)

func main() {
	fmt.Println(debug.Debug())

	config := gateway.Config{
		MaxRecvBuff: 2048,
	}

	var t = test.T()
	p, err := gateway.New(":19986", &config)
	require.NoError(t, err)

	go func() {
		time.Sleep(time.Second)
		require.NoError(t, p.AddForward(netip.MustParseAddrPort("45.131.69.50:19986")))  // 莫斯科
		require.NoError(t, p.AddForward(netip.MustParseAddrPort("103.94.185.61:19986"))) // 洛杉矶

		for {
			time.Sleep(time.Second * 3)

			up, down := p.Speed()
			println("up", up, "down", down)
		}
	}()

	err = p.Serve()
	require.NoError(t, err)
}
