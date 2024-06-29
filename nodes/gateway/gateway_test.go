//go:build linux
// +build linux

package gateway_test

import (
	"fmt"
	"testing"

	"github.com/lysShub/anton-planet-accelerator/nodes/gateway"
	"github.com/lysShub/netkit/debug"
	"github.com/stretchr/testify/require"
)

// go test -race -v -tags "debug" -run TestXxxx

func TestXxxx(t *testing.T) {
	fmt.Println(debug.Debug())

	config := gateway.Config{
		MaxRecvBuff: 1536,

		// Forwards: []struct {
		// 	Faddr netip.AddrPort
		// 	LocID bvvd.LocID
		// }{
		// 	{
		// 		Faddr: netip.MustParseAddrPort("45.150.236.6:19986"),
		// 		LocID: bvvd.Moscow,
		// 	},
		// },
	}

	p, err := gateway.New(":19986", &config)
	require.NoError(t, err)

	err = p.Serve()
	require.NoError(t, err)
}
