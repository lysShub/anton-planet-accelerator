package heap

import (
	"sync"
	"sync/atomic"
	"time"
)

// todo: add close
type Heap[T any] struct {
	mu          sync.RWMutex
	rw          *sync.Cond
	s           []T
	start, size int
}

func NewHeap[T any](cap int) *Heap[T] {
	if cap <= 0 {
		panic("require greater than 0")
	}
	var h = &Heap[T]{
		s: make([]T, cap),
	}
	h.rw = sync.NewCond(&h.mu)
	return h
}

func (h *Heap[T]) Put(t T) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for !h.putLocked(t) {
		h.rw.Wait()
	}
}

func (h *Heap[T]) MustPut(t T) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for !h.putLocked(t) {
		h.popTailLocked()
	}
}

func (h *Heap[T]) putLocked(t T) bool {
	if len(h.s) == h.size {
		return false
	}

	i := (h.start + h.size)
	if i >= len(h.s) {
		i = i - len(h.s)
	}

	h.s[i] = t
	h.size += 1
	h.rw.Broadcast()
	return true
}

func (h *Heap[T]) PopTail() (val T) {
	h.mu.Lock()
	defer h.mu.Unlock()

	var ok bool
	for !ok {
		val, ok = h.popTailLocked()
		if !ok {
			h.rw.Wait()
		}
	}
	return val
}

func (h *Heap[T]) popTailLocked() (val T, ok bool) {
	if ok = h.size > 0; !ok {
		return
	}

	val = h.s[h.start]

	h.size -= 1
	h.start = (h.start + 1)
	if h.start >= len(h.s) {
		h.start = h.start - len(h.s)
	}

	h.rw.Broadcast()
	return val, true
}

func (h *Heap[T]) Pop(fn func(e T) (pop bool)) (val T) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.popByLocked(fn, nil)
}

func (h *Heap[T]) PopDeadline(fn func(e T) (pop bool), deadline time.Time) (val T, ok bool) {
	var dead atomic.Bool
	defer time.AfterFunc(time.Until(deadline), func() {
		dead.Store(true)
		h.rw.Broadcast()
	}).Stop()

	h.mu.Lock()
	defer h.mu.Unlock()
	return h.popByLocked(fn, &dead), !dead.Load()
}

func (h *Heap[T]) popByLocked(fn func(T) bool, dead *atomic.Bool) (val T) {
	i := h.syncRangeLocked(fn, dead)
	if i < 0 {
		if dead != nil {
			dead.Store(true)
		} else {
			panic("impossible")
		}
		return val
	} else {
		return h.del(i)
	}
}

func (h *Heap[T]) del(i int) T {
	val := h.s[i]
	copy(h.s[i:], h.s[i+1:])
	if h.start+h.size > len(h.s) && i >= h.start {
		h.s[len(h.s)-1] = h.s[0]
		copy(h.s[0:], h.s[1:])
	}
	h.size -= 1
	return val
}

func (h *Heap[T]) Size() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.size
}

func (h *Heap[T]) Range(fn func(e T) (stop bool)) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.syncRangeLocked(fn, nil)
}

func (h *Heap[T]) RangeDeadline(fn func(T) bool, deadline time.Time) (dead bool) {
	var d atomic.Bool
	defer time.AfterFunc(time.Until(deadline), func() {
		d.Store(true)
		h.rw.Broadcast()
	}).Stop()

	h.mu.Lock()
	defer h.mu.Unlock()
	return h.syncRangeLocked(fn, &d) < 0
}

func (h *Heap[T]) syncRangeLocked(fn func(T) bool, dead *atomic.Bool) (hitIdx int) {
	var i = -1
	for i < 0 && (dead == nil || !dead.Load()) {

		// todo: optimize, only visit last one(?) after first range
		i = h.rangeLocked(fn)
		if i < 0 {
			h.rw.Wait()
		}
	}
	return i
}

func (h *Heap[T]) rangeLocked(fn func(T) (stop bool)) (hitIdx int) {
	n := min(h.start+h.size, len(h.s))
	for i := h.start; i < n; i++ {
		if fn(h.s[i]) {
			return i
		}
	}
	if n == len(h.s) {
		n = h.start + h.size - n
		for i := 0; i < n; i++ {
			if fn(h.s[i]) {
				return i
			}
		}
	}
	return -1
}
