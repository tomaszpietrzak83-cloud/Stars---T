//go:build !wasi && !wasm

package cs

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type mixedReport struct {
	Hull  string
	Slots []ShipDesignSlot
	Known bool
}
type mixedCandidate struct {
	Counts      []int
	Cost, Ships int
}
type mixedOutcome struct {
	Wiped, Unfinished  bool
	Escaped            bool
	LostCost           int
	SurvivorDamageCost float64
	MaxSurvivorDamage  float64
	DamagedSurvivors   int
}
type mixedStudy struct {
	Battles int
	Cache   map[string]mixedOutcome
}

const mixedAutomatic BattleTactic = "StudyAutomatic"

func mixedMakeReports(actual []ShipDesign, intel string) []mixedReport {
	reports := []mixedReport{}
	for i, d := range actual {
		known := intel == "known" || intel == "one-hidden" && i == 0
		report := mixedReport{Hull: d.Hull, Known: known}
		if known {
			report.Slots = slices.Clone(d.Slots)
		}
		reports = append(reports, report)
	}
	return reports
}

func mixedPlan(tactic BattleTactic) BattlePlan {
	return BattlePlan{Num: 1, Name: "Study", Tactic: tactic, PrimaryTarget: BattleTargetArmedShips, SecondaryTarget: BattleTargetAny, AttackWho: BattleAttackWhoEnemies}
}

func mixedCost(d ShipDesign) int {
	// One explicit reference price for resources and minerals; not a live scarcity model.
	p := proposalTestPlayer()
	var err error
	d.Spec, err = ComputeShipDesignSpec(&rules, p.TechLevels, p.Race.Spec, &d)
	if err != nil {
		panic(err)
	}
	c := d.Spec.Cost
	return c.Resources + c.Ironium + c.Boranium + c.Germanium
}

func (s *mixedStudy) battle(own, enemy []ShipDesign, counts, enemyCounts []int, op, ep BattleTactic, seed int64) mixedOutcome {
	keyBytes, _ := json.Marshal([]any{own, enemy, counts, enemyCounts, op, ep, seed})
	key := string(keyBytes)
	if result, ok := s.Cache[key]; ok {
		return result
	}
	s.Battles++
	ownTokens, enemyTokens := []ScenarioShipToken{}, []ScenarioShipToken{}
	ownShips, enemyShips := 0, 0
	for i, n := range counts {
		if n > 0 {
			ownTokens = append(ownTokens, ScenarioShipToken{Design: own[i].Name, Quantity: n})
			ownShips += n
		}
	}
	for i, n := range enemyCounts {
		enemyTokens = append(enemyTokens, ScenarioShipToken{Design: enemy[i].Name, Quantity: n})
		enemyShips += n
	}
	scenario := TestScenario{Players: []ScenarioPlayer{
		{Player: proposalTestPlayer(), Designs: Designs(own...), BattlePlans: []BattlePlan{mixedPlan(op)}, Fleets: []ScenarioFleet{{Name: "Own", Tokens: ownTokens, BattlePlanNum: 1}}},
		{Player: proposalTestPlayer(), Designs: Designs(enemy...), BattlePlans: []BattlePlan{mixedPlan(ep)}, Fleets: []ScenarioFleet{{Name: "Enemy", Tokens: enemyTokens, BattlePlanNum: 1}}},
	}}
	if op == mixedAutomatic {
		scenario.Players[0].Fleets[0].BattlePlanNum = 0
	}
	owner, opponent := 1, 2
	if seed%2 != 0 {
		scenario.Players[0], scenario.Players[1] = scenario.Players[1], scenario.Players[0]
		owner, opponent = 2, 1
	}
	g := BuildScenario(scenario)
	p, q := g.Players[0], g.Players[1]
	p.Relations = []PlayerRelationship{{Relation: PlayerRelationFriend}, {Relation: PlayerRelationEnemy}}
	q.Relations = []PlayerRelationship{{Relation: PlayerRelationEnemy}, {Relation: PlayerRelationFriend}}
	g.Rules.ResetSeed(seed)
	b := newBattler(slog.New(slog.NewTextHandler(io.Discard, nil)), &g.Rules, 1, map[int]*Player{1: p, 2: q}, g.Fleets, nil)
	record := b.runBattle()
	for _, token := range record.Tokens {
		expected := ep
		if token.PlayerNum == owner {
			expected = op
			if op == mixedAutomatic {
				player := g.Players[owner-1]
				expected = designBattlePlan(&g.Rules, player, player.GetDesign(token.DesignNum), player.BattlePlans[0]).Tactic
			}
		}
		if token.Tactic != expected {
			panic("battle did not use the requested tactic")
		}
	}
	result := mixedOutcome{Wiped: record.Stats.ShipsDestroyedByPlayer[owner] == ownShips, Unfinished: record.Stats.ShipsDestroyedByPlayer[opponent] < enemyShips}
	enemyToken := map[int]bool{}
	for _, token := range record.Tokens {
		if token.PlayerNum == opponent {
			enemyToken[token.Num] = true
		}
	}
	for _, round := range record.ActionsPerRound {
		for _, action := range round {
			if action.Type == TokenActionRanAway && enemyToken[action.TokenNum] {
				result.Escaped = true
			}
		}
	}
	for _, token := range record.DestroyedTokens {
		if token.PlayerNum == owner {
			design := g.Players[owner-1].GetDesign(token.DesignNum)
			result.LostCost += mixedCost(*design) * token.Quantity
		}
	}
	// Persistent armor damage on living ships, not shield damage or damage
	// already counted in destroyed hulls. Scenarios start without damage.
	for _, fleet := range g.Fleets {
		if fleet.PlayerNum == owner {
			for _, token := range fleet.Tokens {
				if token.Quantity <= 0 || token.QuantityDamaged <= 0 {
					continue
				}
				fraction := token.Damage / float64(max(1, token.design.Spec.Armor))
				result.SurvivorDamageCost += float64(mixedCost(*token.design)*token.QuantityDamaged) * fraction
				result.MaxSurvivorDamage = max(result.MaxSurvivorDamage, fraction)
				result.DamagedSurvivors += token.QuantityDamaged
			}
		}
	}
	s.Cache[key] = result
	return result
}

