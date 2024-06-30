package msg

import (
	"encoding/binary"

	"github.com/lysShub/anton-planet-accelerator/bvvd"
	"github.com/lysShub/netkit/packet"
	"github.com/pkg/errors"
)

const MinSize = bvvd.Size + 4

type Message []byte

func (m Message) MsgID() uint32 {
	return binary.BigEndian.Uint32(m[bvvd.Size:])
}

func (m Message) SetMsgID(id uint32) {
	binary.BigEndian.PutUint32(m[bvvd.Size:], id)
}

func (m Message) Payload(to Payload) error {
	return to.Decode(packet.From(m[MinSize:]))
}

func (m Message) SetPayload(from Payload) error {
	return from.Encode(packet.From(m[MinSize:]))
}

type Fields struct {
	bvvd.Fields

	MsgID   uint32
	Payload Payload // option, store msg payload, such as PL
}

type Payload interface {
	Encode(to *packet.Packet) (err error)
	Decode(from *packet.Packet) error
}

func (m *Fields) Encode(to *packet.Packet) (err error) {
	if err := m.Fields.Encode(to); err != nil {
		return err
	}

	if m.MsgID == 0 {
		return errors.Errorf("require message id")
	}
	to.Append(binary.BigEndian.AppendUint32(make([]byte, 0, 4), m.MsgID)...)

	if m.Payload != nil {
		return m.Payload.Encode(to)
	}
	return nil
}

func (m *Fields) Decode(from *packet.Packet) error {
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

	return m.Payload.Decode(from)
}
