package cs

import "slices"

// Synchronize a two-turn assembly before departure. Early arrivals must not
// invalidate a viable pack on the following turn merely because they are idle.
func pacedPatrolRoute(rules *Rules, p *Player, f *Fleet, point Vector, route *expeditionRoute) *expeditionRoute {
	if route.Turns == 2 {
		return route
	}
	var best *expeditionRoute
	for warp := 1; warp <= automaticMaxWarp(p, f); warp++ {
		candidate := planAutomaticRoute(rules, p, []*Fleet{f}, point, MapObjectTarget{}, automaticRoutePolicy{Fast: true, MinWarp: warp, MaxWarp: warp, MaxTurns: 2, AllowThreat: knownWithdrawalRoute(p, f.Position, point)})
		if candidate != nil && !candidate.WaitForFuel && candidate.NetworkRequest == nil && candidate.Turns == 2 && (best == nil || candidate.Fuel < best.Fuel) {
			best = candidate
		}
	}
	if best != nil {
		return best
	}
	// Very close ships cannot stretch a one-unit leg by slowing down. Use a
	// funded perimeter leg, then join the fixed rendezvous on the second turn.
	for _, delta := range []Vector{{12, 0}, {0, 12}, {-12, 0}, {0, -12}} {
		stage := point.Add(delta)
		if stage == f.Position || knownRouteThreat(p, stage, stage) > 0 {
			continue
		}
		out := planAutomaticRoute(rules, p, []*Fleet{f}, stage, MapObjectTarget{}, automaticRoutePolicy{Fast: true, MaxTurns: 1})
		if out == nil || out.WaitForFuel || out.NetworkRequest != nil || out.Turns != 1 {
			continue
		}
		arrival := *f
		arrival.Position, arrival.Fuel = stage, f.Fuel-out.Fuel
		if len(out.ArrivalFuel) > 0 {
			arrival.Fuel = out.ArrivalFuel[0]
		}
		back := planAutomaticRoute(rules, p, []*Fleet{&arrival}, point, MapObjectTarget{}, automaticRoutePolicy{Fast: true, MaxTurns: 1})
		if back == nil || back.WaitForFuel || back.NetworkRequest != nil || back.Turns != 1 {
			continue
		}
		return &expeditionRoute{Waypoints: append(slices.Clone(out.Waypoints), back.Waypoints[1:]...), Turns: 2, Fuel: out.Fuel + back.Fuel, ArrivalFuel: slices.Clone(back.ArrivalFuel)}
	}
	return nil
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
