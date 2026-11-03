package cs

import "slices"

func patrolCompositionValue(p *Player, group []*Fleet) float64 {
	value := 0.0
	for _, f := range group {
		for _, token := range f.Tokens {
			if design := p.GetDesign(token.DesignNum); design != nil {
				c := design.Spec.Cost
				value += float64(c.Resources+c.Ironium+c.Boranium+c.Germanium) * float64(token.Quantity)
			}
		}
	}
	return value
}

// Bounded composition search jointly evaluates mixed native stacks and tactics.
// The score prices readiness damage and repair downtime as well as casualties;
// an extra ship can be worth retaining even when a smaller stack wins.
func (t *turnGenerator) advanceAssessedPatrol(p *Player, leader, enemy *Fleet, job *patrolJob) {
	group := []*Fleet{leader}
	for _, f := range t.game.getFleets(p.Num) {
		if f != leader && !f.Delete && readPatrolJob(f).Leader == leader.Num {
			if patrolAvailable(p, f) && compatiblePatrolPlans(leader, f) {
				group = append(group, f)
			} else {
				delete(f.Tags, patrolTag)
			}
		}
	}
	point := patrolInterceptPosition(enemy)
	unknown := patrolNeedsRecon(p, enemy)
	candidates := []*Fleet{}
	for _, f := range t.game.getFleets(p.Num) {
		if slices.Contains(group, f) || !patrolAvailable(p, f) || !patrolSupportCanLeave(p, f, enemy) || !compatiblePatrolPlans(leader, f) || f.Position.DistanceTo(point) > 400 {
			continue
		}
		candidates = append(candidates, f)
	}
	slices.SortStableFunc(candidates, func(a, b *Fleet) int {
		// Proximity to the battle, engine capability, and condition all matter.
		cost := func(f *Fleet) float64 {
			return f.Position.DistanceTo(point)/max(1, f.StrategicSpeed(automaticMaxWarp(p, f))) + float64(fleetDamagePercent(f))/10
		}
		if cost(a) < cost(b) {
			return -1
		}
		if cost(a) > cost(b) {
			return 1
		}
		return a.Num - b.Num
	})
	result := p.patrolEvaluator.assess(p, group, enemy, point, 1)
	checks := 0
	for _, support := range candidates {
		if checks >= 10 || p.patrolEvaluator.Work >= p.patrolEvaluator.Limit {
			break
		}
		// Do not reserve support without a real funded gathering route.
		route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{support}, leader.Position, MapObjectTarget{}, automaticRoutePolicy{Fast: true, EconomicalPatrol: true, MaxTurns: 8})
		if route == nil || route.WaitForFuel {
			continue
		}
		trial := append(slices.Clone(group), support)
		if prior := readPatrolJob(support); prior.Leader == support.Num {
			for _, member := range t.game.getFleets(p.Num) {
				if member != support && !member.Delete && readPatrolJob(member).Leader == support.Num && patrolAvailable(p, member) && compatiblePatrolPlans(leader, member) && !slices.Contains(trial, member) {
					trial = append(trial, member)
				}
			}
		}
		if unknown {
			// Useful surplus can gather while equipment reconnaissance proceeds.
			if patrolGroupPower(group) >= patrolThreat(p, enemy)*2 && len(group) >= 3 {
				break
			}
			group = trial
			continue
		}
		checks++
		next := p.patrolEvaluator.assess(p, trial, enemy, point, 1)
		opportunity := patrolCompositionValue(p, []*Fleet{support}) * .015 * float64(1+route.Turns)
		if !result.Ready || next.Ready && next.Score+opportunity < result.Score {
			group, result = trial, next
		}
		if result.Ready && checks >= 3 {
			break
		}
	}
	// Try removing redundant whole formations, preserving useful damage reduction.
	if result.Ready {
		for i := len(group) - 1; i > 0 && checks < 12; i-- {
			trial := append(slices.Clone(group[:i]), group[i+1:]...)
			checks++
			next := p.patrolEvaluator.assess(p, trial, enemy, point, 1)
			if next.Ready && next.Score+patrolCompositionValue(p, trial)*.015 < result.Score+patrolCompositionValue(p, group)*.015 {
				group, result = trial, next
			}
		}
	}
	for _, f := range t.game.getFleets(p.Num) {
		old := readPatrolJob(f)
		if old.Leader == leader.Num && !slices.Contains(group, f) {
			delete(f.Tags, patrolTag)
			restorePatrolBattlePlan(p, f)
			if f.GetTag(patrolSupportTag) == "escort" {
				p.Automation.FleetRoles[f.Num] = "escort"
				delete(f.Tags, patrolSupportTag)
			}
			holdAutomaticFleet(p, f, "Released surplus; available for another patrol mission")
		}
	}
	for _, f := range group {
		if f != leader {
			if p.Automation.FleetRoles[f.Num] == "escort" && f.GetTag(patrolSupportTag) == "" {
				f.SetTag(patrolSupportTag, "escort")
				p.Automation.FleetRoles[f.Num] = "patrol"
				if job.SupportDesigns == nil {
					job.SupportDesigns = map[int]int{}
				}
				for _, token := range f.Tokens {
					job.SupportDesigns[token.DesignNum] += token.Quantity
				}
			}
			next := readPatrolJob(f)
			next.Leader, next.Enemy, next.Target = leader.Num, enemy.PlayerNum, enemy.Num
			savePatrolJob(f, next)
		}
	}
	if unknown || !result.Ready {
		job.RecruitPower = max(patrolGroupPower(group)+1, job.RecruitPower)
		job.Status = result.Reason
		t.provisionalPatrolGather(p, leader, enemy, job)
		return
	}
	// The evaluator compared a merged formation. No constituent may attack alone.
	allHere := true
	for _, f := range group[1:] {
		allHere = allHere && f.Position == leader.Position
	}
	if !allHere {
		job.Provisional = false
		if !t.movingPatrolRally(p, leader, job) {
			t.provisionalPatrolGather(p, leader, enemy, job)
		}
		return
	}
	for _, f := range group[1:] {
		if _, err := NewOrderer().Merge(&t.game.Rules, p, []*Fleet{leader, f}); err != nil {
			t.provisionalPatrolGather(p, leader, enemy, job)
			return
		}
		t.game.deleteFleet(f)
		job.Grouped = true
	}
	job.Provisional, job.RecruitPower = false, 0
	job.RequiredShips = 1
	t.releaseExcessPatrolShips(p, leader, enemy, job)
	// Splitting changes the exact stack and fuel. Confirm the final native group.
	result = p.patrolEvaluator.assess(p, []*Fleet{leader}, enemy, point, 1)
	if !result.Ready {
		t.provisionalPatrolGather(p, leader, enemy, job)
		return
	}
	applyPatrolBattlePlan(p, leader, result.Tactic)
	job.Status = "Intercepting " + enemy.Name + " with native combat forecast"
	savePatrolJob(leader, *job)
	if !t.patrolRoute(p, leader, point, true) {
		t.returnPatrolToFuel(p, leader)
	}
}

func patrolGroupPower(group []*Fleet) int {
	power := 0
	for _, f := range group {
		power += patrolPower(f)
	}
	return power
}

func patrolSupportCanLeave(p *Player, f, enemy *Fleet) bool {
	job := readPatrolJob(f)
	if job.Leader == 0 {
		return true
	}
	if job.Leader != f.Num {
		return false
	}
	old := patrolEnemy(p, job)
	if old == nil {
		return true
	}
	// Finish an imminent local interception. Distant lower-priority chores
	// cannot permanently reserve every ship while a colony is being bombed.
	return patrolMissionRank(p, enemy) < patrolMissionRank(p, old) && f.Position.DistanceTo(patrolInterceptPosition(old)) > f.StrategicSpeed(automaticMaxWarp(p, f))
}
