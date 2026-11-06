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
	defer func() { e.PlanningDefence = false }()
	launched := t.launchAssembledPatrolDefence(p)
	for _, mission := range p.patrolQueue {
		if e.Work >= e.Limit {
			break
		}
		if launched[mission.Key] || mission.Kind != "attack" || mission.Tier > 1 || mission.Deadline > 2 {
			continue
		}
		enemy := patrolEnemy(p, patrolJob{Enemy: mission.Enemy, Target: mission.Target})
		if enemy == nil || patrolNeedsRecon(p, enemy) {
			continue
		}
		if mission.Tier == 1 && !patrolReportedBomber(p, enemy) && !patrolReportedCombatant(p, enemy) && !defenceReportedTransport(p, enemy) {
			continue
		}
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
				delete(e.Deferred, mission.Key)
				break
			}
		}
	}
}

func urgentPatrolTarget(p *Player, enemy *Fleet) bool {
	if enemy == nil {
		return false
	}
	bombing, deadline := patrolColonyDeadline(p, enemy)
	return bombing || deadline <= 2 && (patrolReportedBomber(p, enemy) || patrolReportedCombatant(p, enemy) || defenceReportedTransport(p, enemy))
}

func defenceReportedTransport(p *Player, f *Fleet) bool {
	if f.Spec.CargoCapacity > 0 {
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
