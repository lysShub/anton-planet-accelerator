package proxy

import "testing"

func Test_Ch(t *testing.T) {

	var ch = newCh(1)

	go func() {
		var u = &upack{data: make([]byte, 65536)}
		for i := 0; i < 5; i++ {
			u.data[0] = byte(i)
			ch.push(u)
		}
	}()

	var u = &upack{data: make([]byte, 65536)}
	for i := 0; i < 5; i++ {
		ch.pope(u)
		if u.data[0] != byte(i) {
			t.Fatal()
		}
	}
}
