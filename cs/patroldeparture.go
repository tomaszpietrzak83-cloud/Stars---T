package cs

import "fmt"

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
			return t.recaptureDepartureConfirmed(p, f, planet)
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

// The final departure guard sees the actual first leg, after cargo/fuel orders.
// Feed its rejection back to next turn's planner; otherwise it can propose the
// same apparently good interception forever and be stopped here every year.
func (t *turnGenerator) rememberBlockedPatrolDeparture(p *Player, f *Fleet) {
	j := readPatrolJob(f)
	if j.Leader == 0 || j.Target == 0 {
		return
	}
	declined := fmt.Sprintf("%d:%d:%d", j.Enemy, j.Target, t.game.Year+2)
	f.SetTag(declinedPatrolTag, declined)
	for _, member := range t.game.getFleets(p.Num) {
		m := readPatrolJob(member)
		if !member.Delete && m.Leader == j.Leader && m.Enemy == j.Enemy && m.Target == j.Target {
			member.SetTag(declinedPatrolTag, declined)
		}
	}
}
