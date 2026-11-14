package cs

import "math"

// A single tanker evaluates many anchors against unchanged orders. Decode and
// classify those orders once, retaining their original summation order. Rebuild
// for the next tanker, whose predecessor may already have chosen a new anchor.
func (t *turnGenerator) tankerAnchorScorer(player *Player, tanker *Fleet) func(Vector) float64 {
	type colony struct {
		position Vector
		dock     bool
	}
	type corridor struct{ from, to Vector }
	colonies := make([]colony, 0)
	for _, planet := range t.game.getPlanets(player.Num) {
		colonies = append(colonies, colony{planet.Position, planet.Spec.DockCapacity != 0})
	}
	lanes := make([]corridor, 0)
	for _, fleet := range t.game.getFleets(player.Num) {
		if isAutomaticFleet(player, fleet) && player.Automation.FleetRoles[fleet.Num] != "tanker" && len(fleet.Waypoints) > 1 {
			for _, wp := range fleet.Waypoints[1:] {
				lanes = append(lanes, corridor{fleet.Position, wp.Position})
			}
		}
	}
	centers := make([]Vector, 0)
	for _, other := range t.game.getFleets(player.Num) {
		if other == tanker || other.Delete || player.Automation.FleetRoles[other.Num] != "tanker" {
			continue
		}
		center := other.Position
		if job := readTankerJob(other); job != nil && job.Stage == "patrol" {
			center = job.Anchor
		}
		centers = append(centers, center)
	}
	return func(point Vector) float64 {
		demand, nearestDock := 1.0, math.Inf(1)
		for _, planet := range colonies {
			distance := point.DistanceTo(planet.position)
			if planet.dock {
				nearestDock = min(nearestDock, distance)
			} else {
				demand += 4 / (1 + distance/80)
			}
		}
		for _, lane := range lanes {
			demand += 3 / (1 + segmentDistance(point, lane.from, lane.to)/60)
		}
		demand *= min(1.0, 0.1+nearestDock/120)
		for _, center := range centers {
			demand /= 1 + 6*math.Exp(-float64(point.DistanceSquaredTo(center))/float64(120*120))
		}
		return demand
	}
}
