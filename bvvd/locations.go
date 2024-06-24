package bvvd

import (
	"math"

	"github.com/jftuga/geodist"
)

type Location struct {
	Loc geodist.Coord
	ID  LocID
}

var Locs = locations{
	{geodist.Coord{Lat: 55.769, Lon: 37.586}, Moscow},
	{geodist.Coord{Lat: 50.103, Lon: 8.679}, Frankfurt},
	{geodist.Coord{Lat: 35.699, Lon: 139.774}, Tokyo},
	{geodist.Coord{Lat: 40.716, Lon: -74.017}, NewYork},
}

type LocID uint8

const (
	_ LocID = iota
	Moscow
	Frankfurt
	Tokyo
	NewYork
	_end
)

func (l LocID) Valid() bool {
	return 0 < l && l < _end
}

type locations []Location

func (ls locations) Match(dst geodist.Coord) (Location, float64) {
	var dist float64 = math.MaxFloat64
	var idx int
	for i, e := range ls {
		_, d := geodist.HaversineDistance(e.Loc, dst)
		if d < dist {
			dist = d
			idx = i
		}
	}
	return ls[idx], dist
}

// func init() {
// 	global = newLoc(Locs)
// }

// var global *loc

// type loc struct{}

// func newLoc(locs []Loc) *loc {
// 	return &loc{}
// }
