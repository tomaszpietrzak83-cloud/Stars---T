//go:build !wasi && !wasm

package cs

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNativePatrolPrefixLeavesBudgetForFurtherRelief(t *testing.T) {
	fleets := []ScenarioFleet{}
	for i := range 64 {
		fleets = append(fleets, ScenarioFleet{Name: "Hunter " + strconv.Itoa(i), Design: DesignDestroyerDelta.Name})
	}
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
		{Designs: Designs(DesignDestroyerDelta), Fleets: fleets},
		{Designs: Designs(DesignDestroyerDelta), Fleets: []ScenarioFleet{{Name: "Threat", Design: DesignDestroyerDelta.Name, Quantity: 12, Position: Vector{100, 0}}}},
	}})
	p, enemy := u.Player(1), u.Fleet("Threat")
	p.Automation.FleetRoles = map[int]string{}
	pool := []*Fleet{}
	for i := range 64 {
		f := u.Fleet("Hunter " + strconv.Itoa(i))
		p.Automation.FleetRoles[f.Num] = "patrol"
		pool = append(pool, f)
	}
	p.discoverer.discoverDesign(enemy.Tokens[0].design, true)
	p.discoverer.discoverFleet(enemy, true)
	u.Run((*turnGenerator).startPatrolEvaluation)
	report := p.GetFleetIntel(2, enemy.Num)
	checks := 0
	group, result := readyPatrolPrefix(p, pool, report, enemy.Position, &checks, 32)
	require.True(t, result.Ready, result.Reason)
	require.Greater(t, len(group), 1)
	require.Less(t, len(group), len(pool), "do not reserve every hunter")
	require.Less(t, p.patrolEvaluator.Work, p.patrolEvaluator.Limit/2, "leave capacity for other urgent colonies")
	independent := &patrolEvaluator{rules: &u.Game.Rules, cache: map[[32]byte]patrolAssessment{}, Limit: 4096}
	require.True(t, independent.assess(p, group, report, enemy.Position, 1).Ready)
	t.Logf("PREFIX ships=%d checks=%d work=%d", len(group), checks, p.patrolEvaluator.Work)
}

func TestCopiedSimultaneousDefenceBudget(t *testing.T) {
	path := os.Getenv("STARS_DEFENCE_BUDGET_SNAPSHOT")
	if path == "" {
		t.Skip("optional private copied 2492 snapshot")
	}
	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()
	z, err := gzip.NewReader(file)
	require.NoError(t, err)
	defer z.Close()
	var game FullGame
	require.NoError(t, json.NewDecoder(z).Decode(&game))
	require.Equal(t, 2492, game.Year)
	game.TechStore = &StaticTechStore
	game.Rules.SetTechStore(&StaticTechStore)
	game.Year++
	tg := newTurnGenerator(&game)
	tg.ordersPlayerNum = 1
	require.NoError(t, game.computeSpecs())
	tg.startPatrolEvaluation()
	tg.enablePlanningThreatIndex()
	defer tg.disablePlanningThreatIndex()
	tg.refreshAutomaticFuelIntel()
	tg.releaseUnreachablePatrolSupport()
	p := game.getPlayer(1)
	tg.reconcilePatrolJobs(p)
	// Preserve the real priority order; no manual promotion or larger budget.
	tg.planUrgentPatrolDefence(p)
	tg.guardAutomaticFuel()
	tg.enforceAutomaticDeparture()
	assigned := map[string]int{}
	blocked := 0
	for _, f := range game.getFleets(1) {
		if !f.Delete && p.patrolEvaluator.DefenceMembers[f.Num] {
			j := readPatrolJob(f)
			assigned[priorityFleetKey(j.Enemy, j.Target)] += f.Spec.TotalShips
			if len(f.Waypoints) < 2 {
				blocked++
			}
		}
	}
	t.Logf("SIMULTANEOUS_DEFENCE assigned=%v blocked=%d work=%d limit=%d", assigned, blocked, p.patrolEvaluator.Work, p.patrolEvaluator.Limit)
	require.Positive(t, assigned["16:25"])
	require.Positive(t, assigned["16:16"])
	require.Zero(t, blocked)
	require.LessOrEqual(t, p.patrolEvaluator.Work, 4096)
}
