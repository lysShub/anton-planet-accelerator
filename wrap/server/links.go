package wrap

import (
	"log/slog"

	"github.com/lysShub/fatun/conn"
	"github.com/lysShub/fatun/links"
)

type Links struct {
	links.LinksManager
	logger *slog.Logger
}

func WrapLinks(mgr links.LinksManager, logger *slog.Logger) *Links {
	return &Links{LinksManager: mgr, logger: logger}
}

func (l *Links) Add(link links.Uplink, conn conn.Conn) (localPort uint16, err error) {
	for _, e := range l.Cleanup() {
		l.logger.Info("del link",
			slog.String("client", conn.RemoteAddr().String()),
			slog.String("link", e.String()))
	}

	port, err := l.LinksManager.Add(link, conn)
	if err != nil {
		return 0, err
	}

	l.logger.Info("add link",
		slog.String("client", conn.RemoteAddr().String()),
		slog.String("uplink", link.String()),
		slog.Int("port", int(port)))

	return port, err
}
