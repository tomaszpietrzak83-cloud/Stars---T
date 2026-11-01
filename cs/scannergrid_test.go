//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"math/rand"
	"testing"
)

func TestScannerGridMatchesNativeVisibilityAndParallelOrder(t *testing.T) {
	rng := rand.New(rand.NewSource(72))
	sensors := []scanner{{Position: Vector{-128, 0}, Range: 0, RangePen: 0}}
	for i := 0; i < 75; i++ {
		sensors = append(sensors, scanner{Position: Vector{rng.Intn(4000) - 2000, rng.Intn(4000) - 2000}, Range: rng.Intn(900) - 1, RangePen: rng.Intn(500) - 1, CloakReductionFactor: rng.Float64()})
	}
	u := &Universe{}
	p := &Player{Num: 1}
	scan := playerScanner{universe: u, player: p}
	for i := 0; i < 5000; i++ {
		u.Fleets = append(u.Fleets, &Fleet{MapObject: MapObject{PlayerNum: 2, Position: Vector{rng.Intn(5000) - 2500, rng.Intn(5000) - 2500}}, Spec: FleetSpec{CloakPercent: rng.Intn(130)}, OrbitingPlanetNum: i % 2})
	}
	for _, global := range []bool{false, true} {
		if global {
			sensors = append(sensors, scanner{Range: 1000000, RangePen: NoScanner, CloakReductionFactor: 3})
		}
		expected := make([]uint8, len(u.Fleets))
		for i, f := range u.Fleets {
			for _, s := range sensors {
				if scan.fleetInScannerRange(f, s) {
					expected[i] |= 1
					break
				}
			}
			for _, s := range sensors[:20] {
				if scan.fleetInScannerRange(f, s) {
					expected[i] |= 2
					break
				}
			}
		}
		require.Equal(t, expected, scan.fleetVisibility(sensors, sensors[:20], 1))
		require.Equal(t, expected, scan.fleetVisibility(sensors, sensors[:20], 8))
		grid := newScannerGrid(sensors)
		for _, f := range u.Fleets {
			last := -1
			for _, i := range grid.at(f.Position) {
				require.Greater(t, i, last)
				last = i
			}
		}
	}
}
