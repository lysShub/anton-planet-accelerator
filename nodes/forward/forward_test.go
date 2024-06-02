//go:build linux
// +build linux

package forward_test

import (
	"testing"

	"github.com/lysShub/anton-planet-accelerator/nodes/forward"
	"github.com/stretchr/testify/require"
)

func TestXxxx(t *testing.T) {
	// var ip = netip.MustParseAddr("172.241.71.116")

	// s := time.Now()
	// country, err := GetCountry(ip)
	// require.NoError(t, err)
	// fmt.Println(country, time.Since(s))

	config := &forward.Config{
		MaxRecvBuffSize: 1536,
	}

	f, err := forward.New(":19986", config)
	require.NoError(t, err)

	err = f.Serve()
	require.NoError(t, err)
}
