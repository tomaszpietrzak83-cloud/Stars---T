//go:build !wasi && !wasm

package cs

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func landingBaseUniverse(t *testing.T, overwhelming bool) (*testUniverse, *Fleet, *landingOperation) {
	return landingBaseUniverseWithContact(t, overwhelming, false)
}

func landingBaseUniverseWithContact(t *testing.T, overwhelming, localContact bool) (*testUniverse, *Fleet, *landingOperation) {
	fleets := []ScenarioFleet{{Name: "Army", Design: DesignTeamster.Name, At: "Home", Cargo: Cargo{Colonists: 10}}, {Name: "Scout", Design: DesignLongRangeScout.Name, At: "Target"}}
	for n := 0; n < 40; n++ {
		fleets = append(fleets, ScenarioFleet{Name: fmt.Sprint("Guard", n), Design: DesignBattleBeam.Name, Position: Vector{-20, 0}})
	}
	enemies := []ScenarioFleet{}
	if overwhelming {
		enemies = append(enemies, ScenarioFleet{Name: "Defenders", Design: DesignBattleBeam.Name, Quantity: 100, At: "Target"})
	}
	if localContact {
		enemies = append(enemies, ScenarioFleet{Name: "Local scout", Design: DesignLongRangeScout.Name, Position: Vector{-60, -20}})
	}
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
		{Designs: Designs(DesignTeamster, DesignLongRangeScout, DesignBattleBeam, DesignSpaceStation), Fleets: fleets},
		{Designs: Designs(DesignSpaceStation, DesignBattleBeam, DesignLongRangeScout), Fleets: enemies},
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
	require.Zero(t, reserved)
	require.Equal(t, 40, released)
	require.Equal(t, 10, leader.Cargo.Colonists)
	require.Len(t, u.Fleet("Scout").Waypoints, 1, "known overwhelming defenders must not consume replacement scouts")
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
	require.GreaterOrEqual(t, attack.Spec.TotalShips*100, selected*95, "actual losses remain within five percent")
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
		if p.Automation.FleetRoles[f.Num] == "patrol" {
			require.False(t, landingManaged(f), "reconnaissance must not reserve the attack force")
		}
	}
	u.Run((*turnGenerator).fleetMove, (*turnGenerator).scanLandingContacts, (*turnGenerator).fleetRefuel, plan)
	require.Equal(t, u.Game.Year, readLandingOperation(leader).ScanYear)
	require.Equal(t, 16, p.GetPlanetIntel(op.Target).ReportAge, "orbital scan does not invent fresh surface data")
	launched := false
	for _, f := range u.Game.getFleets(1) {
		launched = launched || f.GetTag(recaptureAssaultTag) != ""
	}
	if !launched {
		u.Run((*turnGenerator).fleetMove, (*turnGenerator).fleetRefuel, plan)
		for _, f := range u.Game.getFleets(1) {
			launched = launched || f.GetTag(recaptureAssaultTag) != ""
		}
	}
	require.True(t, launched, "fresh observed contacts unlock a short assessed assembly and assault")
	u.Run((*turnGenerator).planRecaptureAssaults, (*turnGenerator).guardAutomaticFuel, (*turnGenerator).enforceAutomaticDeparture, (*turnGenerator).fleetMove, (*turnGenerator).fleetBattle)
	require.False(t, u.Planet("Target").Spec.HasStarbase, "final departure checks must accept fresh orbital reconnaissance with old surface data")
}

