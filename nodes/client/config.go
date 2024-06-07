package client

import (
	"log/slog"
	"net"
	"net/netip"
	"os"

	"github.com/pkg/errors"
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

// todo: optimzie
func defaultAdapter() (*net.Interface, error) {
	conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.IP{8, 8, 8, 8}, Port: 53})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer conn.Close()
	laddr := netip.MustParseAddrPort(conn.LocalAddr().String()).Addr().As4()

	ifs, err := net.Interfaces()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	for _, i := range ifs {
		if i.Flags&net.FlagRunning == 0 {
			continue
		}

		addrs, err := i.Addrs()
		if err != nil {
			return nil, errors.WithStack(err)
		}
		for _, addr := range addrs {
			if e, ok := addr.(*net.IPNet); ok && e.IP.To4() != nil {
				if [4]byte(e.IP.To4()) == laddr {
					return &i, nil
				}
			}
		}
	}

	return nil, errors.Errorf("not found default adapter")
}
