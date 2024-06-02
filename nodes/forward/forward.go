//go:build linux
// +build linux

package forward

import (
	"fmt"
	"net"
	"sync"

	"github.com/lysShub/anton-planet-accelerator/proto"
	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/packet"
	"github.com/pkg/errors"
)

type Config struct {
	MaxRecvBuffSize int
}

type Forward struct {
	config *Config

	conn *net.UDPConn

	linkMu sync.RWMutex
	links  map[proto.Header]*Raw

	closeErr errorx.CloseErr
}

func New(addr string, config *Config) (*Forward, error) {
	var f = &Forward{config: config, links: map[proto.Header]*Raw{}}

	laddr, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	f.conn, err = net.ListenUDP("udp4", laddr)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return f, nil
}

func (f *Forward) close(cause error) error {
	return f.closeErr.Close(func() (errs []error) {
		errs = append(errs, cause)
		if f.conn != nil {
			errs = append(errs, f.conn.Close())
		}

		f.linkMu.Lock()
		for _, e := range f.links {
			errs = append(errs, e.Close())
		}
		clear(f.links)
		f.linkMu.Unlock()
		return errs
	})
}

func (f *Forward) Serve() error {
	fmt.Println("启动", f.conn.LocalAddr().String())
	return f.recvService()
}

func (f *Forward) recvService() (err error) {
	var (
		pkt = packet.Make(f.config.MaxRecvBuffSize)
		hdr = proto.Header{}
	)

	for {
		n, paddr, err := f.conn.ReadFromUDPAddrPort(pkt.Sets(64, 0xffff).Bytes())
		if err != nil {
			return f.close(err)
		}
		pkt.SetData(n)

		if err := hdr.Decode(pkt); err != nil {
			fmt.Println("decode", err)
			continue
		} else if hdr.Kind != proto.Data {
			fmt.Println("其他操作")
		}

		f.linkMu.RLock()
		raw, has := f.links[hdr]
		f.linkMu.RUnlock()
		if !has {
			raw, err = NewRaw(hdr, paddr, pkt)
			if err != nil {
				return f.close(err)
			}

			fmt.Println("new conn", hdr.ID, hdr.Server)

			f.linkMu.Lock()
			f.links[hdr] = raw
			f.linkMu.Unlock()
			go f.sendService(raw)
		}

		if err = raw.Send(pkt); err != nil {
			f.deleteRaw(raw)
		}
	}
}

func (f *Forward) sendService(raw *Raw) (_ error) {
	var (
		pkt = packet.Make(f.config.MaxRecvBuffSize)
	)

	for {
		if err := raw.Recv(pkt.Sets(64, 0xffff)); err != nil {
			return f.deleteRaw(raw)
		}

		_, err := f.conn.WriteToUDPAddrPort(pkt.Bytes(), raw.Proxyer())
		if err != nil {
			return f.close(err)
		}
	}
}

func (f *Forward) deleteRaw(raw *Raw) error {
	fmt.Println("delect raw", raw.LocalAddr(), raw.RemoteAddrPort())

	f.linkMu.Lock()
	delete(f.links, raw.Header())
	f.linkMu.Unlock()
	return raw.Close()
}