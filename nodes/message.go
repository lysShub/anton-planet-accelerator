package nodes

import (
	"encoding/binary"
	"net/netip"

	"github.com/lysShub/anton-planet-accelerator/bvvd"
	"github.com/lysShub/netkit/packet"
	"github.com/pkg/errors"
)

type Message struct {
	ID    uint32
	Kind  bvvd.Kind
	LocID bvvd.LocID
	Raw   any

	// other field
	Peer netip.AddrPort
}

func (m *Message) Encode(to *packet.Packet) (err error) {
	switch m.Kind {
	case bvvd.PingProxyer, bvvd.PackLossClientUplink, bvvd.PackLossProxyerDownlink:
	case bvvd.PingForward, bvvd.PackLossProxyerUplink:
		if !m.LocID.Valid() {
			return errors.Errorf("require location %s", m.LocID.String())
		}
	default:
		return errors.Errorf("unknown message kind %s", m.Kind.String())
	}
	if m.ID == 0 {
		return errors.Errorf("require message id")
	}

	if err = (&bvvd.Fields{
		Kind:   m.Kind,
		Proto:  0,
		DataID: 0,
		LocID:  m.LocID,
		Client: netip.AddrPortFrom(netip.IPv4Unspecified(), 0),
		Server: netip.IPv4Unspecified(),
	}).Encode(to); err != nil {
		return err
	}
	to.Append(binary.BigEndian.AppendUint32(make([]byte, 0, 4), m.ID)...)

	if m.Raw != nil {
		switch m.Kind {
		case bvvd.PackLossClientUplink, bvvd.PackLossProxyerUplink, bvvd.PackLossProxyerDownlink:
			pl, ok := m.Raw.(PL)
			if !ok {
				return errors.Errorf("invalid data type %T", m.Raw)
			}
			if err = pl.Encode(to); err != nil {
				return err
			}
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
	m.LocID = hdr.LocID
	switch m.Kind {
	case bvvd.PingProxyer, bvvd.PackLossClientUplink, bvvd.PackLossProxyerDownlink:
	case bvvd.PingForward, bvvd.PackLossProxyerUplink:
		if !m.LocID.Valid() {
			return errors.Errorf("require location %s", m.LocID.String())
		}
	default:
		return errors.Errorf("unknown message kind %s", m.Kind.String())
	}

	if from.Data() < 4 {
		return errors.Errorf("too small %d", from.Data())
	}
	m.ID = binary.BigEndian.Uint32(from.Detach(make([]byte, 4)))

	if from.Data() > 0 {
		switch m.Kind {
		case bvvd.PackLossClientUplink, bvvd.PackLossProxyerUplink, bvvd.PackLossProxyerDownlink:
			var pl PL
			if err := pl.Decode(from); err != nil {
				return err
			}
			m.Raw = pl
		default:
		}
	}
	return nil
}
