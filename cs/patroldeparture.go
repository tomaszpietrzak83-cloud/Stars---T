package cs

// This is deliberately independent of mission labels: a rendezvous, old order,
// pursuit or routine transit can end at the same position as a hostile fleet.
func (t *turnGenerator) patrolDepartureConfirmed(p *Player, f *Fleet) bool {
	if p.patrolEvaluator == nil || len(f.Waypoints) < 2 || f.InDesignMaintenance() {
		return true
	}
	wp := f.Waypoints[1]
	point := wp.Position
	if wp.WarpSpeed != StargateWarpSpeed {
		reach := f.StrategicSpeed(wp.WarpSpeed)
		if distance := f.Position.DistanceTo(point); distance > reach {
			point = f.Position.ToFloat64().Add(point.Subtract(f.Position).Normalized().Scale(reach)).ToInt(true)
		}
	}
	for _, planet := range p.PlanetIntels {
		if planet != nil && planet.Owned() && planet.PlayerNum != p.Num && !p.IsFriend(planet.PlayerNum) && planet.Spec.HasStarbase && point.DistanceTo(planet.Position) <= 3 {
			return false
		}
	}
	for _, report := range p.FleetIntels {
		if report == nil || report.Delete || report.ReportAge != 0 || report.Starbase || report.PlayerNum == p.Num || p.IsFriend(report.PlayerNum) {
			continue
		}
		if point.DistanceTo(report.Position) > 3 && point.DistanceTo(patrolInterceptPosition(report)) > 3 {
			continue
		}
		result := p.patrolEvaluator.assess(p, []*Fleet{f}, report, point, 1)
		if !result.Ready {
			return false
		}
		applyPatrolBattlePlan(p, f, result.Tactic)
	}
	return true
}
