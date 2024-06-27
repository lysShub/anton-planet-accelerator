package bvvd

import (
	"math"

	"github.com/jftuga/geodist"
)

type Forward struct {
	Coord    geodist.Coord
	Location Location
}

var Forwards = forwards{
	{geodist.Coord{Lat: 55.769, Lon: 37.586}, Moscow},
	// {geodist.Coord{Lat: 50.103, Lon: 8.679}, Frankfurt},
	// {geodist.Coord{Lat: 35.699, Lon: 139.774}, Tokyo},
	{geodist.Coord{Lat: 40.716, Lon: -74.017}, NewYork},
}

type Location uint8

func (l Location) Valid() bool {
	return 0 < l && l < _end
}

const (
	_ Location = iota
	Moscow
	Frankfurt
	Tokyo
	NewYork
	_end
)

type forwards []Forward

func (ls forwards) Match(dst geodist.Coord) (Forward, float64) {
	var dist float64 = math.MaxFloat64
	var idx int
	for i, e := range ls {
		_, d := geodist.HaversineDistance(e.Coord, dst)
		if d < dist {
			dist, idx = d, i
		}
	}
	return ls[idx], dist
}
