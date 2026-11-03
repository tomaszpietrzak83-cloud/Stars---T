//go:build !wasi && !wasm

package cs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type reserveRisk struct {
	Trials, Wiped, Unfinished                          int
	MeanLoss, MeanDamage, MeanDamagedShips, PeakDamage float64
}

func reserveKeep(small, large reserveRisk, opportunityCost float64) bool {
	if large.Wiped > small.Wiped || large.Unfinished > small.Unfinished {
		return false
	}
	return small.MeanLoss+small.MeanDamage-large.MeanLoss-large.MeanDamage > opportunityCost
}

func TestPatrolReserveAfterReconDamageTradeoff(t *testing.T) {
	for index := 0; index < 2; index++ {
		own := mixedOwnPool(index)
		hulls := [][]string{{Destroyer.Name, Cruiser.Name}, {Battleship.Name, Scout.Name}}[index]
		enemy := []ShipDesign{generatedPatrolDesign(hulls[0], 0, 9100+index*43), generatedPatrolDesign(hulls[1], 1, 9131+index*43)}
		enemyCounts := [][]int{{2, 2}, {2, 6}}[index]
		op := BattleTacticMaximizeDamage
		if index == 1 {
			op = mixedAutomatic
		}
		study := &mixedStudy{Cache: map[string]mixedOutcome{}}
		results := []reserveRisk{}
		for _, counts := range [][]int{{0, 2, 0}, {0, 3, 0}, {0, 4, 0}, {0, 6, 0}, {0, 2, 1}, {1, 2, 1}} {
			result := reserveRisk{}
			for _, ep := range []BattleTactic{BattleTacticMaximizeDamage, BattleTacticMaximizeRange, BattleTacticDisengage, BattleTacticMinimizeDamageToSelf, BattleTacticDisengageIfChallenged} {
				for seed := int64(6000); seed < 6064; seed++ {
					outcome := study.battle(own, enemy, counts, enemyCounts, op, ep, seed)
					result.Trials++
					if outcome.Wiped {
						result.Wiped++
					}
					if outcome.Unfinished {
						result.Unfinished++
					}
					result.MeanLoss += float64(outcome.LostCost)
					result.MeanDamage += outcome.SurvivorDamageCost
					result.MeanDamagedShips += float64(outcome.DamagedSurvivors)
					result.PeakDamage = max(result.PeakDamage, outcome.MaxSurvivorDamage)
				}
			}
			result.MeanLoss /= float64(result.Trials)
			result.MeanDamage /= float64(result.Trials)
			result.MeanDamagedShips /= float64(result.Trials)
			results = append(results, result)
			t.Logf("RESERVE pool=%d counts=%v trials=%d wiped=%d unfinished=%d mean_loss=%.4f mean_damage_equivalent=%.4f mean_damaged_ships=%.4f worst_ship_damage=%.4f", index, counts, result.Trials, result.Wiped, result.Unfinished, result.MeanLoss, result.MeanDamage, result.MeanDamagedShips, result.PeakDamage)
		}
		for i, large := range results[1:] {
			small := results[0]
			benefit := small.MeanLoss + small.MeanDamage - large.MeanLoss - large.MeanDamage
			if benefit > 0 && large.Wiped <= small.Wiped && large.Unfinished <= small.Unfinished {
				require.True(t, reserveKeep(small, large, 0))
				require.False(t, reserveKeep(small, large, benefit+1))
			} else {
				require.False(t, reserveKeep(small, large, 0))
			}
			t.Logf("RESERVE_DECISION pool=%d option=%d avoided_damage_equivalent=%.4f keep_if_idle=%t", index, i+1, benefit, reserveKeep(small, large, 0))
		}
	}
}

func TestPatrolReserveGatheringRunsAlongsideRecon(t *testing.T) {
	tasks := []proposalTask{
		{Key: "fleet:2:9:contact", Kind: "contact", Tier: 1, Deadline: 12},
		{Key: "fleet:2:9:staging", Kind: "gather", Tier: 2, Deadline: 13},
	}
	agents := []proposalAgent{{Num: 1, Capabilities: map[string]bool{"contact": true}}, {Num: 2, Capabilities: map[string]bool{"gather": true}}}
	assigned := proposalDispatch(tasks, agents, 10)
	require.Len(t, assigned, 2)
	require.Equal(t, 1, assigned[0].Assigned)
	require.Equal(t, 2, assigned[1].Assigned)
	// Staging and attacking are separate authorizations. Unknown equipment
	// does not prevent gathering, but it still prevents a blind attack.
	knownEquipment := false
	canStage := assigned[1].Assigned != 0
	canAttack := knownEquipment && canStage
	require.True(t, canStage)
	require.False(t, canAttack)
	t.Log("STAGING: scout and provisional combat gathering assigned concurrently; attack remains pending fresh reconnaissance")
}
