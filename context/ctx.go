package context

import (
	ctx1 "context"
	"sync/atomic"
)

type Ctx interface {
	ctx1.Context
	Fatal(err error)
}

// fatalCtx is a context that can throw the exception.
type fatalCtx struct {
	ctx1.Context

	fataled *atomic.Bool
	fatal   error
	cancel  ctx1.CancelFunc
}

var _ ctx1.Context = &fatalCtx{}

func WithFatal(parent Ctx) Ctx {
	if parent == nil {
		panic("")
	}

	ctx, cancel := ctx1.WithCancel(parent)
	return &fatalCtx{
		Context: ctx,
		fataled: &atomic.Bool{},
		cancel:  cancel,
	}
}

func (c *fatalCtx) Fatal(err error) {
	if err == nil {
		return
	}

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
