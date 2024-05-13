package server

type Heap[T any] struct {
	vals []T
	s, n int // start-idx, heap-size
}

func NewHeap[T any](initCap uint) *Heap[T] {
	return &Heap[T]{
		vals: make([]T, initCap),
	}
}

func (h *Heap[T]) Put(t T) {
	if len(h.vals) == h.n {
		h.grow()
	}

	i := (h.s + h.n)
	if i >= len(h.vals) {
		i = i - len(h.vals)
	}

	h.vals[i] = t
	h.n += 1

}

func (h *Heap[T]) Pop() (val T) {
	if h.n == 0 {
		return *new(T)
	}

	val = h.vals[h.s]

	h.n -= 1
	h.s = (h.s + 1)
	if h.s >= len(h.vals) {
		h.s = h.s - len(h.vals)
	}
	return val
}

func (h *Heap[T]) Peek() T {
	if h.n == 0 {
		return *new(T)
	}
	return h.vals[h.s]
}

func (h *Heap[T]) Size() int {
	return h.n
}
func (h *Heap[T]) grow() {
	tmp := make([]T, len(h.vals)*2)

	n1 := copy(tmp, h.vals[h.s:])
	copy(tmp[n1:h.n], h.vals[0:])

	h.vals = tmp
	h.s = 0
}
