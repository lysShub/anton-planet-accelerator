package nodes

// Deduplicate deduplicate series id in [0, maxId], support id loopback, support
// small scale diorder
type Deduplicate struct {
	record []bool
	n, n2  int
	head   int // not include
}

func NewDeduplicate(maxId int) *Deduplicate {
	maxId += 1
	if maxId >= 128 {
		if maxId%2 != 0 {
			panic("not support")
		}
		maxId = maxId / 2
	}

	var d = &Deduplicate{
		record: make([]bool, maxId),
		n:      maxId, n2: maxId / 2,
	}

	return d
}

func (d *Deduplicate) Recved(id int) bool {
	id = id % d.n
	d.forward(id)

	defer func() { d.record[id] = true }()
	return d.record[id]
}

func (d *Deduplicate) forward(id int) {
	delta := d.forwardDelta(id)
	if delta > 0 {
		s1 := (d.head + d.n2) % d.n
		s2 := min(s1+delta, d.n)
		clear(d.record[s1:s2])
		if s2 == d.n {
			s3 := delta - (s2 - s1)
			clear(d.record[0:s3])
		}

		d.head = (d.head + delta) % d.n
	}
}

func (d *Deduplicate) forwardDelta(id int) (delta int) {
	if id >= d.head {
		delta = id - d.head + 1
		if delta < d.n2 {
			return delta
		} else {
			return 0
		}
	} else {
		delta = (d.n - d.head) + id + 1
		if delta < d.n2 {
			return delta
		} else {
			return 0
		}

		//  0 1 2 3 4 5

		// head 0 id 2 delta=2  next-head=3
		// head 4 id 1 delta=?  next-head=2
	}
}

// type Deduplicate struct {
// 	record []uint8
// 	n, n3  int
// 	head   int
// }

// func NewDeduplicate(offset int) *Deduplicate {
// 	var d = &Deduplicate{
// 		record: make([]uint8, offset*3),
// 		n:      offset * 3, n3: offset,
// 	}
// 	return d
// }

// func (d *Deduplicate) Recved(id int) bool {
// 	isHead := (id > d.head && id-d.head < d.n3) ||
// 		(id < d.n3 && d.head > d.n3*2) // id loopback

// 	i := id % d.n
// 	if isHead {
// 		d.head = id
// 		if d.record[i] == d.current {
// 			d.current++
// 		}
// 		d.record[i] = d.current
// 		return false
// 	} else {
// 		if d.record[i] == d.current {
// 			return true
// 		} else {
// 			d.record[i] = d.current
// 			return false
// 		}
// 	}
// }
