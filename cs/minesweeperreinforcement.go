package cs

import "math"

type sweepArrival struct {
	turn, capacity  int
	distance, speed float64
}

func newSweepArrival(rules *Rules, p *Player, field *Minefield, f *Fleet, route *expeditionRoute) sweepArrival {
	stats := rules.MinefieldStatsByType[field.MinefieldType]
	warp := minesweeperSafeWarp(rules, p, f, field)
	return sweepArrival{
		turn:     max(1, route.Turns),
		capacity: int(float64(f.Spec.MineSweep) * stats.SweepFactor),
		distance: route.Waypoints[len(route.Waypoints)-1].Position.DistanceTo(field.Position),
		speed:    f.StrategicSpeed(warp),
	}
}

// Forecast a shrinking field using the same depth limit as the sweep phase.
// The reported field is held fixed until the fleets arrive. New laying and
// natural decay are unknown, so the planner revisits this estimate next turn.
func sweepClearanceTurns(mines int, group []sweepArrival) int {
	const horizon = 40
	for turn := 1; turn <= horizon; turn++ {
		for _, arrival := range group {
			if turn < arrival.turn {
				continue
			}
			distance := max(0, arrival.distance-float64(turn-arrival.turn)*arrival.speed)
			mines -= min(arrival.capacity, max(0, mines-int(math.Ceil(distance*distance))))
		}
		if mines <= 10 {
			return turn
		}
	}
	return horizon + 1
}

func (t *turnGenerator) reinforceMinefield(p *Player, field *Minefield, group []sweepArrival, available []*Fleet, assigned map[int]bool) {
	for {
		bestTurns := sweepClearanceTurns(field.NumMines, group)
		if bestTurns <= 1 {
			return
		}
		var best *Fleet
		var bestRoute *expeditionRoute
		for _, f := range available {
			if assigned[f.Num] {
				continue
			}
			route := t.minesweeperFieldRoute(p, f, field)
			if route == nil || route.Turns >= bestTurns {
				continue
			}
			arrival := newSweepArrival(&t.game.Rules, p, field, f, route)
			turns := sweepClearanceTurns(field.NumMines, append(group, arrival))
			if turns < bestTurns {
				best, bestRoute, bestTurns = f, route, turns
			}
		}
		if best == nil {
			return
		}
		t.applyMinesweeperRoute(p, best, field, bestRoute)
		assigned[best.Num] = true
		group = append(group, newSweepArrival(&t.game.Rules, p, field, best, bestRoute))
	}
}
