//go:build linux
// +build linux

package proxyer

import (
	"fmt"
	"net"
	"net/netip"
	"sync"

	accelerator "github.com/lysShub/anton-planet-accelerator"
	"github.com/lysShub/anton-planet-accelerator/proto"
	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/packet"
	"github.com/pkg/errors"
)

type Proxyer struct {
	config *Config

	conn *net.UDPConn

	sender *net.UDPConn

	clientMu sync.RWMutex
	clients  map[proto.ID]netip.AddrPort // id:client

	route *Route

	closeErr errorx.CloseErr
}

func New(addr string, config *Config) (*Proxyer, error) {
	var p = &Proxyer{config: config, clients: map[proto.ID]netip.AddrPort{}}

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

	p.route = NewRoute(5000)
	return p, nil
}

func (p *Proxyer) close(cause error) error {
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
	_ = p.uplinkService()
	return p.close(nil)
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
		pkt.DetachN(proto.HeaderSize)

		p.clientMu.RLock()
		cli, has := p.clients[hdr.ID]
		p.clientMu.RUnlock()
		if !has {
			fmt.Println("client invalid id", hdr.ID)
		} else if !cli.IsValid() {
			fmt.Println("new client", caddr.String())

			p.clientMu.Lock()
			p.clients[hdr.ID] = caddr
			p.clientMu.Unlock()
		} else if hdr.Kind != proto.Data {
			fmt.Println("其他操作")
		}

		next, err := p.route.Next(hdr.Server)
		if err != nil {
			fmt.Println("route", err)
			continue
		}

		_, err = p.sender.WriteToUDPAddrPort(pkt.Bytes(), netip.AddrPortFrom(next, accelerator.DefatultPort))
		if err != nil {
			return p.close(err)
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
