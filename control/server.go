package control

import (
	"net"
	"time"

	"github.com/lysShub/netkit/errorx"
	"github.com/pkg/errors"
)

type Server struct {
	marshal *Marshal
}

func NewServer(tcp net.Conn) *Server {
	return &Server{
		marshal: NewMarshal(tcp),
	}
}

func (c *Server) Serve() error {
	var msg Message
	for {
		err := c.marshal.Decode(&msg)
		if err != nil {
			return errors.WithStack(err)
		}

		switch msg.Kind() {
		case KindPing:
			err = c.marshal.Encode(&Ping{Request: time.Now()})
		case KindPL:
			err = c.marshal.Encode(&PL{})
		case KindTransmitData:
			err = c.marshal.Encode(&TransmitData{})

		// case KindServerWarn:
		// case KindServerError:
		default:
			err = errorx.WrapTemp(errors.Errorf("not support control message kind %d", msg.Kind()))
		}

		if err != nil {
			_ = c.ServerInfo(err)
			if !errorx.Temporary(err) {
				return err
			}
		}
	}
}

func (c *Server) ServerInfo(err error) error {
	if err == nil {
		return nil
	}

	if errorx.Temporary(err) {
		return c.marshal.Encode(&ServerWarn{Warn: err.Error()})
	} else {
		return c.marshal.Encode(&ServerError{Error: err.Error()})
	}
}
