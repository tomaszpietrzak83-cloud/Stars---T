//go:build !wasi && !wasm

package cs

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOffensiveAdvanceFeedsAssessedOrbitalAssault(t *testing.T) {
	u, army, op := landingBaseUniverse(t, false)
	p := u.Player(1)
	for _, f := range u.Game.getFleets(1) {
		if p.Automation.FleetRoles[f.Num] == "patrol" {
			delete(f.Tags, landingMemberTag)
			f.Position = Vector{-80, 0}
			f.Waypoints = []Waypoint{{Position: f.Position}}
		}
	}
	// The offensive planner supplies actual nearby ships; it does not authorize
	// a base attack or borrow transports. Landing assessment remains in charge.
	u.Run(func(tg *turnGenerator) {
		for _, f := range u.Game.getFleets(1) {
			if p.Automation.FleetRoles[f.Num] == "patrol" {
				require.True(t, tg.advancePatrolFront(p, f))
				require.Empty(t, f.GetTag(landingMemberTag))
			}
		}
	}, (*turnGenerator).guardAutomaticFuel, (*turnGenerator).enforceAutomaticDeparture)
	for step := 0; step < 4; step++ {
		u.Run((*turnGenerator).fleetMove)
	}
	u.Run(func(tg *turnGenerator) {
		tg.prepareLandingBaseAssault(p, army, op, p.GetPlanetIntel(op.Target), nil)
	})
	var attack *Fleet
	for _, f := range u.Game.getFleets(1) {
		if !f.Delete && f.GetTag(recaptureAssaultTag) == strconv.Itoa(op.Target) {
			attack = f
		}
	}
	require.NotNil(t, attack, "forward movement must feed an actual assessed attack")
	u.Run((*turnGenerator).planRecaptureAssaults, (*turnGenerator).guardAutomaticFuel, (*turnGenerator).enforceAutomaticDeparture, (*turnGenerator).fleetMove, (*turnGenerator).fleetBattle)
	require.False(t, u.Planet("Target").Spec.HasStarbase)
	require.Equal(t, 10, army.Cargo.Colonists, "civilian landing waits for cleared orbit")
}

func offensiveFrontUniverse(t *testing.T) *testUniverse {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
		{Designs: Designs(DesignDestroyerDelta, DesignTeamster, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Hunter", Design: DesignDestroyerDelta.Name, At: "Home"}, {Name: "Army", Design: DesignTeamster.Name, At: "Home"}}},
		{Designs: Designs(DesignDestroyerDelta)},
	}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}, {Name: "First", Owner: 2, Position: Vector{100, 0}}, {Name: "Next", Owner: 2, Position: Vector{200, 0}}}})
	p := u.Player(1)
	p.Automation.FleetRoles = map[int]string{u.Fleet("Hunter").Num: "patrol", u.Fleet("Army").Num: "freighter"}
	p.PlanetIntels = []*Planet{u.Planet("Home"), u.Planet("First"), u.Planet("Next")}
	addExpeditionTestDock(p, Vector{})
	return u
}

func TestOffensiveFrontLeavesHomeAndContinuesPastCompletedObservation(t *testing.T) {
	u := offensiveFrontUniverse(t)
	p, f := u.Player(1), u.Fleet("Hunter")
	plan := func(tg *turnGenerator) { require.True(t, tg.patrolFrontier(p, f)) }
	u.Run(plan, (*turnGenerator).guardAutomaticFuel, (*turnGenerator).enforceAutomaticDeparture, (*turnGenerator).fleetMove)
	require.Greater(t, f.Position.X, 0, "nearby enemy planet must not anchor the front at Home")
	require.Equal(t, 0, f.Position.Y, "no sideways observation loop")
	// Finish the funded observation leg; the next plan must advance farther.
	for step := 0; step < 8 && len(f.Waypoints) > 1; step++ {
		u.Run((*turnGenerator).fleetMove)
	}
	require.Len(t, f.Waypoints, 1, "funded offensive leg must not be slowed to a twenty-turn fuel-saving loop")
	before := f.Position.X
	u.Game.Year++
	u.Run(plan, (*turnGenerator).guardAutomaticFuel, (*turnGenerator).enforceAutomaticDeparture, (*turnGenerator).fleetMove)
	require.Greater(t, f.Position.X, before)
	var front patrolFront
	require.NoError(t, json.Unmarshal([]byte(f.GetTag(patrolFrontTag)), &front))
	require.Equal(t, u.Planet("Next").Num, front.Target)
}

