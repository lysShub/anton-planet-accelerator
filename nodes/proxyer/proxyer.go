//go:build linux
// +build linux

package proxyer

import (
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"sync"

	"github.com/lysShub/anton-planet-accelerator/proto"
	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/packet"
	"github.com/pkg/errors"
)

type Proxyer struct {
	config  *Config
	forward netip.AddrPort

	conn *net.UDPConn

	sender *net.UDPConn

	clientMu sync.RWMutex
	clients  map[proto.ID]netip.AddrPort // id:client

	closeErr errorx.CloseErr
}

func New(addr string, forward netip.AddrPort, config *Config) (*Proxyer, error) {
	var p = &Proxyer{
		config:  config,
		forward: forward,
		clients: map[proto.ID]netip.AddrPort{},
	}

	laddr, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	p.conn, err = net.ListenUDP("udp4", laddr)
	if err != nil {
		return nil, p.close(err)
	}

	p.sender, err = net.ListenUDP("udp4", nil)
	if err != nil {
		return nil, p.close(err)
	}

	return p, nil
}

func (p *Proxyer) close(cause error) error {
	cause = errors.WithStack(cause)
	return p.closeErr.Close(func() (errs []error) {
		errs = append(errs, cause)
		if p.conn != nil {
			errs = append(errs, p.conn.Close())
		}
		if p.sender != nil {
			errs = append(errs, p.sender.Close())
		}
		return errs
	})
}

func (p *Proxyer) Serve() error {
	go p.donwlinkService()
	return p.close(p.uplinkService())
}

func (p *Proxyer) AddClient(id proto.ID /* key [16]byte */) {
	p.clientMu.Lock()
	defer p.clientMu.Unlock()
	p.clients[id] = netip.AddrPort{}
}

func (p *Proxyer) uplinkService() (_ error) {
	var (
		pkt = packet.Make(p.config.MaxRecvBuff)
		hdr = &proto.Header{}
	)

	for {
		n, caddr, err := p.conn.ReadFromUDPAddrPort(pkt.Sets(64, 0xffff).Bytes())
		if err != nil {
			return p.close(err)
		}
		pkt.SetData(n)

		if err := hdr.Decode(pkt); err != nil {
			fmt.Println("client decode", err)
			continue
		}
		pkt.AttachN(proto.HeaderSize)

		p.clientMu.RLock()
		cli, has := p.clients[hdr.ID]
		p.clientMu.RUnlock()
		if !has {
			fmt.Println("无效的id", hdr)
			continue
		} else if !cli.IsValid() {
			fmt.Println("new client", caddr.String())

			p.clientMu.Lock()
			p.clients[hdr.ID] = caddr
			p.clientMu.Unlock()
		}

		switch hdr.Kind {
		case proto.PingProxyer:
			_, err = p.conn.WriteToUDPAddrPort(pkt.Bytes(), caddr)
			if err != nil {
				return p.close(err)
			}
		case proto.PacketLossProxyer:
			var pl float64 = 1.11 // todo:

			strPl := strconv.FormatFloat(pl, 'f', 3, 64)
			pkt.Append([]byte(strPl)...)
			_, err = p.conn.WriteToUDPAddrPort(pkt.Bytes(), caddr)
			if err != nil {
				return p.close(err)
			}
		default:
			if hdr.Server.IsPrivate() || hdr.Server.IsLoopback() || hdr.Server.IsUnspecified() || hdr.Server.IsMulticast() {
				// ignore special address
				continue
			}

			_, err = p.sender.WriteToUDPAddrPort(pkt.Bytes(), p.forward)
			if err != nil {
				return p.close(err)
			}
		}
	}
}

func (p *Proxyer) donwlinkService() (_ error) {
	var (
		pkt = packet.Make(p.config.MaxRecvBuff)
		hdr = &proto.Header{}
	)

	for {
		n, _, err := p.sender.ReadFromUDPAddrPort(pkt.Sets(64, 0xffff).Bytes())
		if err != nil {
			return p.close(err)
		}
		pkt.SetData(n)

		if err := hdr.Decode(pkt); err != nil {
			fmt.Println("forward decode", hdr.ID)
		}
		pkt.AttachN(proto.HeaderSize)

		p.clientMu.Lock()
		caddr, has := p.clients[hdr.ID]
		p.clientMu.Unlock()
		if !has {
			fmt.Println("forward invalid id", hdr.ID)
		}

		_, err = p.conn.WriteToUDPAddrPort(pkt.Bytes(), caddr)
		if err != nil {
			return p.close(err)
		}
	}
}
