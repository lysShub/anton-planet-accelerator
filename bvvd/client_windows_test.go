//go:build windows
// +build windows

package bvvd

import (
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/lysShub/divert-go"
	"github.com/stretchr/testify/require"
	"github.com/txthinking/socks5"
)

func TestXxxxx(t *testing.T) {
	divert.MustLoad(divert.DLL)
	defer divert.Release()

	c, err := NewClient(netip.MustParseAddrPort("172.24.131.26:8080"))
	require.NoError(t, err)

	time.Sleep(time.Hour)
	c.Close()
}

func Test_A(t *testing.T) {

	conn, err := socks5.DialTCP("tcp", "", "172.24.131.26:8080")

	// conn, err := net.DialTCP("tcp", nil, &net.TCPAddr{IP: net.IP{172, 24, 131, 26}, Port: 8080})
	require.NoError(t, err)
	conn.Close()
	fmt.Println("ok")
}
