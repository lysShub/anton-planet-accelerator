package msg

import (
	"encoding/binary"
	"unsafe"

	"github.com/lysShub/anton-planet-accelerator/bvvd"
	"github.com/lysShub/anton-planet-accelerator/nodes/internal/stats"
	"github.com/lysShub/netkit/debug"
	"github.com/lysShub/netkit/packet"
	"github.com/lysShub/rawsock/test"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

type Message struct {
	bvvd.Fields

	MsgID   uint32
	Payload any // option, store msg payload, such as PL
}

func (m *Message) Encode(to *packet.Packet) (err error) {

	if err := m.Fields.Encode(to); err != nil {
		return err
	}

	if m.MsgID == 0 {
		return errors.Errorf("require message id")
	}
	to.Append(binary.BigEndian.AppendUint32(make([]byte, 0, 4), m.MsgID)...)

	if m.Payload != nil {
		switch m.Kind {
		case bvvd.PackLossClientUplink, bvvd.PackLossGatewayUplink, bvvd.PackLossGatewayDownlink:
			pl, ok := m.Payload.(stats.PL)
			if !ok {
				return errors.Errorf("invalid data type %T", m.Payload)
			}
			if err = pl.Encode(to); err != nil {
				return err
			}
		case bvvd.PingForward:
			p, ok := m.Payload.(bvvd.Location)
			if !ok {
				return errors.Errorf("invalid data type %T", m.Payload)
			}
			if debug.Debug() {
				require.Equal(test.T(), uintptr(1), unsafe.Sizeof(p))
			}
			to.Append(byte(p))
		default:
		}
	}
	return nil
}

func (m *Message) Decode(from *packet.Packet) error {
	if err := m.Fields.Decode(from); err != nil {
		return err
	}

	if from.Data() < 4 {
		return errors.Errorf("too small %d", from.Data())
	}
	m.MsgID = binary.BigEndian.Uint32(from.Detach(4))
	if m.MsgID == 0 {
		return errors.New("invalid message id")
	}

	switch m.Kind {
	case bvvd.PingGateway:
	case bvvd.PingForward:
		if from.Data() > 0 {
			m.Payload = bvvd.Location(from.Detach(1)[0])
		} else if !m.Forward.IsValid() {
			return errors.New("invalid forward")
		}
	case bvvd.PackLossGatewayUplink, bvvd.PackLossClientUplink, bvvd.PackLossGatewayDownlink:
		if from.Data() > 0 {
			var pl stats.PL
			if err := pl.Decode(from); err != nil {
				return err
			}
			m.Payload = pl
		}
	default:
		return errors.Errorf("unknown message kind %s", m.Kind.String())
	}
	return nil
}
