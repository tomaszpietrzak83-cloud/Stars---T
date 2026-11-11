//go:build !wasi && !wasm

package cs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func addTestMineSwap(t *testing.T, u *testUniverse, template *Fleet, num int, position Vector) *Fleet {
	t.Helper()
	f := *template
	f.Num = num
	f.Position = position
	f.Waypoints = []Waypoint{{Position: position}}
	f.Tags = Tags{}
	u.Game.Fleets = append(u.Game.Fleets, &f)
	require.NoError(t, u.Game.Universe.addFleet(&f))
	u.Player(1).Automation.FleetRoles[num] = "minesweeper"
	return &f
}

func TestMineSwapReinforcesLargeFieldWithoutTakingSmallFieldsFleet(t *testing.T) {
	u := minesweeperUniverse(t)
	p, lead := u.Player(1), u.Fleet("Sweeper")
	point := Vector{68, 0}
	lead.Position = point
	lead.Waypoints = []Waypoint{{Position: point}}
	helper := addTestMineSwap(t, u, lead, 98, point)
	smallSweeper := addTestMineSwap(t, u, lead, 99, Vector{})
	large := newMinefield(u.Player(2), MinefieldTypeHeavy, lead.Spec.MineSweep*4, 1, point)
	small := newMinefield(u.Player(2), MinefieldTypeStandard, 50, 2, smallSweeper.Position)
	for _, m := range []*Minefield{large, small} {
		u.Game.Universe.addMinefield(m)
		report := *m
		p.MinefieldIntels = append(p.MinefieldIntels, &report)
	}
	lead.SetTag(minesweeperTargetTag, "2:1")
	smallSweeper.SetTag(minesweeperTargetTag, "2:2")
	start := large.NumMines
	u.Run((*turnGenerator).planMinesweeperAutomation, (*turnGenerator).fleetMove, (*turnGenerator).fleetSweepMines)
	require.Equal(t, "2:1", lead.GetTag(minesweeperTargetTag))
	require.Equal(t, "2:1", helper.GetTag(minesweeperTargetTag))
	require.Equal(t, "2:2", smallSweeper.GetTag(minesweeperTargetTag))
	require.Equal(t, start-2*lead.Spec.MineSweep, large.NumMines)
	require.True(t, small.Delete)
}

func TestMineSwapRebuildPreservesCoverageAndReleasesNextBatch(t *testing.T) {
	u := minesweeperUniverse(t)
	p, lead := u.Player(1), u.Fleet("Sweeper")
	first := addTestMineSwap(t, u, lead, 97, Vector{})
	second := addTestMineSwap(t, u, lead, 98, Vector{})
	third := addTestMineSwap(t, u, lead, 99, Vector{})
	old := lead.Tokens[0].DesignNum
	p.DesignMaintenance = DesignMaintenance{old: p.Designs[1].Num}
	p.Automation.RebuildFleets = map[int]bool{lead.Num: true, first.Num: true, second.Num: true, third.Num: true}
	p.MinefieldIntels = []*Minefield{newMinefield(u.Player(2), MinefieldTypeHeavy, 2000, 1, Vector{20, 0})}
	lead.SetTag(minesweeperTargetTag, "2:1")
	lead.SetTag("maintenance-pending", "legacy pending")
	deferred := map[*Fleet]bool{}
	u.turn.deferMinesweeperRebuilds(p, deferred)
	require.True(t, deferred[lead])
	require.True(t, deferred[third])
	require.False(t, deferred[first])
	require.False(t, deferred[second])
	u.Run((*turnGenerator).fleetDesignMaintenance)
	require.Empty(t, lead.GetTag("maintenance-pending"))
	require.True(t, p.Automation.RebuildFleets[lead.Num])
	p.MinefieldIntels = nil
	clear(deferred)
	u.turn.deferMinesweeperRebuilds(p, deferred)
	require.False(t, deferred[lead])
}

