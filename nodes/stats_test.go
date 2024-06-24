package nodes

import (
	"math"
	"math/rand"
	"sort"
	"testing"

	"github.com/lysShub/netkit/packet"
	"github.com/stretchr/testify/require"
)

func Test_NewLoopIds(t *testing.T) {
	t.Run("increase", func(t *testing.T) {
		var li = NewLoopIds(0xffff)

		for e := range 0xff {
			v := li.Expand(e)
			require.Equal(t, e, v)
		}
	})
	t.Run("small scale disorder", func(t *testing.T) {
		var li = NewLoopIds(0xffff)

		for e := range disorder(0xfff) {
			v := li.Expand(e)
			require.Equal(t, e, v)
		}
	})
	t.Run("increase and loopback", func(t *testing.T) {
		var li = NewLoopIds(0xff)

		for e := range 0xffff {
			v := li.Expand(int(uint8(e)))
			require.Equal(t, e, v)
		}
	})
	t.Run("small scale disorder and loopback", func(t *testing.T) {
		var li = NewLoopIds(0xff)

		for e := range disorder(0xffff) {
			v := li.Expand(int(uint8(e)))
			require.Equal(t, e, v)
		}
	})
	t.Run("not start 0", func(t *testing.T) {
		var li = NewLoopIds(0xff)

		ids := disorder(0xffff)[13:]
		for e := range ids {
			v := li.Expand(int(uint8(e)))
			require.Equal(t, e, v)
		}
	})
}

func Test_PLStats(t *testing.T) {
	t.Run("base0", func(t *testing.T) {
		var pl = NewPLStats(0xff)
		require.Zero(t, pl.PL(0))
	})
	t.Run("base1", func(t *testing.T) {
		var pl = NewPLStats(0xff)
		pl.ID(0)
		require.Zero(t, pl.PL(0))
	})
	t.Run("base2", func(t *testing.T) {
		var pl = NewPLStats(0xff)
		for i := 0; i < 0xff; i++ {
			pl.ID(i)
		}
		require.Zero(t, pl.PL(0))
	})
	t.Run("base3", func(t *testing.T) {
		var pl = NewPLStats(0xff)
		for i := 11; i < 0xff; i++ {
			pl.ID(i)
		}
		require.Zero(t, pl.PL(0))
	})
	t.Run("base4", func(t *testing.T) {
		var pl = NewPLStats(0xff)
		for i := 0; i < 0xff; i++ {
			pl.ID(int(uint8(i)))
		}
		require.Zero(t, pl.PL(0))
	})
	t.Run("base5", func(t *testing.T) {
		var pl = NewPLStats(0xff)
		for i := 0; i < 0xff+1; i++ {
			pl.ID(int(uint8(i)))
		}
		require.Zero(t, pl.PL(0))
	})
	t.Run("base6", func(t *testing.T) {
		var pl = NewPLStats(0xff)
		for i := 0; i < 0xff+11; i++ {
			pl.ID(int(uint8(i)))
		}
		require.Zero(t, pl.PL(0))
	})
	t.Run("base7", func(t *testing.T) {
		var pl = NewPLStats(0xff)
		for i := 0; i < 0xffff; i++ {
			pl.ID(int(uint8(i)))
		}
		require.Zero(t, pl.PL(0))
	})

	t.Run("50%", func(t *testing.T) {
		var pl = NewPLStats(0xff)
		for i := 0; i < 0xff+11; i++ {
			if i%2 == 0 {
				continue
			}
			pl.ID(int(uint8(i)))
		}
		p := pl.PL(0)
		require.InDelta(t, 0.5, p, 0.01)
	})

	t.Run("1%", func(t *testing.T) {
		var pl = NewPLStats(0xff)
		for i := 0; i < 100; i++ {
			if i == 11 {
				continue
			}
			pl.ID(int(uint8(i)))
		}
		require.InDelta(t, 0.01, pl.PL(0), 0.009)
	})

	t.Run("disorder desc", func(t *testing.T) {
		var pl = NewPLStats(0xff)

		for _, e := range []int{8, 7, 6, 5, 4, 3, 2, 1} {
			pl.ID(e)
		}
		require.Zero(t, pl.PL(0))
	})

	t.Run("disorder base", func(t *testing.T) {
		var pl = NewPLStats(0xffff)

		ids := disorder(1024)
		for _, e := range ids {
			pl.ID(e)
		}
		require.Zero(t, pl.PL(0))
	})

	t.Run("disorder not start 0", func(t *testing.T) {
		var pl = NewPLStats(0xffff)

		ids := disorder(1024)[512:]
		for _, e := range ids {
			pl.ID(e)
		}

		sort.Ints(ids)
		delta := math.Abs(float64(512-ids[0]) / float64(512))

		require.InDelta(t, 0, pl.PL(0), delta)
	})

	t.Run("disorder loop", func(t *testing.T) {
		var pl = NewPLStats(0xff)

		ids := disorder(1024)
		for i, e := range ids {
			ids[i] = int(uint8(e))
		}

		for _, e := range ids {
			pl.ID(e)
		}

		require.Zero(t, pl.PL(0))
	})

	t.Run("disorder twice", func(t *testing.T) {
		var pl = NewPLStats(0xff)

		for _, e := range []int{8, 7, 6, 5, 4, 3, 2, 1} {
			pl.ID(e)
		}
		require.Zero(t, pl.PL(0))

		for _, e := range []int{9, 10, 12, 14, 15, 17} {
			pl.ID(e)
		}
		require.InDelta(t, 0.33, pl.PL(0), 0.01)
	})
}

func Test_PLStats2(t *testing.T) {

	var r = rand.New(rand.NewSource(0))

	t.Run("base", func(t *testing.T) {
		dup := NewPLStats2(0xfff)
		for i := range 0xfff {
			require.False(t, dup.ID(i))

			i2 := max(0, i-r.Intn(64))
			require.True(t, dup.ID(i2))
		}
	})

	t.Run("base-loopback", func(t *testing.T) {
		dup := NewPLStats2(0xff)
		for j := range 0xfff {
			id := int(uint8(j))

			require.False(t, dup.ID(id), j)

			i2 := max(0, id-r.Intn(16))
			require.True(t, dup.ID(i2))
		}
	})

	t.Run("disorder", func(t *testing.T) {
		dup := NewPLStats2(0xff)
		for e := range disorder(0xfff) {
			id := e % 0xff

			require.False(t, dup.ID(id))
		}
	})
}

func Test_PL(t *testing.T) {
	t.Run("base", func(t *testing.T) {
		var pkt = packet.Make()

		var pl PL
		require.Equal(t, "--.-", pl.String())
		require.NoError(t, pl.Encode(pkt))

		var pl2 PL
		require.NoError(t, pl2.Decode(pkt))
		require.Nil(t, pl2.Valid())
		require.Equal(t, "00.0", pl2.String())
	})

	t.Run("string", func(t *testing.T) {
		var pl PL = 0.009
		require.Equal(t, "01.0", pl.String())
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