func TestLandingBaseReconPrefersScoutAndFallsBackToSingleDestroyer(t *testing.T) {
	u, leader, op := landingBaseUniverse(t, false)
	p := u.Player(1)
	scout, destroyer := u.Fleet("Scout"), u.Fleet("Guard0")
	delete(destroyer.Tags, landingMemberTag)
	target := p.GetPlanetIntel(op.Target)
	p.GetShipDesignIntel(target.PlayerNum, target.Spec.StarbaseDesignNum).Slots = nil
	plan := func(tg *turnGenerator) {
		tg.prepareLandingBaseAssault(p, leader, readLandingOperation(leader), target, nil)
	}
	u.Run(plan)
	require.Equal(t, scout.Num, readLandingOperation(leader).Scout)
	require.Empty(t, destroyer.GetTag(landingBaseReconTag))

	scout.Delete = true
	u.Run(plan, (*turnGenerator).enforceAutomaticDeparture)
	require.Equal(t, destroyer.Num, readLandingOperation(leader).Scout)
	require.Equal(t, "patrol", p.Automation.FleetRoles[destroyer.Num])
	require.Greater(t, len(destroyer.Waypoints), 1)
	require.Equal(t, target.Position, destroyer.Waypoints[len(destroyer.Waypoints)-1].Position)
	u.Run((*turnGenerator).fleetMove, (*turnGenerator).scanLandingContacts)
	require.NotEmpty(t, p.GetShipDesignIntel(target.PlayerNum, target.Spec.StarbaseDesignNum).Slots)
	require.Empty(t, destroyer.GetTag(landingBaseReconTag))
	require.Empty(t, destroyer.GetTag(landingMemberTag))
}

func TestLandingBaseReleasesReinforcementsBeyondTwoTurns(t *testing.T) {
	u, leader, op := landingBaseUniverse(t, false)
	p := u.Player(1)
	for _, f := range u.Game.getFleets(1) {
		if p.Automation.FleetRoles[f.Num] == "patrol" {
			distance := 2*f.StrategicSpeed(automaticMaxWarp(p, f)) + 5
			f.Position = Vector{-int(distance), 0}
			f.Waypoints = []Waypoint{{Position: f.Position}}
		}
	}
	u.Run(func(tg *turnGenerator) {
		tg.prepareLandingBaseAssault(p, leader, readLandingOperation(leader), p.GetPlanetIntel(op.Target), nil)
	})
	for _, f := range u.Game.getFleets(1) {
		require.Empty(t, f.GetTag(recaptureAssaultTag), "ships must physically assemble before attacking")
		if p.Automation.FleetRoles[f.Num] == "patrol" {
			require.False(t, landingManaged(f), "distant reinforcements must remain available for patrol")
		}
	}
}

func TestLandingBaseBudgetExhaustionReleasesEveryReservedWarship(t *testing.T) {
	for _, duringAssessment := range []bool{false, true} {
		t.Run(fmt.Sprint("duringAssessment=", duringAssessment), func(t *testing.T) {
			u, leader, op := landingBaseUniverse(t, false)
			p := u.Player(1)
			u.Run(func(tg *turnGenerator) {
				e := tg.recaptureEvaluator(p)
				if duringAssessment {
					e.Limit = 1
				} else {
					e.Work = e.Limit
				}
				tg.prepareLandingBaseAssault(p, leader, op, p.GetPlanetIntel(op.Target), nil)
			})
			for _, f := range u.Game.getFleets(1) {
				if p.Automation.FleetRoles[f.Num] == "patrol" {
					require.False(t, landingManaged(f))
					require.Empty(t, f.GetTag(recaptureAssaultTag))
				}
			}
		})
	}
}