func TestPatrolInterceptsFreshExposedReportedMinelayer(t *testing.T) {
	layer := ShipDesign{Name: "Layer", Hull: MiniMineLayer.Name, Slots: []ShipDesignSlot{
		{HullSlotIndex: 1, HullComponent: QuickJump5.Name, Quantity: 1},
		{HullSlotIndex: 2, HullComponent: MineDispenser40.Name, Quantity: 1},
	}}
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
		{Designs: Designs(DesignDestroyerDelta, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Patrol", Design: DesignDestroyerDelta.Name, Quantity: 3}}},
		{Designs: Designs(layer), Fleets: []ScenarioFleet{{Name: "Layer", Design: layer.Name, Position: Vector{10, 0}}}},
	}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}}})
	u.Game.Year++
	p, patrol, enemy := u.Player(1), u.Fleet("Patrol"), u.Fleet("Layer")
	p.Automation.FleetRoles = map[int]string{patrol.Num: "patrol"}
	p.discoverer.discoverDesign(enemy.Tokens[0].design, false)
	p.discoverer.discoverFleet(enemy, true)
	report := p.GetFleetIntel(enemy.PlayerNum, enemy.Num)
	require.True(t, exposedReportedMinelayer(p, report))
	report.ReportAge = 1
	require.False(t, exposedReportedMinelayer(p, report))
	report.ReportAge, report.OrbitingPlanetNum = 0, 1
	require.False(t, exposedReportedMinelayer(p, report))
	report.OrbitingPlanetNum = 0
	p.discoverer.discoverDesign(enemy.Tokens[0].design, true)
	require.True(t, exposedReportedMinelayer(p, report), "revealed minelaying equipment identifies the target")
	u.Run((*turnGenerator).startPatrolEvaluation, func(tg *turnGenerator) {
		tg.planUrgentPatrolDefence(p)
		tg.planMinelayerInterceptions(p)
	})
	require.Contains(t, readPatrolJob(patrol).Status, "Intercepting reported minelayer")
	u.Run((*turnGenerator).enforceAutomaticDeparture, (*turnGenerator).fleetMove, (*turnGenerator).fleetBattle)
	require.True(t, enemy.Delete)
}

func TestMineSwapHeadsInwardAtHighestSafeWarp(t *testing.T) {
	u := minesweeperUniverse(t)
	p, f := u.Player(1), u.Fleet("Sweeper")
	f.Position = Vector{50, 0}
	f.Waypoints = []Waypoint{{Position: f.Position}}
	heavy := newMinefield(u.Player(2), MinefieldTypeHeavy, 10_000, 1, Vector{})
	standard := newMinefield(u.Player(2), MinefieldTypeStandard, 4_000, 2, Vector{})
	p.MinefieldIntels = []*Minefield{heavy}
	route := u.turn.minesweeperFieldRoute(p, f, heavy)
	require.NotNil(t, route)
	require.Equal(t, 1, route.Turns)
	require.Equal(t, 6, route.Waypoints[1].WarpSpeed)
	require.Equal(t, Vector{14, 0}, route.Waypoints[1].Position)
	p.MinefieldIntels = append(p.MinefieldIntels, standard)
	route = u.turn.minesweeperFieldRoute(p, f, heavy)
	require.NotNil(t, route)
	require.Equal(t, 4, route.Waypoints[1].WarpSpeed)
	require.Equal(t, Vector{34, 0}, route.Waypoints[1].Position)
}

