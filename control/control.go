package control

import (
	"encoding/gob"
	"io"
	"time"

	"github.com/pkg/errors"
)

type Marshal struct {
	enc *gob.Encoder
	dec *gob.Decoder
}

func NewMarshal(tcp io.ReadWriter) *Marshal {
	return &Marshal{
		enc: gob.NewEncoder(tcp),
		dec: gob.NewDecoder(tcp),
	}
}

func (m *Marshal) Encode(msg Message) error {
	return errors.WithStack(m.enc.Encode(&msg))
}

func (m *Marshal) Decode(msg *Message) error {
	return errors.WithStack(m.dec.Decode(msg))
}

type Message interface {
	Kind() Kind
	Clone() Message
}

type Kind uint8

var enums []Message

func init() {
	gob.Register(Ping{})
	gob.Register(PL{})
	gob.Register(TransmitData{})
	gob.Register(ServerWarn{})
	gob.Register(ServerError{})
}

const (
	_ Kind = iota
	KindPing
	KindPL
	KindTransmitData
	KindServerWarn
	KindServerError
)

type Ping struct {
	Request  time.Time
	Response time.Time
}

func (p Ping) Kind() Kind     { return KindPing }
func (p Ping) Clone() Message { return Ping{Request: p.Request, Response: p.Response} }

type PL struct{ PL float32 }

func (p PL) Kind() Kind     { return KindPL }
func (p PL) Clone() Message { return PL{PL: p.PL} }

type TransmitData struct {
	Uplink   uint32
	Downlink uint32
}

func (t TransmitData) Kind() Kind     { return KindTransmitData }
func (t TransmitData) Clone() Message { return TransmitData{Uplink: t.Uplink} }

type ServerWarn struct{ Warn string }

func (s ServerWarn) Kind() Kind     { return KindServerWarn }
func (s ServerWarn) Clone() Message { return ServerWarn{Warn: s.Warn} }

type ServerError struct{ Error string }

func (s ServerError) Kind() Kind     { return KindServerError }
func (s ServerError) Clone() Message { return ServerError{Error: s.Error} }
