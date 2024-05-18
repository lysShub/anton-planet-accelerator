//go:build windows
// +build windows

package client_test

import (
	"fmt"
	"net/netip"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/google/gopacket/pcapgo"
	"github.com/lysShub/anton-planet-accelerator/client"
	"github.com/lysShub/divert-go"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func TestXxxx(t *testing.T) {
	divert.MustLoad(divert.DLL)
	defer divert.Release()

	// c, err := client.NewClient("172.24.131.26:8080")
	c, err := client.NewClient("103.94.185.61:443") // 旧金山
	// c, err := client.NewClient("8.222.33.114:443") // 吉隆坡
	// c, err := client.NewClient("8.222.83.247:443") // 东京
	// c, err := client.NewClient("45.80.191.120:443") // 富兰克林
	require.NoError(t, err)

	fmt.Println("connected")

	// start := time.Now()
	for /* time.Since(start) < time.Hour */ {
		ping, err := c.Ping()
		require.NoError(t, err)
		fmt.Println("ping", ping.String())

		time.Sleep(time.Second * 3)
	}

	c.Close()
}

/*





 */

func Test_Pcap(t *testing.T) {
	fh, err := os.Open("./warthunder-单局.1.pcap")
	require.NoError(t, err)
	defer fh.Close()

	r, err := pcapgo.NewReader(fh)
	require.NoError(t, err)

	// var m = map[netip.AddrPort]int{}
	var ips = map[netip.Addr]int{}

	var total, tcp, udp int
	for {
		data, _, err := r.ReadPacketData()
		if err != nil && err.Error() == "EOF" {
			break
		}
		n := len(data) - 14 + 16

		total += n
		switch header.IPv4(data[14:]).TransportProtocol() {
		case header.TCPProtocolNumber:
			tcp += n

			// tcp := header.TCP(ip.Payload())

			ip := header.IPv4(data[14:])
			if netip.AddrFrom4(ip.DestinationAddress().As4()).IsPrivate() {
				ips[netip.AddrFrom4(ip.SourceAddress().As4())] += 1
			} else {
				ips[netip.AddrFrom4(ip.DestinationAddress().As4())] += 1
			}
		case header.UDPProtocolNumber:
			udp += n

			// ip := header.IPv4(data[14:])
			// if netip.AddrFrom4(ip.DestinationAddress().As4()).IsPrivate() {
			// 	ips[netip.AddrFrom4(ip.SourceAddress().As4())] += 1
			// } else {
			// 	ips[netip.AddrFrom4(ip.DestinationAddress().As4())] += 1
			// }
		default:
			panic("")
		}
	}

	fmt.Println(total)
	fmt.Println(tcp)
	fmt.Println(udp)

	var ss = []struct {
		// addr  netip.AddrPort
		port  netip.Addr
		bytes int
	}{}
	for k, v := range ips {
		ss = append(ss, struct {
			// addr  netip.AddrPort
			port  netip.Addr
			bytes int
		}{
			// addr: k,
			port:  k,
			bytes: v,
		})
	}
	sort.Slice(ss, func(i, j int) bool {
		return ss[i].bytes > ss[j].bytes
	})

	for _, e := range ss {
		fmt.Println(e.port, e.bytes)
	}
}
