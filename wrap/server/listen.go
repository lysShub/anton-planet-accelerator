package wrap

import (
	"context"
	"log/slog"
	"time"

	"github.com/lysShub/anton-planet-accelerator/wrap"
	"github.com/lysShub/fatun/conn"
	"github.com/lysShub/netkit/errorx"
	"github.com/pkg/errors"
)

/*
	包含功能：
		流量统计，实时延时，动态限流

*/

type Listener struct {
	conn.Listener
	config Config
}

func WrapListener(l conn.Listener, config Config) (*Listener, error) {
	return &Listener{Listener: l, config: config}, nil
}

func (l *Listener) Accept() (conn.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return newConn(conn, l.config)
}

type Conn struct {
	conn.Conn
	config Config

	marshal  *wrap.Marshal
	closeErr errorx.CloseErr
}

type Config struct {
	Logger *slog.Logger

	HandshakeTimeout time.Duration
}

func newConn(conn conn.Conn, config Config) (*Conn, error) {
	var c = &Conn{Conn: conn, config: config}

	c.config.Logger.Info("accpet connect", slog.String("client", conn.RemoteAddr().String()))
	go c.serve()
	return c, nil
}

func (c *Conn) close(cause error) error {
	return c.closeErr.Close(func() (errs []error) {
		errs = append(errs, cause)
		errs = append(errs, c.Conn.Close())

		if cause == nil {
			c.config.Logger.Info("connect close", slog.String("client", c.RemoteAddr().String()))
		} else {
			c.config.Logger.Error(cause.Error(), errorx.Trace(cause), slog.String("client", c.RemoteAddr().String()))
		}
		return errs
	})
}

func (c *Conn) serve() (_ error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.config.HandshakeTimeout)
	defer cancel()
	tcp, err := c.Conn.BuiltinConn(ctx)
	if err != nil {
		return c.close(err)
	}
	c.marshal = wrap.NewMarshal(tcp)

	for {
		var msg wrap.Message
		err := c.marshal.Decode(&msg)
		if err != nil {
			return errors.WithStack(err)
		}

		switch msg.Kind() {
		case wrap.KindPing:
			err = c.marshal.Encode(&wrap.Ping{Request: time.Now()})
		case wrap.KindPL:
			err = c.marshal.Encode(&wrap.PL{})
		case wrap.KindTransmitData:
			err = c.marshal.Encode(&wrap.TransmitData{})

		// case KindServerWarn:
		// case KindServerError:
		default:
			err = errorx.WrapTemp(errors.Errorf("not support control message kind %d", msg.Kind()))
		}

		if err != nil {
			if errorx.Temporary(err) {
				c.config.Logger.Warn(err.Error(), errorx.Trace(err))
			} else {
				return c.close(err)
			}
		}
	}
}
