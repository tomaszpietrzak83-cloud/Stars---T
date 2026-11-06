//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPatrolDefencePreemptsHuntingAndReachesBombers(t *testing.T) {
	bomber := campaignDesign("Invading bomber", MiniBomber.Name, "bomber")
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
		{Designs: Designs(DesignJihadCruiser, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Ready", Design: DesignJihadCruiser.Name, Quantity: 8, Position: Vector{0, 0}}}},
		{Designs: Designs(DesignDestroyerDelta, bomber, DesignLongRangeScout), Fleets: []ScenarioFleet{{Name: "Bombarding", Position: Vector{50, 0}, Tokens: []ScenarioShipToken{{Design: DesignDestroyerDelta.Name, Quantity: 10}, {Design: bomber.Name, Quantity: 4}}}, {Name: "Distraction", Design: DesignLongRangeScout.Name, Position: Vector{0, 5}}}},
	}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}, {Name: "Colony", Owner: 1, Position: Vector{50, 0}, Cargo: Cargo{Colonists: 2000}}}})
	p, f, enemy, prey := u.Player(1), u.Fleet("Ready"), u.Fleet("Bombarding"), u.Fleet("Distraction")
	p.Automation.FleetRoles = map[int]string{f.Num: "patrol"}
	for _, d := range u.Player(2).Designs {
		p.discoverer.discoverDesign(d, true)
	}
	p.discoverer.discoverFleet(enemy, true)
	p.discoverer.discoverFleet(prey, true)
	savePatrolJob(f, patrolJob{Leader: f.Num, Enemy: 2, Target: prey.Num})
	u.Run((*turnGenerator).startPatrolEvaluation, (*turnGenerator).splitAutomaticCombatPools, (*turnGenerator).planPatrolAutomation)
	require.Equal(t, enemy.Num, readPatrolJob(f).Target, "bombing preempts a nearby opportunistic hunt")
	require.True(t, defencePatrolCommitted(p, f))
	require.Greater(t, len(f.Waypoints), 1)
	for i := 0; i < 3 && !enemy.Delete; i++ {
		u.Run((*turnGenerator).fleetMove, (*turnGenerator).fleetBattle)
	}
	remainingBombers := 0
	if !enemy.Delete {
		for _, tok := range enemy.Tokens {
			if tok.design.Spec.Bomber {
				remainingBombers += tok.Quantity
			}
		}
	}
	require.Zero(t, remainingBombers, "actual native battle must stop the bombing")
}

func TestCompletedPatrolRebuildDoesNotWaitForOtherYards(t *testing.T) {
	u := maintenanceUniverse(t)
	p, f := u.Player(1), u.Fleet("Fleet")
	p.Automation.FleetRoles = map[int]string{f.Num: "patrol"}
	work := &fleetMaintenance{OriginalRole: "patrol", Stage: "rally", Group: "far-away-order", Expected: 20, RallyPlanetNum: u.Planet("Yard").Num}
	writeMaintenance(f, work)
	f.SetTag(replacementGroupTag, work.Group)
	u.Run((*turnGenerator).fleetDesignMaintenance)
	require.False(t, f.InDesignMaintenance())
	require.Empty(t, f.GetTag(replacementGroupTag))
	require.Equal(t, "patrol", p.Automation.FleetRoles[f.Num])
}
