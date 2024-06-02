//go:build windows
// +build windows

package client_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	accelerator "github.com/lysShub/anton-planet-accelerator"
	"github.com/lysShub/anton-planet-accelerator/nodes/client"
	"github.com/lysShub/divert-go"
	"github.com/lysShub/netkit/debug"
	"github.com/stretchr/testify/require"
)

// go test -race -v -tags "debug" -run TestXxxx

func TestXxxx(t *testing.T) {
	divert.MustLoad(divert.DLL)

	fmt.Println(debug.Debug(), accelerator.Warthunder)

	config := &client.Config{
		MaxRecvBuff: 1536,
		TcpMssDelta: -64,
	}

	c, err := client.New("8.137.91.200:19986", 1, config)
	require.NoError(t, err)
	c.Start()

	for {
		dur, err := c.PingProxyer(context.Background())
		require.NoError(t, err)

		fmt.Println("ping", dur.String(), time.Now())

		time.Sleep(time.Second)
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
