package fudp

import (
	"fmt"

	"github.com/tmthrgd/go-memset"
)

// TODO: 放弃DF, 放弃DF后idx亦可抛弃, 只保留groupId即可
// 基于:
//   如果2:1的fec, 有以下原始数据
//    [1 2 3 4 0 0 0 0 0 0 0 0 0 0]
//    [4 3 2 1 0 0 0 0 0 0 0 0 0 0]
//   则fec结果有:
//    [11 0 5 12 0 0 0 0 0 0 0 0 0 0]

type group struct {
	bs    []Upack
	ghash uint16
	count int
}

func newGroup() *group { return nil }

func (g *group) put(p Upack) bool {
	if g.ghash != p.GroupHash() {
		// 新group
		// g.reset()

	} else {
		i := p.GROUPIdx()
		g.grow(int(i))
		if len(g.bs[i]) < len(p) {
			g.bs[i] = make(Upack, len(p))
		}
		memset.Memset(g.bs[i], 0)
		n := copy(g.bs[i], p)
		g.bs[i] = g.bs[i][:n]

		g.count++
	}

	return false
}

func (g *group) reset(hash uint16) {
	if g.count+1 == len(g.bs) {
		if g.bs[len(g.bs)-1][0] != 0 {
			fmt.Println("可以恢复")
		} else {
			fmt.Println("parity丢弃")
		}
	}

	g.bs = g.bs[:0]
	g.ghash = hash
	g.count = 0
}

func (g *group) grow(n int) {
	if len(g.bs) > n {
		return
	}
	if cap(g.bs) > n {
		g.bs = g.bs[:n]
	}
	for len(g.bs) < n {
		// g.bs = append(g.bs, make(Upack, g.maxSize))
	}
}

type buff struct {
	n uint16
	b []group
}

func (b *buff) getG(ghash uint16) *group {
	return &b.b[ghash%b.n]
}

func (b *buff) Put(u Upack) {
	b.getG(u.GroupHash()).put(u)
}

// type buff struct {
// 	b map[uint16]*group
// }

// func (b *buff) gIdx(ghash uint16) *group {
// 	if g, ok := b.b[ghash]; ok {
// 		return g
// 	} else {
// 		g = newGroup()
// 		b.b[ghash] = g
// 		return g
// 	}
// }

// func (b *buff) Put(p Upack) bool {
// 	g := b.gIdx(p.GroupHash())
// 	return g.put(p)
// }

// func (b *buff) Get(d []byte) (n int, err error) {

// 	return
// }
