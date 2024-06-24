package admin

import (
	"encoding/gob"
	"net/netip"
	"reflect"

	"github.com/jftuga/geodist"
	"github.com/lysShub/anton-planet-accelerator/proto"
)

type Message interface {
	Kind() Kind
}

//go:generate stringer -output message_gen.go -type=Kind
type Kind uint8

const (
	_ Kind = iota
	KindClientNew
	KindProxyerNew
	KindForwardNew
	KindClientRoute
	KindProxyAddForward
	KindForwardStop
)

func init() {
	register(ClientNew{})
	register(ClientRoute{})
}
func register(v any) {
	gob.RegisterName(reflect.TypeOf(v).Name(), v)
}

type ClientNew struct {
	User         string
	PasswordHash string

	Ok  bool
	Msg string
	ID  proto.ID
	Key [16]byte
}

func (ClientNew) Kind() Kind { return KindClientNew }

// 新增Proxyer请求
type ProxyerNew struct {
	Name          string
	Token         string // 私有算法,验证token，确保Proxer的合法性 todo: 用tls双向认证
	Addr          netip.AddrPort
	ProxyLocation geodist.Coord // 亲和属于此地区的的forward

	Ok  bool
	Msg string
}

func (ProxyerNew) Kind() Kind { return KindProxyerNew }

// 新增Forward请求
type ForwardNew struct {
	Name     string
	Token    string
	Addr     netip.AddrPort
	Location geodist.Coord

	Ok  bool
	Msg string
}

func (ForwardNew) Kind() Kind { return KindForwardNew }

type ClientRoute struct {
	Server netip.AddrPort

	Ok      bool
	Msg     string
	Proxyer netip.AddrPort
}

func (ClientRoute) Kind() Kind { return KindClientRoute }

// 为Proxyer新增Forward
type ProxyAddForward struct {
	Name     string
	Addr     netip.AddrPort
	Location geodist.Coord

	Ok  bool
	Msg string
}

// Forward 请求停止想其转发 (通常是端口资源耗尽)
type ForwardStop struct {
	Name string
	Addr netip.AddrPort
}
