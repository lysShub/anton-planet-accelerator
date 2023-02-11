package fudp

import (
	"testing"

	"github.com/tmthrgd/go-memset"
)

func Test_A(t *testing.T) {

	a := do()

	t.Log(a)
}

func do() []byte {
	var a = make([]byte, 4)
	for i := range a {
		a[i] = byte(i)
	}

	defer func() {
		memset.Memset(a, 0)
	}()
	return a
}
