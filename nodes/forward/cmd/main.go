//go:build linux
// +build linux

package main

import (
	"fmt"

	"github.com/lysShub/anton-planet-accelerator/bvvd"
	"github.com/lysShub/anton-planet-accelerator/nodes/forward"
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/rawsock/test"
	"github.com/stretchr/testify/require"
)

// go build -tags "debug" -race .
func main() {
	fmt.Println(debug.Debug())
	t := test.T()

	config := &forward.Config{
		MaxRecvBuffSize: 2048,
	}

	f, err := forward.New(":19986", bvvd.Moscow, config)
	require.NoError(t, err)

	err = f.Serve()
	require.NoError(t, err)
}
