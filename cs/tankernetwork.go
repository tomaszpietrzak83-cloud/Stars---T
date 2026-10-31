package cs

import (
	"fmt"
	"math"
	"slices"
)

// The network covers owned colonies and actual friendly traffic. Docks are
// permanent nodes; mobile nodes fill gaps rather than duplicating dock coverage.
func (t *turnGenerator) tankerNetworkAnchors(player *Player) []Vector {
	set := map[Vector]bool{}
	add := func(position Vector) {
		if t.game.Area.X > 0 {
			position.X = Clamp(position.X, 0, t.game.Area.X)
			position.Y = Clamp(position.Y, 0, t.game.Area.Y)
		}
		set[position] = true
	}
	for _, planet := range t.game.getPlanets(player.Num) {
		add(planet.Position)
		if planet.Spec.DockCapacity != 0 {
			for _, offset := range []Vector{{96, 0}, {-96, 0}, {0, 96}, {0, -96}, {68, 68}, {-68, 68}, {68, -68}, {-68, -68}} {
				add(planet.Position.Add(offset))
			}
		}
	}
	for _, fleet := range t.game.getFleets(player.Num) {
		if !isAutomaticFleet(player, fleet) || player.Automation.FleetRoles[fleet.Num] == "tanker" {
			continue
		}
		add(fleet.Position)
		for _, wp := range fleet.Waypoints[min(1, len(fleet.Waypoints)):] {
			add(wp.Position)
			add(Vector{(wp.Position.X + fleet.Position.X) / 2, (wp.Position.Y + fleet.Position.Y) / 2})
		}
	}
	result := make([]Vector, 0, len(set))
	for point := range set {
		result = append(result, point)
	}
	slices.SortFunc(result, func(a, b Vector) int {
		if a.X != b.X {
			return a.X - b.X
		}
		return a.Y - b.Y
	})
	return result
}

func (t *turnGenerator) tankerAnchorValue(player *Player, tanker *Fleet, point Vector) float64 {
	demand := 1.0
	nearestDock := math.Inf(1)
	for _, planet := range t.game.getPlanets(player.Num) {
		distance := point.DistanceTo(planet.Position)
		if planet.Spec.DockCapacity != 0 {
			nearestDock = min(nearestDock, distance)
		} else {
			demand += 4 / (1 + distance/80)
		}
	}
	for _, fleet := range t.game.getFleets(player.Num) {
		if isAutomaticFleet(player, fleet) && player.Automation.FleetRoles[fleet.Num] != "tanker" && len(fleet.Waypoints) > 1 {
			for _, wp := range fleet.Waypoints[1:] {
				demand += 3 / (1 + segmentDistance(point, fleet.Position, wp.Position)/60)
			}
		}
	}
	// Less benefit next to an unlimited fixed fuel source.
	demand *= min(1.0, 0.1+nearestDock/120)
	for _, other := range t.game.getFleets(player.Num) {
		if other == tanker || other.Delete || player.Automation.FleetRoles[other.Num] != "tanker" {
			continue
		}
		center := other.Position
		if job := readTankerJob(other); job != nil && job.Stage == "patrol" {
			center = job.Anchor
		}
		demand /= 1 + 6*math.Exp(-float64(point.DistanceSquaredTo(center))/float64(120*120))
	}
	return demand
}

func (t *turnGenerator) patrolAutomaticTanker(player *Player, tanker *Fleet, job *automaticTankerJob) bool {
	stock := max(1, tanker.Spec.FuelCapacity/3)
	if tanker.Fuel < stock && tanker.Spec.FuelGeneration == 0 {
		return false
	}
	if job == nil || job.Stage != "patrol" || t.game.Year-job.PatrolYear >= 15 && tanker.Position.DistanceTo(job.Anchor) < 40 {
		var chosen *automaticTankerJob
		best := 0.0
		anchors := t.tankerNetworkAnchors(player)
		values := map[Vector]float64{}
		for _, point := range anchors {
			values[point] = t.tankerAnchorValue(player, tanker, point) / (1 + point.DistanceTo(tanker.Position)/320)
		}
		slices.SortStableFunc(anchors, func(a, b Vector) int {
			if values[a] > values[b] {
				return -1
			}
			if values[a] < values[b] {
				return 1
			}
			return 0
		})
		for _, point := range anchors[:min(32, len(anchors))] {
			route := planAutomaticRoute(&t.game.Rules, player, []*Fleet{tanker}, point, MapObjectTarget{}, automaticRoutePolicy{MaxWarp: 4, DeliveryFuel: []int{stock}})
			if route == nil || route.WaitForFuel {
				continue
			}
			value := t.tankerAnchorValue(player, tanker, point) / (1 + float64(route.Turns)/20)
			if value > best {
				best = value
				chosen = &automaticTankerJob{Stage: "patrol", Anchor: point, Position: point, PatrolYear: t.game.Year}
			}
		}
		if chosen == nil {
			// A generating ship can safely replenish locally without inventing fuel.
			reserve := automaticReturnFuel(&t.game.Rules, player, tanker, tanker.Position, expeditionStations(player, tanker), false)
			if tanker.Spec.FuelGeneration > 0 && tanker.Fuel >= reserve {
				holdAutomaticFleet(player, tanker, "Regenerating fuel before network deployment")
				writeTankerJob(tanker, &automaticTankerJob{Stage: "patrol", Anchor: tanker.Position, Position: tanker.Position, PatrolYear: t.game.Year - 15, Status: "Regenerating fuel before network deployment"})
				return true
			}
			return false
		}
		job = chosen
	}
	if tanker.Position == job.Position {
		job.PatrolIndex = (job.PatrolIndex + 1) % 4
		offset := []Vector{{24, 0}, {0, 24}, {-24, 0}, {0, -24}}[job.PatrolIndex]
		job.Position = job.Anchor.Add(offset)
		if t.game.Area.X > 0 {
			job.Position.X = Clamp(job.Position.X, 0, t.game.Area.X)
			job.Position.Y = Clamp(job.Position.Y, 0, t.game.Area.Y)
		}
	}
	route := planAutomaticRoute(&t.game.Rules, player, []*Fleet{tanker}, job.Position, MapObjectTarget{}, automaticRoutePolicy{MaxWarp: 4, DeliveryFuel: []int{stock}})
	if route == nil {
		reserve := automaticReturnFuel(&t.game.Rules, player, tanker, tanker.Position, expeditionStations(player, tanker), false)
		if tanker.Spec.FuelGeneration == 0 || tanker.Fuel < reserve {
			return false
		}
		holdAutomaticFleet(player, tanker, "")
		job.Status = "Regenerating fuel at network node"
	} else {
		applyExpeditionRoute(player, tanker, route)
		job.Status = fmt.Sprintf("Fuel network patrol, warp <=4; assigned area %s", job.Anchor.String())
	}
	writeTankerJob(tanker, job)
	return true
}
