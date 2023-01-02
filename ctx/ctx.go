package ctx

import (
	"context"
	"sync/atomic"
)

type Ctx interface {
	context.Context
	Exception(err error)
}

// 常规的Ctx的作用是, 在Ctx树中、父Ctx节点通过上下文控制子Ctx节点。
// 此Ctx相对常规, 增加了子Ctx异常抛出功能, 父Ctx接收到抛出的异常后, 主动结束任务。

// exceptCtx is a context that can throw the exception.
type exceptCtx struct {
	context.Context

	excepted *atomic.Bool
	except   error
	cancel   context.CancelFunc
}

func WithException(parent context.Context) Ctx {
	if parent == nil {
		panic("")
	}

	ctx, cancel := context.WithCancel(parent)
	return &exceptCtx{
		Context:  ctx,
		excepted: &atomic.Bool{},
		cancel:   cancel,
	}
}

func (c *exceptCtx) Exception(err error) {
	if c.excepted.CompareAndSwap(false, true) {
		c.except = err
		c.cancel()
	}
}

func (c *exceptCtx) Err() error {
	if c.excepted.Load() {
		return c.except
	} else {
		return c.Context.Err()
	}
}
