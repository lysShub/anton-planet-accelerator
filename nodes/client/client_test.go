//go:build windows
// +build windows

package client_test

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"testing"
	"time"

	accelerator "github.com/lysShub/anton-planet-accelerator"
	"github.com/lysShub/anton-planet-accelerator/nodes/client"
	"github.com/lysShub/divert-go"
	"github.com/lysShub/netkit/debug"
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
	}
	os.Remove(config.PcapPath)

	proxyers := []netip.AddrPort{
		// netip.MustParseAddrPort("8.137.91.200:19986"),  // 洛杉矶
		netip.MustParseAddrPort("39.106.138.35:19986"), // 莫斯科
	}

	c, err := client.New(proxyers, config)
	require.NoError(t, err)

	c.Start()

	for {
		stats, err := c.NetworkStats(time.Second)
		if !errors.Is(err, os.ErrDeadlineExceeded) {
			err = nil
			// fmt.Println("timeout")
		}
		require.NoError(t, err)

		fmt.Println(stats.String())
		fmt.Println()

		time.Sleep(time.Second * 3)
	}
}
