package heap_test

import (
	"testing"
	"time"

	"github.com/lysShub/anton-planet-accelerator/nodes/internal/heap"
	"github.com/stretchr/testify/require"
)

func Test_Heap(t *testing.T) {
	t.Run("put pop", func(t *testing.T) {
		h := heap.NewHeap[int](4)

		vals := []int{1, 2, 3, 4}
		for _, e := range vals {
			h.Put(e)
		}
		require.Equal(t, 4, h.Size())

		for _, e := range vals {
			require.Equal(t, e, h.Pop())
		}
		require.Zero(t, h.Size())
	})

	t.Run("put full block", func(t *testing.T) {
		h := heap.NewHeap[int](4)

		s := time.Now()
		go func() {
			time.Sleep(time.Second * 2)
			h.Pop()
		}()

		vals := []int{1, 2, 3, 4, 5}
		for _, e := range vals {
			h.Put(e)
		}
		require.Greater(t, time.Since(s), time.Second)
		require.Equal(t, h.Size(), 4)

		for i, e := range vals {
			if i == 0 {
				continue
			}
			require.Equal(t, e, h.Pop())
		}
		require.Zero(t, h.Size())
	})

	t.Run("pop empty block", func(t *testing.T) {
		h := heap.NewHeap[int](4)

		s := time.Now()
		go func() {
			time.Sleep(time.Second * 2)
			h.Put(1)
		}()

		val := h.Pop()
		require.Greater(t, time.Since(s), time.Second)
		require.Equal(t, 1, val)
	})

	t.Run("MustPut", func(t *testing.T) {
		h := heap.NewHeap[int](4)

		vals := []int{1, 2, 3, 4, 5, 6, 7}
		for _, e := range vals {
			h.MustPut(e)
		}
		require.Equal(t, 4, h.Size())

		for _, e := range vals[3:] {
			require.Equal(t, e, h.Pop())
		}
		require.Zero(t, h.Size())
	})

	t.Run("range", func(t *testing.T) {
		h := heap.NewHeap[int](4)

		vals := []int{1, 2, 3, 4, 5, 6, 7}
		for _, e := range vals {
			h.MustPut(e)
		}
		require.Equal(t, 4, h.Size())

		es := []int{}
		h.Range(func(e int) (stop bool) {
			es = append(es, e)
			return
		})
		require.Equal(t, 4, h.Size())

		for i, e := range es {
			require.Equal(t, vals[i+3], e)
		}
	})

	t.Run("PopBy", func(t *testing.T) {
		h := heap.NewHeap[int](4)

		vals := []int{1, 2, 3, 4, 5, 6, 7}
		for _, e := range vals {
			h.MustPut(e)
		}
		require.Equal(t, 4, h.Size())

		val := h.PopBy(func(e int) (pop bool) {
			return e == 5
		})
		require.Equal(t, 5, val)
		require.Equal(t, 3, h.Size())

		es := []int{}
		for h.Size() > 0 {
			es = append(es, h.Pop())
		}
		require.Equal(t, []int{4, 6, 7}, es)
	})

	t.Run("PopBy 2", func(t *testing.T) {
		h := heap.NewHeap[int](4)

		vals := []int{1, 2, 3, 4, 5, 6, 7}
		for _, e := range vals {
			h.MustPut(e)
		}
		require.Equal(t, 4, h.Size())

		val := h.PopBy(func(e int) (pop bool) {
			return e == 4
		})
		require.Equal(t, 4, val)
		require.Equal(t, 3, h.Size())

		es := []int{}
		for h.Size() > 0 {
			es = append(es, h.Pop())
		}
		require.Equal(t, []int{5, 6, 7}, es)
	})

	t.Run("PopBy block", func(t *testing.T) {
		h := heap.NewHeap[int](4)

		vals := []int{1, 2, 3}
		for _, e := range vals {
			h.MustPut(e)
		}
		require.Equal(t, 3, h.Size())

		go func() {
			time.Sleep(time.Second * 2)
			h.MustPut(4)
			h.MustPut(5)
		}()

		s := time.Now()
		val := h.PopBy(func(e int) (pop bool) {
			return e == 5
		})
		require.Equal(t, 5, val)
		require.Equal(t, 3, h.Size())
		require.Greater(t, time.Since(s), time.Second)
	})

	t.Run("PopByDeadline not dead", func(t *testing.T) {
		h := heap.NewHeap[int](4)

		vals := []int{1, 2, 3}
		for _, e := range vals {
			h.MustPut(e)
		}
		require.Equal(t, 3, h.Size())

		go func() {
			time.Sleep(time.Second * 2)
			h.MustPut(4)
			h.MustPut(5)
		}()

		s := time.Now()
		val, dead := h.PopByDeadline(func(e int) (pop bool) {
			return e == 5
		}, time.Now().Add(time.Minute))
		require.False(t, dead)
		require.Equal(t, 5, val)
		require.Equal(t, 3, h.Size())
		require.Greater(t, time.Since(s), time.Second)
	})

	t.Run("PopByDeadline dead", func(t *testing.T) {
		h := heap.NewHeap[int](4)

		vals := []int{1, 2, 3}
		for _, e := range vals {
			h.MustPut(e)
		}
		require.Equal(t, 3, h.Size())

		go func() {
			time.Sleep(time.Second * 2)
			h.MustPut(4)
			h.MustPut(5)
		}()

		s := time.Now()
		val, dead := h.PopByDeadline(func(e int) (pop bool) {
			return e == 5
		}, time.Now().Add(time.Second))
		require.True(t, dead)
		require.Equal(t, 0, val)
		require.Equal(t, 3, h.Size())
		require.Less(t, time.Since(s), time.Second*2)
	})

	t.Run("PopByDeadline expire", func(t *testing.T) {
		h := heap.NewHeap[int](4)

		vals := []int{1, 2, 3}
		for _, e := range vals {
			h.MustPut(e)
		}
		require.Equal(t, 3, h.Size())

		go func() {
			time.Sleep(time.Second * 2)
			h.MustPut(4)
			h.MustPut(5)
		}()

		s := time.Now()
		val, dead := h.PopByDeadline(func(e int) (pop bool) {
			return e == 5
		}, time.Now().Add(-time.Second))
		require.True(t, dead)
		require.Equal(t, 0, val)
		require.Equal(t, 3, h.Size())
		require.Less(t, time.Since(s), time.Millisecond*50)
	})
}
