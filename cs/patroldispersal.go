package cs

import "slices"

// A reinforcement request is a fresh assessment on the next planning pass,
// not ownership of hunters while no viable attack exists.
func (t *turnGenerator) disperseUnreadyPatrol(p *Player, leader, enemy *Fleet, job *patrolJob) {
	if p.patrolEvaluator.Deferred == nil {
		p.patrolEvaluator.Deferred = map[string]bool{}
	}
	p.patrolEvaluator.Deferred[priorityFleetKey(enemy.PlayerNum, enemy.Num)] = true
	job.Grouped = !t.ordersOnly
	savePatrolJob(leader, *job)
	// Only this pack needs immediate reassignment. Replanning every idle
	// patrol here repeats the global planner for each unsuccessful pack.
	released := map[int]bool{}
	existing := map[int]bool{}
	for _, f := range t.game.getFleets(p.Num) {
		existing[f.Num] = true
		if readPatrolJob(f).Leader == leader.Num {
			released[f.Num] = true
		}
	}
	t.releasePatrol(p, leader)
	if p.patrolEvaluator.PlanningDefence {
		return
	}
	// Split children must receive work in this pass too, even when created after
	// the outer planner took its fleet snapshot.
	for _, f := range slices.Clone(t.game.getFleets(p.Num)) {
		if existing[f.Num] && !released[f.Num] {
			continue
		}
		if !patrolAvailable(p, f) || readPatrolJob(f).Leader != 0 || !f.Idle() {
			continue
		}
		if !t.assignSoloPatrol(p, f) {
			t.patrolFrontier(p, f)
		}
	}
}

func (t *turnGenerator) assignSoloPatrol(p *Player, f *Fleet) bool {
	// Assignments are unchanged until this function selects a target and returns.
	// Index them once instead of decoding every fleet's JSON for each enemy.
	type targetKey struct{ owner, num int }
	claimed := make(map[targetKey]bool)
	for _, other := range t.game.getFleets(p.Num) {
		if other.Delete || other == f {
			continue
		}
		j := readPatrolJob(other)
		if j.Leader != 0 {
			claimed[targetKey{j.Enemy, j.Target}] = true
		}
	}
	for _, enemy := range sortedPatrolTargets(p, f) {
		if enemy == nil || enemy.Delete || enemy.ReportAge != 0 || enemy.Starbase || enemy.PlayerNum == p.Num || p.IsFriend(enemy.PlayerNum) || patrolNeedsRecon(p, enemy) || declinedPatrolTarget(f, enemy, t.game.Year) || p.patrolEvaluator.Deferred[priorityFleetKey(enemy.PlayerNum, enemy.Num)] {
			continue
		}
		if claimed[targetKey{enemy.PlayerNum, enemy.Num}] {
			continue
		}
		point := patrolInterceptPosition(enemy)
		route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, point, MapObjectTarget{}, automaticRoutePolicy{Fast: true, AllowThreat: true, MaxTurns: 2, MinWarp: automaticCruiseWarp(p, f)})
		if route == nil || route.WaitForFuel {
			continue
		}
		result := p.patrolEvaluator.assess(p, []*Fleet{f}, enemy, point, 1)
		if !result.Ready {
			continue
		}
		savePatrolJob(f, patrolJob{Leader: f.Num, Enemy: enemy.PlayerNum, Target: enemy.Num, RequiredShips: 1, Status: "Dispersed hunting: intercepting " + enemy.Name})
		applyPatrolBattlePlan(p, f, result.Tactic)
		applyExpeditionRoute(p, f, route)
		return true
	}
	return false
}

// Early arrivals cover the perimeter while peers approach. Every leg is funded
// and must permit return by the original deadline. If this is impossible the
// caller abandons the assembly instead of turning it into an indefinite wait.
func (t *turnGenerator) screenPatrolRally(p *Player, f *Fleet, by int) bool {
	j := readPatrolJob(f)
	remaining := by - t.game.Year
	if remaining < 2 {
		return false
	}
	for _, delta := range []Vector{{12, 0}, {0, 12}, {-12, 0}, {0, -12}} {
		point := j.Rally.Add(delta)
		out := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, point, MapObjectTarget{}, automaticRoutePolicy{Fast: true, EconomicalPatrol: true, MaxTurns: 1})
		if out == nil || out.WaitForFuel {
			continue
		}
		arrival := *f
		arrival.Position, arrival.Fuel = point, f.Fuel-out.Fuel
		if len(out.ArrivalFuel) > 0 {
			arrival.Fuel = out.ArrivalFuel[0]
		}
		back := planAutomaticRoute(&t.game.Rules, p, []*Fleet{&arrival}, j.Rally, MapObjectTarget{}, automaticRoutePolicy{Fast: true, EconomicalPatrol: true, MaxTurns: remaining - out.Turns})
		if back == nil || back.WaitForFuel || out.Turns+back.Turns > remaining {
			continue
		}
		applyExpeditionRoute(p, f, out)
		j.Status = "Covering rendezvous perimeter; return before attack deadline"
		savePatrolJob(f, j)
		return true
	}
	return false
}
