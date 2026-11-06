//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAssembledReliefLaunchesBeforeSpeculativeRecruitment(t *testing.T) {
	bomber := campaignDesign("Invading bomber", MiniBomber.Name, "bomber")
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
		{Designs: Designs(DesignJihadCruiser, DesignSpaceStation), Fleets: []ScenarioFleet{
			{Name: "Leader", Design: DesignJihadCruiser.Name, Quantity: 4},
			{Name: "Arrived", Design: DesignJihadCruiser.Name, Quantity: 4},
		}},
		{Designs: Designs(DesignJihadCruiser, DesignDestroyerDelta, bomber), Fleets: []ScenarioFleet{
			{Name: "Overwhelming", Design: DesignJihadCruiser.Name, Quantity: 200, Position: Vector{100, 0}},
			{Name: "Bombarding", Position: Vector{50, 0}, Tokens: []ScenarioShipToken{{Design: DesignDestroyerDelta.Name, Quantity: 10}, {Design: bomber.Name, Quantity: 4}}},
		}},
	}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}, {Name: "Colony", Owner: 1, Position: Vector{50, 0}, Cargo: Cargo{Colonists: 2000}}, {Name: "Other", Owner: 1, Position: Vector{100, 0}, Cargo: Cargo{Colonists: 2000}}}})
	p, leader, arrived, enemy, strong := u.Player(1), u.Fleet("Leader"), u.Fleet("Arrived"), u.Fleet("Bombarding"), u.Fleet("Overwhelming")
	p.Automation.FleetRoles = map[int]string{leader.Num: "patrol", arrived.Num: "patrol"}
	for _, d := range u.Player(2).Designs {
		p.discoverer.discoverDesign(d, true)
	}
	p.discoverer.discoverFleet(enemy, true)
	p.discoverer.discoverFleet(strong, true)
	for _, f := range []*Fleet{leader, arrived} {
		savePatrolJob(f, patrolJob{Leader: leader.Num, Enemy: 2, Target: enemy.Num, Provisional: true, RendezvousBy: u.Game.Year})
	}
	u.Run((*turnGenerator).startPatrolEvaluation)
	p.patrolEvaluator.Limit = 512
	p.patrolQueue = []patrolMission{{Key: priorityFleetKey(2, strong.Num), Kind: "attack", Enemy: 2, Target: strong.Num, Tier: 0}, {Key: priorityFleetKey(2, enemy.Num), Kind: "attack", Enemy: 2, Target: enemy.Num, Tier: 0}}
	u.Run(func(tg *turnGenerator) { tg.planUrgentPatrolDefence(p) }, (*turnGenerator).guardAutomaticFuel, (*turnGenerator).enforceAutomaticDeparture)
	require.True(t, defencePatrolCommitted(p, leader))
	require.Equal(t, 8, leader.Spec.TotalShips)
	require.False(t, readPatrolJob(leader).Provisional)
	require.Equal(t, enemy.Num, readPatrolJob(leader).Target)
	require.Greater(t, len(leader.Waypoints), 1)
	u.Run((*turnGenerator).fleetMove, (*turnGenerator).fleetBattle)
	require.Equal(t, enemy.Position, leader.Position)
	require.True(t, enemy.Delete, "the assembled force must actually stop the bombardment")
}
