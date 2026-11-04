//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func civilianRaidUniverse(t *testing.T) *testUniverse {
	bomber := campaignDesign("Refit bomber", B17Bomber.Name, "bomber")
	raider := DesignJihadCruiser
	raider.Slots = append([]ShipDesignSlot(nil), raider.Slots...)
	for i := range raider.Slots {
		if StaticTechStore.GetHullComponent(raider.Slots[i].HullComponent).HullSlotType&HullSlotTypeEngine != 0 {
			raider.Slots[i].HullComponent = RadiatingHydroRamScoop.Name
		}
	}
	for i := range bomber.Slots {
		if StaticTechStore.GetHullComponent(bomber.Slots[i].HullComponent).HullSlotType&HullSlotTypeEngine != 0 {
			bomber.Slots[i].HullComponent = RadiatingHydroRamScoop.Name
		}
	}
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
		{Designs: Designs(bomber, DesignDestroyerDelta, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Refit", Design: bomber.Name, Quantity: 5, At: "Exposed"}}},
		{Designs: Designs(raider), Fleets: []ScenarioFleet{{Name: "Raid", Design: raider.Name, Quantity: 20, Position: Vector{160, 0}}}},
	}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Position: Vector{-20, 0}, Starbase: DesignSpaceStation.Name}, {Name: "Exposed", Owner: 1, Starbase: DesignSpaceStation.Name, Position: Vector{100, 0}, Cargo: Cargo{Colonists: 2500, Ironium: 10000, Boranium: 10000, Germanium: 10000}}}})
	p, f, e := u.Player(1), u.Fleet("Refit"), u.Fleet("Raid")
	p.Automation.FleetRoles = map[int]string{f.Num: "bomber"}
	p.discoverer.discoverDesign(e.Tokens[0].design, true)
	p.discoverer.discoverFleet(e, true)
	r := p.GetFleetIntel(2, e.Num)
	r.WarpSpeed = 9
	r.Heading = VectorFloat64{X: -1}
	u.Run((*turnGenerator).startPatrolEvaluation)
	return u
}

func TestCivilianRefitSafetyPreservesMissionAndPaidQueue(t *testing.T) {
	u := civilianRaidUniverse(t)
	p, f := u.Player(1), u.Fleet("Refit")
	work := &fleetMaintenance{Group: "bomber-refit", OriginalRole: "bomber", Stage: "rally", RallyPlanetNum: u.Planet("Exposed").Num, OriginalOrders: FleetOrders{Waypoints: []Waypoint{{Position: f.Position}, {Position: Vector{200, 0}, WarpSpeed: 6}}}}
	writeMaintenance(f, work)
	f.SetTag(replacementGroupTag, work.Group)
	paid := Cost{Ironium: 5, Resources: 11}
	item := NewProductionQueueItemShip(2, p.Designs[0]).WithTag(replacementOrderTag, encodeMaintenance(work))
	item.Allocated = paid
	u.Planet("Exposed").ProductionQueue = []ProductionQueueItem{*item}
	u.Run(func(tg *turnGenerator) {
		require.False(t, tg.civilianSiteSafe(p, f, f.Position, 1))
		r := tg.safeMaintenanceRally(p, f, work)
		require.NotNil(t, r)
		require.Equal(t, u.Planet("Home").Num, r.Num)
	})
	updated := decodeMaintenance(u.Planet("Exposed").ProductionQueue[0].GetTag(replacementOrderTag))
	require.Equal(t, work.RallyPlanetNum, updated.RallyPlanetNum)
	require.Equal(t, paid, u.Planet("Exposed").ProductionQueue[0].Allocated)
	u.Run((*turnGenerator).planAutomaticSafety)
	require.True(t, f.InAutomaticSafety())
	require.True(t, f.InDesignMaintenance())
	require.Greater(t, len(f.Waypoints), 1)
	require.NotEqual(t, f.Position, f.Waypoints[1].Position)
	before := f.FleetOrders
	u.Run((*turnGenerator).fleetDesignMaintenance)
	require.Equal(t, before, f.FleetOrders, "maintenance must not overwrite evacuation")
	require.Equal(t, work.OriginalOrders, readMaintenance(f).OriginalOrders)
	// Once the reported raid is gone, safety restores the suspended refit rather than canceling it.
	p.FleetIntels = nil
	u.Run((*turnGenerator).startPatrolEvaluation, (*turnGenerator).planAutomaticSafety)
	require.False(t, f.InAutomaticSafety())
	require.True(t, f.InDesignMaintenance())
}

func TestCivilianProductionDefersRaidWithoutLosingAllocation(t *testing.T) {
	u := civilianRaidUniverse(t)
	p, planet := u.Player(1), u.Planet("Exposed")
	bomber, dd := p.Designs[0], p.Designs[1]
	planet.Spec.ResourcesPerYearAvailable = 10000
	item := NewProductionQueueItemShip(2, bomber)
	item.design = bomber
	item.Allocated = Cost{Ironium: 5, Resources: 11}
	defender := NewProductionQueueItemShip(1, dd)
	defender.design = dd
	planet.ProductionQueue = []ProductionQueueItem{*item, *defender}
	u.Run(func(tg *turnGenerator) {
		require.False(t, tg.civilianConstructionSafe(p, planet, bomber))
		require.True(t, tg.civilianConstructionSafe(p, planet, dd))
		producer := newProducer(testLogger, &u.Game.Rules, planet, p)
		producer.deferShip = func(d *ShipDesign) bool { return !tg.civilianConstructionSafe(p, planet, d) }
		result, err := producer.produce()
		require.NoError(t, err)
		require.Len(t, planet.ProductionQueue, 1)
		require.Equal(t, item.Allocated, planet.ProductionQueue[0].Allocated)
		require.Equal(t, 2, planet.ProductionQueue[0].Quantity)
		require.Len(t, result.tokens, 1)
		require.Equal(t, dd.Num, result.tokens[0].DesignNum)
		p.FleetIntels = nil
		tg.civilianWarnings = nil
		require.True(t, tg.civilianConstructionSafe(p, planet, bomber))
	})
}

func TestPatrolMassLimitsExcludeStationAndRejectBadExchange(t *testing.T) {
	outcome := patrolBattleOutcome{MissionSucceeded: true, LostShips: 1, LostValue: 1, LostMass: 50, EnemyMassDestroyed: 49}
	require.False(t, outcome.safe(100, 10000, 10000), "cheap light enemies cannot justify heavier losses")
	outcome.EnemyMassDestroyed = 100
	require.True(t, outcome.safe(100, 10000, 10000))
	require.False(t, outcome.safe(100, 10000, 500), "own tonnage loss must also remain within five percent")
	u := civilianRaidUniverse(t)
	p := u.Player(1)
	f := u.Fleet("Refit")
	base := u.Planet("Exposed").Starbase
	model, err := newPatrolBattleCopy(&u.Game.Rules, p, []*Fleet{f, base}, []*Fleet{p.GetFleetIntel(2, u.Fleet("Raid").Num)})
	require.NoError(t, err)
	require.Equal(t, 5, model.ownShips)
	require.Equal(t, float64(f.Spec.Mass), model.ownMass)
}
