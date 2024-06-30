package client

import (
	"log/slog"
	"net/netip"
	"os"

	"github.com/lysShub/anton-planet-accelerator/bvvd"
)

type Config struct {
	Name string

	MaxRecvBuff int
	TcpMssDelta int

	LogPath string
	logger  *slog.Logger

	PcapPath string

	FixRoute bool
	Location bvvd.Location
	Gateways []netip.AddrPort
}

func (c *Config) init() *Config {
	if c.Name == "" {
		panic("require game name")
	}

	if c.MaxRecvBuff < 1500 {
		c.MaxRecvBuff = 1500
	}
	if c.TcpMssDelta >= 0 {
		c.TcpMssDelta = -64
	}

	var fh *os.File
	if c.LogPath == "" {
		fh = os.Stdout
	} else {
		var err error
		fh, err = os.OpenFile(c.LogPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o666)
		if err != nil {
			panic(err)
		}
	}
	c.logger = slog.New(slog.NewJSONHandler(fh, nil))

	if c.Location.Valid() != nil {
		panic("require location")
	}

	if len(c.Gateways) == 0 {
		panic("require gateways")
	}

	return c
}
