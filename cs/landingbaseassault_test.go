//go:build !wasi && !wasm

package cs

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func landingBaseUniverse(t *testing.T, overwhelming bool) (*testUniverse, *Fleet, *landingOperation) {
	fleets := []ScenarioFleet{{Name: "Army", Design: DesignTeamster.Name, At: "Home", Cargo: Cargo{Colonists: 10}}, {Name: "Scout", Design: DesignLongRangeScout.Name, At: "Target"}}
	for n := 0; n < 40; n++ {
		fleets = append(fleets, ScenarioFleet{Name: fmt.Sprint("Guard", n), Design: DesignBattleBeam.Name, Position: Vector{-20, 0}})
	}
	enemies := []ScenarioFleet{}
	if overwhelming {
		enemies = append(enemies, ScenarioFleet{Name: "Defenders", Design: DesignBattleBeam.Name, Quantity: 100, At: "Target"})
	}
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
		{Designs: Designs(DesignTeamster, DesignLongRangeScout, DesignBattleBeam, DesignSpaceStation), Fleets: fleets},
		{Designs: Designs(DesignSpaceStation, DesignBattleBeam), Fleets: enemies},
	}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}, {Name: "Target", Owner: 2, Starbase: DesignSpaceStation.Name, Position: Vector{30, 0}, Cargo: Cargo{Colonists: 100}}}})
	p, leader, scout, target := u.Player(1), u.Fleet("Army"), u.Fleet("Scout"), u.Planet("Target")
	p.Automation.FleetRoles = map[int]string{leader.Num: "freighter", scout.Num: "scout"}
	op := &landingOperation{Target: target.Num, Owner: 2, Need: 10, Scout: scout.Num, Started: u.Game.Year, Rally: u.Planet("Home").Position, Phase: "base", Base: true, EscortDesigns: map[int]bool{}}
	saveLandingOperation(leader, op)
	scout.SetTag(landingMemberTag, strconv.Itoa(leader.Num))
	for n := 0; n < 40; n++ {
		f := u.Fleet(fmt.Sprint("Guard", n))
		p.Automation.FleetRoles[f.Num] = "patrol"
		f.SetTag(landingMemberTag, strconv.Itoa(leader.Num)) // legacy over-reservation
	}
	u.Run((*turnGenerator).scanLandingContacts)
	scout.Position = op.Rally
	scout.Waypoints = []Waypoint{{Position: op.Rally}}
	return u, leader, op
}

func TestLandingBaseReleasesImpossibleLegacyOverReservation(t *testing.T) {
	u, leader, op := landingBaseUniverse(t, true)
	p := u.Player(1)
	u.Run(func(tg *turnGenerator) {
		tg.prepareLandingBaseAssault(p, leader, readLandingOperation(leader), p.GetPlanetIntel(op.Target), nil)
	})
	reserved, released := 0, 0
	for _, f := range u.Game.getFleets(1) {
		if p.Automation.FleetRoles[f.Num] != "patrol" {
			continue
		}
		if landingManaged(f) {
			reserved += f.Spec.TotalShips
		} else {
			released += f.Spec.TotalShips
		}
		require.Empty(t, f.GetTag(recaptureAssaultTag), "an impossible assault must not launch")
	}
	require.Equal(t, 2, reserved)
	require.Equal(t, 38, released)
	require.Equal(t, 10, leader.Cargo.Colonists)
}

func TestLandingBaseChoosesSufficientShortRallyAndDestroysBase(t *testing.T) {
	u, leader, op := landingBaseUniverse(t, false)
	p := u.Player(1)
	plan := func(tg *turnGenerator) {
		tg.prepareLandingBaseAssault(p, leader, readLandingOperation(leader), p.GetPlanetIntel(op.Target), nil)
	}
	u.Run(plan)
	selected := 0
	for _, f := range u.Game.getFleets(1) {
		if p.Automation.FleetRoles[f.Num] == "patrol" && landingManaged(f) {
			selected += f.Spec.TotalShips
			require.Greater(t, len(f.Waypoints), 1, "selected guards actually depart for the short rally")
		}
	}
	require.Positive(t, selected)
	require.Less(t, selected, 40, "a sufficient base attack must not consume every hunter")
	u.Run((*turnGenerator).fleetMove, (*turnGenerator).fleetRefuel, plan)
	var attack *Fleet
	for _, f := range u.Game.getFleets(1) {
		if f.GetTag(recaptureAssaultTag) == strconv.Itoa(op.Target) {
			attack = f
		}
	}
	require.NotNil(t, attack)
	require.Equal(t, selected, attack.Spec.TotalShips)
	require.Equal(t, op.Rally, attack.Position)
	require.Greater(t, len(attack.Waypoints), 1)
	u.Run((*turnGenerator).planRecaptureAssaults, (*turnGenerator).guardAutomaticFuel, (*turnGenerator).enforceAutomaticDeparture, (*turnGenerator).fleetMove, (*turnGenerator).fleetBattle)
	require.Equal(t, u.Planet("Target").Position, attack.Position)
	require.False(t, u.Planet("Target").Spec.HasStarbase, "the assessed group must destroy the real base")
	require.Equal(t, 10, leader.Cargo.Colonists, "transports remain behind until fresh reconnaissance")
}

