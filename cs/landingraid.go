package cs

import "fmt"

const landingRaidTag = "automatic-landing-raid"

func (t *turnGenerator) landingEscortRaid(p *Player, leader *Fleet, op *landingOperation) {
	if leader.Position != op.Rally || leader.InAutomaticSafety() {
		return
	}
	present, incoming := landingOrbitalReports(p, op.Rally)
	if len(present)+len(incoming) != 0 {
		return
	}
	guards := []*Fleet{}
	ships := 0
	for _, f := range t.landingMembers(p, leader) {
		role := p.Automation.FleetRoles[f.Num]
		if role != "patrol" && role != "escort" {
			continue
		}
		if f.GetTag(landingRaidTag) != "" {
			return
		}
		if f.Position == op.Rally && f.Spec.HasWeapons && !f.InAutomaticSafety() && !f.InDesignMaintenance() && !fuelNetworkManaged(f) && !defencePatrolCommitted(p, f) && f.GetTag("automatic-landing-screen") == "" {
			guards = append(guards, f)
			ships += f.Spec.TotalShips
		}
	}
	// A quiet rally can lend one ship while the remaining escort stays with
	// the transports. The full escort requirement still applies at launch.
	reserve := max(1, p.Automation.transportPolicy().LandingEscortShips-1)
	for _, f := range guards {
		if f.Spec.TotalShips != 1 || ships-1 < reserve || patrolNeededForColony(p, f) {
			continue
		}
		for _, enemy := range sortedPatrolTargets(p, f) {
			if enemy == nil || enemy.Delete || enemy.Starbase || enemy.ReportAge != 0 || enemy.PlayerNum == p.Num || p.IsFriend(enemy.PlayerNum) ||
				reportedFleetShips(enemy) > 3 || patrolNeedsRecon(p, enemy) || declinedPatrolTarget(f, enemy, t.game.Year) {
				continue
			}
			if f.Position.DistanceTo(patrolInterceptPosition(enemy)) > f.StrategicSpeed(automaticMaxWarp(p, f))+3 {
				continue
			}
			key := fmt.Sprintf("%d:%d", enemy.PlayerNum, enemy.Num)
			claimed := false
			for _, other := range t.game.getFleets(p.Num) {
				job := readPatrolJob(other)
				if !other.Delete && (other.GetTag(landingRaidTag) == key || job.Leader != 0 && job.Enemy == enemy.PlayerNum && job.Target == enemy.Num || job.CleanupEnemy == enemy.PlayerNum && job.CleanupTarget == enemy.Num) {
					claimed = true
					break
				}
			}
			if claimed {
				continue
			}
			route, forecast := t.patrolCleanupRoute(p, f, enemy, op.Rally, 2)
			if route == nil {
				continue
			}
			f.SetTag(landingRaidTag, key)
			applyPatrolBattlePlan(p, f, forecast.Tactic)
			applyExpeditionRoute(p, f, route)
			return
		}
	}
}
