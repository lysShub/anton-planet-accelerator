package proxy

import (
	"testing"
	"warthunder/fudp"
)

func Test_Ch(t *testing.T) {

	var ch = newCh(1)

	go func() {
		var u = fudp.NewUpack()
		for i := 0; i < 5; i++ {
			u.Data = append(u.Data, byte(i))
			ch.push(u)
		}
	}()

	var u = fudp.NewUpack()
	for i := 0; i < 5; i++ {
		ch.pope(u)
		if u.Data[0] != byte(i) {
			t.Fatal()
		}
	}
}
