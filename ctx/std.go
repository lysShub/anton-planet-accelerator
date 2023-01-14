package ctx

import (
	"context"
	"time"
)

type normalCtx struct {
	context.Context

	fatal func(err error)
}

func (c *normalCtx) Fatal(err error) {
	if c.fatal != nil {
		c.fatal(err)
	}
}

type CancelFunc = context.CancelFunc

var Canceled = context.Canceled
var DeadlineExceeded = context.DeadlineExceeded

func WithCancel(parent Ctx) (ctx Ctx, cancel CancelFunc) {
	c := &normalCtx{}
	c.Context, cancel = context.WithCancel(parent)
	c.fatal = parent.Fatal

	return c, cancel
}

func WithDeadline(parent Ctx, deadline time.Time) (ctx Ctx, cancel CancelFunc) {
	c := &normalCtx{}
	c.Context, cancel = context.WithDeadline(parent, deadline)
	c.fatal = parent.Fatal

	return c, cancel
}

// func WithTimeout(parent Ctx, deadline time.Duration) (ctx Ctx, oCtx context.Context, cancel CancelFunc) {
// 	c := &normalCtx{}
// 	oCtx, cancel = context.WithTimeout(parent, deadline)
// 	c.Context = oCtx
// 	c.fatal = parent.Fatal

// 	return c, oCtx, cancel
// }

func WithTimeout(parent Ctx, deadline time.Duration) (ctx Ctx, cancel CancelFunc) {
	c := &normalCtx{}
	c.Context, cancel = context.WithTimeout(parent, deadline)
	c.fatal = parent.Fatal

	return c, cancel
}

func Background() Ctx {
	return &normalCtx{Context: context.Background()}
}

func TODO() Ctx {
	return &normalCtx{Context: context.TODO()}
}

func WithValue(parent Ctx, key, val interface{}) Ctx {
	c := &normalCtx{}
	c.Context = context.WithValue(parent, key, val)
	c.fatal = parent.Fatal

	return c
}
