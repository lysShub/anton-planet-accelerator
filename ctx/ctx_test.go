package ctx_test

import (
	"context"
	"errors"
	"testing"
	"time"
	"warthunder/ctx"
)

func Test_Ctx(t *testing.T) {

	c := ctx.WithFatal(context.Background())
	go subFunc(c)

	<-c.Done()

	t.Log(c.Err())

}

func subFunc(c ctx.Ctx) {
	context.WithTimeout(c, time.Second*2)
	time.Sleep(time.Second * 3)
	c.Fatal(errors.New("致命错误"))
}
