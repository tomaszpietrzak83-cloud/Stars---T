//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestScoutPrioritizesOnlyUnknownIncomingEnemyFleetAndReportsBeforeBattle(t *testing.T) {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Designs: Designs(DesignLongRangeScout, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Scout", Design: DesignLongRangeScout.Name, At: "Home"}}}, {Designs: Designs(DesignDestroyerDelta), Fleets: []ScenarioFleet{{Name: "Raider", Design: DesignDestroyerDelta.Name, Position: Vector{20, 0}}}}}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}, {Name: "Unexplored", Position: Vector{50, 0}}}})
	p, f, enemy := u.Player(1), u.Fleet("Scout"), u.Fleet("Raider")
	p.Automation.FleetRoles = map[int]string{f.Num: "scout"}
	p.PlanetIntels = []*Planet{u.Planet("Home"), u.Planet("Unexplored")}
	intel := *enemy
	intel.WarpSpeed = 5
	intel.Heading = VectorFloat64{1, 0}
	p.FleetIntels = []*Fleet{&intel}
	require.False(t, enemyApproachingColony(p, &intel))
	intel.Heading = VectorFloat64{-1, 0}
	require.True(t, enemyApproachingColony(p, &intel))
	u.Run((*turnGenerator).planScoutAutomation, (*turnGenerator).guardAutomaticFuel, (*turnGenerator).planAutomaticSafety, (*turnGenerator).enforceAutomaticDeparture)
	require.True(t, scoutReconActive(p, f))
	require.Equal(t, MapObjectTypeFleet, f.Waypoints[len(f.Waypoints)-1].TargetType)
	require.False(t, f.InAutomaticSafety(), "intentional reconnaissance accepts combat risk")
	f.Position = enemy.Position
	u.Run((*turnGenerator).scanScoutReconContacts)
	design := p.GetShipDesignIntel(enemy.PlayerNum, enemy.Tokens[0].DesignNum)
	require.NotNil(t, design)
	require.NotEmpty(t, design.Slots)
	require.False(t, scoutReconActive(p, f))
}
