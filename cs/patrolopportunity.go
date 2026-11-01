package cs

import "math"

// Opportunistic hunters reconsider a long chase each year. Fresh one-turn
// opportunities beat distant, receding targets, but not a colony's bomber threat.
func (t *turnGenerator) retargetPatrolOpportunity(p *Player, f *Fleet) {
	job := readPatrolJob(f)
	if job.Leader != f.Num || f.Delete || !patrolAvailable(p, f) {
		return
	}
	current := patrolEnemy(p, job)
	if current == nil || enemyApproachingColony(p, current) && patrolReportedBomber(p, current) {
		return
	}
	distance := f.Position.DistanceTo(current.Position)
	toward := f.Position.Subtract(current.Position).Normalized()
	receding := current.WarpSpeed > 0 && current.Heading.X*toward.X+current.Heading.Y*toward.Y < -0.25
	var best *Fleet
	bestDistance := math.Inf(1)
	for _, candidate := range p.FleetIntels {
		if candidate == nil || candidate == current || candidate.Delete || candidate.Starbase || candidate.ReportAge != 0 || candidate.PlayerNum == p.Num || p.IsFriend(candidate.PlayerNum) || patrolPower(f) < patrolThreat(p, candidate)*3/2+1 {
			continue
		}
		d := f.Position.DistanceTo(candidate.Position)
		if d > f.StrategicSpeed(automaticMaxWarp(p, f)) || d >= distance || !receding && d*1.5 >= distance {
			continue
		}
		assigned := false
		for _, other := range t.game.getFleets(p.Num) {
			otherJob := readPatrolJob(other)
			if other != f && !other.Delete && otherJob.Leader == other.Num && otherJob.Enemy == candidate.PlayerNum && otherJob.Target == candidate.Num && other.Position.DistanceTo(candidate.Position) <= d && patrolPower(other) >= patrolThreat(p, candidate)*3/2+1 {
				assigned = true
				break
			}
		}
		if assigned {
			continue
		}
		route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, candidate.Position, candidate.ToTarget(), automaticRoutePolicy{AllowThreat: true, MaxTurns: 1, MinWarp: automaticCruiseWarp(p, f)})
		if route == nil || route.WaitForFuel {
			continue
		}
		if d < bestDistance || d == bestDistance && (best == nil || candidate.PlayerNum < best.PlayerNum || candidate.PlayerNum == best.PlayerNum && candidate.Num < best.Num) {
			best, bestDistance = candidate, d
		}
	}
	if best == nil {
		return
	}
	for _, member := range t.game.getFleets(p.Num) {
		memberJob := readPatrolJob(member)
		if memberJob.Leader == f.Num {
			memberJob.Enemy, memberJob.Target = best.PlayerNum, best.Num
			memberJob.RecruitPower = 0
			memberJob.GatheringSince = t.game.Year
			memberJob.Status = "Switching from distant pursuit to nearby interception: " + best.Name
			savePatrolJob(member, memberJob)
		}
	}
}
