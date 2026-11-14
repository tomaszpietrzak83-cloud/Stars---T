package cs

// A temporary local interception keeps the main mission identity and its
// original rendezvous deadline. It cannot reset the gathering clock annually.
func patrolTacticalEnemy(p *Player, f *Fleet) *Fleet {
	j := readPatrolJob(f)
	if j.CleanupTarget != 0 {
		return patrolEnemy(p, patrolJob{Enemy: j.CleanupEnemy, Target: j.CleanupTarget})
	}
	return patrolEnemy(p, j)
}

func (t *turnGenerator) patrolGatheringCleanup(p *Player, leader, main *Fleet, job *patrolJob, group []*Fleet) {
	bombing, deadline := patrolColonyDeadline(p, main)
	if bombing || deadline <= 2 {
		return
	}
	for _, f := range group {
		j := readPatrolJob(f)
		if !j.Provisional {
			continue
		}
		rally := j.Rally
		direct := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, rally, MapObjectTarget{}, automaticRoutePolicy{Fast: true, EconomicalPatrol: true, MaxTurns: 8})
		if direct == nil || direct.WaitForFuel {
			continue
		}
		by := job.RendezvousBy
		if by == 0 {
			by = t.game.Year + direct.Turns
		}
		// Already overdue means return to the pack, not another detour.
		allowed := by - t.game.Year
		if allowed < 1 {
			continue
		}
		for _, enemy := range sortedPatrolTargets(p, f) {
			// A higher-priority nearby target is not a reason to reject a safe
			// cleanup. Claims, urgent defence and the original deadline still
			// prevent stealing another patrol's work or delaying the main mission.
			if enemy == main || reportedFleetShips(enemy) > 3 || patrolNeedsRecon(p, enemy) {
				continue
			}
			claimed := false
			for _, other := range t.game.getFleets(p.Num) {
				oj := readPatrolJob(other)
				if other != f && !other.Delete && (oj.Enemy == enemy.PlayerNum && oj.Target == enemy.Num && oj.Leader != 0 || oj.CleanupEnemy == enemy.PlayerNum && oj.CleanupTarget == enemy.Num) {
					claimed = true
					break
				}
			}
			if claimed {
				continue
			}
			out, forecast := t.patrolCleanupRoute(p, f, enemy, rally, allowed)
			if out == nil {
				continue
			}
			j.RendezvousBy, j.CleanupUsed = by, true
			j.CleanupEnemy, j.CleanupTarget = enemy.PlayerNum, enemy.Num
			j.Status = "Local cleanup on the way to the pack; original rendezvous deadline retained"
			savePatrolJob(f, j)
			applyPatrolBattlePlan(p, f, forecast.Tactic)
			applyExpeditionRoute(p, f, out)
			if f == leader {
				*job = j
			}
			break
		}
	}
}

func (t *turnGenerator) patrolCleanupRoute(p *Player, f, enemy *Fleet, rally Vector, allowed int) (*expeditionRoute, patrolAssessment) {
	point := patrolInterceptPosition(enemy)
	out := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, point, MapObjectTarget{}, automaticRoutePolicy{Fast: true, AllowThreat: true, MaxTurns: 1, MinWarp: automaticCruiseWarp(p, f)})
	if out == nil || out.WaitForFuel || len(out.ArrivalFuel) == 0 || hostileOrbitalBase(p, point) {
		return nil, patrolAssessment{}
	}
	arrival := *f
	arrival.Position, arrival.Fuel = point, out.ArrivalFuel[0]
	back := planAutomaticRoute(&t.game.Rules, p, []*Fleet{&arrival}, rally, MapObjectTarget{}, automaticRoutePolicy{Fast: true, EconomicalPatrol: true, MaxTurns: allowed})
	if back == nil || back.WaitForFuel || out.Turns+back.Turns > allowed {
		return nil, patrolAssessment{}
	}
	forecast := t.defensiveEvaluator(p).assess(p, []*Fleet{f}, enemy, point, 1)
	if !forecast.Ready || forecast.PeakDamage > .1 {
		return nil, patrolAssessment{}
	}
	return out, forecast
}
