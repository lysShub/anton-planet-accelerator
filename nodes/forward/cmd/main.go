//go:build linux
// +build linux

package main

import (
	"fmt"

	"github.com/lysShub/anton-planet-accelerator/nodes/forward"
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/rawsock/test"
	"github.com/stretchr/testify/require"
)

func main() {
	fmt.Println(debug.Debug())
	t := test.T()

	config := &forward.Config{
		MaxRecvBuffSize: 1536,
	}

	f, err := forward.New(":19986", config)
	require.NoError(t, err)

	err = f.Serve()
	require.NoError(t, err)
}
