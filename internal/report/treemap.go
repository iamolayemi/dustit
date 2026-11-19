// Package report renders a self-contained HTML report of a scan.
package report

import "sort"

// Rect is a placed treemap tile.
type Rect struct {
	X, Y, W, H float64
	Index      int // index into the input sizes
}

// Squarify lays sizes out in the rectangle (x, y, w, h) so tiles stay close to square
// (Bruls, Huizing & van Wijk's squarified treemap).
func Squarify(sizes []int64, x, y, w, h float64) []Rect {
	type item struct {
		v   float64
		idx int
	}
	var items []item
	var total float64
	for i, s := range sizes {
		if s > 0 {
			items = append(items, item{float64(s), i})
			total += float64(s)
		}
	}
	if total == 0 || w <= 0 || h <= 0 {
		return nil
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].v > items[j].v })
	scale := w * h / total
	for i := range items {
		items[i].v *= scale // areas
	}

	var out []Rect
	worst := func(row []item, side float64) float64 {
		var sum, mx, mn float64
		mn = 1e308
		for _, it := range row {
			sum += it.v
			mx = max(mx, it.v)
			mn = min(mn, it.v)
		}
		s2 := side * side
		return max(s2*mx/(sum*sum), (sum*sum)/(s2*mn))
	}
	layout := func(row []item) {
		var sum float64
		for _, it := range row {
			sum += it.v
		}
		if w >= h { // column on the left
			cw := sum / h
			cy := y
			for _, it := range row {
				ih := it.v / cw
				out = append(out, Rect{x, cy, cw, ih, it.idx})
				cy += ih
			}
			x += cw
			w -= cw
		} else { // row on top
			rh := sum / w
			cx := x
			for _, it := range row {
				iw := it.v / rh
				out = append(out, Rect{cx, y, iw, rh, it.idx})
				cx += iw
			}
			y += rh
			h -= rh
		}
	}

	var row []item
	for len(items) > 0 {
		side := min(w, h)
		next := append(append([]item(nil), row...), items[0])
		if len(row) == 0 || worst(next, side) <= worst(row, side) {
			row = next
			items = items[1:]
			continue
		}
		layout(row)
		row = nil
	}
	if len(row) > 0 {
		layout(row)
	}
	return out
}
