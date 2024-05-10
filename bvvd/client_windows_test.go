//go:build windows
// +build windows

package bvvd

import (
	"net/netip"
	"testing"
	"time"

	"github.com/lysShub/divert-go"
	"github.com/stretchr/testify/require"
)

func TestXxxxx(t *testing.T) {
	divert.MustLoad(divert.DLL)
	defer divert.Release()

	c, err := NewClient(netip.MustParseAddrPort("103.94.185.61:8080"))
	require.NoError(t, err)

	time.Sleep(time.Hour)
	c.Close()
}