func TestMinelayerPursuitShadowsAcrossTurnsAndRecruitsEscortResponse(t *testing.T) {
	layer := ShipDesign{Name: "Layer", Hull: MiniMineLayer.Name, Slots: []ShipDesignSlot{
		{HullSlotIndex: 1, HullComponent: QuickJump5.Name, Quantity: 1},
		{HullSlotIndex: 2, HullComponent: MineDispenser40.Name, Quantity: 1},
	}}
	patrol := Designs(DesignDestroyerDelta)[0]
	patrol.Slots[0].HullComponent = RadiatingHydroRamScoop.Name
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
		{Designs: Designs(patrol, DesignLongRangeScout, DesignSpaceStation), Fleets: []ScenarioFleet{
			{Name: "Patrol", Design: patrol.Name},
			{Name: "Support", Design: patrol.Name, Quantity: 5, Position: Vector{8, 4}},
			{Name: "Scout", Design: DesignLongRangeScout.Name, Position: Vector{140, 30}},
			{Name: "Spare scout", Design: DesignLongRangeScout.Name, Position: Vector{130, -30}},
		}},
		{Designs: Designs(layer, patrol), Fleets: []ScenarioFleet{
			{Name: "Layer", Design: layer.Name, Position: Vector{145, 0}},
			{Name: "Escort", Design: patrol.Name, Position: Vector{200, 60}},
		}},
	}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}}})
	p, enemy, escort := u.Player(1), u.Fleet("Layer"), u.Fleet("Escort")
	hunter, support := u.Fleet("Patrol"), u.Fleet("Support")
	scout, spare := u.Fleet("Scout"), u.Fleet("Spare scout")
	p.Automation.FleetRoles = map[int]string{hunter.Num: "patrol", scout.Num: "scout", spare.Num: "scout"}
	p.discoverer.discoverDesign(enemy.Tokens[0].design, true)
	enemy.Waypoints = []Waypoint{{Position: enemy.Position}, {Position: Vector{180, 0}, WarpSpeed: 4}}
	p.discoverer.discoverFleet(enemy, true)
	var firstDestination Vector
	scoutNum, turns, recruited := 0, 0, false
	for step := range 9 {
		u.Game.Year++
		u.Run((*turnGenerator).fleetInit, (*turnGenerator).startPatrolEvaluation, func(tg *turnGenerator) {
			tg.planMinelayerScouts(p, tg.game.getFleets(p.Num), nil)
			tg.planPatrolAutomation()
		})
		count := 0
		for _, f := range []*Fleet{scout, spare} {
			if j := readScoutRecon(f); j != nil && j.Minelayer && j.Target == enemy.Num {
				count++
				if scoutNum == 0 {
					scoutNum = f.Num
				}
				require.Equal(t, scoutNum, f.Num, "same scout keeps the contact")
			}
		}
		require.Equal(t, 1, count)
		j := readPatrolJob(hunter)
		if step == 0 {
			require.True(t, j.Minelayer, "open a pursuit outside one-turn range")
			require.NotEqual(t, u.Game.Year, j.ReliefYear)
			require.Greater(t, len(hunter.Waypoints), 1)
			firstDestination = hunter.Waypoints[len(hunter.Waypoints)-1].Position
		}
		if readPatrolJob(support).Minelayer || readPatrolJob(support).Leader == hunter.Num && j.Minelayer || support.Delete && hunter.Spec.TotalShips > 1 {
			recruited = true
			if step == 1 && len(support.Waypoints) > 1 {
				require.NotEqual(t, firstDestination, support.Waypoints[len(support.Waypoints)-1].Position)
			}
		}
		t.Logf("year=%d scout=%d patrol=%v status=%s support=%+v enemy=%v", u.Game.Year, scoutNum, hunter.Position, j.Status, readPatrolJob(support), enemy.Position)
		u.Run((*turnGenerator).enforceAutomaticDeparture, (*turnGenerator).preparePatrolPursuits, (*turnGenerator).fleetMove, (*turnGenerator).mergePatrolRendezvous, (*turnGenerator).scanScoutReconContacts, (*turnGenerator).fleetBattle, (*turnGenerator).fleetRepair)
		turns++
		if enemy.Delete {
			break
		}
		if step == 0 {
			// The second report reveals an escort joining the contact.
			enemy.Waypoints = []Waypoint{{Position: enemy.Position}, {Position: Vector{169, 25}, WarpSpeed: 4}}
			escort.Position = enemy.Position
			escort.Waypoints = append([]Waypoint(nil), enemy.Waypoints...)
			p.Automation.FleetRoles[support.Num] = "patrol"
			p.discoverer.discoverDesign(escort.Tokens[0].design, true)
			p.discoverer.discoverFleet(escort, true)
		}
	}
	require.GreaterOrEqual(t, turns, 2)
	require.True(t, recruited, "the patrol should add support after learning about the escort")
	require.True(t, enemy.Delete, "the pursuit should end in a native kill")
}
