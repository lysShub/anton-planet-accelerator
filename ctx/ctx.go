package ctx

import (
	"context"
	"sync/atomic"
)

type Ctx interface {
	context.Context
	Fatal(err error)
}

// fatalCtx is a context that can throw the exception.
type fatalCtx struct {
	context.Context

	fataled *atomic.Bool
	fatal   error
	cancel  context.CancelFunc
}

var _ context.Context = &fatalCtx{}

func WithFatal(parent context.Context) Ctx {
	if parent == nil {
		panic("")
	}

	ctx, cancel := context.WithCancel(parent)
	return &fatalCtx{
		Context: ctx,
		fataled: &atomic.Bool{},
		cancel:  cancel,
	}
}

func (c *fatalCtx) Fatal(err error) {
	if c.fataled.CompareAndSwap(false, true) {
		c.fatal = err
		c.cancel()
	}
}

func (c *fatalCtx) Err() error {
	if c.fataled.Load() {
		return c.fatal
	} else {
		return c.Context.Err()
	}
}
