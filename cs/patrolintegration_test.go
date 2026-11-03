//go:build !wasi && !wasm

package cs

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPatrolProductionRallyUsesMergedSpeedAndFundedArrival(t *testing.T) {
	slow := ShipDesign{Name: "Rally slow", Hull: Destroyer.Name, Slots: []ShipDesignSlot{{HullSlotIndex: 1, HullComponent: FuelMizer.Name, Quantity: 1}, {HullSlotIndex: 2, HullComponent: XRayLaser.Name, Quantity: 1}}}
	fast := slow
	fast.Name = "Rally fast"
	fast.Slots = append([]ShipDesignSlot(nil), slow.Slots...)
	fast.Slots[0].HullComponent = TransStar10.Name
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Player: proposalTestPlayer(), Designs: Designs(slow, fast), Fleets: []ScenarioFleet{{Name: "Slow", Design: slow.Name}, {Name: "Fast", Design: fast.Name, Position: Vector{100, 0}}}}}})
	p := u.Player(1)
	group := []*Fleet{u.Fleet("Slow"), u.Fleet("Fast")}
	addExpeditionTestDock(p, Vector{})
	addExpeditionTestDock(p, Vector{140, 0})
	policies := []automaticRoutePolicy{{Fast: true, MinWarp: 4, MaxWarp: 4, MaxTurns: 20}, {Fast: true, MinWarp: 9, MaxWarp: 9, MaxTurns: 20}}
	base := choosePatrolRally(&u.Game.Rules, p, group, Vector{140, 0}, []Vector{{50, 0}}, policies)
	require.NotNil(t, base)
	points := []Vector{}
	for x := 0; x <= 100; x += 4 {
		points = append(points, Vector{x, 0})
	}
	chosen := choosePatrolRally(&u.Game.Rules, p, group, Vector{140, 0}, points, policies)
	require.NotNil(t, chosen)
	require.Equal(t, 6, base.Turns)
	require.Equal(t, 4, chosen.Turns)
	for i, f := range group {
		applyExpeditionRoute(p, f, chosen.Routes[i])
	}
	for n := 0; n < max(chosen.Routes[0].Turns, chosen.Routes[1].Turns); n++ {
		u.Run((*turnGenerator).fleetMove)
	}
	require.Equal(t, group[0].Position, group[1].Position)
	expected := patrolMergedFormation(&u.Game.Rules, p, group)
	_, err := NewOrderer().Merge(&u.Game.Rules, p, group)
	require.NoError(t, err)
	require.Equal(t, expected.Tokens, group[0].Tokens)
	require.Equal(t, expected.Spec, group[0].Spec)
}

func TestPatrolFreeEngineWorksAtColonyWithoutBaseAndZeroFuel(t *testing.T) {
	d := ShipDesign{Name: "Free patrol", Hull: Destroyer.Name, Slots: []ShipDesignSlot{{HullSlotIndex: 1, HullComponent: FuelMizer.Name, Quantity: 1}, {HullSlotIndex: 2, HullComponent: XRayLaser.Name, Quantity: 1}}}
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Designs: Designs(d, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Patrol", Design: d.Name, At: "Outpost"}}}}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}, {Name: "Outpost", Owner: 1, Position: Vector{100, 0}}}})
	p, f := u.Player(1), u.Fleet("Patrol")
	p.Automation.FleetRoles = map[int]string{f.Num: "patrol"}
	f.Fuel = 0
	before := f.Position
	u.Run((*turnGenerator).startPatrolEvaluation, (*turnGenerator).planPatrolAutomation, (*turnGenerator).guardAutomaticFuel, (*turnGenerator).recoverIdleAutomaticFleets, (*turnGenerator).enforceAutomaticDeparture)
	require.Len(t, f.Waypoints, 2)
	require.GreaterOrEqual(t, f.Waypoints[1].WarpSpeed, 4)
	require.Equal(t, u.Planet("Outpost").Num, func() int { var n int; fmt.Sscan(f.GetTag(patrolSectorTag), &n); return n }())
	u.Run((*turnGenerator).fleetMove)
	require.NotEqual(t, before, f.Position)
	require.GreaterOrEqual(t, f.Fuel, 0)
}

func TestPatrolLostContactMemoryPersistsAndSearchCannotAttack(t *testing.T) {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Designs: Designs(DesignLongRangeScout), Fleets: []ScenarioFleet{{Name: "Search", Design: DesignLongRangeScout.Name}}}}})
	p, s := u.Player(1), u.Fleet("Search")
	contact := &Fleet{MapObject: MapObject{PlayerNum: 2, Num: 7, Position: Vector{24, 0}}, Tokens: []ShipToken{{DesignNum: 99, Quantity: 10}}, WarpSpeed: 2, Heading: VectorFloat64{X: 1}}
	p.FleetIntels = []*Fleet{contact}
	rememberPatrolReports(p, 2400)
	raw, err := json.Marshal(p.Automation)
	require.NoError(t, err)
	p.Automation = LogisticsAutomation{}
	require.NoError(t, json.Unmarshal(raw, &p.Automation))
	require.Len(t, p.Automation.PatrolMemory, 1)
	p.FleetIntels = nil
	p.Automation.FleetRoles = map[int]string{s.Num: "scout"}
	addExpeditionTestDock(p, Vector{})
	u.Game.Year = 2402
	u.Run(func(tg *turnGenerator) { require.True(t, tg.planLostPatrolSearch(p, s)) })
	require.NotEmpty(t, s.GetTag(patrolSearchTag))
	require.Nil(t, readScoutRecon(s))
	require.False(t, patrolCanAttack(p, s))
	for _, wp := range s.Waypoints {
		require.NotEqual(t, MapObjectTypeFleet, wp.TargetType)
	}
	p.FleetIntels = []*Fleet{contact}
	u.Run(func(tg *turnGenerator) { require.False(t, tg.planLostPatrolSearch(p, s)) })
	rememberPatrolReports(p, 2413)
	require.Len(t, p.Automation.PatrolMemory, 1, "a fresh report renews memory without proving zero fuel")
}

