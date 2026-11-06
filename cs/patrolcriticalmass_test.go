//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"strconv"
	"testing"
)

func TestUrgentPatrolCanReachCriticalMassAboveSearchCheckCount(t *testing.T) {
	fleets := []ScenarioFleet{}
	for i := range 400 {
		fleets = append(fleets, ScenarioFleet{Name: "Hunter " + strconv.Itoa(i), Design: DesignDestroyerDelta.Name})
	}
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
		{Designs: Designs(DesignDestroyerDelta, DesignSpaceStation), Fleets: fleets},
		{Designs: Designs(DesignDestroyerDelta), Fleets: []ScenarioFleet{{Name: "Threat", Design: DesignDestroyerDelta.Name, Quantity: 80, Position: Vector{100, 0}}}},
	}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}, {Name: "Colony", Owner: 1, Position: Vector{100, 0}, Cargo: Cargo{Colonists: 2000}}}})
	p, enemy := u.Player(1), u.Fleet("Threat")
	p.Automation.FleetRoles = map[int]string{}
	pool := []*Fleet{}
	for i := range 400 {
		f := u.Fleet("Hunter " + strconv.Itoa(i))
		p.Automation.FleetRoles[f.Num] = "patrol"
		pool = append(pool, f)
	}
	p.discoverer.discoverDesign(enemy.Tokens[0].design, true)
	p.discoverer.discoverFleet(enemy, true)
	e := &patrolEvaluator{rules: &u.Game.Rules, cache: map[[32]byte]patrolAssessment{}, Limit: 4096}
	report := p.GetFleetIntel(2, enemy.Num)
	result := e.assess(p, pool, report, enemy.Position, 1)
	t.Logf("FULL_FORCE ready=%v work=%d reason=%s", result.Ready, e.Work, result.Reason)
	require.True(t, result.Ready)
	merged := patrolMergedFormation(&u.Game.Rules, p, pool)
	work := e.Work
	require.True(t, e.assess(p, []*Fleet{merged}, report, enemy.Position, 1).Ready)
	require.Equal(t, work, e.Work, "the modeled and physically merged stack must share a forecast")
	merged.Tokens[0].QuantityDamaged, merged.Tokens[0].Damage = 1, 10
	e.assess(p, []*Fleet{merged}, report, enemy.Position, 1)
	require.Greater(t, e.Work, work, "changed damage must invalidate the shared forecast")
	u.Run((*turnGenerator).startPatrolEvaluation)
	leader := pool[0]
	j := patrolJob{Leader: leader.Num, Enemy: 2, Target: enemy.Num, GatheringSince: u.Game.Year}
	savePatrolJob(leader, j)
	u.Run(func(tg *turnGenerator) { tg.planUrgentPatrolDefence(p) }, (*turnGenerator).guardAutomaticFuel, (*turnGenerator).enforceAutomaticDeparture)
	assigned := 0
	for _, f := range u.Game.getFleets(1) {
		job := readPatrolJob(f)
		if !f.Delete && job.Enemy == 2 && job.Target == enemy.Num && job.Leader != 0 {
			assigned += f.Spec.TotalShips
		}
	}
	t.Logf("CRITICAL_MASS assigned=%d work=%d", assigned, p.patrolEvaluator.Work)
	require.Greater(t, assigned, 97, "forecast-check bounds must not cap the number of recruitable ships")
	require.Greater(t, len(leader.Waypoints), 1)
	require.LessOrEqual(t, p.patrolEvaluator.Work, 4096)
	for i := 0; i < 3 && !enemy.Delete; i++ {
		u.Run((*turnGenerator).fleetMove, (*turnGenerator).fleetBattle)
		u.Game.Year++
		u.Run((*turnGenerator).fleetInit)
	}
	require.True(t, enemy.Delete, "the large confirmed force must actually attack and win")
}
