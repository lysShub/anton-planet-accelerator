//go:build linux
// +build linux

package proxyer_test

import (
	"fmt"
	"net/netip"
	"testing"

	"github.com/jftuga/geodist"
	"github.com/lysShub/anton-planet-accelerator/nodes/proxyer"
	"github.com/lysShub/netkit/debug"
	"github.com/stretchr/testify/require"
)

// go test -race -v -tags "debug" -run TestXxxx

var (
	Moscow = geodist.Coord{Lon: 37.56, Lat: 55.75}
)

func TestXxxx(t *testing.T) {
	fmt.Println(debug.Debug())

	config := proxyer.Config{
		MaxRecvBuff: 1536,
	}

	p, err := proxyer.New(":19986", &config)
	require.NoError(t, err)

	p.AddForward(netip.MustParseAddr("45.150.236.6"), Moscow)
	p.AddClient(1)

	err = p.Serve()
	require.NoError(t, err)
}
