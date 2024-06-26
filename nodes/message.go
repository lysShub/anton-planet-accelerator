package nodes

import (
	"encoding/binary"
	"net/netip"
	"unsafe"

	"github.com/lysShub/anton-planet-accelerator/bvvd"
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
	switch m.Kind {
	case bvvd.PingProxyer:
	case bvvd.PingForward:
		if !m.ForwardID.Vaid() && m.Payload == nil {
			return errors.Errorf("PingForward message require ForwardID or Raw")
		}
	case bvvd.PackLossClientUplink:
	case bvvd.PackLossProxyerUplink:
	case bvvd.PackLossProxyerDownlink:
	default:
		return errors.Errorf("unknown message kind %s", m.Kind.String())
	}
	if m.MsgID == 0 {
		return errors.Errorf("require message id")
	}

	if !m.Client.IsValid() {
		m.Client = netip.AddrPortFrom(netip.IPv4Unspecified(), 0)
	}
	if !m.Server.IsValid() {
		m.Server = netip.IPv4Unspecified()
	}
	if err := m.Fields.Encode(to); err != nil {
		return err
	}
	to.Append(binary.BigEndian.AppendUint32(make([]byte, 0, 4), m.MsgID)...)

	if m.Payload != nil {
		switch m.Kind {
		case bvvd.PackLossClientUplink, bvvd.PackLossProxyerUplink, bvvd.PackLossProxyerDownlink:
			pl, ok := m.Payload.(PL)
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
	var hdr bvvd.Fields
	if err := hdr.Decode(from); err != nil {
		return err
	}
	m.Kind = hdr.Kind
	m.ForwardID = hdr.ForwardID
	switch m.Kind {
	case bvvd.PingProxyer, bvvd.PackLossClientUplink, bvvd.PackLossProxyerDownlink:
	case bvvd.PingForward, bvvd.PackLossProxyerUplink:
		if !m.ForwardID.Vaid() && m.Payload == nil {
			return errors.Errorf("PingForward message require ForwardID or Raw")
		}
	default:
		return errors.Errorf("unknown message kind %s", m.Kind.String())
	}

	if from.Data() < 4 {
		return errors.Errorf("too small %d", from.Data())
	}
	m.MsgID = binary.BigEndian.Uint32(from.Detach(make([]byte, 4)))

	if from.Data() > 0 {
		switch m.Kind {
		case bvvd.PackLossClientUplink, bvvd.PackLossProxyerUplink, bvvd.PackLossProxyerDownlink:
			var pl PL
			if err := pl.Decode(from); err != nil {
				return err
			}
			m.Payload = pl
		case bvvd.PingForward:
			m.Payload = bvvd.Location(from.Bytes()[0])
		default:
		}
	}
	return nil
}
