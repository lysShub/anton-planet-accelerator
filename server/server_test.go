//go:build linux
// +build linux

package server_test

import (
	"fmt"
	"testing"

	"github.com/lysShub/anton-planet-accelerator/server"
	"github.com/stretchr/testify/require"
)

func TestXxxx(t *testing.T) {
	s, err := server.NewServer(":443")
	require.NoError(t, err)
	fmt.Println("启动")
	fmt.Println(s.Serve())
}
