package fudp

import "net/netip"

type upack struct {

	// 每个数据需要4个地址, 其中proxyClientAddr、proxyServerAddr是udp包本身地址
	// warthunder<==>proxyClient ------------------ proxyServe_R<==>proxyServe_S ----------------  serverAddr
	//                               -----> 4个地址都有
	//                                             <-----
	// 发送： 代理捕获udp数据包, 将数据包发送proxy-conn; 代理根据warthunder-laddr, 判断是否用新port发送
	//        数据到server, 在一个port中, 有一个map记录了clientAddr和
	//
	// 相比建立session, 这样更简单, 且符合udp的无连接特性。

	cAddrLen, sAddrLen uint8
	cAddr, sAddr       netip.AddrPort
	data               []byte
}

type Fudp struct{}

// read a ip packet
func (s *Fudp) Read(b []byte) {

}

func (s *Fudp) Write(b []byte) {

}
