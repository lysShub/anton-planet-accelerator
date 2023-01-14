package context

import (
	ctx1 "context"
	"time"
)

type normalCtx struct {
	ctx1.Context

	fatal func(err error)
}

func (c *normalCtx) Fatal(err error) {
	if c.fatal != nil {
		c.fatal(err)
	}
}

type CancelFunc = ctx1.CancelFunc

var Canceled = ctx1.Canceled
var DeadlineExceeded = ctx1.DeadlineExceeded

func WithCancel(parent Ctx) (ctx Ctx, cancel CancelFunc) {
	c := &normalCtx{}
	c.Context, cancel = ctx1.WithCancel(parent)
	c.fatal = parent.Fatal

	return c, cancel
}

func WithDeadline(parent Ctx, deadline time.Time) (ctx Ctx, cancel CancelFunc) {
	c := &normalCtx{}
	c.Context, cancel = ctx1.WithDeadline(parent, deadline)
	c.fatal = parent.Fatal

	return c, cancel
}

func WithTimeout(parent Ctx, deadline time.Duration) (ctx Ctx, cancel CancelFunc) {
	c := &normalCtx{}
	c.Context, cancel = ctx1.WithTimeout(parent, deadline)
	c.fatal = parent.Fatal

	return c, cancel
}

func Background() Ctx {
	return &normalCtx{Context: ctx1.Background()}
}

func TODO() Ctx {
	return &normalCtx{Context: ctx1.TODO()}
}

func WithValue(parent Ctx, key, val interface{}) Ctx {
	c := &normalCtx{}
	c.Context = ctx1.WithValue(parent, key, val)
	c.fatal = parent.Fatal

	return c
}
