package bvvd

import (
	"net/netip"
	"testing"
	"time"

	"github.com/lysShub/divert-go"
	"github.com/stretchr/testify/require"
)

func TestXxxx(t *testing.T) {
	divert.MustLoad(divert.DLL)
	defer divert.Release()

	// s:=netip.MustParseAddr("8.137.91.200")
	s := netip.MustParseAddr("103.94.185.61")

	c, err := NewClient(s)
	require.NoError(t, err)

	time.Sleep(time.Hour)
	c.close(nil)
}