func TestLandingBaseStaleReportRequiresScoutBeforeAttack(t *testing.T) {
	u, leader, op := landingBaseUniverse(t, false)
	p := u.Player(1)
	target := p.GetPlanetIntel(op.Target)
	target.ReportAge = 16
	op.ScanYear = 0
	saveLandingOperation(leader, op)
	plan := func(tg *turnGenerator) {
		tg.prepareLandingBaseAssault(p, leader, readLandingOperation(leader), p.GetPlanetIntel(op.Target), nil)
	}
	u.Run(plan)
	scout := u.Fleet("Scout")
	require.Greater(t, len(scout.Waypoints), 1)
	require.Equal(t, target.Position, scout.Waypoints[1].Position)
	require.Equal(t, 16, target.ReportAge, "provisional assessment must not refresh intel")
	for _, f := range u.Game.getFleets(1) {
		require.Empty(t, f.GetTag(recaptureAssaultTag))
	}
	u.Run((*turnGenerator).fleetMove, (*turnGenerator).scanLandingContacts, (*turnGenerator).fleetRefuel, plan)
	require.Equal(t, u.Game.Year, readLandingOperation(leader).ScanYear)
	require.Equal(t, 16, p.GetPlanetIntel(op.Target).ReportAge, "orbital scan does not invent fresh surface data")
	launched := false
	for _, f := range u.Game.getFleets(1) {
		launched = launched || f.GetTag(recaptureAssaultTag) != ""
	}
	require.True(t, launched, "fresh observed contacts unlock the assessed assault")
	u.Run((*turnGenerator).planRecaptureAssaults, (*turnGenerator).guardAutomaticFuel, (*turnGenerator).enforceAutomaticDeparture, (*turnGenerator).fleetMove, (*turnGenerator).fleetBattle)
	require.False(t, u.Planet("Target").Spec.HasStarbase, "final departure checks must accept fresh orbital reconnaissance with old surface data")
}

func TestLandingBasePreservesCommittedDefenderOrders(t *testing.T) {
	for _, baseGone := range []bool{false, true} {
		t.Run(fmt.Sprint("baseGone=", baseGone), func(t *testing.T) {
			u, leader, op := landingBaseUniverse(t, false)
			p, defender := u.Player(1), u.Fleet("Guard0")
			p.patrolEvaluator = &patrolEvaluator{DefenceMembers: map[int]bool{defender.Num: true}}
			defender.Waypoints = []Waypoint{{Position: defender.Position}, {Position: Vector{-40, 0}}}
			job := patrolJob{Leader: defender.Num, Enemy: 2, Target: 1, GatheringSince: u.Game.Year}
			savePatrolJob(defender, job)
			orders := append([]Waypoint(nil), defender.Waypoints...)
			if baseGone {
				p.GetPlanetIntel(op.Target).Spec.HasStarbase = false
			}
			u.Run(func(tg *turnGenerator) {
				tg.advanceLandingOperation(p, leader, readLandingOperation(leader), p.GetPlanetIntel(op.Target))
			})
			require.False(t, landingManaged(defender))
			require.Equal(t, orders, defender.Waypoints)
			require.Equal(t, job, readPatrolJob(defender))
		})
	}
}

func TestLandingBaseGoneReleasesExcessGuards(t *testing.T) {
	u, leader, op := landingBaseUniverse(t, false)
	p := u.Player(1)
	target := p.GetPlanetIntel(op.Target)
	target.Spec.HasStarbase = false
	target.ReportAge = 16
	op.ScanYear = u.Game.Year
	saveLandingOperation(leader, op)
	u.Run(func(tg *turnGenerator) { tg.advanceLandingOperation(p, leader, op, target) })
	guards := 0
	for _, f := range u.Game.getFleets(1) {
		if landingManaged(f) && p.Automation.FleetRoles[f.Num] == "patrol" {
			guards += f.Spec.TotalShips
		}
	}
	require.Equal(t, 2, guards)
	require.False(t, op.Base)
	require.Equal(t, "scan", op.Phase)
}
