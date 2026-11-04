package cs

import (
	"math"
	"slices"
)

const patrolMineSweepTag = "patrol-mine-sweep"

// Local interception has already had first choice. Idle capable patrols clear
// reported hostile fields affecting colonies or active routes, from the rim.
func (t *turnGenerator) patrolSweepObstacle(p *Player, f *Fleet) bool {
	delete(f.Tags, patrolMineSweepTag)
	dedicated := p.Automation.FleetRoles[f.Num] == "minesweeper"
	if f.Spec.MineSweep <= 0 {
		return false
	}
	// A sweep-capable ship physically surveys its current known field. This
	// prevents work on a cleared ghost field, or waiting outside its shrunken rim.
	fields := slices.Clone(p.MinefieldIntels)
	p.MinefieldIntels = slices.DeleteFunc(fields, func(field *Minefield) bool {
		if field == nil || field.PlayerNum == p.Num || p.IsFriend(field.PlayerNum) || f.Position.DistanceTo(field.Position) > field.Radius() {
			return false
		}
		actual := t.game.getMinefield(field.PlayerNum, field.Num)
		if actual == nil || actual.Delete || actual.NumMines <= 0 {
			return true
		}
		field.NumMines = actual.NumMines
		return false
	})
	var best *Minefield
	var route *expeditionRoute
	score := math.Inf(1)
	for _, field := range p.MinefieldIntels {
		if field == nil || field.Delete || field.NumMines <= 0 || field.Detonate || field.PlayerNum == p.Num || p.IsFriend(field.PlayerNum) || !f.willAttack(p, field.PlayerNum) {
			continue
		}
		distance := max(0, f.Position.DistanceTo(field.Position)-field.Radius())
		if distance > 100 && !dedicated {
			continue
		}
		useful := dedicated
		for _, colony := range p.PlanetIntels {
			if colony != nil && colony.OwnedBy(p.Num) && colony.Position.DistanceTo(field.Position) <= field.Radius()+50 {
				useful = true
				break
			}
		}
		if !useful {
			for _, ship := range t.game.getFleets(p.Num) {
				if len(ship.Waypoints) < 2 {
					continue
				}
				from := ship.Position
				for _, wp := range ship.Waypoints[1:] {
					if segmentDistance(field.Position, from, wp.Position) <= field.Radius() {
						useful = true
						break
					}
					from = wp.Position
				}
				if useful {
					break
				}
			}
		}
		if !useful {
			continue
		}
		safe := t.game.Rules.MinefieldStatsByType[field.MinefieldType].MaxSpeed + p.Race.Spec.MinefieldSafeWarpBonus
		point := f.Position
		if distance > 0 {
			point = field.Position.ToFloat64().Add(f.Position.Subtract(field.Position).Normalized().Scale(max(0, field.Radius()-2))).ToInt(true)
		} else if dedicated {
			// Sweeping only removes the outer annulus down to our distance
			// from the center. Advance inside the shrinking field to finish it.
			point = field.Position.ToFloat64().Add(f.Position.Subtract(field.Position).Normalized().Scale(max(0, f.Position.DistanceTo(field.Position)-f.StrategicSpeed(max(1, safe))))).ToInt(true)
		}
		candidate := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, point, MapObjectTarget{}, automaticRoutePolicy{AllowThreat: !dedicated, EconomicalPatrol: true, MaxWarp: min(6, max(1, safe)), MinWarp: 1, MaxTurns: 20})
		if dedicated && distance > 4 {
			candidate = t.minesweeperApproach(p, f, field, point, max(1, safe))
		}
		if candidate == nil || candidate.WaitForFuel {
			continue
		}
		value := float64(candidate.Turns)*1000 + distance
		if value < score {
			best, route, score = field, candidate, value
		}
	}
	if best == nil {
		return false
	}
	applyExpeditionRoute(p, f, route)
	status := "Clearing hostile minefield " + best.Name + " from its rim; return fuel reserved"
	f.SetTag(patrolMineSweepTag, status)
	if dedicated {
		f.SetTag(fuelStatusTag, status)
		return true
	}
	job := readPatrolJob(f)
	job.Status = status
	savePatrolJob(f, job)
	return true
}

// Cruise efficiently outside mines; cross the rim on a separate safe-warp leg.
// Both legs budget actual fuel and the final return, with no fictional top-up.
func (t *turnGenerator) minesweeperApproach(p *Player, f *Fleet, field *Minefield, inside Vector, safe int) *expeditionRoute {
	direction := f.Position.Subtract(field.Position).Normalized()
	outside := field.Position.ToFloat64().Add(direction.Scale(field.Radius() + 2)).ToInt(true)
	approach := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, outside, MapObjectTarget{}, automaticRoutePolicy{EconomicalPatrol: true, MaxTurns: 20})
	if approach == nil || approach.WaitForFuel {
		return nil
	}
	arrival := *f
	arrival.Position = outside
	arrival.Fuel = f.Fuel - approach.Fuel
	if len(approach.ArrivalFuel) > 0 {
		arrival.Fuel = approach.ArrivalFuel[0]
	}
	entry := planAutomaticRoute(&t.game.Rules, p, []*Fleet{&arrival}, inside, MapObjectTarget{}, automaticRoutePolicy{EconomicalPatrol: true, MaxWarp: safe, MaxTurns: 1})
	if entry == nil || entry.WaitForFuel {
		return nil
	}
	return &expeditionRoute{Waypoints: append(slices.Clone(approach.Waypoints), entry.Waypoints[1:]...), Turns: approach.Turns + entry.Turns, Fuel: approach.Fuel + entry.Fuel, ArrivalFuel: entry.ArrivalFuel}
}
