package main

import (
	"fmt"

	"github.com/lysShub/warthunder/client/divert"
	"github.com/lysShub/warthunder/helper"

	"github.com/shirou/gopsutil/process"
)

func main() {

	captureUdp()
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

func captureUdp() {
	// var f = "udp and udp.SrcPort == 19986 and outbound"
	var f = "udp and outbound and remoteAddr==114.116.254.26 and remotePort==19986 "

	h, err := divert.Open(f, divert.LAYER_NETWORK, 11, divert.FLAG_RECV_ONLY)
	if err != nil {
		fmt.Println(1, err)
		return
	}
	defer h.Close()

	var da = make([]byte, 60)
	var addr divert.Address
	var n int
	for {
		n, addr, err = h.Recv(da)
		if err != nil {
			fmt.Println(2, err, addr)
			return
		}

		u := helper.Ipack(da[:n])

		fmt.Println(u.Laddr().String(), u.Raddr().String())

	}

	return
	for {
		n, addr, err = h.Recv(da)
		if err != nil {
			fmt.Println(2, err, addr)
			return
		}
		fmt.Println(da[:n])

		if !addr.IPv6() {

			da[12], da[13], da[14], da[15], da[16], da[17], da[18], da[19] = da[16], da[17], da[18], da[19], da[12], da[13], da[14], da[15]

			addr.Clean()
			fmt.Println(addr.Network())
			if n, err = h.Send(da[:n], &addr); err != nil {
				fmt.Println(3, err)
				return
			} else {
				fmt.Println("send", n)
			}

		}

	}

}
