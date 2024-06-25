//go:build windows
// +build windows

package client_test

import (
	"fmt"
	"net/netip"
	"os"
	"testing"
	"time"

	accelerator "github.com/lysShub/anton-planet-accelerator"
	"github.com/lysShub/anton-planet-accelerator/bvvd"
	"github.com/lysShub/anton-planet-accelerator/nodes/client"
	"github.com/lysShub/divert-go"
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/errorx"
	"github.com/stretchr/testify/require"
)

func TestXxxx(t *testing.T) {
	divert.MustLoad(divert.DLL)

	// accelerator.Warthunder = "chrome.exe"

	fmt.Println(debug.Debug(), accelerator.Warthunder)

	config := &client.Config{
		MaxRecvBuff: 2048,
		TcpMssDelta: -64,
		PcapPath:    "client.pcap",

		LocID: bvvd.Moscow.LocID(),
		Proxyers: []netip.AddrPort{
			netip.MustParseAddrPort("39.106.138.35:19986"), // 莫斯科
		},
	}
	os.Remove(config.PcapPath)

	// proxyers := []netip.AddrPort{
	// 	// netip.MustParseAddrPort("8.137.91.200:19986"),  // 洛杉矶
	// 	netip.MustParseAddrPort("39.106.138.35:19986"), // 莫斯科
	// }

	c, err := client.New(config)
	require.NoError(t, err)

	require.NoError(t, c.Start())

	for {
		stats, err := c.NetworkStats(time.Second)
		if errorx.Temporary(err) {
			fmt.Println("warn", err.Error())
			err = nil
		}
		require.NoError(t, err)

		fmt.Println(stats.String())
		fmt.Println()

		time.Sleep(time.Second * 3)
	}
}
