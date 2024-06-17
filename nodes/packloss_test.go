package nodes_test

import (
	"math"
	"math/rand"
	"sort"
	"testing"

	"github.com/lysShub/anton-planet-accelerator/nodes"
	"github.com/stretchr/testify/require"
)

func Test_PLStats(t *testing.T) {
	t.Run("base0", func(t *testing.T) {
		var pl = nodes.NewPLStatsWithDimension(64)
		require.Zero(t, pl.PL())
	})
	t.Run("base1", func(t *testing.T) {
		var pl = nodes.NewPLStatsWithDimension(64)
		pl.ID(0)
		require.Zero(t, pl.PL())
	})
	t.Run("base2", func(t *testing.T) {
		var pl = nodes.NewPLStatsWithDimension(64)
		for i := 0; i < 0xff; i++ {
			pl.ID(i)
		}
		require.Zero(t, pl.PL())
	})
	t.Run("base3", func(t *testing.T) {
		var pl = nodes.NewPLStatsWithDimension(64)
		for i := 11; i < 0xff; i++ {
			pl.ID(i)
		}
		require.Zero(t, pl.PL())
	})
	t.Run("base4", func(t *testing.T) {
		var pl = nodes.NewPLStatsWithDimension(64)
		for i := 0; i < 0xff; i++ {
			pl.ID(int(uint8(i)))
		}
		require.Zero(t, pl.PL())
	})
	t.Run("base5", func(t *testing.T) {
		var pl = nodes.NewPLStatsWithDimension(64)
		for i := 0; i < 0xff+1; i++ {
			pl.ID(int(uint8(i)))
		}
		require.Zero(t, pl.PL())
	})
	t.Run("base6", func(t *testing.T) {
		var pl = nodes.NewPLStatsWithDimension(64)
		for i := 0; i < 0xff+11; i++ {
			pl.ID(int(uint8(i)))
		}
		require.Zero(t, pl.PL())
	})
	t.Run("base7", func(t *testing.T) {
		var pl = nodes.NewPLStatsWithDimension(64)
		for i := 0; i < 0xffff; i++ {
			pl.ID(int(uint8(i)))
		}
		require.Zero(t, pl.PL())
	})

	t.Run("50%", func(t *testing.T) {
		var pl = nodes.NewPLStatsWithDimension(64)
		for i := 0; i < 0xff+11; i++ {
			if i%2 == 0 {
				continue
			}
			pl.ID(int(uint8(i)))
		}
		require.InDelta(t, 0.5, pl.PL(), 0.1)
	})

	t.Run("1%", func(t *testing.T) {
		var pl = nodes.NewPLStatsWithDimension(64)
		for i := 0; i < 100; i++ {
			if i == 11 {
				continue
			}
			pl.ID(int(uint8(i)))
		}
		require.InDelta(t, 0.01, pl.PL(), 0.01)
	})

	t.Run("disorder0", func(t *testing.T) {
		var pl = nodes.NewPLStatsWithDimension(4)

		for _, e := range []int{8, 7, 6, 5, 4, 3, 2, 1} {
			pl.ID(e)
		}
		require.InDelta(t, 0, pl.PL(), 0.001)
	})

	t.Run("disorder1", func(t *testing.T) {
		var pl = nodes.NewPLStatsWithDimension(64)

		ids := disorder(1024)
		for _, e := range ids {
			pl.ID(e)
		}
		require.InDelta(t, 0, pl.PL(), 0.01)
	})

	t.Run("disorder2", func(t *testing.T) {
		var pl = nodes.NewPLStatsWithDimension(64)

		ids := disorder(1024)[512:]
		for _, e := range ids {
			pl.ID(e)
		}

		sort.Ints(ids)
		delta := math.Abs(float64(512-ids[0]) / float64(512))

		require.InDelta(t, 0, pl.PL(), delta*2)
	})

	t.Run("disorder3", func(t *testing.T) {
		var pl = nodes.NewPLStatsWithDimension(64)

		ids := disorder(1024)
		for i, e := range ids {
			ids[i] = int(uint8(e))
		}

		for _, e := range ids {
			pl.ID(e)
		}

		require.InDelta(t, 0, pl.PL(), 0.1)
	})
}

func disorder(size int) []int {
	var b = make([]int, 0, size)
	for e := range size {
		b = append(b, e)
	}

	var dimension = 32
	for i := range b {
		j := i + (rand.Int()%32 - (dimension / 2))
		if j >= 0 && j < size {
			b[i], b[j] = b[j], b[i]
			if dist(b[i], i) > dimension || dist(b[j], j) > dimension {
				b[i], b[j] = b[j], b[i]
			}
		}
	}
	return b
}

func dist(a, b int) int {
	if a > b {
		return a - b
	}
	return b - a
}
