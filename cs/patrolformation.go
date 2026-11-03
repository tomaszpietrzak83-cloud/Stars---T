package cs

import (
	"math"
	"slices"
)

func compatiblePatrolPlans(a, b *Fleet) bool {
	aa, ab := a.BattlePlanNum == 0 || isManagedPatrolPlan(a), b.BattlePlanNum == 0 || isManagedPatrolPlan(b)
	return aa && ab || !aa && !ab && a.BattlePlanNum == b.BattlePlanNum
}

// Same token/damage arithmetic as native Merge, without orders, transfers,
// deletion or messages. The native merge is still required before departure.
func patrolMergedFormation(rules *Rules, p *Player, group []*Fleet) *Fleet {
	if len(group) == 0 {
		return nil
	}
	result := *group[0]
	result.Tokens = slices.Clone(group[0].Tokens)
	for _, f := range group[1:] {
		if !compatiblePatrolPlans(group[0], f) {
			return nil
		}
		for _, token := range f.Tokens {
			existing := result.getTokenByDesign(token.DesignNum)
			if existing == nil {
				result.Tokens = append(result.Tokens, token)
				continue
			}
			existing.Quantity += token.Quantity
			if token.QuantityDamaged > 0 {
				damage := token.QuantityDamaged*int(token.Damage) + existing.QuantityDamaged*int(existing.Damage)
				existing.QuantityDamaged += token.QuantityDamaged
				existing.Damage = math.Floor(float64(damage) / float64(existing.QuantityDamaged))
			}
		}
		result.Cargo = result.Cargo.Add(f.Cargo)
		result.Fuel += f.Fuel
	}
	result.Spec = ComputeFleetSpec(rules, p, &result)
	return &result
}
