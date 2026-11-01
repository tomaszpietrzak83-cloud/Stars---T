package ai

import (
	"github.com/sirgwain/craig-stars/cs"
	"math"
	"slices"
	"sort"
)

// A turn-local KD tree. The active map remains authoritative as scouts claim
// destinations; removed planets are skipped without rebuilding the tree.
type planetIntelIndex struct {
	planet      *cs.Planet
	left, right *planetIntelIndex
	axis        int
}

func newPlanetIntelIndex(active map[int]*cs.Planet) *planetIntelIndex {
	points := make([]*cs.Planet, 0, len(active))
	for _, p := range active {
		if p != nil {
			points = append(points, p)
		}
	}
	var build func([]*cs.Planet, int) *planetIntelIndex
	build = func(points []*cs.Planet, axis int) *planetIntelIndex {
		if len(points) == 0 {
			return nil
		}
		sort.Slice(points, func(i, j int) bool {
			a, b := points[i].Position.X, points[j].Position.X
			if axis == 1 {
				a, b = points[i].Position.Y, points[j].Position.Y
			}
			if a == b {
				return points[i].Num < points[j].Num
			}
			return a < b
		})
		mid := len(points) / 2
		return &planetIntelIndex{points[mid], build(points[:mid], 1-axis), build(points[mid+1:], 1-axis), axis}
	}
	return build(slices.Clone(points), 0)
}

func (tree *planetIntelIndex) nearest(position cs.Vector, active map[int]*cs.Planet) *cs.Planet {
	var best *cs.Planet
	distance := math.MaxInt
	var visit func(*planetIntelIndex)
	visit = func(node *planetIntelIndex) {
		if node == nil {
			return
		}
		p := node.planet
		if active[p.Num] == p {
			d := position.DistanceSquaredTo(p.Position)
			if d < distance || d == distance && (best == nil || p.Num < best.Num) {
				best, distance = p, d
			}
		}
		delta := position.X - p.Position.X
		if node.axis == 1 {
			delta = position.Y - p.Position.Y
		}
		near, far := node.left, node.right
		if delta > 0 {
			near, far = far, near
		}
		visit(near)
		// Equality is intentional: a planet across the split may win the Num tie.
		if delta*delta <= distance {
			visit(far)
		}
	}
	visit(tree)
	return best
}
