package cs

import (
	"encoding/json"
	"slices"
	"strconv"
)

// Nearby colony relief has priority over lending hunters to civilian escorts.
// Dedicated escorts and fleets already travelling in a fuel network retain
// their existing responsibilities; this only reserves the borrowed patrol pool.
func patrolNeededForColony(p *Player, f *Fleet) bool {
	if p.Automation.FleetRoles[f.Num] != "patrol" {
		var old bomberBorrow
		if json.Unmarshal([]byte(f.GetTag(bomberBorrowTag)), &old) != nil || old.Role != "patrol" {
			return false
		}
	}
	queue := p.patrolQueue
	if queue == nil {
		queue = makePatrolMissionQueue(p, 0)
	}
	for _, mission := range queue {
		if mission.Kind != "attack" || mission.Tier > 1 || mission.Deadline > 1 {
			continue
		}
		enemy := p.GetFleetIntel(mission.Enemy, mission.Target)
		if enemy != nil && enemy.ReportAge == 0 && f.Position.DistanceTo(patrolInterceptPosition(enemy)) <= f.StrategicSpeed(automaticMaxWarp(p, f))+3 && (patrolReportedBomber(p, enemy) || patrolReportedCombatant(p, enemy) || defenceReportedTransport(p, enemy)) {
			return true
		}
	}
	return false
}

func (t *turnGenerator) fundedPatrolSupport(p *Player, f, target *Fleet) bool {
	if f.Position == target.Position {
		return true
	}
	if f.Position.DistanceTo(target.Position) > 2*f.StrategicSpeed(automaticMaxWarp(p, f)) {
		return false
	}
	route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, target.Position, MapObjectTarget{}, automaticRoutePolicy{Fast: true, AllowThreat: true, MaxTurns: 2, MinWarp: automaticCruiseWarp(p, f)})
	return route != nil && !route.WaitForFuel
}

func releaseBorrowedHunter(p *Player, f *Fleet) bool {
	var old bomberBorrow
	if json.Unmarshal([]byte(f.GetTag(bomberBorrowTag)), &old) != nil || old.Role != "patrol" {
		return false
	}
	delete(f.Tags, escortTag)
	restoreBomberProtection(p, f)
	// Old support fuel requests would otherwise reclaim the released hunter.
	if !fuelNetworkManaged(f) && readTankerJob(f) == nil {
		delete(f.Tags, fuelRequestTag)
		delete(f.Tags, fuelResumeTag)
	}
	holdAutomaticFleet(p, f, "Support unavailable promptly; released to independent patrol")
	return true
}

func (t *turnGenerator) releaseUnreachablePatrolSupport() {
	for _, p := range t.automaticPlayers() {
		for _, f := range t.game.getFleets(p.Num) {
			if f.Delete || landingManaged(f) || f.GetTag(bomberBorrowTag) == "" || fuelNetworkManaged(f) || readTankerJob(f) != nil {
				continue
			}
			if patrolNeededForColony(p, f) && releaseBorrowedHunter(p, f) {
				continue
			}
			num, _ := strconv.Atoi(f.GetTag(escortTag))
			target := t.game.getFleet(p.Num, num)
			excluded := false
			if target != nil {
				if plan := readFuelNetworkPlan(target); plan != nil {
					excluded = !slices.Contains(plan.Members, f.Num)
				}
			}
			if target == nil || target.Delete || excluded || !t.fundedPatrolSupport(p, f, target) {
				releaseBorrowedHunter(p, f)
			}
		}
	}
}

// A route can leave with a smaller convoy than the earlier escort reservation.
// Release excluded hunters in the same orders phase, before ships move apart.
func (t *turnGenerator) releaseDepartedPatrolSupport() {
	for _, p := range t.automaticPlayers() {
		for _, f := range slices.Clone(t.game.getFleets(p.Num)) {
			if f.Delete || landingManaged(f) || f.GetTag(bomberBorrowTag) == "" || fuelNetworkManaged(f) || readTankerJob(f) != nil {
				continue
			}
			num, _ := strconv.Atoi(f.GetTag(escortTag))
			target := t.game.getFleet(p.Num, num)
			if target == nil || len(target.Waypoints) < 2 {
				continue
			}
			plan := readFuelNetworkPlan(target)
			if plan == nil || slices.Contains(plan.Members, f.Num) || !releaseBorrowedHunter(p, f) {
				continue
			}
			if patrolAvailable(p, f) && !t.assignSoloPatrol(p, f) {
				t.patrolFrontier(p, f)
			}
		}
	}
}
