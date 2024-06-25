package bvvd

import (
	"fmt"
	"math"

	"github.com/jftuga/geodist"
	"github.com/pkg/errors"
)

type Location struct {
	Loc geodist.Coord
	ID  LocID
}

var Locs = locations{
	{geodist.Coord{Lat: 55.769, Lon: 37.586}, Moscow.LocID()},
	{geodist.Coord{Lat: 50.103, Lon: 8.679}, Frankfurt.LocID()},
	{geodist.Coord{Lat: 35.699, Lon: 139.774}, Tokyo.LocID()},
	{geodist.Coord{Lat: 40.716, Lon: -74.017}, NewYork.LocID()},
}

// LocID
//
//	低4位表示forward地区
//	高4位表示同地区的forward的id, 本质由加入proxyer的顺序决定
//
// 所以最多15个forward地区,每个地区最多15个forward机器(location的0值为非法值, id的0值为广播值)
//
// 当一台forward的资源快耗尽时, 不应该回复新client的PingForward, 这样代理将自动分配给其他机器。
type LocID uint8

type location uint8

// LocID transport to LocID with ID=0
func (l location) LocID() LocID {
	return LocID(l)
}

const (
	_ location = iota
	Moscow
	Frankfurt
	Tokyo
	NewYork
	_end
)

// Overlap adjust is same location
func (l LocID) Overlap(loc LocID) bool {
	return l.Loc() == loc.Loc()
}

func (l LocID) Loc() location {
	return location(l & 0b1111)
}

func (l *LocID) SetLoc(loc location) {
	*l = (*l) | LocID(loc)
}

func (l LocID) ID() uint8 {
	return uint8(l) & 0b11110000
}

func (l *LocID) SetID(id uint8) error {
	if id > 0b1111 {
		return errors.Errorf("id %d is greater than 0b1111", id)
	}
	*l = (*l) | (LocID(id) << 4)
	return nil
}

func (l LocID) String() string {
	return fmt.Sprintf("%s:%d", l.Loc().String(), l.ID())
}

func (l LocID) Valid() bool {
	return l.Loc() > 0
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
