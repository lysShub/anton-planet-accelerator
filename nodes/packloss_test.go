package nodes_test

import (
	"testing"

	"github.com/lysShub/anton-planet-accelerator/nodes"
	"github.com/stretchr/testify/require"
)

func Test_PackLoss(t *testing.T) {
	t.Run("base0", func(t *testing.T) {
		var pl = &nodes.PLStats{}
		require.Zero(t, pl.PL())
	})
	t.Run("base1", func(t *testing.T) {
		var pl = &nodes.PLStats{}
		pl.ID(0)
		require.Zero(t, pl.PL())
	})
	t.Run("base2", func(t *testing.T) {
		var pl = &nodes.PLStats{}
		for i := 0; i < 0xff; i++ {
			pl.ID(i)
		}
		require.Zero(t, pl.PL())
	})
	t.Run("base3", func(t *testing.T) {
		var pl = &nodes.PLStats{}
		for i := 11; i < 0xff; i++ {
			pl.ID(i)
		}
		require.Zero(t, pl.PL())
	})

	t.Run("base4", func(t *testing.T) {
		var pl = &nodes.PLStats{}
		for i := 0; i < 0xff; i++ {
			pl.ID(int(uint8(i)))
		}
		require.Zero(t, pl.PL())
	})

	t.Run("base5", func(t *testing.T) {
		var pl = &nodes.PLStats{}
		for i := 0; i < 0xff+1; i++ {
			pl.ID(int(uint8(i)))
		}
		require.Zero(t, pl.PL())
	})

	t.Run("base6", func(t *testing.T) {
		var pl = &nodes.PLStats{}
		for i := 0; i < 0xff+11; i++ {
			pl.ID(int(uint8(i)))
		}
		require.Zero(t, pl.PL())
	})

	t.Run("base7", func(t *testing.T) {
		var pl = &nodes.PLStats{}
		for i := 0; i < 0xffff; i++ {
			pl.ID(int(uint8(i)))
		}
		require.Zero(t, pl.PL())
	})

	t.Run("50%", func(t *testing.T) {
		var pl = &nodes.PLStats{}
		for i := 0; i < 0xff+11; i++ {
			if i%2 == 0 {
				continue
			}
			pl.ID(int(uint8(i)))
		}
		require.InDelta(t, 0.5, pl.PL(), 0.1)
	})

	t.Run("1%", func(t *testing.T) {
		var pl = &nodes.PLStats{}
		for i := 0; i < 100; i++ {
			if i == 11 {
				continue
			}
			pl.ID(int(uint8(i)))
		}
		require.InDelta(t, 0.01, pl.PL(), 0.01)
	})

	t.Run("100%", func(t *testing.T) {
		var pl = &nodes.PLStats{}
		for i := 0; i < 1024; i++ {
			if i != 0 && i != 1023 {
				continue
			}
			pl.ID(int(uint8(i)))
		}
		require.InDelta(t, 1, pl.PL(), 0.01)
	})

	t.Run("disorder1", func(t *testing.T) {
		var pl = &nodes.PLStats{}
		for _, e := range []int{1, 2, 3, 6, 4, 5, 9, 7, 8, 10} {
			pl.ID(e)
		}
		require.Zero(t, pl.PL())
	})
	t.Run("disorder2", func(t *testing.T) {
		var pl = &nodes.PLStats{}
		for _, e := range []int{1, 3, 6, 4, 5, 9, 7, 8, 10} {
			pl.ID(e)
		}
		require.InDelta(t, 0.1, pl.PL(), 0.01)
	})
}
