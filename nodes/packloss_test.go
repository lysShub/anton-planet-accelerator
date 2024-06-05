package nodes_test

import (
	"testing"

	"github.com/lysShub/anton-planet-accelerator/nodes"
	"github.com/stretchr/testify/require"
)

func Test_PackLoss(t *testing.T) {
	t.Run("base1", func(t *testing.T) {
		var pl = &nodes.PLStats{}
		for i := 0; i < 0xff; i++ {
			pl.Pack(i)
		}
		require.Zero(t, pl.PL())
	})

	t.Run("base2", func(t *testing.T) {
		var pl = &nodes.PLStats{}
		for i := 11; i < 0xff; i++ {
			pl.Pack(i)
		}
		require.Zero(t, pl.PL())
	})

	t.Run("base3", func(t *testing.T) {
		var pl = &nodes.PLStats{}
		for i := 0; i < 0xff; i++ {
			pl.Pack(int(uint8(i)))
		}
		require.Zero(t, pl.PL())
	})

	t.Run("base4", func(t *testing.T) {
		var pl = &nodes.PLStats{}
		for i := 0; i < 0xff+1; i++ {
			pl.Pack(int(uint8(i)))
		}
		require.Zero(t, pl.PL())
	})

	t.Run("base5", func(t *testing.T) {
		var pl = &nodes.PLStats{}
		for i := 0; i < 0xff+11; i++ {
			pl.Pack(int(uint8(i)))
		}
		require.Zero(t, pl.PL())
	})

	t.Run("base6", func(t *testing.T) {
		var pl = &nodes.PLStats{}
		for i := 0; i < 0xffff; i++ {
			pl.Pack(int(uint8(i)))
		}
		require.Zero(t, pl.PL())
	})

	t.Run("50%", func(t *testing.T) {
		var pl = &nodes.PLStats{}
		for i := 0; i < 0xff+11; i++ {
			if i%2 == 0 {
				continue
			}
			pl.Pack(int(uint8(i)))
		}
		require.InDelta(t, 0.5, pl.PL(), 0.1)
	})

	t.Run("1%", func(t *testing.T) {
		var pl = &nodes.PLStats{}
		for i := 0; i < 100; i++ {
			if i == 11 {
				continue
			}
			pl.Pack(int(uint8(i)))
		}
		require.InDelta(t, 0.01, pl.PL(), 0.01)
	})
}
