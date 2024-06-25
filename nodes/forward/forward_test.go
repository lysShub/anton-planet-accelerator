//go:build linux
// +build linux

package forward_test

import (
	"fmt"
	"testing"

	"github.com/lysShub/anton-planet-accelerator/bvvd"
	"github.com/lysShub/anton-planet-accelerator/nodes/forward"
	"github.com/lysShub/netkit/debug"
	"github.com/stretchr/testify/require"
)

func TestXxxx(t *testing.T) {
	fmt.Println(debug.Debug())

	config := &forward.Config{
		MaxRecvBuffSize: 1536,
	}

	f, err := forward.New(":19986", bvvd.Moscow.LocID(), config)
	require.NoError(t, err)

	err = f.Serve()
	require.NoError(t, err)
}
