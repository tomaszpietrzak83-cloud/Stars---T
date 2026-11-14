package cs

import "slices"

// Defence is allocated before independent hunting can consume ships or the
// forecast budget. A failed local leader must not veto every other coalition.
func (t *turnGenerator) planUrgentPatrolDefence(p *Player) {
	e := p.patrolEvaluator
	if e == nil || e.DefencePlanned {
		return
	}
	e.DefencePlanned, e.PlanningDefence = true, true
	e.DefenceMembers = map[int]bool{}
	defer func() {
		e.PlanningDefence = false
		e.PlanningUrgentDefence = false
		for _, f := range t.game.getFleets(p.Num) {
			if e.DefenceMembers[f.Num] && pureAssaultBattleships(p, f) {
				delete(f.Tags, battleshipRallyTag)
				delete(f.Tags, recaptureAssaultTag)
				delete(f.Tags, battleshipNextAssaultTag)
			}
		}
	}()
	launched := t.launchAssembledPatrolDefence(p)
	tasks := []patrolMission{}
	covered := map[Vector]bool{}
	for _, mission := range p.patrolQueue {
		if mission.Kind != "attack" {
			continue
		}
		enemy := patrolEnemy(p, patrolJob{Enemy: mission.Enemy, Target: mission.Target})
		if enemy == nil {
			continue
		}
		if launched[mission.Key] && patrolAttackArrivesThisTurn(p, t.game.getFleets(p.Num), enemy) {
			covered[patrolInterceptPosition(enemy)] = true
			continue
		}
		if urgentPatrolTarget(p, enemy) || priorityFleet(p, enemy) ||
			mission.Territorial && (patrolReportedCombatant(p, enemy) || patrolReportedBomber(p, enemy) || patrolReportedTransport(p, enemy)) {
			tasks = append(tasks, mission)
		}
	}
	limit := e.Limit
	defer func() { e.Limit = limit }()
	staged := map[Vector]bool{}
	for _, immediate := range []bool{true, false} {
		for i, mission := range tasks {
			if immediate && mission.Tier != 0 {
				continue
			}
			if e.Work >= limit {
				break
			}
			enemy := patrolEnemy(p, patrolJob{Enemy: mission.Enemy, Target: mission.Target})
			e.PlanningUrgentDefence = urgentPatrolTarget(p, enemy)
			point := patrolInterceptPosition(enemy)
			if covered[point] {
				// The existing force was forecast against all reported ships at
				// this encounter, including the escorts of its primary target.
				if e.Deferred == nil {
					e.Deferred = map[string]bool{}
				}
				e.Deferred[mission.Key] = true
				continue
			}
			// Keep an expensive assault from consuming every forecast before the
			// next threatened world or explicitly marked fleet gets considered.
			share := max(400, (limit-e.Work)/(len(tasks)-i))
			if mission.Tier == 0 {
				share = max(1200, share)
			}
			e.Limit = min(limit, e.Work+share)

			leaders := []*Fleet{}
			for _, f := range t.game.getFleets(p.Num) {
				if patrolAvailable(p, f) && !e.DefenceMembers[f.Num] && f.GetTag(bomberBorrowTag) == "" && f.Position.DistanceTo(enemy.Position) <= 400 && patrolSupportCanLeave(p, f, enemy) {
					leaders = append(leaders, f)
				}
			}
			slices.SortStableFunc(leaders, func(a, b *Fleet) int {
				ja, jb := readPatrolJob(a), readPatrolJob(b)
				ea := ja.Leader == a.Num && ja.Enemy == enemy.PlayerNum && ja.Target == enemy.Num
				eb := jb.Leader == b.Num && jb.Enemy == enemy.PlayerNum && jb.Target == enemy.Num
				if ea != eb {
					if ea {
						return -1
					}
					return 1
				}
				da, db := a.Position.DistanceTo(enemy.Position)/max(1, a.StrategicSpeed(automaticMaxWarp(p, a))), b.Position.DistanceTo(enemy.Position)/max(1, b.StrategicSpeed(automaticMaxWarp(p, b)))
				if da < db {
					return -1
				}
				if da > db {
					return 1
				}
				return a.Num - b.Num
			})
			needsRecon := patrolNeedsRecon(p, enemy)
			if !needsRecon && t.planPatrolRelief(p, enemy, leaders) {
				delete(e.Deferred, mission.Key)
				covered[point] = true
				continue
			}
			if immediate || staged[point] {
				continue
			}
			if t.continuePatrolRelief(p, enemy, leaders) || t.stagePatrolRelief(p, enemy, leaders) {
				staged[point] = true
				delete(e.Deferred, mission.Key)
				continue
			}
			if needsRecon {
				continue
			}
			for _, leader := range leaders[:min(3, len(leaders))] {
				if leader.Delete || e.DefenceMembers[leader.Num] {
					continue
				}
				j := readPatrolJob(leader)
				if j.Leader != leader.Num || j.Enemy != enemy.PlayerNum || j.Target != enemy.Num {
					t.detachPatrolMember(p, leader)
					j = patrolJob{Leader: leader.Num, Enemy: enemy.PlayerNum, Target: enemy.Num, GatheringSince: t.game.Year}
				}
				delete(e.Deferred, mission.Key)
				savePatrolJob(leader, j)
				t.advanceAssessedPatrol(p, leader, enemy, &j)
				assigned := false
				for _, f := range t.game.getFleets(p.Num) {
					j := readPatrolJob(f)
					if !f.Delete && j.Leader != 0 && j.Enemy == enemy.PlayerNum && j.Target == enemy.Num {
						e.DefenceMembers[f.Num] = true
						assigned = true
					}
				}
				if assigned {
					covered[point] = patrolAttackArrivesThisTurn(p, t.game.getFleets(p.Num), enemy)
					staged[point] = !covered[point]
					delete(e.Deferred, mission.Key)
					break
				}
			}
		}
	}

}

