package cs

import (
	"math"
	"slices"
)

// Keep a Pareto frontier for the complete remaining expedition, including its
// return dock. Never spend fuel on an outbound leg in isolation.
func optimizeScoutItinerary(rules *Rules, p *Player, f *Fleet, m *scoutMission) bool {
	type state struct {
		fuel, years int
		warps       []int
	}
	labels := []state{{fuel: f.Fuel}}
	from := f.Position
	points := m.Plan[min(m.Leg, len(m.Plan)):]
	for _, num := range points {
		planet := p.GetPlanetIntel(num)
		if planet == nil {
			return false
		}
		distance := from.DistanceTo(planet.Position)
		next := []state{}
		for _, label := range labels {
			for warp := 1; warp <= automaticMaxWarp(p, f); warp++ {
				years := max(1, int(math.Ceil(distance/f.StrategicSpeed(warp))))
				if years > p.Automation.scoutReturnTurns() || warp < 4 && years > 2 || label.years+years > max(60, 3*p.Automation.scoutReturnTurns()) {
					continue
				}
				if !automaticFleetLegSafe(rules, p, f, from, planet.Position, warp, false) {
					continue
				}
				_, fuel, _, ok := automaticLegFuel(p, f, warp, distance, label.fuel, 0)
				if !ok {
					continue
				}
				candidate := state{fuel: fuel, years: label.years + years, warps: append(slices.Clone(label.warps), warp)}
				dominated := false
				for _, old := range next {
					if old.years <= candidate.years && old.fuel >= candidate.fuel {
						dominated = true
						break
					}
				}
				if dominated {
					continue
				}
				next = slices.DeleteFunc(next, func(old state) bool { return candidate.years <= old.years && candidate.fuel >= old.fuel })
				next = append(next, candidate)
			}
		}
		if len(next) == 0 {
			return false
		}
		labels = next
		from = planet.Position
	}
	if len(labels) == 0 {
		return false
	}
	best := labels[0]
	for _, label := range labels {
		if label.years < best.years || label.years == best.years && label.fuel > best.fuel {
			best = label
		}
	}
	m.Warps = best.warps
	return true
}
func scoutEconomicalLeg(rules *Rules, p *Player, f *Fleet, dest *Planet) *expeditionRoute {
	var best *expeditionRoute
	score := math.Inf(1)
	for warp := 1; warp <= automaticMaxWarp(p, f); warp++ {
		distance := f.Position.DistanceTo(dest.Position)
		years := max(1, int(math.Ceil(distance/f.StrategicSpeed(warp))))
		if years > p.Automation.scoutReturnTurns() || warp < 4 && years > 2 {
			continue
		}
		if !automaticFleetLegSafe(rules, p, f, f.Position, dest.Position, warp, false) {
			continue
		}
		_, left, burned, ok := automaticLegFuel(p, f, warp, distance, f.Fuel, 0)
		if !ok {
			continue
		}
		value := float64(burned) + float64(years)*float64(max(1, f.Spec.FuelCapacity))/60
		if value < score {
			score = value
			best = &expeditionRoute{Turns: years, Fuel: burned, ArrivalFuel: []int{left}, Waypoints: []Waypoint{{Position: f.Position}, {Position: dest.Position, MapObjectTarget: dest.ToTarget(), WarpSpeed: warp}}}
		}
	}
	return best
}
func fundedScoutWaypoints(rules *Rules, p *Player, f *Fleet) bool {
	if p.Automation.FleetRoles[f.Num] != "scout" || len(f.Waypoints) < 3 {
		return false
	}
	last := f.Waypoints[len(f.Waypoints)-1]
	dock := p.GetPlanetIntel(last.TargetNum)
	if dock == nil || !dock.OwnedBy(p.Num) || !f.CanFuel(p, dock) {
		return false
	}
	fuel := f.Fuel
	from := f.Position
	years := 0
	for _, wp := range f.Waypoints[1:] {
		if wp.WarpSpeed <= 0 || wp.WarpSpeed > automaticMaxWarp(p, f) || !automaticFleetLegSafe(rules, p, f, from, wp.Position, wp.WarpSpeed, false) {
			return false
		}
		distance := from.DistanceTo(wp.Position)
		years += max(1, int(math.Ceil(distance/f.StrategicSpeed(wp.WarpSpeed))))
		_, left, _, ok := automaticLegFuel(p, f, wp.WarpSpeed, distance, fuel, 0)
		if !ok {
			return false
		}
		fuel = left
		from = wp.Position
	}
	return years <= max(60, 3*p.Automation.scoutReturnTurns())
}