func mixedModels(reports []mixedReport, variant int) []ShipDesign {
	designs := []ShipDesign{}
	for i, report := range reports {
		d := generatedPatrolDesign(report.Hull, (variant+i)%2, 4100+variant*101+i*17)
		if report.Known {
			d.Slots = slices.Clone(report.Slots)
		}
		// Neutral public names/IDs: hidden design names, specs and pointers are absent.
		d.Name = fmt.Sprintf("Hypothesis-%d", i)
		if err := d.Validate(&rules, proposalTestPlayer()); err != nil {
			panic(err)
		}
		designs = append(designs, d)
	}
	return designs
}

func mixedCandidates(own []ShipDesign, limit int) []mixedCandidate {
	result := []mixedCandidate{}
	for a := 0; a <= limit; a++ {
		for b := 0; b <= limit; b++ {
			for c := 0; c <= limit; c++ {
				if a+b+c == 0 {
					continue
				}
				counts := []int{a, b, c}
				cost := 0
				for i, n := range counts {
					cost += mixedCost(own[i]) * n
				}
				result = append(result, mixedCandidate{counts, cost, a + b + c})
			}
		}
	}
	slices.SortStableFunc(result, func(a, b mixedCandidate) int {
		if a.Cost != b.Cost {
			return a.Cost - b.Cost
		}
		return a.Ships - b.Ships
	})
	return result
}

func (s *mixedStudy) accepts(own []ShipDesign, c mixedCandidate, models [][]ShipDesign, enemyCounts []int, op BattleTactic, enemyPlans []BattleTactic, samples int) bool {
	for _, model := range models {
		for _, ep := range enemyPlans {
			loss := 0
			for seed := 0; seed < samples; seed++ {
				outcome := s.battle(own, model, c.Counts, enemyCounts, op, ep, int64(seed))
				if outcome.Wiped || outcome.Unfinished {
					return false
				}
				loss += outcome.LostCost
			}
			// A short screen may reject a demonstrated defeat, but four unlucky
			// loss samples must not inflate the group before the full average.
			if samples >= 64 && float64(loss)/float64(samples*c.Cost) > .25 {
				return false
			}
		}
	}
	return true
}

func mixedOwnPool(index int) []ShipDesign {
	hulls := [][]string{{Destroyer.Name, Cruiser.Name, Privateer.Name}, {Cruiser.Name, Nubian.Name, Scout.Name}}[index%2]
	own := []ShipDesign{}
	for i, h := range hulls {
		own = append(own, generatedPatrolDesign(h, (index+i)%2, 6100+index*67+i*13))
	}
	// Give one hull genuine mixed support weapons; no copied preset layouts.
	replaced := false
	for i, slot := range own[1].Slots {
		component := StaticTechStore.GetHullComponent(slot.HullComponent)
		hullSlot := StaticTechStore.GetHull(own[1].Hull).Slots[slot.HullSlotIndex-1]
		if !replaced && component.Power > 0 && hullSlot.Type&HullSlotTypeWeapon != 0 {
			own[1].Slots[i].HullComponent = PulsedSapper.Name
			replaced = true
		}
	}
	for _, d := range own {
		if err := d.Validate(&rules, proposalTestPlayer()); err != nil {
			panic(err)
		}
	}
	return own
}

