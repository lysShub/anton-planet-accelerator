//go:build windows
// +build windows

package client_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/lysShub/anton-planet-accelerator/client"
	"github.com/lysShub/divert-go"
	"github.com/stretchr/testify/require"
)

func TestXxxx(t *testing.T) {
	divert.MustLoad(divert.DLL)

	// accelerator.Warthunder = "curl.exe"

	// c, err := client.New("8.137.91.200:443", &client.Config{})
	// c, err := client.New("103.94.185.61:443", &client.Config{})
	c, err := client.New("103.200.30.195:443", &client.Config{})
	require.NoError(t, err)
	defer c.Close()

	require.NoError(t, c.Run())

	for {
		dur, err := c.Ping()
		require.NoError(t, err)

		fmt.Println("ping", dur)

		time.Sleep(time.Second * 3)
	}
}
