package cs

import (
	"math"
)

// Fuel auditing must validate the scheduled speed, not replace it with the
// fastest route and undo synchronized arrivals.
func fundedPatrolRallyWaypoints(rules *Rules, p *Player, f *Fleet, year int) bool {
	job := readPatrolJob(f)
	if !job.Provisional || job.Leader == 0 || job.CleanupTarget != 0 || patrolEnemy(p, job) == nil || len(f.Waypoints) < 2 || f.InAutomaticSafety() || f.InDesignMaintenance() || f.Waypoints[len(f.Waypoints)-1].Position != job.Rally {
		return false
	}
	budget := *f
	fuel, from, turns := f.Fuel, f.Position, 0
	for _, wp := range f.Waypoints[1:] {
		if wp.WarpSpeed < 1 || wp.WarpSpeed > automaticMaxWarp(p, f) || !automaticFleetLegSafe(rules, p, &budget, from, wp.Position, wp.WarpSpeed, false) {
			return false
		}
		turns += max(1, int(math.Ceil(from.DistanceTo(wp.Position)/max(1, f.StrategicSpeed(wp.WarpSpeed)))))
		_, left, _, ok := automaticLegFuel(p, &budget, wp.WarpSpeed, from.DistanceTo(wp.Position), fuel, 0)
		if !ok {
			return false
		}
		fuel, from = left, wp.Position
		budget.Position, budget.Fuel = from, fuel
		if planet := p.GetPlanetIntel(wp.TargetNum); planet != nil && planet.OwnedBy(p.Num) && budget.CanFuel(p, planet) {
			fuel, budget.Fuel = f.Spec.FuelCapacity, f.Spec.FuelCapacity
		}
	}
	return turns <= min(2, job.RendezvousBy-year) && fuel >= automaticReturnFuel(rules, p, &budget, from, expeditionStations(p, &budget), false)
}

// Synchronize a two-turn assembly before departure. Early arrivals must not
// invalidate a viable pack on the following turn merely because they are idle.
func pacedPatrolRoute(rules *Rules, p *Player, f *Fleet, point Vector, route *expeditionRoute) *expeditionRoute {
	arrivesOnSecondTurn := func(candidate *expeditionRoute) bool {
		if candidate == nil || candidate.Turns != 2 {
			return false
		}
		// Native movement snaps to a destination less than one light-year
		// beyond this turn's range. The route planner's ceil alone therefore
		// cannot distinguish a paced arrival from an early arrival.
		return len(candidate.Waypoints) > 2 || (len(candidate.Waypoints) == 2 && f.Position.DistanceTo(point)-f.StrategicSpeed(candidate.Waypoints[1].WarpSpeed) >= 1)
	}
	if arrivesOnSecondTurn(route) {
		return route
	}
	var best *expeditionRoute
	for warp := 1; warp <= automaticMaxWarp(p, f); warp++ {
		candidate := planAutomaticRoute(rules, p, []*Fleet{f}, point, MapObjectTarget{}, automaticRoutePolicy{Fast: true, MinWarp: warp, MaxWarp: warp, MaxTurns: 2, AllowThreat: knownWithdrawalRoute(p, f.Position, point)})
		if arrivesOnSecondTurn(candidate) && !candidate.WaitForFuel && candidate.NetworkRequest == nil && (best == nil || candidate.Fuel < best.Fuel) {
			best = candidate
		}
	}
	if best != nil {
		return best
	}
	// Early arrival is valid for an assessed, bounded rendezvous. Do not
	// manufacture a perimeter flight merely to keep a ship moving.
	return route
}

func synchronizePatrolRally(rules *Rules, p *Player, group []*Fleet, target Vector, rally *patrolRally) bool {
	if rally.GatherTurns != 2 {
		return true
	}
	merged := patrolMergedFormation(rules, p, group)
	if merged == nil {
		return false
	}
	merged.Position, merged.Fuel = rally.Position, 0
	for i, f := range group {
		route := pacedPatrolRoute(rules, p, f, rally.Position, rally.Routes[i])
		if route == nil {
			return false
		}
		rally.Routes[i] = route
		fuel := f.Fuel - route.Fuel
		if len(route.ArrivalFuel) > 0 {
			fuel = route.ArrivalFuel[0]
		}
		merged.Fuel += max(0, fuel)
	}
	merged.Fuel = min(merged.Fuel, merged.Spec.FuelCapacity)
	attack := planAutomaticRoute(rules, p, []*Fleet{merged}, target, MapObjectTarget{}, automaticRoutePolicy{Fast: true, AllowThreat: true, MaxTurns: 20, MinWarp: automaticCruiseWarp(p, merged)})
	return attack != nil && !attack.WaitForFuel
}
