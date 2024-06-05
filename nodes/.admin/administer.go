package admin

import (
	"encoding/gob"
	"log/slog"
	"net"

	"github.com/lysShub/netkit/errorx"
	"github.com/pkg/errors"
)

type Control struct {
	logger *slog.Logger

	l *net.TCPListener

	closeErr errorx.CloseErr
}

// todo: add tls
func New(clientListenAddr string) (*Control, error) {
	var c = &Control{}

	laddr, err := net.ResolveTCPAddr("tcp4", clientListenAddr)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	c.l, err = net.ListenTCP("tcp4", laddr)
	if err != nil {
		return nil, c.close(err)
	}

	return c, nil
}

func (c *Control) close(cause error) error {
	cause = errors.WithStack(cause)
	return c.closeErr.Close(func() (errs []error) {
		errs = append(errs, cause)
		if c.l != nil {
			errs = append(errs, errors.WithStack(c.l.Close()))
		}

		return errs
	})
}

func (c *Control) Serve() error {
	for {
		conn, err := c.l.AcceptTCP()
		if err != nil {
			return c.close(err)
		}

		go c.muxHandle(conn)
	}
}

func (c *Control) muxHandle(conn *net.TCPConn) (_ error) {
	var initMsg Message

	dec := gob.NewDecoder(conn)

	err := dec.Decode(&initMsg)
	if err != nil {
		c.logger.Error(err.Error(), slog.String("remote", conn.RemoteAddr().String()), errorx.Trace(nil))
		return conn.Close()
	}

	switch kind := initMsg.Kind(); kind {
	case KindClientNew:

	case KindProxyerNew:

	case KindForwardNew:
	default:
		c.logger.Warn("invalid init message kind", slog.String("kind", kind.String()))
	}
	return nil
}

func (c *Control) addClient(client ClientNew) error {
	c.logger.Info("add client", slog.String("user", client.User))

	return nil
}
