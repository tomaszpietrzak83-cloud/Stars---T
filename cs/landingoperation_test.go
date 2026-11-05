//go:build !wasi && !wasm

package cs

import (
	"fmt"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

func landingOperationUniverse(t *testing.T) *testUniverse {
	fleets := []ScenarioFleet{{Name: "Scout", Design: DesignLongRangeScout.Name, At: "Home"}, {Name: "Cover", Design: DesignBattleBeam.Name, Quantity: 2, At: "Home"}}
	for n := 0; n < 16; n++ {
		fleets = append(fleets, ScenarioFleet{Name: fmt.Sprint("Transport", n), Design: DesignTeamster.Name, Quantity: 3, At: "Home"})
	}
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Designs: Designs(DesignTeamster, DesignSpaceStation, DesignLongRangeScout, DesignBattleBeam), Fleets: fleets}, {Designs: Designs(DesignBattleBeam, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Enemy", Design: DesignBattleBeam.Name, Position: Vector{500, 500}}}}}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name, Cargo: Cargo{Colonists: 50000}}, {Name: "Target", Owner: 2, Position: Vector{70, 0}, Cargo: Cargo{Colonists: 1500}}}})
	p := u.Player(1)
	p.Automation.FleetRoles = map[int]string{}
	p.Automation.DisableMineralSupply = true
	p.Automation.DisableSalvage = true
	for _, f := range u.Game.getFleets(1) {
		p.Automation.FleetRoles[f.Num] = "freighter"
	}
	p.Automation.FleetRoles[u.Fleet("Scout").Num] = "scout"
	p.Automation.FleetRoles[u.Fleet("Cover").Num] = "patrol"
	target := u.Planet("Target")
	target.Hab = u.Planet("Home").Hab
	target.BaseHab = target.Hab
	target.Spec.MaxPopulation = 1000000
	target.Spec.Habitability = 100
	intel := *target
	intel.ReportAge = 12
	p.PlanetIntels = []*Planet{u.Planet("Home"), &intel}
	p.FleetIntels = nil
	return u
}

func landingLeader(u *testUniverse) *Fleet {
	for _, f := range u.Game.getFleets(1) {
		if !f.Delete && readLandingOperation(f) != nil {
			return f
		}
	}
	return nil
}

func TestLandingOperationLoadsMergesScansAndDropsOver250K(t *testing.T) {
	u := landingOperationUniverse(t)
	p := u.Player(1)
	target := u.Planet("Target")
	initial := u.Planet("Home").Cargo.Colonists
	u.Run((*turnGenerator).planLandingOperations)
	leader := landingLeader(u)
	require.NotNil(t, leader)
	op := readLandingOperation(leader)
	require.Equal(t, "gather", op.Phase)
	loaded := 0
	for _, f := range u.turn.landingMembers(p, leader) {
		loaded += f.Cargo.Colonists
	}
	require.Greater(t, loaded, 2500)
	require.Equal(t, initial-loaded, u.Planet("Home").Cargo.Colonists)
	// Real movement to the rendezvous, then native merging; scout must not
	// clear an orbit just because a planning tag or old report exists.
	for n := 0; n < 6 && readLandingOperation(leader).Phase == "gather"; n++ {
		u.Run((*turnGenerator).fleetMove, (*turnGenerator).fleetRefuel, (*turnGenerator).planLandingOperations)
	}
	op = readLandingOperation(leader)
	require.Equal(t, "scan", op.Phase)
	require.Equal(t, loaded, leader.Cargo.Colonists)
	require.Greater(t, leader.Spec.TotalShips, 3)
	require.Equal(t, op.Rally, leader.Position)
	require.LessOrEqual(t, leader.Position.DistanceTo(target.Position), leader.StrategicSpeed(automaticMaxWarp(p, leader)))
	u.Run((*turnGenerator).planLandingOperations)
	require.Equal(t, "scan", readLandingOperation(leader).Phase)
	u.Run((*turnGenerator).fleetMove, (*turnGenerator).scanLandingContacts)
	require.Equal(t, target.Position, u.Fleet("Scout").Position)
	require.NotZero(t, readLandingOperation(leader).ScanYear)
	// Recon does not veto the landing because ground defenses or population
	// differ from the preparation estimate.
	target.Defenses = 10
	target.Spec.DefenseCoverage = .1
	u.Run((*turnGenerator).planLandingOperations)
	require.Equal(t, "assault", readLandingOperation(leader).Phase)
	require.True(t, u.Fleet("Cover").Delete, "patrols physically merge into the loaded fleet")
	require.True(t, leader.Spec.HasWeapons)
	u.Run((*turnGenerator).enforceAutomaticDeparture, (*turnGenerator).fleetMove, (*turnGenerator).fleetBattle, (*turnGenerator).resolveLandingDrops, (*turnGenerator).resolveColonistDrops)
	require.Equal(t, 1, target.PlayerNum)
	require.Zero(t, leader.Cargo.Colonists)
	require.Equal(t, "landed", readLandingOperation(leader).Phase)
	u.Run((*turnGenerator).planLandingOperations)
	require.False(t, landingManaged(leader))
	patrols := 0
	for _, f := range u.Game.getFleets(1) {
		if !f.Delete && p.Automation.FleetRoles[f.Num] == "patrol" {
			patrols += f.Spec.TotalShips
		}
	}
	require.Equal(t, 2, patrols, "surviving guards resume patrol duties: %v", leader.Tags)
}

func TestLandingIncomingWarshipRequiresPatrolAlreadyOnStation(t *testing.T) {
	u := landingOperationUniverse(t)
	p := u.Player(1)
	target := u.Planet("Target")
	leader := u.Fleet("Transport0")
	op := &landingOperation{Target: target.Num, ScanYear: u.Game.Year}
	enemy := u.Fleet("Enemy")
	enemy.Position = Vector{target.Position.X + 25, target.Position.Y}
	enemy.Heading = VectorFloat64{X: -1}
	enemy.WarpSpeed = 5
	enemy.ReportAge = 0
	p.discoverer.discoverDesign(enemy.Tokens[0].design, true)
	p.FleetIntels = []*Fleet{enemy}
	require.False(t, u.turn.landingOrbitClear(p, leader, op))
	guard := u.Fleet("Cover")
	guard.Position = target.Position
	guard.Waypoints = []Waypoint{{Position: guard.Position}}
	guard.Tokens[0].Quantity = 30
	require.NoError(t, u.Game.computeSpecs())
	require.True(t, u.turn.landingOrbitClear(p, leader, op))
	guard.Waypoints = append(guard.Waypoints, Waypoint{Position: Vector{500, 500}, WarpSpeed: 5})
	require.False(t, u.turn.landingOrbitClear(p, leader, op), "a departing patrol is not protection")
}

func TestTransportCrossingCourseDoesNotEvadeButMeetingDoes(t *testing.T) {
	u := landingOperationUniverse(t)
	p := u.Player(1)
	f := u.Fleet("Transport0")
	e := u.Fleet("Enemy")
	f.Position = Vector{0, 0}
	f.Waypoints = []Waypoint{{Position: f.Position}, {Position: Vector{100, 0}, WarpSpeed: 10}}
	e.Position = Vector{50, -50}
	e.Heading = VectorFloat64{Y: 1}
	e.WarpSpeed = 10
	e.ReportAge = 0
	p.discoverer.discoverDesign(e.Tokens[0].design, true)
	p.FleetIntels = []*Fleet{e}
	require.Zero(t, interceptionThreat(p, f), "crossing mid-turn is not a native battle")
	e.Position = Vector{100, -100}
	require.Positive(t, interceptionThreat(p, f), "same end-of-year position remains dangerous even in deep space")
}

func TestLandingOperationCompletesInFullNativeTurns(t *testing.T) {
	u := landingOperationUniverse(t)
	startYear := u.Game.Year
	seenGather, seenScan, landed := false, false, false
	for range 30 {
		u.GenerateTurn()
		for _, f := range u.Game.getFleets(1) {
			if op := readLandingOperation(f); op != nil {
				seenGather = seenGather || op.Phase == "gather"
				seenScan = seenScan || op.Phase == "scan"
				landed = landed || op.Phase == "landed"
			}
		}
	}
	require.Equal(t, startYear+30, u.Game.Year)
	require.True(t, seenGather)
	require.True(t, seenScan)
	require.True(t, landed)
	require.Equal(t, 1, u.Planet("Target").PlayerNum)
	for _, f := range u.Game.getFleets(1) {
		require.False(t, landingManaged(f), "completed landing must release its ships")
	}
}

func TestEmptyLegacyLandingReleasesPendingDeliveryAndLoadsAgain(t *testing.T) {
	for _, kind := range []string{"invade", "invade-load", "landing-reserve"} {
		t.Run(kind, func(t *testing.T) {
			u := landingOperationUniverse(t)
			f := u.Fleet("Transport0")
			saveLogisticsWork(f, &logisticsJob{Kind: kind, TargetNum: u.Planet("Target").Num})
			f.SetTag("maintenance-pending", "Finish current navigation/delivery before rebuilding")
			u.Run((*turnGenerator).migrateLegacyLandings)
			require.Nil(t, logisticsWork(f))
			require.Empty(t, f.GetTag("maintenance-pending"))
			require.Len(t, f.Waypoints, 1)
			u.Run((*turnGenerator).fleetDesignMaintenance, (*turnGenerator).planLandingOperations)
			require.True(t, landingManaged(f))
			require.Positive(t, f.Cargo.Colonists)
		})
	}
}

func TestEmptyLegacyLandingHonorsRequestedRebuild(t *testing.T) {
	scenario := maintenanceScenario()
	scenario.Players[0].Designs = Designs(DesignTeamster, DesignTeamster, DesignSpaceStation)
	scenario.Players[0].Designs[0].Name = "Old Transport"
	scenario.Players[0].Designs[1].Name = "New Transport"
	scenario.Players[0].Fleets = []ScenarioFleet{{Name: "Fleet", Design: "Old Transport", At: "Yard"}}
	u := newTestUniverse(t, scenario)
	requestRebuild(u)
	p, f := u.Player(1), u.Fleet("Fleet")
	p.Automation.FleetRoles = map[int]string{f.Num: "freighter"}
	saveLogisticsWork(f, &logisticsJob{Kind: "invade-load", TargetNum: 999})
	f.SetTag("maintenance-pending", "Finish current navigation/delivery before rebuilding")
	u.Run((*turnGenerator).migrateLegacyLandings, (*turnGenerator).fleetDesignMaintenance)
	require.Nil(t, logisticsWork(f))
	require.True(t, f.InDesignMaintenance(), "release the impossible delivery without cancelling the requested rebuild")
}

func TestLegacyLandingWaitsForScoutAndProtectors(t *testing.T) {
	u := landingOperationUniverse(t)
	p := u.Player(1)
	f := u.Fleet("Transport0")
	target := u.Planet("Target")
	f.Cargo.Colonists = 200
	saveLogisticsWork(f, &logisticsJob{Kind: "invade", TargetNum: target.Num, DeliveryNum: target.Num, Position: target.Position})
	delete(p.Automation.FleetRoles, u.Fleet("Scout").Num)
	u.Run((*turnGenerator).migrateLegacyLandings, (*turnGenerator).planLogisticsAutomation, (*turnGenerator).resolveAutomaticInvasions)
	require.Equal(t, "landing-rally", logisticsWork(f).Kind)
	leader := landingLeader(u)
	require.NotNil(t, leader, "loading starts even while a scout is unavailable")
	require.Equal(t, "gather", readLandingOperation(leader).Phase)
	require.Positive(t, leader.Cargo.Colonists)
	require.Equal(t, 200, f.Cargo.Colonists)
	require.Greater(t, len(f.Waypoints), 1)
	require.NotEqual(t, target.Position, f.Waypoints[1].Position, "an unscouted army may gather, not attack")
	p.Automation.FleetRoles[u.Fleet("Scout").Num] = "scout"
	u.Run((*turnGenerator).planLandingOperations)
	require.NotNil(t, landingLeader(u))
}

func TestLandingBaseDiscoveryHoldsArmyAndRequestsPatrolAssault(t *testing.T) {
	u := landingOperationUniverse(t)
	p := u.Player(1)
	u.Run((*turnGenerator).planLandingOperations)
	leader := landingLeader(u)
	require.NotNil(t, leader)
	for range 6 {
		u.Run((*turnGenerator).fleetMove, (*turnGenerator).planLandingOperations)
		if readLandingOperation(leader).Phase == "scan" {
			break
		}
	}
	target := u.Planet("Target")
	intel := p.GetPlanetIntel(target.Num)
	// The scout reports a real orbital base through the same native discovery
	// path as the existing recapture tests. Population is not a departure gate.
	base := u.Planet("Home").Starbase
	copyBase := *base
	copyBase.PlayerNum = 2
	copyBase.Position = target.Position
	target.Starbase = &copyBase
	target.Spec.HasStarbase = true
	target.Spec.StarbaseDesignNum = base.Tokens[0].DesignNum
	u.Fleet("Scout").Position = target.Position
	u.Run((*turnGenerator).scanLandingContacts, (*turnGenerator).planLandingOperations)
	require.Equal(t, "base", readLandingOperation(leader).Phase)
	require.Positive(t, leader.Cargo.Colonists)
	require.Equal(t, readLandingOperation(leader).Rally, leader.Position)
	require.True(t, landingBaseRequested(p, target.Num, u.Game.getFleets(1)))
	require.True(t, intel.Spec.HasStarbase)
	// A later physical scout observation confirms destruction. The operation
	// returns to reconnaissance and retries, preserving the loaded army.
	target.Starbase = nil
	target.Spec.HasStarbase = false
	u.Run((*turnGenerator).scanLandingContacts, (*turnGenerator).planLandingOperations)
	require.Equal(t, "scan", readLandingOperation(leader).Phase)
	require.Positive(t, leader.Cargo.Colonists)
}

func TestLandingLoadsOvercrowdedSourceBeforeScoutOrGuardsExist(t *testing.T) {
	u := landingOperationUniverse(t)
	p := u.Player(1)
	home := u.Planet("Home")
	delete(p.Automation.FleetRoles, u.Fleet("Scout").Num)
	delete(p.Automation.FleetRoles, u.Fleet("Cover").Num)
	before := home.Cargo.Colonists
	u.Run((*turnGenerator).planLogisticsAutomation)
	leader := landingLeader(u)
	require.NotNil(t, leader)
	require.Less(t, home.Cargo.Colonists, before)
	require.GreaterOrEqual(t, home.Cargo.Colonists, sourcePopulationReserve(p, home))
	loaded := 0
	for _, f := range u.turn.landingMembers(p, leader) {
		loaded += f.Cargo.Colonists
	}
	require.Greater(t, loaded, 2500)
	require.Equal(t, 0, readLandingOperation(leader).Scout)
	require.Equal(t, "gather", readLandingOperation(leader).Phase)
}

func TestCompletedTransportRebuildImmediatelyJoinsLandingPreparation(t *testing.T) {
	u := landingOperationUniverse(t)
	p, f := u.Player(1), u.Fleet("Transport0")
	work := &fleetMaintenance{OriginalRole: "freighter", Stage: "rally", Group: "unfinished-distant-yard", Expected: 20, RallyPlanetNum: u.Planet("Home").Num}
	writeMaintenance(f, work)
	f.SetTag(replacementGroupTag, work.Group)
	f.SetTag("maintenance-pending", "old rebuild")
	u.Run((*turnGenerator).fleetDesignMaintenance, (*turnGenerator).planLandingOperations)
	require.False(t, f.InDesignMaintenance())
	require.Empty(t, f.GetTag(replacementGroupTag))
	require.Empty(t, f.GetTag("maintenance-pending"))
	require.Equal(t, "freighter", p.Automation.FleetRoles[f.Num])
	require.True(t, landingManaged(f))
	require.Positive(t, f.Cargo.Colonists)
}

func TestLandingReinforcementsFinishExistingArmyBeforeStartingAnother(t *testing.T) {
	u := landingOperationUniverse(t)
	p := u.Player(1)
	for _, f := range u.Game.getFleets(1) {
		if p.Automation.FleetRoles[f.Num] == "freighter" && f != u.Fleet("Transport0") {
			delete(p.Automation.FleetRoles, f.Num)
		}
	}
	u.Run((*turnGenerator).planLandingOperations)
	leader := landingLeader(u)
	require.NotNil(t, leader)
	// Another reported target and newly available transports must not steal
	// the reinforcements needed by the first, still incomplete army.
	other := *p.GetPlanetIntel(u.Planet("Target").Num)
	other.Num += 1000
	other.Position.X += 10
	p.PlanetIntels = append(p.PlanetIntels, &other)
	p.Automation.FleetRoles[u.Fleet("Transport1").Num] = "freighter"
	u.Run((*turnGenerator).planLandingOperations)
	member := u.Fleet("Transport1")
	require.Nil(t, readLandingOperation(member))
	require.Equal(t, fmt.Sprint(leader.Num), member.GetTag(landingMemberTag))
	require.Positive(t, member.Cargo.Colonists)
}

func TestLandingProtectionCannotBeReassignedByOtherPlanners(t *testing.T) {
	for _, role := range []string{"patrol", "escort"} {
		t.Run(role, func(t *testing.T) {
			u := landingOperationUniverse(t)
			p, guard, client := u.Player(1), u.Fleet("Cover"), u.Fleet("Transport15")
			p.Automation.FleetRoles[guard.Num] = role
			p.Automation.FleetRoles[client.Num] = "bomber"
			u.Run((*turnGenerator).startPatrolEvaluation, (*turnGenerator).planLandingOperations)
			require.True(t, landingManaged(guard))
			orders := slices.Clone(guard.Waypoints)
			assertReserved := func() {
				t.Helper()
				require.True(t, landingManaged(guard))
				require.Equal(t, role, p.Automation.FleetRoles[guard.Num])
				require.Equal(t, orders, guard.Waypoints)
				require.Empty(t, guard.GetTag(escortTag))
				require.Empty(t, guard.GetTag(bomberBorrowTag))
				require.Nil(t, readTankerJob(guard))
				require.Zero(t, readPatrolJob(guard).Leader)
			}
			u.Run(func(tg *turnGenerator) { tg.assignBomberProtection(p, client, u.Planet("Target").Position, true) })
			assertReserved()
			u.Run(func(tg *turnGenerator) {
				job := &logisticsJob{Kind: "bomber", Position: Vector{10, 0}}
				tg.prepareConvoyWithMinimum(p, client, job, MapObjectTarget{}, 1)
			})
			assertReserved()
			u.Run((*turnGenerator).planEscortStaging, (*turnGenerator).planPatrolAutomation)
			assertReserved()
			client.Position, client.Fuel = Vector{5, 0}, 0
			saveAutomaticFuelRequest(client, automaticFuelRequest{Destination: Vector{}, Members: []int{client.Num}, Urgent: true})
			u.Run((*turnGenerator).planAutomaticTankers, (*turnGenerator).releaseCombatCruiseNetworks)
			assertReserved()
			require.False(t, automaticFuelDonor(p, guard))
		})
	}
}

func TestLandingScoutSelectionPrefersNearbyFundedScoutOverFleetNumber(t *testing.T) {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Designs: Designs(DesignLongRangeScout, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Far", Design: DesignLongRangeScout.Name, Position: Vector{400, 0}}, {Name: "Near", Design: DesignLongRangeScout.Name, Position: Vector{50, 0}}}}}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}}})
	p, far, near := u.Player(1), u.Fleet("Far"), u.Fleet("Near")
	p.Automation.FleetRoles = map[int]string{far.Num: "scout", near.Num: "scout"}
	p.PlanetIntels = []*Planet{u.Planet("Home")}
	require.Less(t, far.Num, near.Num)
	require.Same(t, near, u.turn.pickLandingScout(p, Vector{35, 0}))
	near.SetTag(landingMemberTag, "another-operation")
	require.NotSame(t, near, u.turn.pickLandingScout(p, Vector{35, 0}), "do not steal another army's scout")
}

func TestLandingReservationsUseAlternativePopulationSources(t *testing.T) {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Designs: Designs(DesignTeamster, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "A", Design: DesignTeamster.Name, At: "Home"}, {Name: "B", Design: DesignTeamster.Name, At: "Home"}}}, {}}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}, {Name: "Other", Owner: 1, Position: Vector{10, 0}, Starbase: DesignSpaceStation.Name}, {Name: "Enemy", Owner: 2, Position: Vector{50, 0}, Cargo: Cargo{Colonists: 1000}}}})
	p := u.Player(1)
	home, other, target := u.Planet("Home"), u.Planet("Other"), u.Planet("Enemy")
	target.Hab = home.Hab
	target.BaseHab = home.Hab
	target.Spec.MaxPopulation = 1000000
	p.PlanetIntels = []*Planet{home, other, target}
	p.Automation.FleetRoles = map[int]string{u.Fleet("A").Num: "freighter", u.Fleet("B").Num: "freighter"}
	home.Cargo.Colonists = sourcePopulationReserve(p, home) + u.Fleet("A").Spec.CargoCapacity
	other.Cargo.Colonists = sourcePopulationReserve(p, other) + u.Fleet("B").Spec.CargoCapacity
	u.Run((*turnGenerator).planLandingOperations)
	require.NotNil(t, landingLeader(u))
	sources := map[int]bool{}
	for _, f := range []*Fleet{u.Fleet("A"), u.Fleet("B")} {
		j := logisticsWork(f)
		require.NotNil(t, j)
		sources[j.TargetNum] = true
	}
	require.Len(t, sources, 2, "one claimed source must not leave another transport idle while another colony can fill it")
}
