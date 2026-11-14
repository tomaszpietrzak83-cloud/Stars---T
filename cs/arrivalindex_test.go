//go:build !wasi && !wasm

package cs

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestArrivalCellsPreserveExactReportOrderAndGeometry(t *testing.T) {
	rng := rand.New(rand.NewSource(1113))
	index := &routeThreatIndex{}
	for i := 0; i < 2000; i++ {
		index.threats = append(index.threats, indexedRouteThreat{
			next: Vector{rng.Intn(2048) - 1024, rng.Intn(2048) - 1024},
		})
	}
	// Include exact cell borders, negative coordinates and the radius boundary.
	for _, x := range []int{-65, -64, -63, -1, 0, 1, 63, 64, 65} {
		index.threats = append(index.threats, indexedRouteThreat{next: Vector{x, x}})
	}
	index.buildArrivalCells()
	for _, threat := range index.threats {
		for _, delta := range []Vector{{}, {3, 0}, {-3, 0}, {0, 3}, {0, -3}, {2, 2}, {3, 3}, {4, 0}} {
			to := threat.next.Add(delta)
			want, got := []int{}, []int{}
			for i, candidate := range index.threats {
				if candidate.threatensRoute(nil, to, false) {
					want = append(want, i)
				}
			}
			for _, i := range index.arrivalCells[arrivalCellAt(to)] {
				if index.threats[i].threatensRoute(nil, to, false) {
					got = append(got, i)
				}
			}
			require.Equal(t, want, got)
		}
	}
}

func BenchmarkArrivalThreatFilter(b *testing.B) {
	index := &routeThreatIndex{}
	for i := range 4000 {
		index.threats = append(index.threats, indexedRouteThreat{next: Vector{(i % 80) * 40, (i / 80) * 40}, power: 100})
	}
	index.buildArrivalCells()
	for _, indexed := range []bool{false, true} {
		name := "linear"
		if indexed {
			name = "indexed"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				to := index.threats[i%len(index.threats)].next
				total := 0
				if indexed {
					for _, n := range index.arrivalCells[arrivalCellAt(to)] {
						if index.threats[n].threatensRoute(nil, to, false) {
							total += index.threats[n].power
						}
					}
				} else {
					for _, threat := range index.threats {
						if threat.threatensRoute(nil, to, false) {
							total += threat.power
						}
					}
				}
				if total != 100 {
					b.Fatal(total)
				}
			}
		})
	}
}
