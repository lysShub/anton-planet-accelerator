//go:build linux
// +build linux

package forward

import (
	"fmt"
	"net"
	"strconv"
	"sync"

	"github.com/lysShub/anton-planet-accelerator/proto"
	"github.com/lysShub/netkit/errorx"
	"github.com/lysShub/netkit/packet"
	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Config struct {
	MaxRecvBuffSize int
}

type Forward struct {
	config *Config

	conn *net.UDPConn

	linkMu sync.RWMutex
	links  map[link]*Raw

	closeErr errorx.CloseErr
}

type link struct {
	header      proto.Header
	processPort uint16
	serverPort  uint16
}

func New(addr string, config *Config) (*Forward, error) {
	var f = &Forward{config: config, links: map[link]*Raw{}}

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
		}

		switch hdr.Kind {
		case proto.PingForward:
			hdr.Encode(pkt.Sets(64, 0xffff))
			_, err := f.conn.WriteToUDPAddrPort(pkt.Bytes(), paddr)
			if err != nil {
				return f.close(err)
			}
		case proto.PacketLossForward:
			var pl float64 = 1.11 // todo:
			strPl := strconv.FormatFloat(pl, 'f', 3, 64)

			hdr.Encode(pkt.Sets(64, 0xffff))
			pkt.Append([]byte(strPl)...)
			_, err = f.conn.WriteToUDPAddrPort(pkt.Bytes(), paddr)
			if err != nil {
				return f.close(err)
			}
		case proto.Data:
			t := header.TCP(pkt.Bytes()) // only get port, tcp/udp is same
			link := link{header: hdr, processPort: t.SourcePort(), serverPort: t.DestinationPort()}

			f.linkMu.RLock()
			raw, has := f.links[link]
			f.linkMu.RUnlock()
			if !has {
				raw, err = NewRaw(link, paddr)
				if err != nil {
					return f.close(err)
				}

				fmt.Println("new conn", hdr.ID, hdr.Server)

				f.linkMu.Lock()
				f.links[link] = raw
				f.linkMu.Unlock()
				go f.sendService(raw)
			}

			if err = raw.Send(pkt); err != nil {
				f.deleteRaw(raw)
			}
		default:
			fmt.Println("无效的 kind", hdr.Kind)
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
	delete(f.links, raw.Link())
	f.linkMu.Unlock()
	return raw.Close()
}
