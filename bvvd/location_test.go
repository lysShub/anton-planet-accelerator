package bvvd_test

import (
	"fmt"
	"testing"

	"github.com/lysShub/anton-planet-accelerator/bvvd"
)

func Test_Location(t *testing.T) {

	for _, loc := range bvvd.Locations {
		fmt.Println(loc.String())
	}

}