func TestPatrolLocalDebrisPickupPrecedesEmptyReturnAndIDReuse(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Transport")
	field := u.Game.Salvages[0]
	field.Cargo = Cargo{Ironium: 33}
	f.Position = field.Position
	p.SalvageIntels = []*Salvage{field}
	saveLogisticsWork(f, &logisticsJob{Kind: "return", Position: Vector{}})
	u.Run((*turnGenerator).planPatrolSalvage, (*turnGenerator).resolveLogisticsAutomation)
	require.Equal(t, 33, f.Cargo.Ironium)
	require.Zero(t, field.Cargo.Ironium)
	f.Cargo = Cargo{}
	old := f.Position
	field.Position = old.Add(Vector{20, 0})
	field.Delete = false
	field.Cargo = Cargo{Ironium: 40}
	saveLogisticsWork(f, &logisticsJob{Kind: "salvage", TargetNum: field.Num, Position: old})
	u.Run(func(tg *turnGenerator) { require.Nil(t, tg.currentLogisticsWork(p, f)) }, (*turnGenerator).resolveLogisticsAutomation)
	require.Zero(t, f.Cargo.Ironium)
	require.Equal(t, 40, field.Cargo.Ironium)
	require.Empty(t, f.GetTag("empty-salvage"))
}

func TestPatrolReconDeduplicatesDesignsAndCancelsReusedContact(t *testing.T) {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
		{Designs: Designs(DesignLongRangeScout, DesignDestroyerDelta), Fleets: []ScenarioFleet{{Name: "Scout A", Design: DesignLongRangeScout.Name}, {Name: "Scout B", Design: DesignLongRangeScout.Name}, {Name: "Patrol", Design: DesignDestroyerDelta.Name}}},
		{Designs: Designs(DesignDestroyerDelta), Fleets: []ScenarioFleet{{Name: "Group A", Design: DesignDestroyerDelta.Name, Quantity: 8, Position: Vector{30, 0}}, {Name: "Group B", Design: DesignDestroyerDelta.Name, Quantity: 8, Position: Vector{35, 0}}}},
	}})
	p, a, b := u.Player(1), u.Fleet("Scout A"), u.Fleet("Scout B")
	p.Race.Spec.DiscoverDesignOnScan = false
	p.Automation.FleetRoles = map[int]string{a.Num: "scout", b.Num: "scout", u.Fleet("Patrol").Num: "patrol"}
	for _, name := range []string{"Group A", "Group B"} {
		e := u.Fleet(name)
		p.discoverer.discoverDesign(e.Tokens[0].design, false)
		p.discoverer.discoverFleet(e, false)
	}
	addExpeditionTestDock(p, Vector{})
	u.Run(func(tg *turnGenerator) {
		require.True(t, tg.planScoutRecon(p, a))
		require.False(t, tg.planScoutRecon(p, b))
	})
	job := readScoutRecon(a)
	require.NotNil(t, job)
	report := p.GetFleetIntel(job.Enemy, job.Target)
	report.Tokens[0].DesignNum = 999
	u.Run((*turnGenerator).refreshScoutReconOrders)
	require.Nil(t, readScoutRecon(a))
	require.Len(t, a.Waypoints, 1)
}
