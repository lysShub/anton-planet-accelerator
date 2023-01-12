package context

import (
	octx "context"
	"time"
)

type Context interface {
	octx.Context
}

type CancelFunc = octx.CancelFunc

func WithCancel(parent Context) (ctx Context, cancel CancelFunc) {
	return octx.WithCancel(octx.Background())
}

func WithDeadline(parent Context, deadline time.Time) (Context, CancelFunc) {
	return octx.WithDeadline(octx.Background(), deadline)
}

func WithTimeout(parent Context, timeout time.Duration) (Context, CancelFunc) {
	return octx.WithTimeout(octx.Background(), timeout)
}

func Background() Context {
	return octx.Background()
}

func TODO() Context {
	return octx.TODO()
}

func WithValue(parent Context, key, val interface{}) Context {
	return octx.WithValue(octx.Background(), key, val)
}
