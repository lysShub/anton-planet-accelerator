package helper

// IP packet parser

import (
	"encoding/binary"
	"net/netip"
	"unsafe"
)

type Ipack []byte

func (u Ipack) Validate() bool {
	switch u.Version() {
	case 4:
		// github.com/google/gopacket/layers

		return len(u) > 20 &&
			len(u) > int(u[0]&0x0f)*4 &&
			((u[6]>>5)&0b1 == 0 && uint16(len(u)) == binary.BigEndian.Uint16(u[2:4]))
	case 6:
		return len(u) > 40 &&
			binary.BigEndian.Uint16(u[4:6]) == uint16(len(u))+40
	default:
		return false
	}
}

func (u Ipack) Version() uint8 {
	if len(u) < 1 {
		return 0
	}
	return u[0] >> 4
}

func (u Ipack) hdrLen() int {
	switch u.Version() {
	case 4:
		return int(u[0]&0x0f) * 4
	case 6:
		return 40
	default:
		return 0
	}
}

func (u Ipack) Laddr() (addr netip.AddrPort) {
	if !u.Validate() {
		return addr
	}

	hl := u.hdrLen()
	switch u.Version() {
	case 4:
		return netip.AddrPortFrom(
			netip.AddrFrom4(*(*[4]byte)(unsafe.Pointer(&u[12]))),
			binary.BigEndian.Uint16(u[hl:hl+2]),
		)
	case 6:
		return netip.AddrPortFrom(
			netip.AddrFrom16(*(*[16]byte)(unsafe.Pointer(&u[8]))),
			binary.BigEndian.Uint16(u[hl:hl+2]),
		)
	default:
		return addr
	}
}

func (u Ipack) Raddr() (addr netip.AddrPort) {
	if !u.Validate() {
		return addr
	}

	hl := u.hdrLen()
	switch u.Version() {
	case 4:
		return netip.AddrPortFrom(
			netip.AddrFrom4(*(*[4]byte)(unsafe.Pointer(&u[16]))),
			binary.BigEndian.Uint16(u[hl+2:hl+4]),
		)
	case 6:
		return netip.AddrPortFrom(
			netip.AddrFrom16(*(*[16]byte)(unsafe.Pointer(&u[24]))),
			binary.BigEndian.Uint16(u[hl+2:hl+4]),
		)
	default:
		return addr
	}
}
