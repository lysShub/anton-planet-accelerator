package proxyer_test

import (
	"fmt"
	"testing"

	"github.com/jftuga/geodist"
)

func TestXxx(t *testing.T) {

	var (
		a = geodist.Coord{Lat: 52.3667, Lon: 4.89454}
		b = geodist.Coord{Lat: 60.0098, Lon: 30.374}
	)

	_, dist, err := geodist.VincentyDistance(a, b)

	fmt.Println(dist, err)
}
