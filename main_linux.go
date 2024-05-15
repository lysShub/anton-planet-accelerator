//go:build linux
// +build linux

package main

import (
	"fmt"
	"net/netip"

	"github.com/lysShub/anton-planet-accelerator/server"
	"github.com/lysShub/rawsock/helper/bind"
	"github.com/lysShub/rawsock/test"
)

func main() {
	err := bind.SetGRO(test.LocIP(), netip.AddrFrom4([4]byte{8, 8, 8, 8}), false)
	if err != nil {
		panic(err)
	}

	s, err := server.NewServer(":443")
	if err != nil {
		panic(err)
	}

	fmt.Println("启动")
	fmt.Println(s.Serve())
}
