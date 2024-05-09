弃用，家庭移动网络似乎不通（可能是NAT的原因, 下面ut在两台vps上是可以通的）

ref:
 https://help.aliyun.com/zh/ecs/classic-network-under-different-areas-between-ecs-examples-of-how-to-build-ipip-tunnel

 https://zhuanlan.zhihu.com/p/53038611



``` go
package main

import (
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func Test_Send(t *testing.T) {

	// unix.IPPROTO_IP

	conn, err := net.DialIP("ip4:ipencap", &net.IPAddr{IP: net.IP{172, 23, 193, 179}}, &net.IPAddr{IP: net.IP{103, 94, 185, 61}})
	require.NoError(t, err)
	defer conn.Close()

	var ip = header.IPv4{
		0x45, 0x00,
		0x00, 0x3c, 0xbf, 0x3c, 0x00, 0x00, 0x80, 0x01, 0xd3, 0x7e, 0xc0, 0xa8, 0x2b, 0x23, 0x14, 0xc6,
		0xa7, 0x74, 0x08, 0x00, 0x49, 0xa0, 0x00, 0x01, 0x03, 0xbb, 0x61, 0x62, 0x63, 0x64, 0x65, 0x66,
		0x67, 0x68, 0x69, 0x6a, 0x6b, 0x6c, 0x6d, 0x6e, 0x6f, 0x70, 0x71, 0x72, 0x73, 0x74, 0x75, 0x76,
		0x77, 0x61, 0x62, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69,
	}

	n, err := conn.Write(ip)
	require.NoError(t, err)
	fmt.Println(len(ip), n)
}

func Test_Recv(t *testing.T) {

	l, err := net.ListenIP("ip4:ipencap", &net.IPAddr{IP: net.IP{172, 23, 193, 179}})
	require.NoError(t, err)

	var b = make([]byte, 1536)

	for {
		n, err := l.Read(b)
		require.NoError(t, err)
		fmt.Println(b[:n])
	}
}
```