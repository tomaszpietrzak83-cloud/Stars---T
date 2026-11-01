package cs

import (
	"math"
	"runtime"
	"sync"
)

const scannerCell = 128

type scannerGrid struct {
	all    []scanner
	cells  map[Vector][]int
	global []int
}

func scanCell(x int) int {
	if x < 0 {
		return (x+1)/scannerCell - 1
	}
	return x / scannerCell
}
func newScannerGrid(scanners []scanner) *scannerGrid {
	g := &scannerGrid{all: scanners, cells: map[Vector][]int{}}
	for i, s := range scanners {
		if s.Range < 0 && s.RangePen < 0 {
			continue
		}
		r := max(s.Range, s.RangePen) + 1
		if r > 100000 || s.CloakReductionFactor > 2 || math.IsNaN(s.CloakReductionFactor) {
			g.global = append(g.global, i)
			continue
		}
		lo, hi := Vector{scanCell(s.Position.X - r), scanCell(s.Position.Y - r)}, Vector{scanCell(s.Position.X + r), scanCell(s.Position.Y + r)}
		if (hi.X-lo.X+1)*(hi.Y-lo.Y+1) > 4096 {
			g.global = append(g.global, i)
			continue
		}
		for x := lo.X; x <= hi.X; x++ {
			for y := lo.Y; y <= hi.Y; y++ {
				key := Vector{x, y}
				g.cells[key] = append(g.cells[key], i)
			}
		}
	}
	return g
}
func (g *scannerGrid) at(pos Vector) []int {
	local := g.cells[Vector{scanCell(pos.X), scanCell(pos.Y)}]
	if len(g.global) == 0 {
		return local
	}
	// Retain original scanner order: planet discovery can depend on the first match.
	out := make([]int, 0, len(local)+len(g.global))
	i, j := 0, 0
	for i < len(local) || j < len(g.global) {
		if j == len(g.global) || i < len(local) && local[i] < g.global[j] {
			out = append(out, local[i])
			i++
		} else {
			out = append(out, g.global[j])
			j++
		}
	}
	return out
}
func (g *scannerGrid) sees(scan *playerScanner, f *Fleet) bool {
	if f.Spec.CloakPercent < 0 || f.Spec.CloakPercent > 100 {
		for _, s := range g.all {
			if scan.fleetInScannerRange(f, s) {
				return true
			}
		}
		return false
	}
	for _, i := range g.at(f.Position) {
		if scan.fleetInScannerRange(f, g.all[i]) {
			return true
		}
	}
	return false
}

// Pure visibility is parallel; discovery, ally sharing and report order remain serial.
func (scan *playerScanner) fleetVisibility(scanners, cargo []scanner, workers int) []uint8 {
	a, b := newScannerGrid(scanners), newScannerGrid(cargo)
	fleets := scan.universe.Fleets
	flags := make([]uint8, len(fleets))
	if len(fleets) < 4096 {
		workers = 1
	}
	workers = max(1, min(workers, len(fleets)))
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		lo, hi := len(fleets)*worker/workers, len(fleets)*(worker+1)/workers
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := lo; i < hi; i++ {
				f := fleets[i]
				if f.Delete || f.OwnedBy(scan.player.Num) {
					continue
				}
				if a.sees(scan, f) {
					flags[i] |= 1
				}
				if b.sees(scan, f) {
					flags[i] |= 2
				}
			}
		}()
	}
	wg.Wait()
	return flags
}
func scannerWorkers() int { return runtime.GOMAXPROCS(0) }
