package cs

const emergencyReturnTag = "emergency-return"
const emergencyRiskTag = "emergency-return-risk"

// A threat at the starting point must not make every possible escape illegal.
// Permit only corridors whose distance from every relevant threat increases
// monotonically; approaching or crossing another threat remains forbidden.
func knownWithdrawalRoute(p *Player, from, to Vector) bool {
	movement := to.Subtract(from).ToFloat64()
	if movement.LengthSquared() == 0 {
		return false
	}
	seen := false
	margin := p.Automation.escortMarginAsFloat()
	for _, enemy := range p.FleetIntels {
		if enemy == nil || enemy.Delete || enemy.ReportAge != 0 || enemy.PlayerNum == p.Num || p.IsFriend(enemy.PlayerNum) || !patrolReportedCombatant(p, enemy) || segmentDistance(enemy.Position, from, to) > margin {
			continue
		}
		seen = true
		start := from.Subtract(enemy.Position).ToFloat64()
		if from.DistanceTo(enemy.Position) > margin || start.X*movement.X+start.Y*movement.Y < 0 || to.DistanceTo(enemy.Position) <= from.DistanceTo(enemy.Position) {
			return false
		}
	}
	return seen
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
		if wp.WarpSpeed < 1 || wp.WarpSpeed > automaticMaxWarp(p, f) || !automaticLegSafe(rules, p, from, wp.Position, wp.WarpSpeed, f.GetTag(emergencyRiskTag) != "", f.StrategicSpeed(wp.WarpSpeed)) {
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
