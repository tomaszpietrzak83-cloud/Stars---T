//go:build !wasi && !wasm

package cs

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestSlowPreyOnCopiedSnapshot(t *testing.T) {
	path := os.Getenv("STARS_SLOW_PREY_SNAPSHOT")
	if path == "" {
		t.Skip("optional isolated snapshot")
	}
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	var game FullGame
	require.NoError(t, json.Unmarshal(raw, &game))
	game.TechStore = &StaticTechStore
	game.Rules.SetTechStore(&StaticTechStore)
	game.Universe.setLogger(testLogger)
	require.NoError(t, game.computeSpecs())
	tg := newTurnGenerator(&game)
	tg.ordersPlayerNum = 1
	p := game.getPlayer(1)
	year := game.Year
	prey := p.GetFleetIntel(3, 32)
	require.NotNil(t, prey)
	require.True(t, slowPatrolPrey(p, prey))
	require.Equal(t, 46, reportedFleetShips(prey))
	if fleet := game.getFleet(1, 99); fleet != nil {
		original := readPatrolJob(fleet)
		require.True(t, patrolNeedsRecon(p, patrolEnemy(p, original)))
		tg.advancePatrol(p, fleet)
		require.Len(t, fleet.Waypoints, 1, "the seven-ship group's previous unknown-target order must not depart")
		require.Contains(t, readPatrolJob(fleet).Status, "scout")
	}
	for _, e := range tg.slowPreyTargets(p) {
		t.Logf("Selected %d:%d %s", e.PlayerNum, e.Num, e.Name)
	}
	tg.refreshSlowPreyOrders()
	tg.planSlowPreyObservers()
	tg.planSlowPreyContainment()
	tg.planPatrolSalvage()
	tg.planLogisticsAutomation()
	tg.planPatrolAutomation()
	tg.planSlowPreyContainment()
	tg.planPatrolSalvage()
	tg.guardAutomaticFuel()
	tg.planAutomaticSafety()
	tg.enforceAutomaticDeparture()
	watchers, screens, collectors := 0, 0, 0
	for _, f := range game.getFleets(1) {
		if job := readScoutRecon(f); job != nil && job.Enemy == 3 && job.Target == 32 {
			watchers++
			t.Logf("Observer %s", f.Name)
		}
		if job := readContainment(f); job != nil && job.Enemy == 3 && job.Target == 32 {
			require.Greater(t, len(f.Waypoints), 1, "final safety/fuel checks must preserve funded containment movement")
			if f.Spec.Radiating {
				require.Equal(t, 6, f.Waypoints[1].WarpSpeed)
			}
			if job.Kind == "collector" {
				collectors++
			} else {
				screens++
			}
			t.Logf("%s: %s", f.Name, job.Status)
		}
	}
	require.Positive(t, watchers)
	require.Positive(t, screens)
	require.Positive(t, collectors)
	require.Equal(t, year, game.Year)
}

func slowPreyUniverse(t *testing.T) *testUniverse {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
		{Designs: Designs(DesignDestroyerDelta, DesignLongRangeScout, DesignTeamster, DesignSpaceStation), Fleets: []ScenarioFleet{
			{Name: "P1", Design: DesignDestroyerDelta.Name, At: "Home"}, {Name: "P2", Design: DesignDestroyerDelta.Name, At: "Home"}, {Name: "P3", Design: DesignDestroyerDelta.Name, At: "Home"},
			{Name: "S1", Design: DesignLongRangeScout.Name, At: "Home"}, {Name: "S2", Design: DesignLongRangeScout.Name, At: "Home"}, {Name: "Collector", Design: DesignTeamster.Name, At: "Home"}}},
		{Designs: Designs(DesignDestroyerDelta), Fleets: []ScenarioFleet{{Name: "Slow", Position: Vector{180, 0}, Tokens: []ScenarioShipToken{{Design: DesignDestroyerDelta.Name, Quantity: 46}}}}},
	}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}}})
	p := u.Player(1)
	p.Automation.FleetRoles = map[int]string{}
	for _, n := range []string{"P1", "P2", "P3"} {
		p.Automation.FleetRoles[u.Fleet(n).Num] = "patrol"
	}
	for _, n := range []string{"S1", "S2"} {
		p.Automation.FleetRoles[u.Fleet(n).Num] = "scout"
	}
	p.Automation.FleetRoles[u.Fleet("Collector").Num] = "freighter"
	e := *u.Fleet("Slow")
	e.WarpSpeed = 2
	e.Heading = VectorFloat64{1, 0}
	p.FleetIntels = []*Fleet{&e}
	addExpeditionTestDock(p, Vector{})
	return u
}

