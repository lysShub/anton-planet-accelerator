package main

import (
	"core/divert"
	"fmt"

	"github.com/shirou/gopsutil/process"
)

func main() {

	// var f = "processId=23984"
	var f = "udp.DstPort == 19986"

	h, err := divert.Open(f, divert.LAYER_NETWORK, 1121, divert.FLAG_RECV_ONLY)
	if err != nil {
		fmt.Println(1, err)
		return
	}
	defer h.Close()

	var _b = make([]byte, 512)
	for {
		n, addr, err := h.Recv(_b)
		if err != nil {
			fmt.Println(2, err)
			return
		}

		fmt.Println(n)
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
