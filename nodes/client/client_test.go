//go:build windows
// +build windows

package client_test

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	accelerator "github.com/lysShub/anton-planet-accelerator"
	"github.com/lysShub/anton-planet-accelerator/nodes/client"
	"github.com/lysShub/divert-go"
	"github.com/lysShub/netkit/debug"
	"github.com/stretchr/testify/require"
)

/*
0xff, 0xa9, 0x0, 0x50, 0xdf, 0x61, 0xac, 0xea, 0xe5, 0xb8, 0x0, 0x0, 0x80, 0x2, 0xfa, 0xf0, 0xf6, 0xc8, 0x0, 0x0, 0x2, 0x4, 0x5, 0xf4, 0x1, 0x3, 0x3, 0x8, 0x1, 0x1, 0x4, 0x2
0xff, 0xa9, 0x0, 0x50, 0xdf, 0x61, 0xac, 0xea, 0xe5, 0xb8, 0x0, 0x0, 0x80, 0x2, 0xfa, 0xf0, 0xf6, 0xc8, 0x0, 0x0, 0x2, 0x4, 0x5, 0xf4, 0x1, 0x3, 0x3, 0x8, 0x1, 0x1, 0x4, 0x2

*/
// go test -race -v -tags "debug" -run TestXxxx

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

	c, err := client.New("8.137.91.200:19986", 1, config)
	require.NoError(t, err)
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
