package context_test

import (
	"errors"
	"testing"
	"time"
	"warthunder/context"
)

func Test_Fatal(t *testing.T) {

	ctx := context.Background()
	go t1(ctx)

	<-ctx.Done()
	t.Log(ctx.Err())
}

func t1(ctx context.Context) {
	ctx, fatal := context.WithFatal(context.Background())

	for i := 0; i < 3; i++ {
		time.Sleep(time.Second)
		select {
		case <-ctx.Done():
			return
		default:
		}
	}
	fatal(errors.New("fatal error"))
}
