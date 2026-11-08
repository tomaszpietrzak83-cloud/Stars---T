package cs

const emergencyReturnTag = "emergency-return"
const emergencyRiskTag = "emergency-return-risk"

// A threatened starting point must not prevent departure to an unoccupied
// destination. Passing a reported enemy does not initiate a native battle.
func knownWithdrawalRoute(p *Player, from, to Vector) bool {
	return from != to && knownRouteThreat(p, from, from) > 0 && knownRouteThreat(p, from, to) == 0
}
func (t *turnGenerator) routeEmergencyReturn(p *Player, f *Fleet) bool {
	var best *expeditionRoute
	risk := false
	for _, dock := range expeditionStations(p, f) {
		if dock.Position == f.Position {
			continue
		}
		route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, dock.Position, dock.ToTarget(), automaticRoutePolicy{EmergencyReturn: true, MaxTurns: 10000})
		if route == nil {
			route = emergencyDetourRoute(&t.game.Rules, p, f, dock)
		}
		if route != nil && !route.WaitForFuel && (best == nil || route.Turns < best.Turns) {
			best = route
		}
	}
	{ // Recovery must keep moving for every role, not only scouts.
		for _, dock := range expeditionStations(p, f) {
			route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, dock.Position, dock.ToTarget(), automaticRoutePolicy{EmergencyReturn: true, AllowThreat: true, MaxTurns: 10000})
			if route != nil && !route.WaitForFuel && (best == nil || route.Turns+3 < best.Turns) {
				best = route
				risk = true
			}
		}
	}
	if best == nil {
		return false
	}
	applyExpeditionRoute(p, f, best)
	if risk {
		f.SetTag(emergencyRiskTag, "Returning despite hostile fleet reports; fuel and mine safety remain enforced")
	} else {
		delete(f.Tags, emergencyRiskTag)
	}
	f.SetTag(emergencyReturnTag, "Legacy or emergency recovery: slow funded return, requesting mobile fuel")
	f.SetTag(fuelStatusTag, "Moving towards a fuel dock at the fastest fully funded emergency speed; rescue requested")
	saveAutomaticFuelRequest(f, automaticFuelRequest{Destination: f.Position, Members: []int{f.Num}, Urgent: true})
	return true
}
func fundedEmergencyReturn(rules *Rules, p *Player, f *Fleet) bool {
	if f.GetTag(emergencyReturnTag) == "" || len(f.Waypoints) < 2 {
		return false
	}
	last := f.Waypoints[len(f.Waypoints)-1]
	dock := p.GetPlanetIntel(last.TargetNum)
	if dock == nil || !dock.OwnedBy(p.Num) || !dock.Spec.HasStarbase || dock.Spec.DockCapacity == 0 {
		return false
	}
	fuel, from := f.Fuel, f.Position
	for _, wp := range f.Waypoints[1:] {
		if wp.WarpSpeed < 1 || wp.WarpSpeed > automaticMaxWarp(p, f) || !automaticFleetLegSafe(rules, p, f, from, wp.Position, wp.WarpSpeed, f.GetTag(emergencyRiskTag) != "") {
			return false
		}
		_, left, _, ok := automaticLegFuel(p, f, wp.WarpSpeed, from.DistanceTo(wp.Position), fuel, 0)
		if !ok {
			return false
		}
		fuel = left
		from = wp.Position
		planet := p.GetPlanetIntel(wp.TargetNum)
		if planet != nil && planet.OwnedBy(p.Num) && planet.Spec.HasStarbase && planet.Spec.DockCapacity != 0 {
			fuel = f.Spec.FuelCapacity
		}
	}
	return fuel >= 0
}
