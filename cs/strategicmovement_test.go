//go:build !wasi && !wasm

package cs

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func strategicTestFleet(t *testing.T, engine string) (*Player, *Fleet, *Universe) {
	t.Helper()
	player := NewPlayer(1, NewRace().WithSpec(&rules)).WithNum(1).WithTechLevels(TechLevel{26, 26, 26, 26, 26, 26}).withSpec(&rules)
	player.Relations = []PlayerRelationship{{Relation: PlayerRelationFriend}}
	design := NewShipDesign(1, 1).WithName("Fast scout").WithHull("EEE Pathfinder").WithSlots([]ShipDesignSlot{{HullComponent: engine, HullSlotIndex: 1, Quantity: 1}}).WithSpec(&rules, player)
	require.NoError(t, design.Validate(&rules, player))
	player.Designs = append(player.Designs, design)
	fleet := newFleetForDesign(player, design, 1, 1, design.Name, []Waypoint{NewPositionWaypoint(Vector{}, 0), NewPositionWaypoint(Vector{6000, 0}, 10)})
	fleet.Spec = ComputeFleetSpec(&rules, player, &fleet)
	fleet.Fuel = fleet.Spec.FuelCapacity
	universe := &Universe{log: testLogger, Fleets: []*Fleet{&fleet}}
	require.NoError(t, universe.buildMaps([]*Player{player}))
	return player, &fleet, universe
}

func TestStrategicLegacyEnginesAndMissingSaveField(t *testing.T) {
	var old Engine
	require.NoError(t, json.Unmarshal([]byte(`{"idealSpeed":10,"maxSafeSpeed":10}`), &old))
	require.Equal(t, 100.0, old.StrategicSpeed(10))
	for _, component := range append(append(TechHullComponents(), EEETechHullComponents()...), EEETraderComponents()...) {
		for warp := 0; warp <= 10; warp++ {
			require.Equal(t, float64(warp*warp), component.Engine.StrategicSpeed(warp), component.Name)
		}
	}
	for _, component := range EEEStrategicEngines() {
		require.Zero(t, component.StrategicSpeed(11), "gate sentinel cannot become warp 11")
		require.Zero(t, component.StrategicSpeed(12))
		for warp := 1; warp <= 8; warp++ {
			require.Equal(t, float64(warp*warp), component.StrategicSpeed(warp))
		}
		require.Equal(t, 81*component.StrategicSpeedFactor, component.StrategicSpeed(9))
	}
}

func TestStrategicDrivesActuallyCrossGalacticMapFaster(t *testing.T) {
	for _, tc := range []struct {
		name  string
		years int
		speed float64
	}{
		{TransStar10.Name, 60, 100}, {"EEE Horizon Drive", 40, 150}, {"EEE Deep-Space Drive", 30, 200}, {"EEE Galactic Drive", 20, 300},
	} {
		t.Run(tc.name, func(t *testing.T) {
			player, fleet, universe := strategicTestFleet(t, tc.name)
			require.Equal(t, tc.speed, newFleetMove(fleet).budget)
			require.Equal(t, tc.years, fleet.getYearsToWaypoint(player, 1))
			for year := 0; year < tc.years; year++ {
				require.Len(t, fleet.Waypoints, 2, "must not arrive ahead of predicted year")
				require.Nil(t, fleet.moveFleet(&rules, universe, newTestPlayerGetter(player)))
			}
			require.Equal(t, Vector{6000, 0}, fleet.Position)
			require.Len(t, fleet.Waypoints, 1)
			require.GreaterOrEqual(t, fleet.Fuel, 0)
		})
	}
}

