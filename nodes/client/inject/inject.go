package inject

import "gvisor.dev/gvisor/pkg/tcpip/header"

type Inject interface {
	Inject(ip header.IPv4) error
	Close() error
}
