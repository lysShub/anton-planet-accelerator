package proto

import (
	"math/rand"
	"syscall"
	"testing"

	"bou.ke/monkey"
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/rawsock/test"
	"github.com/stretchr/testify/require"
)

func Test_Proto(t *testing.T) {
	monkey.Patch(debug.Debug, func() bool { return false })

	msg := "hello world"

	var pkt = packet.From([]byte(msg))
	var h1 = Header{
		Server: test.RandIP(),
		Proto:  syscall.IPPROTO_UDP,
		ID:     ID(uint16(rand.Uint32())),
		Kind:   PlProxyer,
	}
	h1.Encode(pkt)

	var h2 Header
	h2.Decode(pkt)
	require.Equal(t, h1, h2)
	require.Equal(t, msg, string(pkt.Bytes()))
}
