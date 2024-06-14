package conn

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/bpf"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func Test_iphdrLen(t *testing.T) {
	var ips = [][]byte{
		func() header.IPv4 {
			var b = make(header.IPv4, 64)
			b.Encode(&header.IPv4Fields{
				Protocol: uint8(header.TCPProtocolNumber),
				SrcAddr:  tcpip.AddrFrom4([4]byte{3: 1}),
				DstAddr:  tcpip.AddrFrom4([4]byte{1: 1}),
			})
			return b[:b.HeaderLength()]
		}(),
		func() header.IPv4 {
			var b = make(header.IPv4, 64)
			b.Encode(&header.IPv4Fields{
				Protocol: uint8(header.TCPProtocolNumber),
				SrcAddr:  tcpip.AddrFrom4([4]byte{0: 4}),
				DstAddr:  tcpip.AddrFrom4([4]byte{2: 1}),
				Options: header.IPv4OptionsSerializer{
					&header.IPv4SerializableRouterAlertOption{},
				},
			})
			return b[:b.HeaderLength()]
		}(),
		func() header.IPv6 {
			var b = make(header.IPv6, 64)
			b.Encode(&header.IPv6Fields{
				SrcAddr: tcpip.AddrFrom16([16]byte{11: 4}),
				DstAddr: tcpip.AddrFrom16([16]byte{2: 1}),
			})
			return b[:b.NextHeader()]
		}(),
	}

	var ins = iphdrLen()
	ins = append(ins,
		bpf.TXA{},
		bpf.RetA{},
	)

	for _, ip := range ips {
		v, err := bpf.NewVM(ins)
		require.NoError(t, err)
		n, err := v.Run(ip)
		require.NoError(t, err)
		require.Equal(t, len(ip), n)
	}

}
