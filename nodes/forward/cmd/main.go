//go:build linux
// +build linux

package main

import (
	"github.com/lysShub/anton-planet-accelerator/nodes/forward"
	"github.com/lysShub/rawsock/test"
	"github.com/stretchr/testify/require"
)

// go run -tags "debug" . 莫斯科
func main() {
	t := test.T()

	config := &forward.Config{
		MaxRecvBuffSize: 2048,
	}

	f, err := forward.New(":19986", config)
	require.NoError(t, err)

	err = f.Serve()
	require.NoError(t, err)
}
