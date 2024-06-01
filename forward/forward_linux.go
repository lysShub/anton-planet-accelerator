//go:build linux
// +build linux

package main

import (
	"fmt"
	"net"
	"net/netip"
	"sync/atomic"
	"time"

	"github.com/lysShub/rawsock/test"
	"github.com/stretchr/testify/require"
)

func main() {
	var (
		laddr = netip.AddrPortFrom(test.LocIP(), 19986)
		next  = netip.AddrPortFrom(netip.MustParseAddr("47.253.116.190"), 19986)
	)

	forwarder(laddr, next)
}

func forwarder(laddr netip.AddrPort, next netip.AddrPort) {
	fmt.Println("listen:", laddr.String(), "next:", next.String())

	var t = test.T()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: laddr.Addr().AsSlice(), Port: int(laddr.Port())})
	require.NoError(t, err)

	var up, down atomic.Int32
	go func() {
		for {
			time.Sleep(time.Second)
			println(up.Swap(0), down.Swap(0))
		}
	}()

	var client netip.AddrPort
	var b = make([]byte, 1536)
	for {
		n, addr, err := conn.ReadFromUDPAddrPort(b)
		require.NoError(t, err)
		if !client.IsValid() {
			client = addr
			fmt.Println("client", client.String())
		}

		if addr == next {
			n, err = conn.WriteToUDPAddrPort(b[:n], client)
			require.NoError(t, err)
			down.Add(int32(n))
		} else if addr == client {
			n, err = conn.WriteToUDPAddrPort(b[:n], next)
			require.NoError(t, err)
			up.Add(int32(n))
		}
	}

}

/*
// todo: 仅udp forward
type forward struct {
	addr netip.AddrPort
	conn *net.IPConn

	nexts  []netip.Addr
	sender *net.IPConn

	uplinksMu sync.RWMutex
	uplinks   map[netip.AddrPort]uint16 // client-addr:local-port

	downlinksMu sync.RWMutex
	donwlinks   map[uint16]netip.AddrPort // local-port:client-addr
}

func New(listenUdpPort uint16) (*forward, error) {
	var f = &forward{}

	// var err error
	// f.conn, err = net.ListenUDP("udp4", &net.UDPAddr{Port: int(listenUdpPort)})
	// if err != nil {
	// 	return nil, f.close(err)
	// }

	return f, nil
}

func (f *forward) close(cause error) error {
	panic("")
}

func (f *forward) Run() (_ error) {
	// var b = make([]byte, 1536)

	for {
		// n, addr, err := f.conn.ReadFromUDPAddrPort(b)
		// if err != nil {
		// 	return f.close(err)
		// }

		// f.uplinksMu.RLock()
		// conn, has := f.uplinks[addr]
		// f.uplinksMu.RUnlock()
		// if !has {

		// }

		// _, err = conn.Write(b[:n])
		// if err != nil {
		// 	return f.close(err)
		// }
	}
}

func (f *forward) UpdateNexts(rss net.Conn) {
}

// todo: 需要指明proto
func (f *forward) AddNext(next netip.AddrPort) {

}
func (f *forward) DelNext(net netip.AddrPort) {

}

func (f *forward) newConn(dst netip.Addr) (net.Conn, error) {

	var _ links.LinksManager

	if len(f.nexts) == 0 {
		panic("")
	} else if len(f.nexts) >= 1 {
		// conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: f.nexts[0].AsSlice(), Port: int(f.addr.Port())})
		// if err != nil {
		// 	return nil, err
		// }
		panic("")
	}
	panic("")
}
*/