func TestSlowPreySurveillanceContainmentAndCollector(t *testing.T) {
	contacts := 0
	u := slowPreyUniverse(t)
	p := u.Player(1)
	u.Run((*turnGenerator).refreshSlowPreyOrders, (*turnGenerator).planSlowPreyObservers, (*turnGenerator).planScoutAutomation, (*turnGenerator).planSlowPreyContainment)
	for _, n := range []string{"S1", "S2"} {
		f := u.Fleet(n)
		require.NotEmpty(t, f.GetTag(preyWatchTag))
		require.Len(t, f.Waypoints, 2)
		if readScoutRecon(f).Watching {
			require.Greater(t, f.Waypoints[1].Position.DistanceTo(p.FleetIntels[0].Position), 32.0)
		} else {
			contacts++
			require.Equal(t, MapObjectTypeFleet, f.Waypoints[1].TargetType)
		}
	}
	require.Equal(t, 1, contacts, "one scout must inspect unknown equipment before the patrol attacks")
	count := 0
	for _, n := range []string{"P1", "P2", "P3"} {
		f := u.Fleet(n)
		if f.GetTag(containmentTag) != "" {
			count++
			require.Len(t, f.Waypoints, 2)
			require.Zero(t, readPatrolJob(f).Leader, "unknown large fleet is observed, not blindly attacked")
		}
	}
	require.Equal(t, 2, count, "leave a patrol available for defense/support interception")
	f := u.Fleet("Collector")
	require.Equal(t, "collector", readContainment(f).Kind)
	for _, plan := range p.BattlePlans {
		if plan.Num == f.BattlePlanNum {
			require.Equal(t, BattleTacticDisengage, plan.Tactic)
		}
	}
	// Watch and containment must end after the target speeds up.
	p.FleetIntels[0].WarpSpeed = 7
	u.Run((*turnGenerator).refreshSlowPreyOrders)
	require.Empty(t, f.GetTag(containmentTag))
	require.Empty(t, f.GetTag(collectionBattlePlanTag))
	require.Empty(t, u.Fleet("S1").GetTag(preyWatchTag))
}

func TestSlowPreySupportUsesObservedClosingCourse(t *testing.T) {
	u := slowPreyUniverse(t)
	p := u.Player(1)
	support := patrolTestEnemy(1, p)
	support.Num = 92
	support.Position = Vector{80, 0}
	support.WarpSpeed = 6
	support.Heading = VectorFloat64{1, 0}
	p.FleetIntels = append(p.FleetIntels, support)
	require.NotNil(t, slowPreySupport(p, support))
	require.Equal(t, -1, patrolTargetPriority(p, support, false))
	support.Heading = VectorFloat64{-1, 0}
	require.Nil(t, slowPreySupport(p, support))
	support.Heading = VectorFloat64{1, 0}
	support.ReportAge = 1
	require.Nil(t, slowPreySupport(p, support))
	support.ReportAge = 0
	support.PlayerNum = p.Num
	require.Nil(t, slowPreySupport(p, support))
}

func TestSparePatrolInterceptsSlowPreySupport(t *testing.T) {
	u := slowPreyUniverse(t)
	p := u.Player(1)
	support := patrolTestEnemy(1, p)
	support.Num = 92
	support.Position = Vector{80, 0}
	support.WarpSpeed = 4
	support.Heading = VectorFloat64{1, 0}
	p.FleetIntels = append(p.FleetIntels, support)
	u.Run((*turnGenerator).planSlowPreyContainment, (*turnGenerator).planPatrolAutomation, (*turnGenerator).planSlowPreyContainment)
	found := false
	for _, f := range u.Game.getFleets(p.Num) {
		job := readPatrolJob(f)
		if job.Leader == f.Num && job.Target == support.Num {
			found = true
			require.True(t, patrolCanAttack(p, f))
			require.Len(t, f.Waypoints, 2)
		}
	}
	require.True(t, found, "a patrol must cut off the approaching support while others contain the formation")
}

