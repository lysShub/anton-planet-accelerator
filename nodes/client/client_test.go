//go:build windows
// +build windows

package client

import (
	"fmt"
	"testing"
)

func TestXxxx(t *testing.T) {

	i, err := defaultAdapter()
	fmt.Println(i, err)
}
