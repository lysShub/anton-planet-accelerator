package main

import (
	"anton/client/divert"
	"anton/client/proxy"
	"anton/ctx"
	"context"
	"fmt"
	"time"

	"github.com/shirou/gopsutil/process"
)

func main() {

	proxy.NewProxy(ctx.WithException(context.Background()))

	time.Sleep(time.Minute)
	return

	// var f = "processId=23984"
	var f = "udp and processId=20740"
	// var f = "udp and remoteAddr == 114.116.254.26"

	h, err := divert.Open(f, divert.LAYER_SOCKET, 1211, divert.FLAG_READ_ONLY|divert.FLAG_SNIFF)
	if err != nil {
		fmt.Println(1, err)
		return
	}
	defer h.Close()

	var b = make([]byte, 0)
	for {
		n, addr, err := h.Recv(b)
		if err != nil {
			fmt.Println(2, err)
			return
		}

		a := addr.Socket()
		bb := addr.IPv6()

		_, op1 := addr.Header.Event.String()
		fmt.Println(n, a, op1, bb)
		continue

		s := addr.Flow()

		p, err := process.NewProcess(int32(s.ProcessId))
		if err != nil {
			fmt.Println(3, err)
			return
		}
		name, err := p.Name()
		if err != nil {
			fmt.Println(4, err)
			return
		}

		_, op := addr.Header.Event.String()
		fmt.Printf("%s	%d	%s	%s %s:%d\n", op, s.ProcessId, name, s.Protocol, s.RemoteAddr(), s.LocalPort)
	}

}
