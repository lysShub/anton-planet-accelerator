package client

import (
	"log/slog"
	"os"
	"time"

	"github.com/lysShub/rawsock"
)

type Config struct {
	MaxRecvBuff int

	CertPath string

	DialTimeout time.Duration

	LogPath string
	logger  *slog.Logger

	RawConnOpts []rawsock.Option

	PcapBuiltinPath string
}

func (c *Config) init() *Config {
	if c.DialTimeout <= 0 {
		c.DialTimeout = time.Second * 5
	}

	var fh *os.File
	var err error
	if c.LogPath == "" {
		fh = os.Stdout
	} else {
		fh, err = os.OpenFile(c.LogPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o666)
		if err != nil {
			panic(err)
		}
	}
	c.logger = slog.New(slog.NewJSONHandler(fh, nil))

	return c
}
