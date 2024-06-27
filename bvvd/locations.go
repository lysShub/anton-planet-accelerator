package bvvd

import (
	"math"
	"slices"

	"github.com/jftuga/geodist"
)

type Location uint8

func (l Location) Valid() bool {
	return 0 < l && l < _end
}

func (l Location) Region() Region {
	if int(l)-1 < len(Regions) && Regions[l-1].Location == l {
		return Regions[l-1]
	}
	return Regions.Region(l)
}

type Region struct {
	Coord    geodist.Coord
	Location Location
}

var Regions = regions{
	{geodist.Coord{Lat: 55.769, Lon: 37.586}, Moscow},
	{geodist.Coord{Lat: 50.103, Lon: 8.679}, Frankfurt},
	{geodist.Coord{Lat: 35.699, Lon: 139.774}, Tokyo},
	{geodist.Coord{Lat: 40.716, Lon: -74.017}, NewYork},
}

const (
	_ Location = iota
	Moscow
	Frankfurt
	Tokyo
	NewYork
	_end
)

type regions []Region

func (ls regions) Match(dst geodist.Coord) (Region, float64) {
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

func (ls regions) Region(loc Location) Region {
	for _, e := range ls {
		if e.Location == loc {
			return e
		}
	}
	panic("invalid loction")
}

func (ls regions) SortByLoction(loc Location) (fs []Region) {
	idx := -1
	for i, e := range ls {
		if e.Location == loc {
			idx = i
			break
		}
	}
	if idx < 0 {
		panic("invalid loction")
	}

	coord := ls[idx].Coord
	fs = append(fs, ls...)

	slices.SortFunc(fs, func(a, b Region) int {
		_, d1 := geodist.HaversineDistance(a.Coord, coord)
		_, d2 := geodist.HaversineDistance(b.Coord, coord)
		if d1 < d2 {
			return -1
		} else if d1 > d2 {
			return 1
		}
		return 0
	})
	return fs
}