func TestContainmentProtectsCargoAndManualBattlePlan(t *testing.T) {
	u := slowPreyUniverse(t)
	p := u.Player(1)
	f := u.Fleet("Collector")
	f.Cargo.Ironium = 5
	u.Run((*turnGenerator).planSlowPreyContainment)
	require.Empty(t, f.GetTag(containmentTag))
	f.Cargo = Cargo{}
	original := f.BattlePlanNum
	u.Run((*turnGenerator).planSlowPreyContainment)
	require.NotEqual(t, original, f.BattlePlanNum)
	p.TakeManualControl(f)
	require.Equal(t, original, f.BattlePlanNum)
	require.Empty(t, f.GetTag(containmentTag))
}

func TestRadiatingContainmentUsesFreeWarpSix(t *testing.T) {
	d := DesignDestroyerDelta
	d.Slots = append([]ShipDesignSlot(nil), d.Slots...)
	for i := range d.Slots {
		if d.Slots[i].HullComponent == DaddyLongLegs7.Name {
			d.Slots[i].HullComponent = RadiatingHydroRamScoop.Name
		}
	}
	// Replace the actual engine slot regardless of the fixture's engine name.
	d.Slots[0].HullComponent = RadiatingHydroRamScoop.Name
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Designs: Designs(d), Fleets: []ScenarioFleet{{Name: "Rad", Design: d.Name}}}}})
	p, f := u.Player(1), u.Fleet("Rad")
	require.True(t, f.Spec.Radiating)
	require.Equal(t, 6, economicalContainmentPolicy(p, f).MinWarp)
}

func TestCollectorJoinsCoveredArmedBattleAndLoadsNextTurn(t *testing.T) {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
		{Designs: Designs(DesignStalwartDefender, DesignTeamster, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Hunter", Design: DesignStalwartDefender.Name, Quantity: 12, At: "Home"}, {Name: "Collector", Design: DesignTeamster.Name, At: "Home"}}},
		{Designs: Designs(DesignDestroyerDelta), Fleets: []ScenarioFleet{{Name: "Armed", Design: DesignDestroyerDelta.Name, Position: Vector{25, 0}}}},
	}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}}})
	p, h, f, e := u.Player(1), u.Fleet("Hunter"), u.Fleet("Collector"), u.Fleet("Armed")
	p.Automation.FleetRoles = map[int]string{h.Num: "patrol", f.Num: "freighter"}
	p.discoverer.discoverDesign(e.Tokens[0].design, true)
	p.discoverer.discoverFleet(e, true)
	addExpeditionTestDock(p, Vector{})
	u.Run((*turnGenerator).planPatrolAutomation, (*turnGenerator).planPatrolSalvage, (*turnGenerator).guardAutomaticFuel, (*turnGenerator).planAutomaticSafety, (*turnGenerator).enforceAutomaticDeparture)
	require.NotEmpty(t, f.GetTag(anticipatedSalvageTag))
	require.Positive(t, collectionArrivalCover(p, f, u.Game.getFleets(p.Num)))
	require.Len(t, f.Waypoints, 2)
	u.Run((*turnGenerator).fleetMove, (*turnGenerator).fleetBattle)
	require.Equal(t, h.Position, f.Position)
	require.True(t, e.Delete)
	require.False(t, f.Delete)
	require.NotEmpty(t, u.Game.Salvages)
	scan := newPlayerScanner(u.Game.Universe, u.Game.Players, &u.Game.Rules, p)
	scan.scanSalvages(nil)
	p.FleetIntels = nil
	u.Game.Year++
	u.Run((*turnGenerator).planPatrolSalvage, (*turnGenerator).planLogisticsAutomation, (*turnGenerator).resolveLogisticsAutomation)
	require.Positive(t, f.Cargo.ToMineral().Total())
}
