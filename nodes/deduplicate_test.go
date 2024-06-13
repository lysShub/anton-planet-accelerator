package nodes

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Dedeuplicate(t *testing.T) {

	var r = rand.New(rand.NewSource(0))

	t.Run("base", func(t *testing.T) {
		dup := NewDeduplicate(0xfff)
		for i := range 0xfff {
			require.False(t, dup.Recved(i))

			i2 := max(0, i-r.Intn(64))
			require.True(t, dup.Recved(i2))
		}
	})

	t.Run("base-loopback", func(t *testing.T) {
		dup := NewDeduplicate(0xff)
		for j := range 0xfff {
			id := int(uint8(j))

			require.False(t, dup.Recved(id), j)

			i2 := max(0, id-r.Intn(16))
			require.True(t, dup.Recved(i2))
		}
	})

	t.Run("disorder", func(t *testing.T) {
		dup := NewDeduplicate(0xff)
		for e := range genDisorderIDs(0xfff) {
			id := e % 0xff

			require.False(t, dup.Recved(id))
		}
	})
}

func genDisorderIDs(n int) []int {
	var b = make([]int, 0, n)
	for e := range n {
		b = append(b, e)
	}

	const scale = 16

	for i := range b {
		j := min(i+rand.Intn(scale), len(b)-1)
		b[i], b[j] = b[j], b[i]

		if dist(b[i], i) > scale || dist(b[j], j) > scale {
			b[i], b[j] = b[j], b[i]
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
