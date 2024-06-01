package client

import (
	"context"
	"log/slog"
	"net"
	"time"

	accelerator "github.com/lysShub/anton-planet-accelerator"
	wrap "github.com/lysShub/anton-planet-accelerator/wrap/client"
	"github.com/lysShub/fatun"
	"github.com/lysShub/fatun/conn"
	"github.com/lysShub/fatun/conn/udp"
	"github.com/pkg/errors"
)

type Client struct {
	config *Config

	conn   *wrap.Conn
	client *fatun.Client
}

func New(server string, config *Config) (*Client, error) {
	var a = &Client{config: config.init()}

	raddr, err := net.ResolveUDPAddr("udp4", server)
	if err != nil {
		return nil, err
	}
	u, err := udp.Dial(nil, raddr)
	if err != nil {
		return nil, err
	}
	rawConn, err := conn.NewConn[conn.Default](u, &conn.Config{
		MaxRecvBuff:     config.MaxRecvBuff,
		PcapBuiltinPath: config.PcapBuiltinPath,
	})
	if err != nil {
		return nil, err
	}

	// handshake
	ctx, cancel := context.WithTimeout(context.Background(), config.DialTimeout)
	defer cancel()
	if _, err := rawConn.BuiltinConn(ctx); err != nil {
		return nil, err
	}
	a.config.logger.Info("connect", slog.String("server", rawConn.RemoteAddr().String()))

	a.conn, err = wrap.WrapConn(rawConn, a.config.Wrap())
	if err != nil {
		return nil, err
	}

	a.client, err = fatun.NewClient[conn.Default](func(c *fatun.Client) {
		c.MaxRecvBuff = config.MaxRecvBuff
		c.TcpMssDelta = -64
		c.Logger = a.config.logger
		c.Conn = a.conn
	})
	if err != nil {
		return nil, err
	}

	a.client.Run()
	return a, nil
}

func (c *Client) Run() error {
	if cap, ok := c.client.Capturer.(interface{ Enable(process string) }); !ok {
		return errors.Errorf("unexpect fatun Capture %T", c.client.Capturer)
	} else {
		cap.Enable(accelerator.Warthunder)
	}

	c.client.Run()
	return nil
}

func (c *Client) Close() error {
	println("client close!!!!!!!!!!!!!")
	return nil
}

func (c *Client) Ping() (time.Duration, error) { return c.conn.Ping() }
