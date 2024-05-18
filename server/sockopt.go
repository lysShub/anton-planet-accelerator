//go:build linux
// +build linux

package server

import (
	"net"
	"slices"
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/net/bpf"
	"golang.org/x/sys/unix"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func setHdrinclAndBpfFilterLocalPorts(conn *net.IPConn, skipLocalPorts ...uint16) error {
	raw, err := conn.SyscallConn()
	if err != nil {
		return errors.WithStack(err)
	}

	var prog *unix.SockFprog
	if rawIns, err := bpf.Assemble(bpfSkipLocalPorts(skipLocalPorts...)); err != nil {
		return errors.WithStack(err)
	} else {
		prog = &unix.SockFprog{
			Len:    uint16(len(rawIns)),
			Filter: (*unix.SockFilter)(unsafe.Pointer(&rawIns[0])),
		}
	}

	var e error
	err = raw.Control(func(fd uintptr) {
		if e = unix.SetsockoptSockFprog(
			int(fd), unix.SOL_SOCKET, unix.SO_ATTACH_FILTER, prog,
		); e != nil {
			return
		}
		e = unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_HDRINCL, 1)
	})
	if e != nil {
		return errors.WithStack(e)
	} else if err != nil {
		return errors.WithStack(err)
	}
	return nil

}

func bpfSkipLocalPorts(ports ...uint16) []bpf.Instruction {
	slices.Sort(ports)
	ports = slices.Compact(ports)
	if len(ports) == 0 {
		return []bpf.Instruction{bpf.RetConstant{Val: 0xffff}}
	}

	var ins = []bpf.Instruction{
		// load ip version to A
		bpf.LoadAbsolute{Off: 0, Size: 1},
		bpf.ALUOpConstant{Op: bpf.ALUOpShiftRight, Val: 4},

		// ipv4
		bpf.JumpIf{Cond: bpf.JumpNotEqual, Val: 4, SkipTrue: 1},
		bpf.LoadMemShift{Off: 0},

		// ipv6
		bpf.JumpIf{Cond: bpf.JumpNotEqual, Val: 6, SkipTrue: 1},
		bpf.LoadConstant{Dst: bpf.RegX, Val: 40},
		/*
		  reg X store ipHdrLen
		*/
	}
	for _, e := range ports {
		ins = append(ins,
			bpf.LoadIndirect{Off: header.TCPDstPortOffset, Size: 2},
			bpf.JumpIf{Cond: bpf.JumpNotEqual, Val: uint32(e), SkipTrue: 1},
			bpf.RetConstant{Val: 0},
		)
	}
	ins = append(ins,
		bpf.RetConstant{Val: 0xffff},
	)
	return ins
}