func TestLandingBlockedBaseLeavesHuntersWorkingAcrossYears(t *testing.T) {
	u, leader, op := landingBaseUniverseWithContact(t, true, true)
	p := u.Player(1)
	for _, f := range u.Game.getFleets(1) {
		if p.Automation.FleetRoles[f.Num] == "patrol" {
			f.Position = Vector{-80, 0}
			f.Waypoints = []Waypoint{{Position: f.Position}}
		}
	}
	contact := u.Fleet("Local scout")
	p.discoverer.discoverDesign(contact.Tokens[0].design, true)
	for year := 0; year < 3; year++ {
		u.Game.Year++
		op.ScanYear = u.Game.Year
		saveLandingOperation(leader, op)
		contact.Position = Vector{-60 + year*10, -20}
		p.discoverer.discoverFleet(contact, true)
		u.Run((*turnGenerator).fleetInit, (*turnGenerator).startPatrolEvaluation, func(tg *turnGenerator) {
			tg.prepareLandingBaseAssault(p, leader, op, p.GetPlanetIntel(op.Target), nil)
		}, (*turnGenerator).planPatrolAutomation, (*turnGenerator).guardAutomaticFuel, (*turnGenerator).enforceAutomaticDeparture)
		moving := 0
		before := map[int]Vector{}
		for _, f := range u.Game.getFleets(1) {
			if f.Delete || p.Automation.FleetRoles[f.Num] != "patrol" {
				continue
			}
			require.False(t, landingManaged(f), "an impossible base must not own the hunters in year %d", year)
			if len(f.Waypoints) > 1 {
				before[f.Num] = f.Position
			}
		}
		u.Run((*turnGenerator).fleetMove)
		for num, position := range before {
			if f := u.Game.getFleet(1, num); f != nil && f.Position != position {
				moving++
			}
		}
		require.Positive(t, moving, "released patrols must actually move in year %d", year)
	}
}

func TestLandingBaseRendezvousDeadlineDoesNotSlide(t *testing.T) {
	u, leader, op := landingBaseUniverse(t, false)
	p := u.Player(1)
	plan := func(tg *turnGenerator) {
		tg.prepareLandingBaseAssault(p, leader, readLandingOperation(leader), p.GetPlanetIntel(op.Target), nil)
	}
	u.Run(plan)
	deadline := readLandingOperation(leader).BaseRallyBy
	require.Equal(t, u.Game.Year+2, deadline)
	u.Game.Year++
	u.Run((*turnGenerator).startPatrolEvaluation, plan)
	require.Equal(t, deadline, readLandingOperation(leader).BaseRallyBy)
	// Simulate blocked departures: no member actually moved during assembly.
	u.Game.Year = deadline
	u.Run((*turnGenerator).startPatrolEvaluation, plan)
	u.Run(plan) // repeated previews in this year cannot restart the deadline
	for _, f := range u.Game.getFleets(1) {
		if p.Automation.FleetRoles[f.Num] == "patrol" {
			require.False(t, landingManaged(f), "an uncompleted rendezvous must release its ships")
		}
	}
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

func TestLandingUnknownBaseReceivesFirstEquipmentReconnaissance(t *testing.T) {
	u, leader, op := landingBaseUniverse(t, false)
	p, scout := u.Player(1), u.Fleet("Scout")
	target := p.GetPlanetIntel(op.Target)
	design := p.GetShipDesignIntel(target.PlayerNum, target.Spec.StarbaseDesignNum)
	design.Slots = nil
	op.ScanYear = 0
	saveLandingOperation(leader, op)
	plan := func(tg *turnGenerator) {
		tg.advanceLandingOperation(p, leader, readLandingOperation(leader), p.GetPlanetIntel(op.Target))
	}
	u.Run(plan)
	require.Greater(t, len(scout.Waypoints), 1, "unknown base cannot be assessed until the scout visits")
	require.Equal(t, target.Position, scout.Waypoints[1].Position)
	for _, f := range u.Game.getFleets(1) {
		require.Empty(t, f.GetTag(recaptureAssaultTag), "unknown equipment must not authorize an attack")
	}
	u.Run((*turnGenerator).guardAutomaticFuel, (*turnGenerator).enforceAutomaticDeparture, (*turnGenerator).fleetMove, (*turnGenerator).scanLandingContacts)
	require.Equal(t, target.Position, scout.Position)
	require.NotEmpty(t, p.GetShipDesignIntel(target.PlayerNum, target.Spec.StarbaseDesignNum).Slots)
	require.Equal(t, u.Game.Year, readLandingOperation(leader).ScanYear)
	require.Equal(t, op.Rally, leader.Position)
	require.Equal(t, 10, leader.Cargo.Colonists)
}
