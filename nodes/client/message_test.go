package client_test

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestXxxxx(t *testing.T) {

	var mu sync.RWMutex
	var rw = sync.NewCond(&mu)

	go func() {
		time.Sleep(time.Second)
		rw.Broadcast()
	}()

	mu.RLock()
	rw.Wait()
	mu.RUnlock()

	fmt.Println("pass")

}
