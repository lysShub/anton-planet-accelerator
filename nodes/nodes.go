package nodes

import "time"

const (
	GatewayNetwork = "udp4"
	ForwardNetwork = "udp4"
	PLScale        = 64
	Keepalive      = time.Second * 30
)
