package nodes

import "time"

const (
	ProxyerNetwork = "udp4"
	ForwardNetwork = "udp4"
	PLScale        = 64
	Keepalive      = time.Second * 30
)
