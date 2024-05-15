package control_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/lysShub/anton-planet-accelerator/control"
	"github.com/lysShub/rawsock/test"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func Test_Marshal(t *testing.T) {
	m := control.NewMarshal(bytes.NewBuffer(nil))

	{
		err := m.Encode(control.Ping{Request: time.Unix(1, 1)})
		require.NoError(t, err)

		var msg control.Message
		err = m.Decode(&msg)
		require.NoError(t, err)
		p, ok := msg.(control.Ping)
		require.True(t, ok)
		require.Equal(t, time.Unix(1, 1), p.Request)
	}
	{
		err := m.Encode(control.PL{PL: 1})
		require.NoError(t, err)

		var msg control.Message
		err = m.Decode(&msg)
		require.NoError(t, err)
		p, ok := msg.(control.PL)
		require.True(t, ok)
		require.Equal(t, float32(1), p.PL)
	}
	{
		err := m.Encode(control.TransmitData{Uplink: 1})
		require.NoError(t, err)

		var msg control.Message
		err = m.Decode(&msg)
		require.NoError(t, err)
		p, ok := msg.(control.TransmitData)
		require.True(t, ok)
		require.Equal(t, uint32(1), p.Uplink)
	}
	{
		err := m.Encode(control.ServerWarn{Warn: "xx"})
		require.NoError(t, err)

		var msg control.Message
		err = m.Decode(&msg)
		require.NoError(t, err)
		p, ok := msg.(control.ServerWarn)
		require.True(t, ok)
		require.Equal(t, "xx", p.Warn)
	}
	{
		err := m.Encode(control.ServerError{Error: "xx"})
		require.NoError(t, err)

		var msg control.Message
		err = m.Decode(&msg)
		require.NoError(t, err)
		p, ok := msg.(control.ServerError)
		require.True(t, ok)
		require.Equal(t, "xx", p.Error)
	}
}

func Test_Control(t *testing.T) {
	t.Skip("todo")

	c, s := test.NewMockConn(t, nil, nil)
	eg, _ := errgroup.WithContext(context.Background())

	eg.Go(func() error {
		ctr := control.NewServer(s)

		err := ctr.Serve()
		require.NoError(t, err)

		return nil
	})
	eg.Go(func() error {
		time.Sleep(time.Second)

		ctr := control.NewClient(c)
		eg.Go(func() error {
			return ctr.Serve()
		})

		ping, err := ctr.Ping()
		require.NoError(t, err)
		require.Less(t, ping, time.Second)

		return nil
	})

	eg.Wait()
}
