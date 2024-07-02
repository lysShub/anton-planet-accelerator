package bvvd

import (
	"slices"

	"github.com/jftuga/geodist"
	"github.com/lysShub/netkit/packet"
	"github.com/pkg/errors"
)

//go:generate stringer -output location_gen.go -type=Location

type Location uint8

func (l Location) Valid() error {
	if !l.valid() {
		return errors.Errorf("invalid location %d", l)
	}
	return nil
}

func (l Location) valid() bool {
	return 0 < l && l < _end
}

func (l Location) Coord() geodist.Coord {
	if l.valid() {
		return infos[int(l)].coord
	}
	panic(l.String())
}

func (l Location) Hans() string {
	if l.valid() {
		return infos[int(l)].hans
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

func (l Location) Encode(to *packet.Packet) error {
	to.Append(byte(l))
	return nil
}

func (l *Location) Decode(from *packet.Packet) error {
	if from.Data() < 1 {
		return errors.New("too small")
	}

	*l = Location(from.Detach(1)[0])
	return l.Valid()
}

const (
	_ Location = iota
	Moscow
	Frankfurt
	Tokyo
	NewYork
	LosAngeles
	_end
)

var infos = []struct {
	coord geodist.Coord
	hans  string
}{
	{},
	{coord: geodist.Coord{Lat: 55.769, Lon: 37.586}, hans: "莫斯科"},
	{coord: geodist.Coord{Lat: 50.103, Lon: 8.679}, hans: "法兰克福"},
	{coord: geodist.Coord{Lat: 35.699, Lon: 139.774}, hans: "东京"},
	{coord: geodist.Coord{Lat: 40.716, Lon: -74.017}, hans: "纽约"},
	{coord: geodist.Coord{Lat: 34.07, Lon: -118.25}, hans: "洛杉矶"},
	{},
}

var Locations = locations{
	Moscow, Frankfurt, Tokyo, NewYork, LosAngeles,
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
