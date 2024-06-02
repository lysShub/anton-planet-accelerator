//go:build linux
// +build linux

package proxyer_test

import (
	"fmt"
	"testing"

	"github.com/lysShub/anton-planet-accelerator/nodes/proxyer"
	"github.com/lysShub/netkit/debug"
	"github.com/stretchr/testify/require"
)

// go test -race -v -tags "debug" -run TestXxxx

func TestXxxx(t *testing.T) {
	fmt.Println(debug.Debug())

	config := proxyer.Config{
		MaxRecvBuff: 1536,
	}

	p, err := proxyer.New(":19986", &config)
	require.NoError(t, err)

	// p.AddForward()

	err = p.Serve()
	require.NoError(t, err)
}
