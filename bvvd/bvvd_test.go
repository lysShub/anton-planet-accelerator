package bvvd

import (
	"math/rand"
	"net/netip"
	"syscall"
	"testing"

	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/rawsock/test"
	"github.com/stretchr/testify/require"
)

func Test_Fields(t *testing.T) {
	msg := "hello world"

	var pkt = packet.Make().Append([]byte(msg)...)
	var h1 = Fields{
		Kind:   PackLossClientUplink,
		Proto:  syscall.IPPROTO_UDP,
		DataID: byte(rand.Uint32()),
		LocID:  Moscow.LocID(),
		Client: netip.AddrPortFrom(test.RandIP(), test.RandPort()),
		Server: test.RandIP(),
	}
	require.NoError(t, h1.Encode(pkt))

	var h2 Fields
	h2.Decode(pkt)
	require.Equal(t, h1, h2)
	require.Equal(t, msg, string(pkt.Bytes()))
}

func Test_Bvvd(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		msg := "hello world"
		var pkt = packet.Make().Append([]byte(msg)...)
		var f = Fields{
			Kind:   PackLossClientUplink,
			Proto:  syscall.IPPROTO_UDP,
			DataID: byte(rand.Uint32()),
			LocID:  Moscow.LocID(),
			Client: netip.AddrPortFrom(test.RandIP(), test.RandPort()),
			Server: test.RandIP(),
		}
		require.NoError(t, f.Encode(pkt))

		slave := Bvvd(pkt.Bytes())

		require.Equal(t, f.Kind, slave.Kind())
		require.Equal(t, f.Proto, slave.Proto())
		require.Equal(t, f.DataID, slave.DataID())
		require.Equal(t, f.LocID, slave.LocID())
		require.Equal(t, f.Client, slave.Client())
		require.Equal(t, f.Server, slave.Server())
	})

	t.Run("set", func(t *testing.T) {
		var f = Fields{
			Kind:   PackLossClientUplink,
			Proto:  syscall.IPPROTO_UDP,
			DataID: byte(rand.Uint32()),
			LocID:  Moscow.LocID(),
			Client: netip.AddrPortFrom(test.RandIP(), test.RandPort()),
			Server: test.RandIP(),
		}
		var pkt = packet.Make(0, Size)

		slave := Bvvd(pkt.Bytes())
		slave.SetKind(f.Kind)
		slave.SetProto(f.Proto)
		slave.SetDataID(f.DataID)
		slave.SetLocID(f.LocID)
		slave.SetClient(f.Client)
		slave.SetServer(f.Server)

		var f2 Fields
		f2.Decode(pkt)

		require.Equal(t, f, f2)
	})

}
