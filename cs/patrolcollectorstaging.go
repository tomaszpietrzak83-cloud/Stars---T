package cs

import (
	"encoding/json"
)

// A collector must start approaching before the final one-turn interception.
// Its safe staging point is separate from permission to enter an actual battle.
func (t *turnGenerator) stagePatrolCollector(p *Player, hunter, enemy *Fleet, battle Vector) {
	if !patrolCanAttack(p, hunter) {
		battle = patrolInterceptPosition(enemy)
	}
	for _, f := range t.game.getFleets(p.Num) {
		var pending anticipatedSalvage
		if json.Unmarshal([]byte(f.GetTag(anticipatedSalvageTag)), &pending) == nil && pending.Enemy == enemy.PlayerNum && pending.Target == enemy.Num {
			return
		}
	}
	var chosen *Fleet
	var route *expeditionRoute
	amount := estimatedPatrolDebris(p, enemy)
	capacityLimit := t.salvageCollectorCapacityLimit(p, amount)
	for _, f := range t.game.getFleets(p.Num) {
		if f.Spec.CargoCapacity > capacityLimit || f.Delete || f.Starbase || p.Automation.FleetRoles[f.Num] != "freighter" || f.CargoMass() != 0 || f.Spec.CargoCapacity <= 0 || f.InDesignMaintenance() || f.InAutomaticSafety() || f.GetTag("maintenance-pending") != "" || fuelNetworkManaged(f) || readTankerJob(f) != nil || f.GetTag(tankerAwaitTag) != "" || p.FleetInActiveAssembly(f) || f.GetTag(traderConvoyTag) != "" || f.GetTag(anticipatedSalvageTag) != "" {
			continue
		}
		if work := logisticsWork(f); work != nil && work.Kind != "return" {
			continue
		}
		stand := max(40, min(70, f.StrategicSpeed(automaticMaxWarp(p, f))-8))
		position := battle.ToFloat64().Add(f.Position.Subtract(battle).Normalized().Scale(stand)).ToInt(true)
		if f.Position.DistanceTo(battle) <= stand {
			position = f.Position
		}
		candidate := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, position, MapObjectTarget{}, automaticRoutePolicy{Fast: true, MaxTurns: 8})
		if candidate == nil || candidate.WaitForFuel {
			continue
		}
		if chosen == nil || betterSalvageCollector(f.Spec.CargoCapacity, candidate.Turns, f.Num, chosen.Spec.CargoCapacity, route.Turns, chosen.Num, amount) {
			chosen, route = f, candidate
		}
	}
	if chosen == nil {
		return
	}
	pending := anticipatedSalvage{Position: route.Waypoints[len(route.Waypoints)-1].Position, Until: t.game.Year + route.Turns + 2, Hunter: hunter.Num, Enemy: enemy.PlayerNum, Target: enemy.Num, Staging: true}
	raw, _ := json.Marshal(pending)
	chosen.SetTag(anticipatedSalvageTag, string(raw))
	delete(chosen.Tags, logisticsTag)
	delete(chosen.Tags, containmentTag)
	setCollectionBattlePlan(p, chosen)
	applyExpeditionRoute(p, chosen, route)
	chosen.SetTag(fuelStatusTag, "Staging near a future patrol battle; actual pickup and covered arrival remain separate")
}
