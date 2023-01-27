package fudp

import (
	"encoding/binary"
	"net/netip"
	"unsafe"
)

// type Upack struct {

// 	// 每个数据需要4个地址, 其中proxyClientAddr、proxyServerAddr是udp包本身地址
// 	// warthunder<==>proxyClient ------------------ proxyServe_R<==>proxyServe_S ----------------  serverAddr
// 	//                               -----> 4个地址都有
// 	//                                             <-----
// 	// 发送： 代理捕获udp数据包, 将数据包发送proxy-conn; 代理根据warthunder-laddr, 判断是否用新port发送
// 	//        数据到server, 在一个port中, 有一个map记录了clientAddr和
// 	//
// 	// 相比建立session, 这样更简单, 且符合udp的无连接特性。

// 	// cAddrLen(1B) + CAddr(nB) + sAddrLen(1B) + SAddr(nB) + Data(nB)

// 	pIdx uint16 // proxy session index, 五元组确定一个session

// 	CAddrLen, SAddrLen uint8

// 	// CAddr: warThunder 本地地址
// 	// SAddr: warThunder 请求的远程地址
// 	CAddr, SAddr netip.AddrPort
// 	Data         []byte
// }

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