func TestStrategicFleetMinimumDoesNotTowOrdinaryShips(t *testing.T) {
	player, fleet, _ := strategicTestFleet(t, "EEE Galactic Drive")
	_, medium, _ := strategicTestFleet(t, "EEE Deep-Space Drive")
	_, ordinary, _ := strategicTestFleet(t, TransStar10.Name)
	fleet.Tokens = append(fleet.Tokens, medium.Tokens[0])
	fleet.Spec = ComputeFleetSpec(&rules, player, fleet)
	require.Equal(t, 200.0, fleet.StrategicSpeed(10))
	fleet.Tokens = append(fleet.Tokens, ordinary.Tokens[0])
	fleet.Spec = ComputeFleetSpec(&rules, player, fleet)
	require.Equal(t, 100.0, fleet.StrategicSpeed(10))
	fleet.Tokens = fleet.Tokens[:1]
	fleet.Spec = ComputeFleetSpec(&rules, player, fleet)
	require.Equal(t, 300.0, fleet.StrategicSpeed(10))
}

func TestStrategicFuelUsesActualDistanceAndPursuitDoesNotRoundEachStep(t *testing.T) {
	player, fleet, universe := strategicTestFleet(t, "EEE Galactic Drive")
	initial := fleet.Fuel
	cost := fleet.GetFuelCost(player, 10, 300)
	move := newFleetMove(fleet)
	for range 5 {
		require.Nil(t, fleet.moveStep(&rules, universe, newTestPlayerGetter(player), move, 60))
	}
	require.Equal(t, Vector{300, 0}, fleet.Position)
	require.Equal(t, cost, initial-fleet.Fuel)
	require.Equal(t, cost, move.fuelUsed)
	// Three times the distance costs three times the unrounded fuel, not one
	// ordinary warp-10 turn's fuel. Fuel generation is disabled above free warp.
	require.InDelta(t, 3*fleet.fuelUsage(player, 10, 100), fleet.fuelUsage(player, 10, 300), 1e-9)
	require.Zero(t, move.fuelGenerated)
	player, fleet, universe = strategicTestFleet(t, "EEE Galactic Drive")
	fleet.Fuel = 5
	move = newFleetMove(fleet)
	fleet.moveStep(&rules, universe, newTestPlayerGetter(player), move, move.budget)
	require.True(t, move.ranOutOfFuel)
	require.Zero(t, fleet.Fuel)
	require.Less(t, fleet.Position.X, 300)
	require.Equal(t, 1, fleet.Waypoints[1].WarpSpeed)
	require.Equal(t, 1.0, fleet.StrategicSpeed(fleet.Waypoints[1].WarpSpeed))
}

func TestStrategicMineCollisionBeyondOrdinaryRangeAndDuringShortPursuit(t *testing.T) {
	for _, tc := range []struct {
		field int
		step  float64
	}{{200, 300}, {2, 4}} {
		player, fleet, universe := strategicTestFleet(t, "EEE Galactic Drive")
		enemy := NewPlayer(2, NewRace().WithSpec(&rules)).WithNum(2).withSpec(&rules)
		universe.Minefields = []*Minefield{newMinefield(enemy, MinefieldTypeStandard, 100, 1, Vector{tc.field, 0})}
		localRules := NewRulesWithSeed(0)
		localRules.random = newFloat64Random(0)
		move := newFleetMove(fleet)
		hit := fleet.moveStep(&localRules, universe, newTestPlayerGetter(player, enemy), move, tc.step)
		require.NotNil(t, hit)
		require.EqualValues(t, fleetMoveInterruptedHitMinefield, hit.reason)
		require.Less(t, move.moved, tc.step)
	}
}

func TestStrategicPursuitUsesTargetVelocityWithoutItsEngine(t *testing.T) {
	_, fleet, _ := strategicTestFleet(t, "EEE Galactic Drive")
	target := &Fleet{WarpSpeed: 10, ObservedSpeed: 200}
	require.Equal(t, 200.0, target.observedStrategicSpeed())
	years, dist, caught := pursueAtSpeed(VectorFloat64{}, VectorFloat64{X: 100}, VectorFloat64{X: target.observedStrategicSpeed()}, fleet.StrategicSpeed(10))
	require.True(t, caught)
	require.Equal(t, 1, years)
	require.Equal(t, 300.0, dist)
	_, _, caught = pursueAtSpeed(VectorFloat64{}, VectorFloat64{X: 100}, VectorFloat64{X: 200}, TransStar10.StrategicSpeed(10))
	require.False(t, caught)
	target.WarpSpeed = StargateWarpSpeed
	require.Zero(t, target.observedStrategicSpeed())
}

