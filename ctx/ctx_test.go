package ctx_test

import (
	"errors"
	"sync"
	"testing"
	"time"
	"warthunder/ctx"

	"github.com/stretchr/testify/require"
)

func Test_Ctx_Timeout(t *testing.T) {

	{ // test timeout
		c, _ := ctx.WithTimeout(ctx.Background(), time.Second)
		<-c.Done()
		require.Equal(t, c.Err(), ctx.DeadlineExceeded)
	}

	{ // test cancel
		c, cancel := ctx.WithTimeout(ctx.Background(), time.Second)
		go func() {
			time.Sleep(time.Millisecond * 10)
			cancel()
		}()
		<-c.Done()
		require.Equal(t, c.Err(), ctx.Canceled)
	}

	{ // will not cancel parent
		c1 := ctx.WithFatal(ctx.Background())
		c, _ := ctx.WithTimeout(c1, time.Second)
		<-c.Done()
		require.Equal(t, c.Err(), ctx.DeadlineExceeded)
		require.NoError(t, c1.Err())
		_, deaded := c1.Deadline()
		require.False(t, deaded)
	}

	{ // will cancel child
		c1, _ := ctx.WithTimeout(ctx.Background(), time.Second)
		c2, _ := ctx.WithTimeout(c1, time.Second*3)

		start := time.Now()
		<-c2.Done()

		require.Less(t, time.Since(start), time.Second*2)
		_, ok := c2.Deadline()
		require.True(t, ok)
	}

}

func Test_Ctx_Fatal(t *testing.T) {
	{
		c := ctx.WithFatal(ctx.Background())
		c.Fatal(errors.New("fatal"))

		<-c.Done()
		<-c.Done()
		require.Equal(t, c.Err(), errors.New("fatal"))
		_, ok := c.Deadline()
		require.False(t, ok)
	}

	{
		c1 := ctx.WithFatal(ctx.Background())
		c2 := ctx.WithFatal(c1)
		c2.Fatal(errors.New("fatal"))

		<-c2.Done()
		select {
		case <-c1.Done():
			t.FailNow()
		default:
		}
	}

	{
		c1 := ctx.WithFatal(ctx.Background())
		c2 := ctx.WithFatal(c1)
		c1.Fatal(errors.New("fatal"))

		<-c1.Done()
		<-c2.Done()
		require.Equal(t, c1.Err(), errors.New("fatal"))
		require.Equal(t, c2.Err(), ctx.Canceled)
	}

	{
		c, cancel := ctx.WithCancel(ctx.Background())
		c1 := ctx.WithFatal(c)
		cancel()

		<-c1.Done()
		require.Equal(t, c1.Err(), ctx.Canceled)
	}

	{
		c, _ := ctx.WithCancel(ctx.Background())
		c1 := ctx.WithFatal(c)
		c1.Fatal(errors.New("fatal"))

		<-c1.Done()
		require.Equal(t, c1.Err(), errors.New("fatal"))
	}

	{
		c := ctx.WithFatal(ctx.Background())
		c1, _ := ctx.WithCancel(c)
		c.Fatal(errors.New("fatal"))

		<-c1.Done()
		require.Equal(t, c1.Err(), ctx.Canceled)
	}

	{
		c := ctx.WithFatal(ctx.Background())
		c1, cancel := ctx.WithCancel(c)
		cancel()

		<-c1.Done()
		select {
		case <-c.Done():
			t.FailNow()
		default:
		}
	}

	{
		c := ctx.WithFatal(ctx.Background())

		start := time.Now()
		wg := &sync.WaitGroup{}
		wg.Add(3)
		go func(c ctx.Ctx) {
			defer wg.Done()

			c, _ = ctx.WithCancel(c)
			time.Sleep(time.Second)
			c.Fatal(errors.New("fatal"))
		}(c)

		go func(c ctx.Ctx) {
			defer wg.Done()

			c, _ = ctx.WithTimeout(c, time.Second*2)
			<-c.Done()
		}(c)

		go func(c ctx.Ctx) {
			defer wg.Done()

			_, cancel := ctx.WithTimeout(c, time.Millisecond*1500)
			cancel()
		}(c)

		wg.Wait()

		require.Less(t, time.Since(start), time.Millisecond*1100)
		require.Less(t, time.Millisecond*900, time.Since(start))
		require.Equal(t, c.Err(), errors.New("fatal"))
	}

}