func patrolAttackArrivesThisTurn(p *Player, fleets []*Fleet, enemy *Fleet) bool {
	point := patrolInterceptPosition(enemy)
	for _, f := range fleets {
		j := readPatrolJob(f)
		if f.Delete || j.Leader != f.Num || j.Provisional || j.Enemy != enemy.PlayerNum || j.Target != enemy.Num {
			continue
		}
		if f.Position == point && len(f.Waypoints) == 1 {
			return true
		}
		if len(f.Waypoints) != 2 || f.Waypoints[1].Position != point {
			continue
		}
		wp := f.Waypoints[1]
		if wp.WarpSpeed == StargateWarpSpeed || f.Position.DistanceTo(point) <= f.StrategicSpeed(wp.WarpSpeed) {
			return true
		}
	}
	return false
}

func urgentPatrolTarget(p *Player, enemy *Fleet) bool {
	if enemy == nil {
		return false
	}
	bombing, deadline := patrolColonyDeadline(p, enemy)
	return bombing || deadline <= 2 && (patrolReportedBomber(p, enemy) || patrolReportedCombatant(p, enemy) || defenceReportedTransport(p, enemy))
}

func defenceReportedTransport(p *Player, f *Fleet) bool {
	if f.Spec.CargoCapacity > 0 || patrolReportedTransport(p, f) {
		return true
	}
	for _, token := range f.Tokens {
		if d := p.GetShipDesignIntel(f.PlayerNum, token.DesignNum); d != nil && d.Spec.CargoCapacity > 0 {
			return true
		}
	}
	return false
}

func defencePatrolCommitted(p *Player, f *Fleet) bool {
	return p.patrolEvaluator != nil && p.patrolEvaluator.DefenceMembers[f.Num]
}

// Opportunistic searches may spend the normal budget after defence is assigned.
// Final departure/pursuit still needs a real forecast of the actual encounter.
func patrolFleetEvaluator(p *Player, f *Fleet) *patrolEvaluator {
	e := p.patrolEvaluator
	if e == nil || !defencePatrolCommitted(p, f) {
		return e
	}
	if e.DefenceChecks == nil {
		e.DefenceChecks = &patrolEvaluator{rules: e.rules, cache: map[[32]byte]patrolAssessment{}, Limit: 4096}
	}
	return e.DefenceChecks
}
