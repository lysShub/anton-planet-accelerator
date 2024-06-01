//go:build windows
// +build windows

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/lysShub/anton-planet-accelerator/client"
	"github.com/lysShub/divert-go"
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/rawsock/test"
	"github.com/stretchr/testify/require"
)

// go run -tags="debug -race" .
func main() {
	divert.MustLoad(divert.DLL)

	os.Remove("client-builtin.pcap")

	fmt.Println(debug.Debug(), time.Now())

	var t = test.T()

	// accelerator.Warthunder = "chrome.exe"

	cfg := &client.Config{
		MaxRecvBuff:     1564,
		DialTimeout:     time.Second * 15,
		PcapBuiltinPath: "client-builtin.pcap",
	}

	c, err := client.New("1.1.1.1:19986", cfg)
	require.NoError(t, err)
	defer c.Close()

	require.NoError(t, c.Run())

	for {
		dur, err := c.Ping()
		require.NoError(t, err, time.Now())

		fmt.Println("ping", dur, time.Now().String())

		time.Sleep(time.Second)
	}
}
