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

	c, err := client.NewClient("172.24.131.26:8080")
	require.NoError(t, err)

	fmt.Println("connected")

	time.Sleep(time.Hour)
	c.Close()
}
