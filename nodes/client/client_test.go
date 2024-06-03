//go:build windows
// +build windows

package client_test

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jftuga/geodist"
	accelerator "github.com/lysShub/anton-planet-accelerator"
	"github.com/lysShub/anton-planet-accelerator/nodes/client"
	"github.com/lysShub/divert-go"
	"github.com/lysShub/netkit/debug"
	"github.com/stretchr/testify/require"
)

// go test -race -v -tags "debug" -run TestXxxx

var (
	Moscow = geodist.Coord{Lon: 37.56, Lat: 55.75}
)

func TestXxxx(t *testing.T) {
	divert.MustLoad(divert.DLL)

	accelerator.Warthunder = "curl.exe"

	fmt.Println(debug.Debug(), accelerator.Warthunder)

	config := &client.Config{
		MaxRecvBuff: 1536,
		TcpMssDelta: -64,
		PcapPath:    "client.pcap",
	}
	os.Remove(config.PcapPath)

	c, err := client.New(1, config)
	require.NoError(t, err)
	c.AddProxyer(netip.MustParseAddrPort("8.137.91.200:19986"), Moscow)

	c.Start()

	for {
		stats, err := c.NetworkStats(time.Second)
		if !errors.Is(err, os.ErrDeadlineExceeded) {
			require.NoError(t, err)
		}

		fmt.Printf("%#v\n\n", stats)

		time.Sleep(time.Second * 4)
	}
}

func TestVvvv(t *testing.T) {

	pl := time.Since(time.Unix(0, 0)).Seconds()

	str := strconv.FormatFloat(pl, 'f', 3, 64)

	fmt.Println(str)

	v, err := strconv.ParseFloat(str, 64)
	require.NoError(t, err)
	fmt.Println(v)
}
