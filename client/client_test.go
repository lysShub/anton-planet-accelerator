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
	defer divert.Release()

	// c, err := client.NewClient("172.24.131.26:8080")
	// c, err := client.NewClient("103.94.185.61:443") // 旧金山
	c, err := client.NewClient("8.222.33.114:443") // 吉隆坡
	require.NoError(t, err)

	fmt.Println("connected")

	// start := time.Now()
	for /* time.Since(start) < time.Hour */ {
		ping, err := c.Ping()
		require.NoError(t, err)
		fmt.Println("ping", ping.String())

		time.Sleep(time.Second * 3)
	}

	c.Close()
}
