package cs

import "encoding/json"

const collectionBattlePlanTag = "logistics-collection-battle-plan"

type collectionBattlePlan struct {
	Previous int `json:"previous"`
	Assigned int `json:"assigned"`
}

func setCollectionBattlePlan(p *Player, f *Fleet) {
	if p.Automation.FleetRoles[f.Num] != "freighter" {
		return
	}
	var previous collectionBattlePlan
	if json.Unmarshal([]byte(f.GetTag(collectionBattlePlanTag)), &previous) == nil {
		return
	}
	planNum := -1
	for _, plan := range p.BattlePlans {
		if plan.Num > 0 && plan.Tactic == BattleTacticDisengage {
			planNum = plan.Num
			break
		}
	}
	if planNum < 0 {
		planNum = p.GetNextBattlePlanNum()
		if planNum == 0 {
			planNum = 1
		}
		p.BattlePlans = append(p.BattlePlans, BattlePlan{Num: planNum, Name: "Debris collection retreat", Tactic: BattleTacticDisengage, PrimaryTarget: BattleTargetNone, SecondaryTarget: BattleTargetNone, AttackWho: BattleAttackWhoEnemiesAndNeutrals})
	}
	data, _ := json.Marshal(collectionBattlePlan{Previous: f.BattlePlanNum, Assigned: planNum})
	f.SetTag(collectionBattlePlanTag, string(data))
	f.BattlePlanNum = planNum
}

func restoreCollectionBattlePlan(p *Player, f *Fleet, force bool) {
	var saved collectionBattlePlan
	if json.Unmarshal([]byte(f.GetTag(collectionBattlePlanTag)), &saved) != nil {
		return
	}
	job := logisticsWork(f)
	if !force && p.Automation.FleetRoles[f.Num] == "freighter" && (f.GetTag(anticipatedSalvageTag) != "" || f.GetTag(containmentTag) != "" || job != nil && job.Kind == "salvage") {
		return
	}
	if f.BattlePlanNum == saved.Assigned {
		f.BattlePlanNum = saved.Previous
	}
	delete(f.Tags, collectionBattlePlanTag)
}

// Only a superior hunter physically reaching this same battle supplies cover.
func collectionArrivalCover(p *Player, f *Fleet, fleets []*Fleet) int {
	var pending anticipatedSalvage
	if json.Unmarshal([]byte(f.GetTag(anticipatedSalvageTag)), &pending) != nil || len(f.Waypoints) != 2 {
		return 0
	}
	if f.Waypoints[1].Position != pending.Position || f.Position.DistanceTo(pending.Position) > f.StrategicSpeed(f.Waypoints[1].WarpSpeed)+1 {
		return 0
	}
	for _, hunter := range fleets {
		if hunter.Num == pending.Hunter && !hunter.Delete && patrolCanAttack(p, hunter) && len(hunter.Waypoints) == 2 && hunter.Waypoints[1].Position == pending.Position && hunter.Position.DistanceTo(pending.Position) <= hunter.StrategicSpeed(hunter.Waypoints[1].WarpSpeed)+1 {
			return patrolPower(hunter)
		}
	}
	return 0
}
