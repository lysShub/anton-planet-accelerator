//go:build linux
// +build linux

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/lysShub/anton-planet-accelerator/bvvd"
	"github.com/lysShub/anton-planet-accelerator/nodes/forward"
	"github.com/lysShub/rawsock/test"
	"github.com/stretchr/testify/require"
)

// go run -tags "debug" . 莫斯科
func main() {
	var loc bvvd.Location
	for i, e := range os.Args {
		if i > 0 && !loc.Valid() {
			for _, l := range bvvd.Locations {
				if l.Hans() == strings.TrimSpace(e) {
					loc = l
				}
			}
		}
	}
	if !loc.Valid() {
		fmt.Println("require location")
		return
	}

	t := test.T()

	config := &forward.Config{
		Location:        loc,
		ForwardID:       1,
		MaxRecvBuffSize: 2048,
	}

	f, err := forward.New(":19986", config)
	require.NoError(t, err)

	err = f.Serve()
	require.NoError(t, err)
}

func match(name string) bool {
	for _, loc := range bvvd.Locations {
		if loc.Hans() == name {
			return true
		}
	}
	return false
}
