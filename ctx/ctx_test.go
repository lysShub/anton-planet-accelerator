package ctx

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_Origin(t *testing.T) {

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(time.Second + time.Millisecond)
		cancel()
	}()
	ctx = WithException(ctx)
	start := time.Now()

	// sub function
	var wg = &sync.WaitGroup{}
	wg.Add(2)
	go func() {
		<-ctx.Done()
		require.True(t, time.Since(start) > time.Second)
		wg.Done()
	}()
	go func() {
		<-ctx.Done()
		require.True(t, time.Since(start) > time.Second)
		wg.Done()
	}()
	wg.Wait()
}

func Test_New(t *testing.T) {

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(time.Second * 2)
		cancel()
	}()
	ctx = WithException(ctx)
	start := time.Now()

	// sub function
	var wg = &sync.WaitGroup{}
	wg.Add(3)
	go func() {
		time.Sleep(time.Second + time.Millisecond)
		ctx.(Ctx).Exception(errors.New("sub function has error"))
		wg.Done()
	}()
	go func() {
		<-ctx.Done()
		require.True(t, time.Since(start) > time.Second)
		wg.Done()
	}()
	go func() {
		<-ctx.Done()
		require.True(t, time.Since(start) > time.Second)
		wg.Done()
	}()
	wg.Wait()
}
