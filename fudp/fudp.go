package fudp

import (
	"fmt"
	"net"
	"net/netip"
)

// udp + fec

type Upack struct {

	// 每个数据需要4个地址, 其中proxyClientAddr、proxyServerAddr是udp包本身地址
	// warthunder<==>proxyClient ------------------ proxyServe_R<==>proxyServe_S ----------------  serverAddr
	//                               -----> 4个地址都有
	//                                             <-----
	// 发送： 代理捕获udp数据包, 将数据包发送proxy-conn; 代理根据warthunder-laddr, 判断是否用新port发送
	//        数据到server, 在一个port中, 有一个map记录了clientAddr和
	//
	// 相比建立session, 这样更简单, 且符合udp的无连接特性。

	// cAddrLen(1B) + CAddr(nB) + sAddrLen(1B) + SAddr(nB) + Data(nB)

	pIdx uint16 // proxy session index, 五元组确定一个session

	CAddrLen, SAddrLen uint8

	// CAddr: warThunder 本地地址
	// SAddr: warThunder 请求的远程地址
	CAddr, SAddr netip.AddrPort
	Data         []byte
}

func NewUpack() *Upack {
	return &Upack{
		Data: make([]byte, 0, 65535),
	}
}

func (u *Upack) Marshal() []byte {

	return nil
}

func (u *Upack) Unmarshal(b []byte) {

}

type Fudp struct {
	proxyConn net.Conn
}

// read a ip packet
func (s *Fudp) Read(p *Upack) {

	fmt.Println(p)

}

func (s *Fudp) Write(p *Upack) {

}
