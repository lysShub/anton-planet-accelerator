package context

import (
	"sync/atomic"
)

// 定义自己的ctx接口
// 有Fatal方法, 当调用Fatal方法时, 会将最近(向上)WithFatal之下的任务cancel掉, 并且返回错误

// fatalCtx is a context that can throw the exception.
type fatalCtx struct {
	Context

	fataled *atomic.Bool
	error   error
	cancel  CancelFunc
}

func (c *fatalCtx) fatal(err error) {
	if c.fataled.CompareAndSwap(false, true) {
		c.error = err
		c.cancel()
	}
}

type Fatal func(err error)

// WithFatal returns a new context that can throw the fatal error.
func WithFatal(parent Context) (ctx Context, fatal Fatal) {
	if parent == nil {
		parent = Background()
	}

	ctx, cancel := WithCancel(parent)
	c := &fatalCtx{
		Context: ctx,
		fataled: &atomic.Bool{},
		cancel:  cancel,
	}
	return c, c.fatal
}

func (c *fatalCtx) Err() error {
	if c.fataled.Load() {
		return c.error
	} else {
		return c.Context.Err()
	}
}
