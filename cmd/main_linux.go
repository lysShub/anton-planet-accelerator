//go:build linux
// +build linux

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/lysShub/anton-planet-accelerator/server"
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/rawsock/test"
	"github.com/stretchr/testify/require"
)

// go run -tags="debug -race" .
func main() {
	os.Remove("server-sender.pcap")
	var err error
	// fatun.SenderPcap, err = pcap.File("server-sender.pcap")
	if err != nil {
		panic(err)
	}

	var t = test.T()

	fmt.Println(debug.Debug(), time.Now().String())

	cfg := &server.Config{
		MaxRecvBuff:      1564,
		HandshakeTimeout: time.Second * 15,
		PcapBuiltinPath:  "server-builtin.pcap",
	}

	s, err := server.New(":19986", cfg)
	require.NoError(t, err)
	defer s.Close()

	require.NoError(t, s.Serve())
}
