package nodes

import (
	"math/rand"
	"testing"

	"github.com/lysShub/anton-planet-accelerator/bvvd"
	"github.com/lysShub/netkit/packet"
	"github.com/stretchr/testify/require"
)

func Test_Message(t *testing.T) {

	var pkt = packet.Make()
	var msg = Message{MsgID: rand.Uint32(), Payload: bvvd.Location(0)}
	msg.Kind = bvvd.PingForward
	require.NoError(t, msg.Encode(pkt))

	var msg2 Message

	require.NoError(t, msg2.Decode(pkt))

	require.Equal(t, msg, msg2)
}
