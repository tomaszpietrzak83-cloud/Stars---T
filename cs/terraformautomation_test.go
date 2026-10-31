//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAutomaticTerraformCompletesThenMoves(t *testing.T) {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Player: NewPlayer(1, NewRace().WithLRT(TT)), Designs: Designs(DesignRemoteTerraformer, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Terraformer", Design: DesignRemoteTerraformer.Name, At: "First"}}}, {}}, Planets: []ScenarioPlanet{{Name: "First", Owner: 1, Starbase: DesignSpaceStation.Name, Hab: ScenarioValue(Hab{48, 50, 50}), Cargo: Cargo{Colonists: 1000}}, {Name: "Next", Owner: 1, Position: Vector{20, 0}, Hab: ScenarioValue(Hab{47, 50, 50}), Cargo: Cargo{Colonists: 1000}}, {Name: "Foreign", Owner: 2, Position: Vector{1, 0}, Hab: ScenarioValue(Hab{40, 50, 50})}}})
	p, f := u.Player(1), u.Fleet("Terraformer")
	p.PlanetIntels = []*Planet{u.Planet("First"), u.Planet("Next"), u.Planet("Foreign")}
	p.Automation.FleetRoles = map[int]string{f.Num: "terraformer"}
	require.NoError(t, p.ValidateLogisticsAutomation(p.Automation, u.Game.Fleets))
	u.Run((*turnGenerator).planTerraformAutomation)
	require.Equal(t, u.Planet("First").Num, readTerraformJob(f).PlanetNum)
	u.Run((*turnGenerator).fleetRemoteTerraform)
	require.Equal(t, Hab{50, 50, 50}, u.Planet("First").Hab)
	u.Run((*turnGenerator).planTerraformAutomation)
	require.Equal(t, u.Planet("Next").Num, readTerraformJob(f).PlanetNum)
	u.Planet("Next").Hab = Hab{50, 50, 50}
 u.Planet("Next").TerraformedAmount = Hab{3,0,0}
	u.Run((*turnGenerator).planTerraformAutomation)
	require.Zero(t, readTerraformJob(f).PlanetNum)
	f.Spec.TerraformRate = 0
	require.NoError(t, p.ValidateLogisticsAutomation(p.Automation, u.Game.Fleets))
	u.Run((*turnGenerator).pruneAutomationAssignments)
	require.Equal(t,"terraformer",p.Automation.FleetRoles[f.Num])
}
