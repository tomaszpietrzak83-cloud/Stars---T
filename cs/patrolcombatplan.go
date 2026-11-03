package cs

import "encoding/json"

const patrolBattlePlanTag = "patrol-battle-plan"

type patrolManagedPlan struct{ Previous, Assigned int }

func automaticPatrolBattlePlan(p *Player, f *Fleet) bool {
	if f.BattlePlanNum != 0 && !isManagedPatrolPlan(f) {
		return false
	}
	for _, token := range f.Tokens {
		if d := p.GetDesign(token.DesignNum); d != nil && d.Spec.DefaultBattlePlan > 0 {
			return false
		}
	}
	return true
}

func isManagedPatrolPlan(f *Fleet) bool {
	var saved patrolManagedPlan
	return json.Unmarshal([]byte(f.GetTag(patrolBattlePlanTag)), &saved) == nil && saved.Assigned == f.BattlePlanNum
}

func restorePatrolBattlePlan(p *Player, f *Fleet) {
	var saved patrolManagedPlan
	if json.Unmarshal([]byte(f.GetTag(patrolBattlePlanTag)), &saved) != nil {
		return
	}
	if f.BattlePlanNum == saved.Assigned {
		f.BattlePlanNum = saved.Previous
		plan := patrolPlanForFleet(p, f)
		f.battlePlan = &plan
	}
	delete(f.Tags, patrolBattlePlanTag)
}

func applyPatrolBattlePlan(p *Player, f *Fleet, tactic BattleTactic) {
	if !automaticPatrolBattlePlan(p, f) {
		restorePatrolBattlePlan(p, f)
		return
	}
	if f.BattlePlanNum != 0 && !isManagedPatrolPlan(f) {
		return
	}
	if tactic == "" {
		restorePatrolBattlePlan(p, f)
		return
	}
	previous := f.BattlePlanNum
	if isManagedPatrolPlan(f) {
		var saved patrolManagedPlan
		_ = json.Unmarshal([]byte(f.GetTag(patrolBattlePlanTag)), &saved)
		previous = saved.Previous
	}
	number := 0
	for _, plan := range p.BattlePlans {
		if plan.Num > 0 && plan.Name == "Patrol: "+string(tactic) && plan.Tactic == tactic && plan.PrimaryTarget == BattleTargetArmedShips && plan.SecondaryTarget == BattleTargetAny && plan.AttackWho == BattleAttackWhoEnemies {
			number = plan.Num
			break
		}
	}
	if number == 0 {
		number = p.GetNextBattlePlanNum()
		p.BattlePlans = append(p.BattlePlans, BattlePlan{Num: number, Name: "Patrol: " + string(tactic), Tactic: tactic, PrimaryTarget: BattleTargetArmedShips, SecondaryTarget: BattleTargetAny, AttackWho: BattleAttackWhoEnemies})
	}
	raw, _ := json.Marshal(patrolManagedPlan{Previous: previous, Assigned: number})
	f.SetTag(patrolBattlePlanTag, string(raw))
	f.BattlePlanNum = number
	plan := patrolPlanForFleet(p, f)
	f.battlePlan = &plan
}
