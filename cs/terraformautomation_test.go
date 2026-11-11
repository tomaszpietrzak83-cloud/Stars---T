//go:build !wasi && !wasm

package cs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func alliedTerraformUniverse(t *testing.T) *testUniverse {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Player: NewPlayer(1, NewRace().WithPRT(CA).WithLRT(TT)), Designs: Designs(DesignRemoteTerraformer, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Terraformer", Design: DesignRemoteTerraformer.Name, At: "First"}}}, {}, {}, {}}, Planets: []ScenarioPlanet{
		{Name: "Home", Owner: 1, Position: Vector{1, 0}, Starbase: DesignSpaceStation.Name, Hab: ScenarioValue(Hab{40, 50, 50}), Cargo: Cargo{Colonists: 1000}},
		{Name: "First", Owner: 2, Hab: ScenarioValue(Hab{48, 50, 50}), Cargo: Cargo{Colonists: 1000}},
		{Name: "Next", Owner: 2, Position: Vector{20, 0}, Hab: ScenarioValue(Hab{47, 50, 50}), Cargo: Cargo{Colonists: 1000}},
		{Name: "Neutral", Owner: 3, Position: Vector{2, 0}, Hab: ScenarioValue(Hab{40, 50, 50})},
		{Name: "Enemy", Owner: 4, Position: Vector{3, 0}, Hab: ScenarioValue(Hab{40, 50, 50})},
		{Name: "Empty", Position: Vector{4, 0}, Hab: ScenarioValue(Hab{40, 50, 50})},
	}})
	p, f := u.Player(1), u.Fleet("Terraformer")
	p.Relations[1].Relation = PlayerRelationFriend
	p.Relations[2].Relation = PlayerRelationNeutral
	p.Relations[3].Relation = PlayerRelationEnemy
	p.PlanetIntels = u.Game.Planets
	for _, planet := range p.PlanetIntels {
		planet.ReportAge = 0
	}
	p.Automation.FleetRoles = map[int]string{f.Num: "terraformer"}
	return u
}

func TestAutomaticTerraformCompletesThenMoves(t *testing.T) {
	u := alliedTerraformUniverse(t)
	p, f := u.Player(1), u.Fleet("Terraformer")
	require.NoError(t, p.ValidateLogisticsAutomation(p.Automation, u.Game.Fleets))
	f.SetTag(terraformTag, `{"planetNum":1}`) // migrate an old own-colony assignment
	u.Run((*turnGenerator).planTerraformAutomation)
	require.Equal(t, u.Planet("First").Num, readTerraformJob(f).PlanetNum)
	f.battlePlan.AttackWho = BattleAttackWhoEveryone // assistance must remain positive
	u.Run((*turnGenerator).fleetRemoteTerraform)
	require.Equal(t, Hab{50, 50, 50}, u.Planet("First").Hab)
	u.Run((*turnGenerator).planTerraformAutomation)
	require.Equal(t, u.Planet("Next").Num, readTerraformJob(f).PlanetNum)
	u.Planet("Next").Hab = Hab{50, 50, 50}
	u.Planet("Next").TerraformedAmount = Hab{3, 0, 0}
	u.Run((*turnGenerator).planTerraformAutomation)
	require.Zero(t, readTerraformJob(f).PlanetNum)
	f.Spec.TerraformRate = 0
	require.NoError(t, p.ValidateLogisticsAutomation(p.Automation, u.Game.Fleets))
	u.Run((*turnGenerator).pruneAutomationAssignments)
	require.Equal(t, "terraformer", p.Automation.FleetRoles[f.Num])
}

func TestAutomaticTerraformUsesAlliedPreferences(t *testing.T) {
	u := alliedTerraformUniverse(t)
	f, ally := u.Fleet("Terraformer"), u.Player(2)
	// Our ideal is 50; the ally needs 70, so helping must increase gravity.
	ally.Race.HabLow.Grav, ally.Race.HabHigh.Grav = 55, 85
	ally.Race.WithSpec(&u.Game.Rules)
	planet := u.Planet("First")
	planet.BaseHab, planet.Hab = Hab{68, 50, 50}, Hab{68, 50, 50}
	u.Planet("Next").Hab, u.Planet("Next").BaseHab = Hab{70, 50, 50}, Hab{70, 50, 50}
	u.Run((*turnGenerator).planTerraformAutomation)
	require.Equal(t, planet.Num, readTerraformJob(f).PlanetNum)
	before := ally.Race.GetPlanetHabitability(planet.Hab)
	u.Run((*turnGenerator).fleetRemoteTerraform)
	require.Equal(t, 70, planet.Hab.Grav)
	require.Greater(t, ally.Race.GetPlanetHabitability(planet.Hab), before)
}

func TestAutomaticTerraformStopsWhenAllianceEnds(t *testing.T) {
	u := alliedTerraformUniverse(t)
	p, f := u.Player(1), u.Fleet("Terraformer")
	u.Run((*turnGenerator).planTerraformAutomation)
	p.Relations[1].Relation = PlayerRelationEnemy
	f.SetTag(tankerAwaitTag, "123") // even a paused planner must not permit an attack
	before := u.Planet("First").Hab
	u.Run((*turnGenerator).fleetRemoteTerraform)
	require.Equal(t, before, u.Planet("First").Hab)
	delete(f.Tags, tankerAwaitTag)
	u.Run((*turnGenerator).planTerraformAutomation)
	require.Zero(t, readTerraformJob(f).PlanetNum)
	for _, waypoint := range f.Waypoints[1:] {
		require.Equal(t, u.Planet("Home").Num, waypoint.TargetNum, "only a refuelling return may remain")
	}
}

func TestAutomaticTerraformRejectsCappedForeignReport(t *testing.T) {
	u := alliedTerraformUniverse(t)
	p, f := u.Player(1), u.Fleet("Terraformer")
	u.Planet("First").Hab = Hab{50, 50, 50}
	ability := NewTerraformer().GetTerraformAbility(p).Grav
	require.Positive(t, ability)
	report := *u.Planet("Next")
	report.BaseHab.Grav = 30
	report.Hab.Grav = 30 + ability
	report.TerraformedAmount = Hab{} // real foreign reports omit this field
	p.PlanetIntels = []*Planet{u.Planet("Home"), u.Planet("First"), &report}
	u.Run((*turnGenerator).planTerraformAutomation)
	require.Zero(t, readTerraformJob(f).PlanetNum)
	require.Equal(t, Hab{}, report.TerraformedAmount, "planning must not mutate intel")
}

func TestAutomaticTerraformSkipsUnknownAndAlreadyInstaformedAllies(t *testing.T) {
	u := alliedTerraformUniverse(t)
	f := u.Fleet("Terraformer")
	u.Player(2).Race = *NewRace().WithPRT(CA).WithSpec(&u.Game.Rules)
	u.Planet("First").Hab = Hab{50, 50, 50}
	u.Planet("Next").ReportAge = ReportAgeUnexplored
	u.Run((*turnGenerator).planTerraformAutomation)
	require.Zero(t, readTerraformJob(f).PlanetNum)
}
