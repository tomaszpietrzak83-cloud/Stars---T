package cs

import (
	"cmp"
	"math"
	"slices"
)

func automaticMinesweeper(player *Player, fleet *Fleet) bool {
	return player.Automation.FleetRoles[fleet.Num] == "minesweeper" && fleet.Spec.MineSweep > 0
}

func automaticMineClearance(field *Minefield) float64 {
	radius := field.Radius()
	return radius + max(3, math.Ceil(radius*0.05))
}

func mineObstructsNavigation(player *Player, field *Minefield) bool {
	return field != nil && !field.Delete && field.NumMines > 0 && (field.PlayerNum != player.Num || field.Detonate)
}

// Every final leg (including a gate arrival) is rejected inside these fields.
// Keep boundary cases in the exact geometry path to preserve floating-point ties.
func automaticDestinationMineBlocked(player *Player, destination Vector, sweep bool) bool {
	for _, field := range player.MinefieldIntels {
		if mineObstructsNavigation(player, field) && (!sweep || field.Detonate) && destination.DistanceTo(field.Position) < automaticMineClearance(field)-1e-7 {
			return true
		}
	}
	return false
}

type routeMineKey struct{ from, to Vector }
type routeMineObstacle struct {
	field                                     *Minefield
	radius, clearance, closest, startDistance float64
}

// Geometry is independent of warp and sweeping capability. Retain it only
// within the current route search; evaluate movement/detonation rules below
// for every speed. Empty results are cached too, with bounded memory.
func routeMineObstacles(player *Player, from, to Vector) []routeMineObstacle {
	key := routeMineKey{from, to}
	index := player.routeThreatIndex
	if index != nil && index.mineLegs != nil {
		if value, ok := index.mineLegs[key]; ok {
			return value
		}
	}
	var result []routeMineObstacle
	for _, field := range player.MinefieldIntels {
		if !mineObstructsNavigation(player, field) {
			continue
		}
		radius, clearance := field.Radius(), automaticMineClearance(field)
		center := field.Position
		if !routeBoundsOverlap(from, to, center, center, clearance) {
			continue
		}
		closest := segmentDistance(center, from, to)
		if closest > clearance {
			continue
		}
		startDistance := from.DistanceTo(center)
		result = append(result, routeMineObstacle{field, radius, clearance, closest, startDistance})
	}
	if index != nil && index.mineLegs != nil {
		if len(index.mineLegs) >= 4096 {
			clear(index.mineLegs)
		}
		index.mineLegs[key] = result
	}
	return result
}

func automaticMineLegSafe(rules *Rules, player *Player, from, to Vector, warp int, step float64, sweep bool) bool {
	for _, obstacle := range routeMineObstacles(player, from, to) {
		field := obstacle.field
		radius, clearance, closest, startDistance := obstacle.radius, obstacle.clearance, obstacle.closest, obstacle.startDistance
		center := field.Position
		if field.Detonate {
			// A detonating field can only be escaped before this year's blast.
			end := to.ToFloat64()
			if distance := from.DistanceTo(to); distance > step {
				end = from.ToFloat64().Add(to.Subtract(from).Normalized().Scale(step))
			}
			if startDistance > clearance || end.DistanceTo(center.ToFloat64()) <= clearance || to.DistanceTo(center) <= clearance {
				return false
			}
			continue
		}
		safeWarp := rules.MinefieldStatsByType[field.MinefieldType].MaxSpeed + player.Race.Spec.MinefieldSafeWarpBonus
		if safeWarp == 0 {
			safeWarp = 3
		}
		if sweep && (warp <= safeWarp || closest > radius+1) {
			continue
		}
		// A fleet already inside may leave outward, at safe warp if it still
		// crosses actual mines. This also handles newly expanded fields.
		if startDistance <= clearance && to.DistanceTo(center) > clearance &&
			closest >= startDistance-1e-9 &&
			(startDistance > radius+1 || warp <= safeWarp) {
			continue
		}
		return false
	}
	return true
}

func mineDetourPoints(player *Player, from, to Vector) []Vector {
	var nearby []*Minefield
	blocked := false
	for _, field := range player.MinefieldIntels {
		if !mineObstructsNavigation(player, field) {
			continue
		}
		clearance := automaticMineClearance(field)
		distance := segmentDistance(field.Position, from, to)
		if distance <= clearance+12 {
			nearby = append(nearby, field)
			blocked = blocked || distance <= clearance
		}
	}
	if !blocked {
		return nil
	}
	slices.SortStableFunc(nearby, func(a, b *Minefield) int {
		return cmp.Compare(segmentDistance(a.Position, from, to), segmentDistance(b.Position, from, to))
	})
	var points []Vector
	const sides = 8
	for _, field := range nearby[:min(4, len(nearby))] {
		// Even the chord between adjacent points must clear the growth margin.
		radius := automaticMineClearance(field)/math.Cos(math.Pi/sides) + 2
		for side := range sides {
			angle := 2 * math.Pi * float64(side) / sides
			point := field.Position.ToFloat64().Add(VectorFloat64{X: radius * math.Cos(angle), Y: radius * math.Sin(angle)}).ToInt(true)
			points = append(points, point)
		}
	}
	return points
}

func mineDetourReturnFuel(rules *Rules, player *Player, fleet *Fleet, from Vector, stations []*Planet, allowThreat bool, limit, best int) int {
	probe := *fleet
	probe.Position, probe.Fuel = from, fleet.Spec.FuelCapacity
	// Every destination search uses this same probe and immutable reports.
	// Dock-to-dock and mine-detour edges otherwise repeat for every return dock.
	legCache := make(map[automaticLegCacheKey]bool)
	for _, station := range stations {
		if automaticMineLegSafe(rules, player, from, station.Position, automaticMaxWarp(player, fleet), 0, false) {
			continue
		}
		route := planAutomaticRoute(rules, player, []*Fleet{&probe}, station.Position, station.ToTarget(), automaticRoutePolicy{
			OneWay: true, AllowThreat: allowThreat, MaxTurns: limit, EmergencyReturn: true,
			returnLegCache: legCache,
		})
		if route == nil || route.WaitForFuel {
			continue
		}
		required := 0
		for i := len(route.Waypoints) - 1; i > 0; i-- {
			wp := route.Waypoints[i]
			distance := route.Waypoints[i-1].Position.DistanceTo(wp.Position)
			var ok bool
			required, _, _, ok = automaticLegFuel(player, &probe, wp.WarpSpeed, distance, probe.Fuel, required)
			if !ok {
				required = best
				break
			}
		}
		best = min(best, required)
		// As in the direct-return budget, zero is the minimum possible
		// reserve. Further dock searches cannot improve this answer.
		if best == 0 {
			return 0
		}
	}
	return best
}