func TestStrategicRouteEtaAndMixedConvoyStayTogether(t *testing.T) {
	player, fleet, _ := strategicTestFleet(t, "EEE Galactic Drive")
	addExpeditionTestDock(player, Vector{})
	destination := Vector{600, 0}
	route := planAutomaticRoute(&rules, player, []*Fleet{fleet}, destination, MapObjectTarget{}, automaticRoutePolicy{})
	require.NotNil(t, route)
	require.Equal(t, 2, route.Turns)
	require.Equal(t, 10, route.Waypoints[1].WarpSpeed)
	_, ordinary, _ := strategicTestFleet(t, TransStar10.Name)
	route = planAutomaticRoute(&rules, player, []*Fleet{fleet, ordinary}, destination, MapObjectTarget{}, automaticRoutePolicy{})
	require.NotNil(t, route)
	require.LessOrEqual(t, route.Waypoints[1].WarpSpeed, 8)
	require.Equal(t, fleet.StrategicSpeed(route.Waypoints[1].WarpSpeed), ordinary.StrategicSpeed(route.Waypoints[1].WarpSpeed))
	require.Equal(t, int(math.Ceil(600/fleet.StrategicSpeed(route.Waypoints[1].WarpSpeed))), route.Turns)
}

func TestStrategicAutomaticFuelAccountsForFewerGeneratorYears(t *testing.T) {
	player, fleet, _ := strategicTestFleet(t, "EEE Galactic Drive")
	fleet.Spec.FuelGeneration = 10
	_, left, burned, ok := automaticLegFuel(player, fleet, 10, 600, 1000, 0)
	require.True(t, ok)
	require.Equal(t, 1000-burned+2*10, left, "two movement years earn two generator ticks")
	fleet.Spec.Engine.StrategicSpeedFactor = 0
	_, left, burned, ok = automaticLegFuel(player, fleet, 10, 600, 1000, 0)
	require.True(t, ok)
	require.Equal(t, 1000-burned+6*10, left)
}

func TestEEELateHullsValidateUnlockAndPreserveSpecialists(t *testing.T) {
	player := NewPlayer(1, NewRace().WithSpec(&rules)).WithTechLevels(TechLevel{26, 26, 26, 26, 26, 26}).withSpec(&rules)
	for _, hull := range EEELateHulls() {
		require.NotNil(t, StaticTechStore.GetHull(hull.Name))
		design := NewShipDesign(1, 1).WithName(hull.Name).WithHull(hull.Name).WithSlots([]ShipDesignSlot{{HullComponent: "EEE Galactic Drive", HullSlotIndex: 1, Quantity: hull.Slots[0].Capacity}}).WithSpec(&rules, player)
		require.NoError(t, design.Validate(&rules, player))
		require.Equal(t, hull.Mass+22*hull.Slots[0].Capacity, design.Spec.Mass)
		require.LessOrEqual(t, design.Spec.Movement, rules.MovementMax)
	}
	prospector := StaticTechStore.GetHull("EEE Prospector")
	player.Race.WithLRT(OBRM)
	require.False(t, player.HasTech(&prospector.Tech))
	bulk := StaticTechStore.GetHull("EEE Bulk Carrier")
	require.Less(t, bulk.CargoCapacity, SuperFreighter.CargoCapacity)
	require.Less(t, bulk.FuelCapacity, SuperFreighter.FuelCapacity)
	require.Less(t, StaticTechStore.GetHull("EEE Linebreaker").Armor, Dreadnought.Armor)
}

