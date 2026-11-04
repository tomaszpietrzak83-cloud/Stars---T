package cs

import "strings"

// designBattlePlan is evaluated per physical battle stack, so a merged fleet
// can keep its beam ships and missile ships at different effective ranges.
// An explicitly selected non-default fleet plan overrides design preferences.
func designBattlePlan(rules *Rules, player *Player, design *ShipDesign, fleetPlan BattlePlan) BattlePlan {
	if !player.AIControlled && dedicatedMinesweeperSpec(design.Spec) {
		fleetPlan.Name, fleetPlan.Tactic = "Minesweeper retreat", BattleTacticDisengage
		fleetPlan.PrimaryTarget, fleetPlan.SecondaryTarget = BattleTargetNone, BattleTargetNone
		return fleetPlan
	}
	if fleetPlan.Num != 0 || design.Spec.Starbase {
		return fleetPlan
	}
	if preferred := design.Spec.DefaultBattlePlan; preferred > 0 {
		for _, plan := range player.BattlePlans {
			if plan.Num == preferred-1 {
				return plan
			}
		}
	}
	plan := fleetPlan
	if !design.Spec.HasWeapons {
		plan.Name, plan.Tactic = "Chicken", BattleTacticDisengage
		plan.PrimaryTarget, plan.SecondaryTarget = BattleTargetNone, BattleTargetNone
		return plan
	}
	beamPower, torpedoPower := 0, 0
	for _, slot := range design.Spec.WeaponSlots {
		component := rules.techs.GetHullComponent(slot.HullComponent)
		if component == nil {
			continue
		}
		power := component.Power * slot.Quantity
		if component.Category == TechCategoryTorpedo {
			torpedoPower += power
		} else if !component.DamageShieldsOnly {
			beamPower += power
		}
	}
	plan.PrimaryTarget, plan.SecondaryTarget = BattleTargetArmedShips, BattleTargetAny
	if torpedoPower > 0 && torpedoPower >= beamPower {
		for _, candidate := range player.BattlePlans {
			if strings.EqualFold(candidate.Name, "Sniper") {
				plan = candidate
				break
			}
		}
		// Automatic mode guarantees ranged tactics, even in saves whose original
		// Sniper was the old unarmed-target/disengage preset. Explicit choices
		// above retain every user-edited field instead.
		plan.Name, plan.Tactic = "Sniper", BattleTacticMaximizeRange
		plan.PrimaryTarget, plan.SecondaryTarget = BattleTargetArmedShips, BattleTargetAny
	} else if beamPower > 0 {
		plan.Name, plan.Tactic = "Beam assault", BattleTacticMaximizeDamage
	} else {
		plan.Name, plan.Tactic = "Shield support", BattleTacticMaximizeDamageRatio
	}
	return plan
}
