package conn

import "golang.org/x/net/bpf"

func FilterPorts(srcPort, dstPort uint16) []bpf.Instruction {
	var ins = iphdrLen()

	ins = append(ins, filterPorts(srcPort, dstPort)...)
	ins = append(ins,
		bpf.RetConstant{Val: 0xffff},
	)
	return ins
}

// iphdrLen store ip header length to reg X
func iphdrLen() []bpf.Instruction {
	return []bpf.Instruction{
		// load ip version to A
		bpf.LoadAbsolute{Off: 0, Size: 1},
		bpf.ALUOpConstant{Op: bpf.ALUOpShiftRight, Val: 4},

		// ipv4
		bpf.JumpIf{Cond: bpf.JumpNotEqual, Val: 4, SkipTrue: 1},
		bpf.LoadMemShift{Off: 0},

		// ipv6
		bpf.JumpIf{Cond: bpf.JumpNotEqual, Val: 6, SkipTrue: 1},
		bpf.LoadConstant{Dst: bpf.RegX, Val: 40},
	}
}

// filterPorts filter tcp/udp port, require regX stored iphdr length.
func filterPorts(srcPort, dstPort uint16) (ins []bpf.Instruction) {
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
	return ins
}
