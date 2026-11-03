//go:build !wasi && !wasm

package cs

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"math/rand"
	"testing"
)

func TestPatrolNativeBattleCopiesIsolateReportsStateAndRNG(t *testing.T) {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Designs: Designs(DesignBattleBeam), Fleets: []ScenarioFleet{{Name: "Own", Design: DesignBattleBeam.Name, Quantity: 3}}}, {Designs: Designs(DesignDestroyerDelta), Fleets: []ScenarioFleet{{Name: "Enemy", Design: DesignDestroyerDelta.Name, Quantity: 2}}}}})
	p, f, e := u.Player(1), u.Fleet("Own"), u.Fleet("Enemy")
	p.Automation.FleetRoles = map[int]string{f.Num: "patrol"}
	p.discoverer.discoverDesign(e.Tokens[0].design, true)
	p.discoverer.discoverFleet(e, true)
	before, err := json.Marshal(u.Game)
	require.NoError(t, err)
	u.Game.Rules.ResetSeed(123)
	model, err := newPatrolBattleCopy(&u.Game.Rules, p, []*Fleet{f}, []*Fleet{p.GetFleetIntel(2, e.Num)})
	require.NoError(t, err)
	expected := model.run(BattleTacticMaximizeDamage, BattleTacticMaximizeDamage, 44)
	for seed := int64(0); seed < 12; seed++ {
		model.run(BattleTacticMaximizeRange, BattleTacticDisengage, seed)
	}
	require.Equal(t, expected, model.run(BattleTacticMaximizeDamage, BattleTacticMaximizeDamage, 44))
	after, err := json.Marshal(u.Game)
	require.NoError(t, err)
	require.Equal(t, string(before), string(after))
	require.Equal(t, rand.New(rand.NewSource(123)).Intn(100000000), u.Game.Rules.random.Intn(100000000))
	// Mutating actual hidden enemy data cannot affect a report-only model.
	e.Tokens[0].design.Slots = nil
	e.Tokens[0].design.Spec.Armor = 999999
	e.Fuel = 999999
	e.BattlePlanNum = 999
	second, err := newPatrolBattleCopy(&u.Game.Rules, p, []*Fleet{f}, []*Fleet{p.GetFleetIntel(2, e.Num)})
	require.NoError(t, err)
	require.Equal(t, expected, second.run(BattleTacticMaximizeDamage, BattleTacticMaximizeDamage, 44))
	p.GetShipDesignIntel(2, e.Tokens[0].DesignNum).Slots = nil
	_, err = newPatrolBattleCopy(&u.Game.Rules, p, []*Fleet{f}, []*Fleet{p.GetFleetIntel(2, e.Num)})
	require.Error(t, err)
}

func TestPatrolPlanRestoresOnManualControlAndHonorsPlayerEdits(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Escort")
	p.Automation.FleetRoles[f.Num] = "patrol"
	old := f.BattlePlanNum
	applyPatrolBattlePlan(p, f, BattleTacticMaximizeDamage)
	require.True(t, isManagedPatrolPlan(f))
	require.NotEqual(t, old, f.BattlePlanNum)
	p.TakeManualControl(f)
	require.Equal(t, old, f.BattlePlanNum)
	p.Automation.FleetRoles[f.Num] = "patrol"
	applyPatrolBattlePlan(p, f, BattleTacticMaximizeRange)
	f.BattlePlanNum = 987
	p.TakeManualControl(f)
	require.Equal(t, 987, f.BattlePlanNum, "an explicit later player choice is binding")
	f.BattlePlanNum = 0
	p.GetDesign(f.Tokens[0].DesignNum).Spec.DefaultBattlePlan = 2
	applyPatrolBattlePlan(p, f, BattleTacticMaximizeDamage)
	require.Zero(t, f.BattlePlanNum, "design-level player preferences stay under native per-design control")
}

func TestProductionPatrolForecastIndependentCombatAndBudget(t *testing.T) {
	accepted := 0
	for _, pair := range [][2]ShipDesign{{DesignBattleBeam, DesignDestroyerDelta}, {DesignDestroyerDelta, DesignBattleBeam}, {DesignJihadCruiser, DesignDestroyerDelta}} {
		u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
			{Designs: Designs(pair[0]), Fleets: []ScenarioFleet{{Name: "Own", Design: pair[0].Name, Quantity: 8}}},
			{Designs: Designs(pair[1]), Fleets: []ScenarioFleet{{Name: "Enemy", Design: pair[1].Name, Quantity: 2}}},
		}})
		p, f, e := u.Player(1), u.Fleet("Own"), u.Fleet("Enemy")
		p.Automation.FleetRoles = map[int]string{f.Num: "patrol"}
		p.discoverer.discoverDesign(e.Tokens[0].design, true)
		p.discoverer.discoverFleet(e, true)
		u.Run((*turnGenerator).startPatrolEvaluation)
		report := p.GetFleetIntel(2, e.Num)
		result := p.patrolEvaluator.assess(p, []*Fleet{f}, report, report.Position, 1)
		if !result.Ready {
			continue
		}
		accepted++
		model, err := newPatrolBattleCopy(&u.Game.Rules, p, []*Fleet{f}, []*Fleet{report})
		require.NoError(t, err)
		for _, ep := range []BattleTactic{"", BattleTacticMaximizeDamage, BattleTacticMaximizeRange, BattleTacticDisengage, BattleTacticMinimizeDamageToSelf, BattleTacticDisengageIfChallenged} {
			for seed := int64(50000); seed < 50032; seed++ {
				outcome := model.run(result.Tactic, ep, seed)
				require.False(t, outcome.Wiped)
				require.False(t, outcome.Unfinished)
			}
		}
		battles := p.patrolEvaluator.Battles
		require.Equal(t, result, p.patrolEvaluator.assess(p, []*Fleet{f}, report, report.Position, 1))
		require.Equal(t, battles, p.patrolEvaluator.Battles, "cache must avoid repeated native work")
		p.patrolEvaluator.cache = map[[32]byte]patrolAssessment{}
		p.patrolEvaluator.Limit = p.patrolEvaluator.Work
		require.False(t, p.patrolEvaluator.assess(p, []*Fleet{f}, report, report.Position, 1).Ready, "budget exhaustion cannot authorize attack")
	}
	require.Positive(t, accepted)
}
