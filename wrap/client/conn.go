package wrap

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/lysShub/anton-planet-accelerator/wrap"
	"github.com/lysShub/fatun/conn"
	"github.com/lysShub/fatun/links"
	"github.com/lysShub/netkit/errorx"
	"github.com/pkg/errors"
)

// todo: 合并control和wrap

type Conn struct {
	conn.Conn
	config *Config

	marshal *wrap.Marshal
	// mux     *MessageMux

	closeErr errorx.CloseErr
}

type Config struct {
	Logger *slog.Logger
}

func WrapConn(conn conn.Conn, config *Config) (*Conn, error) {
	var c = &Conn{Conn: conn, config: config}

	tcp, err := conn.BuiltinConn(context.Background())
	if err != nil {
		return nil, c.close(err)
	}
	c.marshal = wrap.NewMarshal(tcp)
	// c.mux = NewMessageMux()

	go c.serve()
	return c, nil
}

func (c *Conn) close(cause error) error {
	if cause != nil {
		c.config.Logger.Error(cause.Error(), errorx.Trace(cause))
	} else {
		c.config.Logger.Info("client close", errorx.Trace(nil))
	}

	return c.closeErr.Close(func() (errs []error) {
		errs = append(errs, cause)
		errs = append(errs, c.Conn.Close())
		return errs
	})
}

func (c *Conn) serve() (_ error) {
	return
	var msg wrap.Message
	for {
		err := c.marshal.Decode(&msg)
		if err != nil {
			return c.close(errors.WithStack(err))
		}

		switch msg.Kind() {
		case wrap.KindServerError:
			return c.close(errors.Errorf("server error: %s", msg.(*wrap.ServerError).Error))
		case wrap.KindServerWarn:
			c.config.Logger.Warn(fmt.Sprintf("server warn %s", msg.(*wrap.ServerWarn).Warn))
		default:
			// c.mux.Put(msg.Clone())
		}
	}
}

func (c *Conn) Close() error { return c.close(nil) }

func (c *Conn) Ping() (time.Duration, error) {
	err := c.marshal.Encode(wrap.Ping{Request: time.Now()})
	if err != nil {
		return 0, err
	}

	start := time.Now()
	// _ = c.mux.PopBy(wrap.KindPing).(wrap.Ping)

	for {
		var msg wrap.Message
		err := c.marshal.Decode(&msg)
		if err != nil {
			return 0, c.close(errors.WithStack(err))
		}

		_, ok := msg.(wrap.Ping)
		if ok {
			break
		}
	}

	return time.Since(start), nil
}

func (c *Conn) PL() (float32, error) {
	return 0, nil
}

// message multiplexer
type MessageMux struct {
	mu         sync.RWMutex
	buff       *links.Heap[wrap.Message]
	putTrigger *sync.Cond
}

func NewMessageMux() *MessageMux {
	var m = &MessageMux{
		buff: links.NewHeap[wrap.Message](8),
	}
	m.putTrigger = sync.NewCond(&m.mu)
	return m
}

func (m *MessageMux) Pop() (msg wrap.Message) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.buff.Size() == 0 {
		m.putTrigger.Wait()
	}
	return m.buff.Pop()
}

func (m *MessageMux) PopBy(kind wrap.Kind) (msg wrap.Message) {
	for {
		msg = m.popBy(kind)
		if msg != nil {
			return msg
		}
	}
}

func (m *MessageMux) popBy(kind wrap.Kind) (msg wrap.Message) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.buff.Size() == 0 {
		m.putTrigger.Wait()
	}

	msg = m.buff.Pop()
	for msg != nil {
		if msg.Kind() == kind {
			return msg
		} else {
			m.buff.Put(msg)
		}

		msg = m.buff.Pop()
	}
	return nil
}

func (m *MessageMux) Put(msg wrap.Message) {
	if msg == nil {
		return
	}
	defer m.putTrigger.Broadcast()

	m.mu.Lock()
	defer m.mu.Unlock()
	m.buff.Put(msg)
}

func (m *MessageMux) Size() (size int) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.buff.Size()
}
