//go:build linux
// +build linux

package proxyer_test

import (
	"fmt"
	"net/netip"
	"testing"

	"github.com/lysShub/anton-planet-accelerator/bvvd"
	"github.com/lysShub/anton-planet-accelerator/nodes/proxyer"
	"github.com/lysShub/netkit/debug"
	"github.com/stretchr/testify/require"
)

// go test -race -v -tags "debug" -run TestXxxx

func TestXxxx(t *testing.T) {
	fmt.Println(debug.Debug())

	config := proxyer.Config{
		MaxRecvBuff: 1536,

		Forwards: []struct {
			Faddr netip.AddrPort
			LocID bvvd.LocID
		}{
			{
				Faddr: netip.MustParseAddrPort("45.150.236.6:19986"),
				LocID: bvvd.Moscow,
			},
		},
	}

	p, err := proxyer.New(":19986", &config)
	require.NoError(t, err)

	err = p.Serve()
	require.NoError(t, err)
}
