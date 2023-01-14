package context_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/lysShub/warthunder/context"

	"github.com/stretchr/testify/require"
)

func Test_Ctx_Timeout(t *testing.T) {

	{ // test timeout
		c, _ := context.WithTimeout(context.Background(), time.Second)
		<-c.Done()
		require.Equal(t, c.Err(), context.DeadlineExceeded)
	}

	{ // test cancel
		c, cancel := context.WithTimeout(context.Background(), time.Second)
		go func() {
			time.Sleep(time.Millisecond * 10)
			cancel()
		}()
		<-c.Done()
		require.Equal(t, c.Err(), context.Canceled)
	}

	{ // will not cancel parent
		c1 := context.WithFatal(context.Background())
		c, _ := context.WithTimeout(c1, time.Second)
		<-c.Done()
		require.Equal(t, c.Err(), context.DeadlineExceeded)
		require.NoError(t, c1.Err())
		_, deaded := c1.Deadline()
		require.False(t, deaded)
	}

	{ // will cancel child
		c1, _ := context.WithTimeout(context.Background(), time.Second)
		c2, _ := context.WithTimeout(c1, time.Second*3)

		start := time.Now()
		<-c2.Done()

		require.Less(t, time.Since(start), time.Second*2)
		_, ok := c2.Deadline()
		require.True(t, ok)
	}

}

func Test_Ctx_Fatal(t *testing.T) {
	{
		c := context.WithFatal(context.Background())
		c.Fatal(errors.New("fatal"))

		<-c.Done()
		<-c.Done()
		require.Equal(t, c.Err(), errors.New("fatal"))
		_, ok := c.Deadline()
		require.False(t, ok)
	}

	{
		c1 := context.WithFatal(context.Background())
		c2 := context.WithFatal(c1)
		c2.Fatal(errors.New("fatal"))

		<-c2.Done()
		select {
		case <-c1.Done():
			t.FailNow()
		default:
		}
	}

	{
		c1 := context.WithFatal(context.Background())
		c2 := context.WithFatal(c1)
		c1.Fatal(errors.New("fatal"))

		<-c1.Done()
		<-c2.Done()
		require.Equal(t, c1.Err(), errors.New("fatal"))
		require.Equal(t, c2.Err(), context.Canceled)
	}

	{
		c, cancel := context.WithCancel(context.Background())
		c1 := context.WithFatal(c)
		cancel()

		<-c1.Done()
		require.Equal(t, c1.Err(), context.Canceled)
	}

	{
		c, _ := context.WithCancel(context.Background())
		c1 := context.WithFatal(c)
		c1.Fatal(errors.New("fatal"))

		<-c1.Done()
		require.Equal(t, c1.Err(), errors.New("fatal"))
	}

	{
		c := context.WithFatal(context.Background())
		c1, _ := context.WithCancel(c)
		c.Fatal(errors.New("fatal"))

		<-c1.Done()
		require.Equal(t, c1.Err(), context.Canceled)
	}

	{
		c := context.WithFatal(context.Background())
		c1, cancel := context.WithCancel(c)
		cancel()

		<-c1.Done()
		select {
		case <-c.Done():
			t.FailNow()
		default:
		}
	}

	{
		c := context.WithFatal(context.Background())

		start := time.Now()
		wg := &sync.WaitGroup{}
		wg.Add(3)
		go func(c context.Ctx) {
			defer wg.Done()

			c, _ = context.WithCancel(c)
			time.Sleep(time.Second)
			c.Fatal(errors.New("fatal"))
		}(c)

		go func(c context.Ctx) {
			defer wg.Done()

			c, _ = context.WithTimeout(c, time.Second*2)
			<-c.Done()
		}(c)

		go func(c context.Ctx) {
			defer wg.Done()

			_, cancel := context.WithTimeout(c, time.Millisecond*1500)
			cancel()
		}(c)

		wg.Wait()

		require.Less(t, time.Since(start), time.Millisecond*1100)
		require.Less(t, time.Millisecond*900, time.Since(start))
		require.Equal(t, c.Err(), errors.New("fatal"))
	}

}
