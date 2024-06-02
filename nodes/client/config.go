package client

import (
	"log/slog"
	"os"
)

type Config struct {
	MaxRecvBuff int
	TcpMssDelta int

	LogPath string
	logger  *slog.Logger

	PcapPath string
}

func (c *Config) init() *Config {
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