func TestOffensiveFrontDoesNotRouteAdvancedShipBackAcrossEnemy(t *testing.T) {
	u := offensiveFrontUniverse(t)
	p, f := u.Player(1), u.Fleet("Hunter")
	p.PlanetIntels = []*Planet{u.Planet("Home"), u.Planet("First")}
	f.Position = Vector{210, 0}
	f.Waypoints = []Waypoint{{Position: f.Position}}
	u.Run(func(tg *turnGenerator) { require.True(t, tg.advancePatrolFront(p, f)) })
	require.Greater(t, f.Waypoints[1].Position.X, u.Planet("First").Position.X,
		"approach from current side; the fuel dock must not dictate the approach side")
}

func TestDeepOffensiveFrontSupportsActiveLandingAndFollowsRetarget(t *testing.T) {
	u := offensiveFrontUniverse(t)
	p, f, army := u.Player(1), u.Fleet("Hunter"), u.Fleet("Army")
	u.Planet("First").Position = Vector{600, 0}
	u.Planet("Next").Position = Vector{700, 0}
	addExpeditionTestDock(p, Vector{400, 0})
	f.Position = Vector{500, 0}
	f.Waypoints = []Waypoint{{Position: f.Position}}
	op := &landingOperation{Target: u.Planet("First").Num, Owner: 2, Need: 10, Rally: Vector{550, 0}, Phase: "gather"}
	saveLandingOperation(army, op)
	u.Run(func(tg *turnGenerator) { require.True(t, tg.advancePatrolFront(p, f)) })
	var front patrolFront
	require.NoError(t, json.Unmarshal([]byte(f.GetTag(patrolFrontTag)), &front))
	require.Equal(t, op.Target, front.Target)
	require.Equal(t, op.Rally, front.Position)
	require.Greater(t, f.Waypoints[1].Position.X, f.Position.X)
	require.Contains(t, f.GetTag(frontierPatrolTag), "support the invasion")
	require.Empty(t, f.GetTag(landingMemberTag), "advance does not reserve idle guards")
	require.Empty(t, p.Automation.PriorityPlanets, "do not manufacture user priorities")
	op.Target, op.Rally = u.Planet("Next").Num, Vector{650, 0}
	saveLandingOperation(army, op)
	u.Run(func(tg *turnGenerator) { require.True(t, tg.advancePatrolFront(p, f)) })
	require.NoError(t, json.Unmarshal([]byte(f.GetTag(patrolFrontTag)), &front))
	require.Equal(t, op.Target, front.Target)
	require.Equal(t, op.Rally, front.Position)
	require.Greater(t, f.Waypoints[1].Position.X, f.Position.X)
}

func TestOffensivePatrolExploresInsteadOfInventingHomeLoops(t *testing.T) {
	u := offensiveFrontUniverse(t)
	p, f := u.Player(1), u.Fleet("Hunter")
	unknown := u.Planet("Next")
	unknown.PlayerNum, unknown.ReportAge = 0, ReportAgeUnexplored
	p.PlanetIntels = []*Planet{u.Planet("Home"), unknown}
	u.Run(func(tg *turnGenerator) { require.True(t, tg.patrolFrontier(p, f)) })
	var front patrolFront
	require.NoError(t, json.Unmarshal([]byte(f.GetTag(patrolFrontTag)), &front))
	require.Equal(t, unknown.Num, front.Target)
	require.Equal(t, unknown.Position, front.Position)
	require.Greater(t, f.Waypoints[1].Position.X, 30, "funded forward leg, not a home orbit")
	// With only fully observed friendly space, report the missing objective,
	// rather than hiding inactivity behind an endless 24/30ly home orbit.
	p.PlanetIntels = []*Planet{u.Planet("Home")}
	f.Waypoints = []Waypoint{{Position: f.Position}}
	u.Run(func(tg *turnGenerator) { require.False(t, tg.patrolFrontier(p, f)) })
	require.Len(t, f.Waypoints, 1)
}
