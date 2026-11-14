package cs

import "math"

// Optimistic travel only: no fuel, threats, mines, dock mass/range restrictions,
// or stock reservations. Any feasible journey must take at least this long.
// Ignoring gate restrictions is intentional: a bound must never exclude a
// route which the actual planner permits.
func maintenanceTravelLowerBound(p *Player, f *Fleet, destination Vector) int {
	return maintenanceTravelLowerBounder(p, f)(destination)
}

func maintenanceTravelLowerBounder(p *Player, f *Fleet) func(Vector) int {
	infinity := int(^uint(0) >> 2)
	step := f.StrategicSpeed(automaticMaxWarp(p, f))
	travel := func(a, b Vector) int {
		if a == b {
			return 0
		}
		if step <= 0 {
			return infinity
		}
		return max(0, int(math.Ceil((a.DistanceTo(b)-1e-9)/step)))
	}
	entry := infinity
	var gates []Vector
	if f.Spec.CanJump {
		entry = 0
	}
	for _, dock := range expeditionStations(p, f) {
		if !dock.Spec.HasStargate {
			continue
		}
		entry = min(entry, travel(f.Position, dock.Position))
		gates = append(gates, dock.Position)
	}
	return func(destination Vector) int {
		best, exit := travel(f.Position, destination), infinity
		for _, position := range gates {
			exit = min(exit, travel(position, destination))
		}
		if entry < infinity && exit < infinity {
			best = min(best, entry+1+exit)
		}
		return best
	}
}
