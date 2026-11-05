package cs

import (
	"log/slog"
	"slices"
)

// Test physical clusters, never a hypothetical stack of ships still in transit.
// A ready cluster may take command even if the old leader is elsewhere.
func (t *turnGenerator) launchLocalPatrol(p *Player, oldLeader, enemy *Fleet, job *patrolJob, members []*Fleet) bool {
	clusters := map[Vector][]*Fleet{}
	for _, f := range members {
		clusters[f.Position] = append(clusters[f.Position], f)
	}
	groups := make([][]*Fleet, 0, len(clusters))
	for _, group := range clusters {
		groups = append(groups, group)
	}
	slices.SortStableFunc(groups, func(a, b []*Fleet) int {
		if patrolFormationShips(a) != patrolFormationShips(b) {
			return patrolFormationShips(b) - patrolFormationShips(a)
		}
		return a[0].Num - b[0].Num
	})
	for _, group := range groups[:min(8, len(groups))] {
		point := patrolInterceptPosition(enemy)
		result := p.patrolEvaluator.assess(p, group, enemy, point, 1)
		if !result.Ready {
			if p.patrolEvaluator.PlanningDefence {
				slog.Debug("local defence not ready", "leader", oldLeader.Num, "enemy", enemy.Num, "ships", patrolFormationShips(group), "reason", result.Reason)
			}
			continue
		}
		merged := patrolMergedFormation(&t.game.Rules, p, group)
		route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{merged}, point, MapObjectTarget{}, automaticRoutePolicy{Fast: true, AllowThreat: true, MaxTurns: 20, MinWarp: automaticCruiseWarp(p, merged)})
		if route == nil || route.WaitForFuel {
			if p.patrolEvaluator.PlanningDefence {
				slog.Debug("local defence lacks attack fuel route", "leader", oldLeader.Num, "enemy", enemy.Num, "ships", patrolFormationShips(group), "fuel", merged.Fuel)
			}
			continue
		}
		leader := group[0]
		for _, f := range group {
			if f == oldLeader {
				leader = oldLeader
				break
			}
		}
		t.transferPatrolCommand(p, leader, job, members)
		for _, f := range group {
			if f == leader {
				continue
			}
			if _, err := NewOrderer().Merge(&t.game.Rules, p, []*Fleet{leader, f}); err != nil {
				return false
			}
			t.game.deleteFleet(f)
			job.Grouped = true
		}
		// Recheck native merged tokens, fuel and plan before assigning departure.
		result = p.patrolEvaluator.assess(p, []*Fleet{leader}, enemy, point, 1)
		if !result.Ready {
			return false
		}
		t.releaseExcessPatrolShips(p, leader, enemy, job)
		result = p.patrolEvaluator.assess(p, []*Fleet{leader}, enemy, point, 1)
		if !result.Ready {
			return false
		}
		applyPatrolBattlePlan(p, leader, result.Tactic)
		job.Provisional, job.RecruitPower, job.RequiredShips = false, 0, 1
		// This assembly is complete. A later encounter may require a new
		// rendezvous; its deadline must not inherit this finished gathering.
		job.RendezvousBy, job.GatheringSince = 0, 0
		job.Rally, job.CleanupUsed = Vector{}, false
		job.CleanupEnemy, job.CleanupTarget = 0, 0
		job.Status = "Intercepting " + enemy.Name + " with local wolf pack; <=5% sampled count/value losses"
		savePatrolJob(leader, *job)
		if !t.patrolRoute(p, leader, point, true) {
			return true
		}
		// A launched force owns no remote reinforcements. They keep hunting;
		// a stronger future contact will trigger a fresh composition search.
		existing := map[int]bool{}
		released := map[int]bool{}
		for _, f := range t.game.getFleets(p.Num) {
			existing[f.Num] = true
		}
		for _, f := range members {
			if f == leader || f.Delete {
				continue
			}
			next := readPatrolJob(f)
			released[f.Num] = true
			next.Leader, next.LeaderID, next.Grouped = f.Num, f.ID, !t.ordersOnly
			savePatrolJob(f, next)
			t.releasePatrol(p, f)
		}
		for _, f := range slices.Clone(t.game.getFleets(p.Num)) {
			if p.patrolEvaluator.PlanningDefence {
				break
			}
			if existing[f.Num] && !released[f.Num] {
				continue
			}
			if patrolAvailable(p, f) && readPatrolJob(f).Leader == 0 && f.Idle() {
				if !t.assignSoloPatrol(p, f) {
					t.patrolFrontier(p, f)
				}
			}
		}
		return true
	}
	return false
}
