package cs

// Validate physically assembled relief before speculative composition searches
// can exhaust the forecast budget. Arrival must lead to an attack when the
// current encounter is still winnable, not another unfunded gathering cycle.
func (t *turnGenerator) launchAssembledPatrolDefence(p *Player) map[string]bool {
	launched := map[string]bool{}
	e := p.patrolEvaluator
	for _, mission := range p.patrolQueue {
		if e.Work >= e.Limit {
			break
		}
		if mission.Kind != "attack" || mission.Tier > 1 || mission.Deadline > 2 {
			continue
		}
		enemy := patrolEnemy(p, patrolJob{Enemy: mission.Enemy, Target: mission.Target})
		if enemy == nil || patrolNeedsRecon(p, enemy) || !urgentPatrolTarget(p, enemy) {
			continue
		}
		for _, leader := range t.game.getFleets(p.Num) {
			job := readPatrolJob(leader)
			if !patrolAvailable(p, leader) || e.DefenceMembers[leader.Num] || job.Leader != leader.Num || job.Enemy != mission.Enemy || job.Target != mission.Target {
				continue
			}
			members := []*Fleet{}
			clusters := map[Vector]int{}
			assembled := false
			for _, f := range t.game.getFleets(p.Num) {
				if patrolAvailable(p, f) && readPatrolJob(f).Leader == leader.Num && compatiblePatrolPlans(leader, f) {
					members = append(members, f)
					clusters[f.Position] += f.Spec.TotalShips
					assembled = assembled || clusters[f.Position] > 1
				}
			}
			if !assembled || !t.launchLocalPatrol(p, leader, enemy, &job, members) {
				continue
			}
			for _, f := range t.game.getFleets(p.Num) {
				j := readPatrolJob(f)
				if !f.Delete && j.Leader != 0 && !j.Provisional && j.Enemy == mission.Enemy && j.Target == mission.Target {
					e.DefenceMembers[f.Num] = true
				}
			}
			launched[mission.Key] = true
			delete(e.Deferred, mission.Key)
			break
		}
	}
	return launched
}
