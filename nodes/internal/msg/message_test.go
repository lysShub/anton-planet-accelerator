package msg

import (
	"math/rand"
	"testing"

	"github.com/lysShub/anton-planet-accelerator/bvvd"
	"github.com/lysShub/netkit/packet"
	"github.com/stretchr/testify/require"
)

func Test_Message(t *testing.T) {

	var pkt = packet.Make()
	var loc = bvvd.Location(0)
	var msg = Fields{MsgID: rand.Uint32(), Payload: &loc}
	msg.Kind = bvvd.PingForward
	require.NoError(t, msg.Encode(pkt))

	var msg2 Fields

	require.NoError(t, msg2.Decode(pkt))
	require.Zero(t, pkt.Data())

	require.Equal(t, msg, msg2)
}
