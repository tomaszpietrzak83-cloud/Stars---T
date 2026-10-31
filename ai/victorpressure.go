package ai

import "github.com/sirgwain/craig-stars/cs"

// Publicly declared victory makes the human the strategic focus. This does not
// reveal hidden colonies, grant resources, or dismantle territorial defenses.
func (ai *aiPlayer) applyVictorPressure() {
	if len(ai.priorityVictors) == 0 {
		return
	}
	for num := range ai.priorityVictors {
		if num > 0 && num <= len(ai.Relations) {
			ai.Relations[num-1].Relation = cs.PlayerRelationEnemy
		}
	}
	ai.config.startAttackingYear = 0
	ai.config.pace = max(ai.config.pace, 1.75)
	ai.config.fleetProductionCutoff = min(ai.config.fleetProductionCutoff, .35)
	ai.config.bomberProductionCutoff = min(ai.config.bomberProductionCutoff, .5)
}