func TestPatrolMixedUnknownAndPlans(t *testing.T) {
	started := time.Now()
	enemyPlans := []BattleTactic{BattleTacticMaximizeDamage, BattleTacticMaximizeRange, BattleTacticDisengage, BattleTacticMinimizeDamageToSelf, BattleTacticDisengageIfChallenged}
	ownPlans := []BattleTactic{mixedAutomatic, BattleTacticMaximizeDamage, BattleTacticMaximizeRange, BattleTacticMinimizeDamageToSelf}
	for index := 0; index < 4; index++ {
		own := mixedOwnPool(index)
		candidates := mixedCandidates(own, 3)
		hulls := [][]string{{Destroyer.Name, Cruiser.Name}, {Battleship.Name, Scout.Name}}[index%2]
		enemyCounts := [][]int{{2, 2}, {2, 6}, {4, 4}, {4, 10}}[index]
		actual := []ShipDesign{generatedPatrolDesign(hulls[0], 0, 9100+index*43), generatedPatrolDesign(hulls[1], 1, 9131+index*43)}
		manifest, err := json.Marshal(map[string]any{"pool": index, "own": own, "enemyActual": actual, "enemyCounts": enemyCounts})
		require.NoError(t, err)
		t.Logf("MIXED_MANIFEST %s", manifest)
		for _, intel := range []string{"known", "one-hidden", "hidden"} {
			t.Run(fmt.Sprintf("pool%d-%s", index, intel), func(t *testing.T) {
				study := &mixedStudy{Cache: map[string]mixedOutcome{}}
				reports := mixedMakeReports(actual, intel)
				models := [][]ShipDesign{mixedModels(reports, 0)}
				if intel != "known" {
					models = append(models, mixedModels(reports, 1), mixedModels(reports, 2))
				}
				var selected *mixedCandidate
				var chosenPlan BattleTactic
				for _, c := range candidates {
					for _, op := range ownPlans {
						if study.accepts(own, c, models, enemyCounts, op, enemyPlans, 4) && study.accepts(own, c, models, enemyCounts, op, enemyPlans, 64) {
							copy := c
							selected = &copy
							chosenPlan = op
							break
						}
					}
					if selected != nil {
						break
					}
				}
				if selected == nil {
					t.Logf("MIXED pool=%d intel=%s action=RECON_OR_REPLAN accepted=false battles=%d", index, intel, study.Battles)
					return
				}
				if intel == "known" {
					// Separate per-plan minima establish whether changing tactics
					// can avoid committing extra ships. Reuses identical battle cache.
					for _, op := range ownPlans {
						found := false
						for _, c := range candidates {
							if study.accepts(own, c, models, enemyCounts, op, enemyPlans, 4) && study.accepts(own, c, models, enemyCounts, op, enemyPlans, 64) {
								require.LessOrEqual(t, selected.Cost, c.Cost)
								t.Logf("PLAN_MIN pool=%d plan=%s counts=%v ships=%d cost=%d", index, op, c.Counts, c.Ships, c.Cost)
								found = true
								break
							}
						}
						if !found {
							t.Logf("PLAN_MIN pool=%d plan=%s unavailable=true", index, op)
						}
					}
				}
				// Counterfactual validation: unknown loadouts do not authorize a real attack.
				// Fresh variants 90/91 and true hidden slots are only used AFTER selection.
				validation := [][]ShipDesign{actual}
				if intel != "known" {
					validation = append(validation, mixedModels(reports, 90), mixedModels(reports, 91))
				}
				wiped, unfinished, trials, lostCost, escaped := 0, 0, 0, 0, 0
				perPlan := map[BattleTactic][4]int{}
				for modelIndex, model := range validation {
					modelWiped, modelUnfinished := 0, 0
					for _, ep := range enemyPlans {
						for sample := int64(0); sample < 64; sample++ {
							seed := 3000 + sample
							if sample >= 32 {
								seed = 5000 + sample - 32
							}
							outcome := study.battle(own, model, selected.Counts, enemyCounts, chosenPlan, ep, seed)
							trials++
							if outcome.Wiped {
								wiped++
								modelWiped++
							}
							if outcome.Unfinished {
								unfinished++
								modelUnfinished++
							}
							if outcome.Escaped {
								escaped++
							}
							stats := perPlan[ep]
							stats[0]++
							if outcome.Wiped {
								stats[1]++
							}
							if outcome.Unfinished {
								stats[2]++
							}
							if outcome.Escaped {
								stats[3]++
							}
							perPlan[ep] = stats
							lostCost += outcome.LostCost
						}
					}
					t.Logf("MIXED_MODEL pool=%d intel=%s model=%d trials=%d wiped=%d unfinished=%d", index, intel, modelIndex, len(enemyPlans)*64, modelWiped, modelUnfinished)
				}
				// Earlier unconditional 25% buffer is measured as committed resources, not
				// automatically accepted: an enlarged stack can change tactical behavior.
				bufferCost, bufferShips := 0, 0
				for i, n := range selected.Counts {
					buffered := (5*n + 3) / 4
					bufferCost += mixedCost(own[i]) * buffered
					bufferShips += buffered
				}
				removable := 0
				for i, n := range selected.Counts {
					if n > 0 && selected.Ships > 1 {
						reduced := *selected
						reduced.Counts = slices.Clone(selected.Counts)
						reduced.Counts[i]--
						reduced.Ships--
						reduced.Cost -= mixedCost(own[i])
						for _, op := range ownPlans {
							if study.accepts(own, reduced, models, enemyCounts, op, enemyPlans, 64) {
								removable++
								break
							}
						}
					}
				}
				require.Zero(t, removable, "a selected ship must have a demonstrated role under the planning criterion")
				action := "RECON"
				if intel == "known" {
					action = "ATTACK"
					require.Zero(t, wiped)
					require.Zero(t, unfinished)
				}
				t.Logf("MIXED pool=%d intel=%s action=%s accepted=true counts=%v ships=%d cost=%d plan=%s buffer_ships=%d buffer_cost=%d trials=%d wiped=%d unfinished=%d escaped=%d mean_loss=%.4f removable=%d battles=%d", index, intel, action, selected.Counts, selected.Ships, selected.Cost, chosenPlan, bufferShips, bufferCost, trials, wiped, unfinished, escaped, float64(lostCost)/float64(trials*selected.Cost), removable, study.Battles)
				for _, ep := range enemyPlans {
					stats := perPlan[ep]
					t.Logf("MIXED_PLAN pool=%d intel=%s enemy_plan=%s trials=%d wiped=%d unfinished=%d escaped=%d", index, intel, ep, stats[0], stats[1], stats[2], stats[3])
				}
			})
		}
	}
	t.Logf("MIXED_COMPLETE elapsed=%s", time.Since(started))
}

