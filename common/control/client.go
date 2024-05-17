package control

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/lysShub/fatcp/links"
	"github.com/pkg/errors"
)

type Client struct {
	marshal *Marshal
	mutx    *MessageMutx
}

func NewClient(tcp net.Conn) *Client {
	return &Client{
		marshal: NewMarshal(tcp),
		mutx:    NewMessageMutx(),
	}
}

func (c *Client) Serve() error {
	var msg Message
	for {
		err := c.marshal.Decode(&msg)
		if err != nil {
			return errors.WithStack(err)
		}

		switch msg.Kind() {
		case KindServerError:
			return errors.Errorf("server error: %s", msg.(*ServerError).Error)
		case KindServerWarn:
			fmt.Println("server warn: ", msg.(*ServerWarn).Warn)
		default:
			c.mutx.Put(msg.Clone())
		}
	}
}

func (c *Client) Ping() (time.Duration, error) {
	err := c.marshal.Encode(Ping{Request: time.Now()})
	if err != nil {
		return 0, err
	}

	start := time.Now()
	_ = c.mutx.PopBy(KindPing).(Ping)
	return time.Since(start), nil
}

type MessageMutx struct {
	mu         sync.RWMutex
	buff       *links.Heap[Message]
	putTrigger *sync.Cond
}

func NewMessageMutx() *MessageMutx {
	var m = &MessageMutx{
		buff: links.NewHeap[Message](8),
	}
	m.putTrigger = sync.NewCond(&m.mu)
	return m
}

func (m *MessageMutx) Pop() (msg Message) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.buff.Size() == 0 {
		m.putTrigger.Wait()
	}
	return m.buff.Pop()
}

func (m *MessageMutx) PopBy(kind Kind) (msg Message) {
	for {
		msg = m.popBy(kind)
		if msg != nil {
			return msg
		}
	}
}

func (m *MessageMutx) popBy(kind Kind) (msg Message) {
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

func (m *MessageMutx) Put(msg Message) {
	if msg == nil {
		return
	}
	defer m.putTrigger.Broadcast()

	m.mu.Lock()
	defer m.mu.Unlock()
	m.buff.Put(msg)
}

func (m *MessageMutx) Size() (size int) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.buff.Size()
}
