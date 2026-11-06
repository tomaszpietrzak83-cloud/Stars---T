package cs

import (
	"log/slog"
	"slices"
)

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
	// Older saves retained the expired assembly deadline during an attack.
	if job.Grouped && !job.Provisional && job.CleanupTarget == 0 {
		job.RendezvousBy, job.GatheringSince = 0, 0
		job.Rally, job.CleanupUsed = Vector{}, false
	}
	job.CleanupEnemy, job.CleanupTarget = 0, 0
	savePatrolJob(leader, *job)
	existing := []*Fleet{leader}
	for _, f := range t.game.getFleets(p.Num) {
		if f != leader && !f.Delete && readPatrolJob(f).Leader == leader.Num {
			if patrolAvailable(p, f) && compatiblePatrolPlans(leader, f) {
				next := readPatrolJob(f)
				next.CleanupEnemy, next.CleanupTarget = 0, 0
				savePatrolJob(f, next)
				existing = append(existing, f)
			} else {
				delete(f.Tags, patrolTag)
			}
		}
	}
	point := patrolInterceptPosition(enemy)
	unknown := patrolNeedsRecon(p, enemy)
	if unknown {
		t.disperseUnreadyPatrol(p, leader, enemy, job)
		return
	}
	// An already assembled local subset can launch immediately. Promised
	// reinforcements are not part of this readiness decision.
	if !unknown && t.launchLocalPatrol(p, leader, enemy, job, existing) {
		return
	}
	group := []*Fleet{leader}
	result := p.patrolEvaluator.assess(p, group, enemy, point, 1)
	// The candidate loop cannot run once its native forecast budget is spent.
	// Avoid computing hundreds of routes that would immediately be discarded.
	if !result.Ready && p.patrolEvaluator.Work >= p.patrolEvaluator.Limit {
		t.disperseUnreadyPatrol(p, leader, enemy, job)
		return
	}
	type candidate struct {
		fleet *Fleet
		route *expeditionRoute
	}
	candidates := []candidate{}
	for _, f := range t.game.getFleets(p.Num) {
		if f == leader || !patrolAvailable(p, f) || (!slices.Contains(existing, f) && !patrolSupportCanLeave(p, f, enemy)) || !compatiblePatrolPlans(leader, f) || f.Position.DistanceTo(point) > 400 {
			continue
		}
		route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, leader.Position, MapObjectTarget{}, automaticRoutePolicy{Fast: true, EconomicalPatrol: true, MaxTurns: 8, AllowThreat: knownWithdrawalRoute(p, f.Position, leader.Position)})
		if route != nil && !route.WaitForFuel {
			candidates = append(candidates, candidate{f, route})
		}
	}
	slices.SortStableFunc(candidates, func(a, b candidate) int {
		if a.route.Turns != b.route.Turns {
			return a.route.Turns - b.route.Turns
		}
		if fleetDamagePercent(a.fleet) != fleetDamagePercent(b.fleet) {
			return fleetDamagePercent(a.fleet) - fleetDamagePercent(b.fleet)
		}
		if a.route.Fuel != b.route.Fuel {
			return a.route.Fuel - b.route.Fuel
		}
		return a.fleet.Num - b.fleet.Num
	})
	checks := 0
	maxChecks := 32
	urgent := urgentPatrolTarget(p, enemy)
	reserveShips := 0
	if urgent {
		maxChecks = 96
	}
	if urgent && !result.Ready && len(candidates) > 8 {
		pool := []*Fleet{leader}
		for _, candidate := range candidates[:min(maxChecks, len(candidates))] {
			pool = append(pool, candidate.fleet)
		}
		if ready, assessment := readyPatrolPrefix(p, pool, enemy, point, &checks, maxChecks); ready != nil {
			group, result = ready, assessment
			reserveShips = (patrolFormationShips(group)*5 + 3) / 4
			n := len(group)
			for n < len(pool) && patrolFormationShips(pool[:n]) < reserveShips {
				n++
			}
			if n > len(group) && checks < maxChecks && p.patrolEvaluator.Work < p.patrolEvaluator.Limit {
				checks++
				if reserve := p.patrolEvaluator.assess(p, pool[:n], enemy, point, 1); reserve.Ready {
					group, result = pool[:n], reserve
				}
			}
		}
	}
	for _, candidate := range candidates {
		if result.Ready && urgent && reserveShips > 0 && patrolFormationShips(group) >= reserveShips {
			break
		}
		support, route := candidate.fleet, candidate.route
		if slices.Contains(group, support) {
			continue
		}
		if checks >= maxChecks || p.patrolEvaluator.Work >= p.patrolEvaluator.Limit {
			break
		}
		trial := append(slices.Clone(group), support)
		checks++
		next := p.patrolEvaluator.assess(p, trial, enemy, point, 1)
		opportunity := patrolCompositionValue(p, []*Fleet{support}) * .015 * float64(1+route.Turns)
		if !result.Ready || next.Ready && (next.Score+opportunity < result.Score || urgent && patrolFormationShips(group) < reserveShips) {
			group, result = trial, next
		}
		if result.Ready {
			if !urgent {
				break
			}
			// Relief must survive newly arriving reports while the force gathers.
			// Keep a bounded reserve; the actual attack still requires a native check.
			if reserveShips == 0 {
				reserveShips = (patrolFormationShips(group)*5 + 3) / 4
			}
			if patrolFormationShips(group) >= reserveShips {
				break
			}
		}
	}
	// Retain only a sufficient formation, including releasing a distant leader.
	if result.Ready && !urgent {
		for i := len(group) - 1; i >= 0 && len(group) > 1 && checks < maxChecks+4; i-- {
			trial := append(slices.Clone(group[:i]), group[i+1:]...)
			checks++
			next := p.patrolEvaluator.assess(p, trial, enemy, point, 1)
			if next.Ready {
				group, result = trial, next
			}
		}
	}
	if !result.Ready {
		if p.patrolEvaluator.PlanningDefence {
			slog.Debug("defence composition rejected", "leader", leader.Num, "enemy", enemy.Num, "ships", patrolFormationShips(group), "checks", checks, "work", p.patrolEvaluator.Work, "reason", result.Reason)
		}
		t.disperseUnreadyPatrol(p, leader, enemy, job)
		return
	}
	oldLeaderNum := leader.Num
	if !slices.Contains(group, leader) {
		leader = group[0]
		job.Leader, job.LeaderID = leader.Num, leader.ID
	}
	for _, f := range t.game.getFleets(p.Num) {
		old := readPatrolJob(f)
		if old.Leader == oldLeaderNum && !slices.Contains(group, f) {
			delete(f.Tags, patrolTag)
			restorePatrolBattlePlan(p, f)
			if f.GetTag(patrolSupportTag) == "escort" {
				p.Automation.FleetRoles[f.Num] = "escort"
				delete(f.Tags, patrolSupportTag)
			}
			if patrolAvailable(p, f) {
				holdAutomaticFleet(p, f, "Released surplus; available for another patrol mission")
			}
		}
	}
	for _, f := range group {
		if f != leader {
			if prior := readPatrolJob(f); prior.Leader != 0 && prior.Leader != leader.Num {
				t.detachPatrolMember(p, f)
				job.Grouped = job.Grouped || prior.Grouped
			}
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
			next.Leader, next.Enemy, next.Target, next.LeaderID = leader.Num, enemy.PlayerNum, enemy.Num, leader.ID
			savePatrolJob(f, next)
		}
	}
	savePatrolJob(leader, *job)
	if t.launchLocalPatrol(p, leader, enemy, job, group) {
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
			if p.patrolEvaluator.PlanningDefence {
				slog.Debug("defence rendezvous rejected", "leader", leader.Num, "enemy", enemy.Num, "ships", patrolFormationShips(group), "deadline", job.RendezvousBy, "year", t.game.Year)
			}
			t.disperseUnreadyPatrol(p, leader, enemy, job)
		}
		return
	}
	for _, f := range group[1:] {
		if _, err := NewOrderer().Merge(&t.game.Rules, p, []*Fleet{leader, f}); err != nil {
			t.disperseUnreadyPatrol(p, leader, enemy, job)
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
		t.disperseUnreadyPatrol(p, leader, enemy, job)
		return
	}
	applyPatrolBattlePlan(p, leader, result.Tactic)
	job.RendezvousBy, job.GatheringSince = 0, 0
	job.Rally, job.CleanupUsed = Vector{}, false
	job.Status = "Intercepting " + enemy.Name + " with native combat forecast"
	savePatrolJob(leader, *job)
	if !t.patrolRoute(p, leader, point, true) {
		t.returnPatrolToFuel(p, leader)
	}
}

func patrolFormationShips(group []*Fleet) int {
	count := 0
	for _, f := range group {
		count += f.Spec.TotalShips
	}
	return count
}

func (t *turnGenerator) reconPatrolReservation(p *Player) int {
	ships := 0
	for _, f := range t.game.getFleets(p.Num) {
		if patrolAvailable(p, f) {
			ships += f.Spec.TotalShips
		}
	}
	// Unknown equipment cannot justify reserving an ever larger fraction of
	// the empire. The provisional force is bounded until the scout reports.
	return max(3, min(32, ships/3))
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
	if defencePatrolCommitted(p, f) {
		return job.Enemy == enemy.PlayerNum && job.Target == enemy.Num
	}
	if f.GetTag(bomberBorrowTag) != "" {
		return false
	}
	if job.Enemy == enemy.PlayerNum && job.Target == enemy.Num {
		return true
	}
	if urgentPatrolTarget(p, enemy) {
		old := patrolEnemy(p, job)
		if old == nil || patrolMissionRank(p, enemy) < patrolMissionRank(p, old) {
			return true
		}
	}
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