func TestPatrolMixedHiddenInputsAreIsolated(t *testing.T) {
	actual := []ShipDesign{generatedPatrolDesign(Destroyer.Name, 0, 9100), generatedPatrolDesign(Cruiser.Name, 1, 9131)}
	for _, intel := range []string{"hidden", "one-hidden"} {
		before := mixedMakeReports(actual, intel)
		changed := Designs(actual...)
		changed[1] = generatedPatrolDesign(Cruiser.Name, 0, 9991)
		changed[1].Name = "PRIVATE-SECRET"
		after := mixedMakeReports(changed, intel)
		require.Equal(t, before, after)
		require.Equal(t, mixedModels(before, 0), mixedModels(after, 0))
		raw, err := json.Marshal(before)
		require.NoError(t, err)
		require.NotContains(t, string(raw), "PRIVATE-SECRET")
	}
}

func TestPatrolMixedFixedCompositionsAndTactics(t *testing.T) {
	for index := 0; index < 2; index++ {
		own := mixedOwnPool(index)
		hulls := [][]string{{Destroyer.Name, Cruiser.Name}, {Battleship.Name, Scout.Name}}[index]
		actual := []ShipDesign{generatedPatrolDesign(hulls[0], 0, 9100+index*43), generatedPatrolDesign(hulls[1], 1, 9131+index*43)}
		enemyCounts := [][]int{{2, 2}, {2, 6}}[index]
		study := &mixedStudy{Cache: map[string]mixedOutcome{}}
		for _, counts := range [][]int{{1, 1, 1}, {0, 2, 1}, {1, 2, 0}} {
			for _, op := range []BattleTactic{mixedAutomatic, BattleTacticMaximizeDamage, BattleTacticMaximizeRange, BattleTacticMinimizeDamageToSelf} {
				for _, ep := range []BattleTactic{BattleTacticMaximizeDamage, BattleTacticMaximizeRange, BattleTacticDisengage, BattleTacticMinimizeDamageToSelf, BattleTacticDisengageIfChallenged} {
					wiped, unfinished, escaped := 0, 0, 0
					for seed := int64(4000); seed < 4032; seed++ {
						outcome := study.battle(own, actual, counts, enemyCounts, op, ep, seed)
						if outcome.Wiped {
							wiped++
						}
						if outcome.Unfinished {
							unfinished++
						}
						if outcome.Escaped {
							escaped++
						}
					}
					t.Logf("FIXED_MIX pool=%d counts=%v own_plan=%s enemy_plan=%s trials=32 wiped=%d unfinished=%d escaped=%d", index, counts, op, ep, wiped, unfinished, escaped)
				}
			}
		}
	}
}
