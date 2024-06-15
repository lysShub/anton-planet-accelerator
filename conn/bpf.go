package conn

import (
	"encoding/binary"
	"net/netip"

	"golang.org/x/net/bpf"
	"gvisor.dev/gvisor/pkg/tcpip"
)

func FilterIPv4AndPorts(srcPort, dstPort uint16) []bpf.Instruction {
	var ins = []bpf.Instruction{
		// load ip version to A
		bpf.LoadAbsolute{Off: 0, Size: 1},
		bpf.ALUOpConstant{Op: bpf.ALUOpShiftRight, Val: 4},

		// filter ipv4
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: 4, SkipTrue: 1},
		bpf.RetConstant{Val: 0},

		// load ipv4 header length to regX
		bpf.LoadMemShift{Off: 0},
	}

	if srcPort != 0 {
		ins = append(ins,
			bpf.LoadIndirect{Off: 0, Size: 2},
			bpf.JumpIf{Cond: bpf.JumpEqual, Val: uint32(srcPort), SkipTrue: 1},
			bpf.RetConstant{Val: 0},
		)
	}
	if dstPort != 0 {
		ins = append(ins,
			bpf.LoadIndirect{Off: 2, Size: 2},
			bpf.JumpIf{Cond: bpf.JumpEqual, Val: uint32(dstPort), SkipTrue: 1},
			bpf.RetConstant{Val: 0},
		)
	}

	ins = append(ins,
		bpf.RetConstant{Val: 0xffff},
	)
	return ins
}

func FilterIPv4AndEndpoint(src, dst netip.AddrPort, proto tcpip.TransportProtocolNumber) (ins []bpf.Instruction) {
	if !src.Addr().Is4() || src.Addr().Is4() != dst.Addr().Is4() {
		panic("only support ipv4")
	}

	// filter ipv4
	ins = []bpf.Instruction{
		bpf.LoadAbsolute{Off: 0, Size: 1},
		bpf.ALUOpConstant{Op: bpf.ALUOpShiftRight, Val: 4},
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: 4, SkipTrue: 1},
		bpf.RetConstant{Val: 0},
	}

	// filter proto
	if proto != 0 {
		ins = append(ins,
			bpf.LoadAbsolute{Off: 9, Size: 1},
			bpf.JumpIf{Cond: bpf.JumpEqual, Val: uint32(proto), SkipTrue: 1},
			bpf.RetConstant{Val: 0},
		)
	}

	if !src.Addr().IsUnspecified() {
		srcInt := binary.BigEndian.Uint32(src.Addr().AsSlice())

		ins = append(ins,
			bpf.LoadAbsolute{Off: 12, Size: 4},
			bpf.JumpIf{Cond: bpf.JumpEqual, Val: srcInt, SkipTrue: 1},
			bpf.RetConstant{Val: 0},
		)
	}
	if !dst.Addr().IsUnspecified() {
		dstInt := binary.BigEndian.Uint32(dst.Addr().AsSlice())

		ins = append(ins,
			bpf.LoadAbsolute{Off: 16, Size: 4},
			bpf.JumpIf{Cond: bpf.JumpEqual, Val: dstInt, SkipTrue: 1},
			bpf.RetConstant{Val: 0},
		)
	}

	ins = append(ins,
		// load ipv4 header length to regX
		bpf.LoadMemShift{Off: 0},
	)

	if src.Port() != 0 {
		ins = append(ins,
			bpf.LoadIndirect{Off: 0, Size: 2},
			bpf.JumpIf{Cond: bpf.JumpEqual, Val: uint32(src.Port()), SkipTrue: 1},
			bpf.RetConstant{Val: 0},
		)
	}
	if dst.Port() != 0 {
		ins = append(ins,
			bpf.LoadIndirect{Off: 2, Size: 2},
			bpf.JumpIf{Cond: bpf.JumpEqual, Val: uint32(dst.Port()), SkipTrue: 1},
			bpf.RetConstant{Val: 0},
		)
	}

	ins = append(ins,
		bpf.RetConstant{Val: 0xffff},
	)
	return ins
}

func FilterIPv4AndLocal(dst netip.AddrPort, proto tcpip.TransportProtocolNumber) (ins []bpf.Instruction) {
	if !dst.Addr().Is4() {
		panic("only support ipv4")
	}

	// filter ipv4
	ins = []bpf.Instruction{
		bpf.LoadAbsolute{Off: 0, Size: 1},
		bpf.ALUOpConstant{Op: bpf.ALUOpShiftRight, Val: 4},
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: 4, SkipTrue: 1},
		bpf.RetConstant{Val: 0},
	}

	// filter proto
	if proto != 0 {
		ins = append(ins,
			bpf.LoadAbsolute{Off: 9, Size: 1},
			bpf.JumpIf{Cond: bpf.JumpEqual, Val: uint32(proto), SkipTrue: 1},
			bpf.RetConstant{Val: 0},
		)
	}

	if !dst.Addr().IsUnspecified() {
		dstInt := binary.BigEndian.Uint32(dst.Addr().AsSlice())

		ins = append(ins,
			bpf.LoadAbsolute{Off: 16, Size: 4},
			bpf.JumpIf{Cond: bpf.JumpEqual, Val: dstInt, SkipTrue: 1},
			bpf.RetConstant{Val: 0},
		)
	}

	ins = append(ins,
		// load ipv4 header length to regX
		bpf.LoadMemShift{Off: 0},
	)

	if dst.Port() != 0 {
		ins = append(ins,
			bpf.LoadIndirect{Off: 2, Size: 2},
			bpf.JumpIf{Cond: bpf.JumpEqual, Val: uint32(dst.Port()), SkipTrue: 1},
			bpf.RetConstant{Val: 0},
		)
	}

	ins = append(ins,
		bpf.RetConstant{Val: 0xffff},
	)
	return ins
}
