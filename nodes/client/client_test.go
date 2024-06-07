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

	"github.com/jftuga/geodist"
	accelerator "github.com/lysShub/anton-planet-accelerator"
	"github.com/lysShub/anton-planet-accelerator/nodes/client"
	"github.com/lysShub/divert-go"
	"github.com/lysShub/netkit/debug"
	"github.com/stretchr/testify/require"
)

var (
	Moscow = geodist.Coord{Lon: 37.56, Lat: 55.75}
)

func TestXxxx(t *testing.T) {
	divert.MustLoad(divert.DLL)

	// accelerator.Warthunder = "curl.exe"

	fmt.Println(debug.Debug(), accelerator.Warthunder)

	config := &client.Config{
		MaxRecvBuff: 1536,
		TcpMssDelta: -64,
		PcapPath:    "client.pcap",
	}
	os.Remove(config.PcapPath)

	proxyers := []netip.AddrPort{
		netip.MustParseAddrPort("8.137.91.200:19986"),
	}

	c, err := client.New(proxyers, config)
	require.NoError(t, err)

	c.Start()

	for {
		stats, err := c.NetworkStats(time.Second)
		if !errors.Is(err, os.ErrDeadlineExceeded) {
			require.NoError(t, err)
		}

		fmt.Printf("Ping: %s + %s    PL: %s  %s \n",
			stats.PingProxyer.String(), stats.PingForward.String(),
			stats.PackLossUplink.String(), stats.PackLossDownlink.String(),
		)

		time.Sleep(time.Second)
	}
}
