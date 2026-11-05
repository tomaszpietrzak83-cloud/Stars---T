package cs

import (
	"encoding/json"
	"strconv"
)

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
			num, _ := strconv.Atoi(f.GetTag(escortTag))
			target := t.game.getFleet(p.Num, num)
			if target == nil || target.Delete || !t.fundedPatrolSupport(p, f, target) {
				releaseBorrowedHunter(p, f)
			}
		}
	}
}
