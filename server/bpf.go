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

func filterLocalPorts(conn *net.IPConn, ports ...uint16) error {
	slices.Sort(ports)
	ports = slices.Compact(ports)
	if len(ports) == 0 {
		return nil
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

	var prog *unix.SockFprog
	if rawIns, err := bpf.Assemble(ins); err != nil {
		return err
	} else {
		prog = &unix.SockFprog{
			Len:    uint16(len(rawIns)),
			Filter: (*unix.SockFilter)(unsafe.Pointer(&rawIns[0])),
		}
	}

	raw, err := conn.SyscallConn()
	if err != nil {
		return errors.WithStack(err)
	}
	var e error
	err = raw.Control(func(fd uintptr) {
		e = unix.SetsockoptSockFprog(
			int(fd), unix.SOL_SOCKET, unix.SO_ATTACH_FILTER, prog,
		)
	})
	if e != nil {
		return errors.WithStack(e)
	} else if err != nil {
		return errors.WithStack(err)
	}
	return nil
}
