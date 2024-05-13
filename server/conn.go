package server

import (
	"sync/atomic"

	"github.com/lysShub/fatcp"
)

type Conn struct {
	*fatcp.Conn
	refs atomic.Int32
}

func (c *Conn) Inc()        { c.refs.Add(1) }
func (c *Conn) Dec()        { c.refs.Add(-1) }
func (c *Conn) Refs() int32 { return c.refs.Load() }
