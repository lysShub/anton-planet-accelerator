package bvvd

import (
	"slices"

	"github.com/jftuga/geodist"
)

//go:generate stringer -output location_gen.go -type=Location

type Location uint8

func (l Location) Valid() bool {
	return 0 < l && l < _end
}

func (l Location) Coord() geodist.Coord {
	if l.Valid() {
		return coords[int(l)]
	}
	panic(l.String())
}

func (l Location) Distance(coord geodist.Coord) float64 {
	_, d := geodist.HaversineDistance(l.Coord(), coord)
	return d
}

func (l Location) Offset(p Location) float64 {
	return l.Distance(p.Coord())
}

const (
	_ Location = iota
	Moscow
	Frankfurt
	Tokyo
	NewYork
	_end
)

var Locations = locations{
	Moscow, Frankfurt, Tokyo, NewYork,
}

var coords = []geodist.Coord{
	{},
	{Lat: 55.769, Lon: 37.586},
	{Lat: 50.103, Lon: 8.679},
	{Lat: 35.699, Lon: 139.774},
	{Lat: 40.716, Lon: -74.017},
	{},
}

type locations []Location

func (ls locations) SortByCoord(coord geodist.Coord) []Location {
	ls1 := slices.Clone(ls)

	slices.SortFunc(ls1, func(a, b Location) int {
		d1 := a.Distance(coord)
		d2 := b.Distance(coord)
		if d1 < d2 {
			return -1
		} else if d1 > d2 {
			return 1
		}
		return 0
	})
	return ls1
}

func (ls locations) Match(coord geodist.Coord) (Location, float64) {
	ls1 := ls.SortByCoord(coord)
	return ls1[0], ls1[0].Distance(coord)
}
