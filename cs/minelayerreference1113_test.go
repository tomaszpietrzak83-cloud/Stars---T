//go:build !wasi && !wasm

package cs

import "slices"

func (t *turnGenerator) referenceMinelayerInterceptions1113(p *Player) {
	if p.patrolEvaluator == nil {
		return
	}
	evaluator := p.patrolEvaluator
	limit := evaluator.Limit
	evaluator.Limit = max(limit, evaluator.Work+384)
	defer func() { evaluator.Limit = limit }()
	fleets := slices.Clone(t.game.getFleets(p.Num))
	claimed := map[string]bool{}
	watched := map[string]bool{}
	for _, f := range fleets {
		if f.Delete {
			continue
		}
		if scout := readScoutRecon(f); scout != nil && scout.Minelayer && scoutReconActive(p, f) {
			watched[priorityFleetKey(scout.Enemy, scout.Target)] = true
		}
		j := readPatrolJob(f)
		if j.Leader != f.Num {
			continue
		}
		key := priorityFleetKey(j.Enemy, j.Target)
		claimed[key] = true
		if !j.Minelayer || defencePatrolCommitted(p, f) {
			continue
		}
		enemy := p.GetFleetIntel(j.Enemy, j.Target)
		if !trackableMinelayer(p, enemy) || !patrolAvailable(p, f) {
			t.releasePatrol(p, f)
			continue
		}
		if enemy.ReportAge != 0 || patrolNeedsRecon(p, enemy) {
			for _, member := range fleets {
				if !member.Delete && readPatrolJob(member).Leader == f.Num {
					holdAutomaticFleet(p, member, "Minelayer pursuit: waiting for a fresh scout report")
				}
			}
			continue
		}
		t.advanceAssessedPatrol(p, f, enemy, &j)
	}
	for _, enemy := range p.FleetIntels {
		if !exposedReportedMinelayer(p, enemy) || patrolNeedsRecon(p, enemy) {
			continue
		}
		key := priorityFleetKey(enemy.PlayerNum, enemy.Num)
		if claimed[key] {
			continue
		}
		point := patrolInterceptPosition(enemy)
		slices.SortStableFunc(fleets, func(a, b *Fleet) int {
			return a.Position.Subtract(point).LengthSquared() - b.Position.Subtract(point).LengthSquared()
		})
		for _, f := range fleets {
			if p.Automation.FleetRoles[f.Num] != "patrol" || !patrolAvailable(p, f) || readPatrolJob(f).Leader != 0 ||
				defencePatrolCommitted(p, f) || localPatrolReserve(f, t.game.Year) || f.carriesInfrastructure() {
				continue
			}
			route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, point, MapObjectTarget{}, automaticRoutePolicy{Fast: true, AllowThreat: true, MaxTurns: 6, MinWarp: automaticCruiseWarp(p, f)})
			if route == nil || route.WaitForFuel || route.NetworkRequest != nil || route.Turns > 1 && !watched[key] {
				continue
			}
			j := patrolJob{Leader: f.Num, Enemy: enemy.PlayerNum, Target: enemy.Num, Minelayer: true, RequiredShips: 1, GatheringSince: t.game.Year}
			if route.Turns <= 1 && len(route.Waypoints) <= 2 {
				group := []*Fleet{f}
				if result := p.patrolEvaluator.assess(p, group, enemy, point, 1); result.Ready {
					j.ReliefYear, j.Rally, j.RequiredShips = t.game.Year, point, f.Spec.TotalShips
					j.Status = "Intercepting reported minelayer: " + enemy.Name
					t.reservePatrolRelief(p, group, &j)
					applyExpeditionRoute(p, f, route)
					applyPatrolBattlePlan(p, f, result.Tactic)
					savePatrolJob(f, j)
					break
				}
			}
			if !watched[key] {
				continue
			}
			savePatrolJob(f, j)
			t.advanceAssessedPatrol(p, f, enemy, &j)
			if slices.ContainsFunc(t.game.getFleets(p.Num), func(member *Fleet) bool {
				active := readPatrolJob(member)
				return !member.Delete && active.Leader == member.Num && active.Enemy == enemy.PlayerNum && active.Target == enemy.Num
			}) {
				break
			}
		}
	}
}