func TestStrategicActualPursuitAndDiscovery(t *testing.T) {
	design := func(name, drive string) ShipDesign {
		return ShipDesign{Name: name, Hull: "EEE Pathfinder", Slots: []ShipDesignSlot{{HullComponent: drive, HullSlotIndex: 1, Quantity: 1}}}
	}
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{
		Designs: []ShipDesign{design("Pursuer", "EEE Galactic Drive"), design("Runner", "EEE Deep-Space Drive")},
		Fleets:  []ScenarioFleet{{Name: "Pursuer", Design: "Pursuer"}, {Name: "Runner", Design: "Runner", Position: Vector{100, 0}}},
	}}})
	pursuer, runner := u.Fleet("Pursuer"), u.Fleet("Runner")
	pursuer.Waypoints = []Waypoint{NewPositionWaypoint(pursuer.Position, 0), NewFleetWaypoint(runner.Position, runner.Num, runner.PlayerNum, runner.Name, 10)}
	runner.Waypoints = []Waypoint{NewPositionWaypoint(runner.Position, 0), NewPositionWaypoint(Vector{1000, 0}, 10)}
	u.Run((*turnGenerator).fleetMove)
	require.Equal(t, Vector{300, 0}, runner.Position)
	require.Equal(t, runner.Position, pursuer.Position)
	observer := NewPlayer(2, NewRace().WithSpec(&rules)).WithNum(2)
	newDiscoverer(testLogger, observer).discoverFleet(runner, false)
	intel := observer.GetFleetIntel(runner.PlayerNum, runner.Num)
	require.Equal(t, 200.0, intel.ObservedSpeed)
	require.Zero(t, intel.Spec.Engine.StrategicSpeedFactor, "observed velocity must not disclose engine research")
}

func TestStrategicLoadedFuelIFEAndCombatRemainIndependent(t *testing.T) {
	player, fleet, _ := strategicTestFleet(t, "EEE Galactic Drive")
	emptyCost := fleet.GetFuelCost(player, 10, 300)
	fleet.Cargo = Cargo{Ironium: 80}
	loadedCost := fleet.GetFuelCost(player, 10, 300)
	require.Greater(t, loadedCost, emptyCost)
	player.Race.WithLRT(IFE).WithSpec(&rules)
	require.Less(t, fleet.GetFuelCost(player, 10, 300), loadedCost)
	design := fleet.Tokens[0].design
	movement := design.getMovement(&rules, 80)
	design.Spec.Engine.StrategicSpeedFactor = 0
	require.Equal(t, movement, design.getMovement(&rules, 80), "strategic boost is not a combat movement bonus")
	require.LessOrEqual(t, movement, rules.MovementMax)
}

func TestEEESpecialistsHaveCompetitiveEquipmentAndNonOverlappingLayouts(t *testing.T) {
	capacity := func(h *TechHull, kind HullSlotType) int {
		n := 0
		for _, slot := range h.Slots {
			if slot.Type == kind {
				n += slot.Capacity
			}
		}
		return n
	}
	bomber := StaticTechStore.GetHull("EEE Expedition Bomber")
	require.Greater(t, capacity(bomber, HullSlotTypeBomb), capacity(&B52Bomber, HullSlotTypeBomb))
	miner := StaticTechStore.GetHull("EEE Prospector")
	require.Greater(t, capacity(miner, HullSlotTypeMining), capacity(&MaxiMiner, HullSlotTypeMining))
	require.Equal(t, 600, StaticTechStore.GetHull("EEE Fleet Tender").FuelGeneration)
	for _, h := range EEELateHulls() {
		seen := map[VectorFloat64]bool{}
		for _, slot := range h.Slots {
			require.False(t, seen[slot.Position], h.Name)
			seen[slot.Position] = true
			if h.CargoSlotSize.X > 0 && h.CargoSlotSize.Y > 0 {
				overlaps := slot.Position.X < h.CargoSlotPosition.X+h.CargoSlotSize.X && slot.Position.X+1 > h.CargoSlotPosition.X && slot.Position.Y < h.CargoSlotPosition.Y+h.CargoSlotSize.Y && slot.Position.Y+1 > h.CargoSlotPosition.Y
				require.False(t, overlaps, h.Name)
			}
		}
	}
}
