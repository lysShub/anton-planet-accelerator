package client_test

import (
	"fmt"
	"net/netip"
	"testing"

	"github.com/jftuga/geodist"
	"github.com/lysShub/anton-planet-accelerator/nodes/client"
	"github.com/stretchr/testify/require"
)

func TestXxx(t *testing.T) {

	var (
		a = geodist.Coord{Lat: 52.3667, Lon: 4.89454}
		b = geodist.Coord{Lat: 60.0098, Lon: 30.374}
	)

	_, dist, err := geodist.VincentyDistance(a, b)

	fmt.Println(dist, err)
}

func Test_IP2Localtion(t *testing.T) {
	loc, err := client.IP2Localtion(netip.MustParseAddr("114.114.114.114"))
	require.NoError(t, err)

	require.InDelta(t, 117, loc.Lon, 15)
	require.InDelta(t, 30, loc.Lat, 15)
}
