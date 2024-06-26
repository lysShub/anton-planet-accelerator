package forward

import (
	"log/slog"
	"os"
)

type Config struct {
	MaxRecvBuffSize int

	LogPath string
	logger  *slog.Logger
}

func (c *Config) init() *Config {
	var err error

	var fh *os.File
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
