//go:build !wasi && !wasm

package cs

import (
	"encoding/json"
	"slices"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func tankerUniverse(t *testing.T) *testUniverse {
	tanker := ShipDesign{Name: "Network Tanker", Hull: SuperFuelXport.Name, Slots: []ShipDesignSlot{{HullComponent: LongHump6.Name, HullSlotIndex: 1, Quantity: 2}}}
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Designs: Designs(DesignTeamster, tanker, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Customer", Design: DesignTeamster.Name, Position: Vector{60, 0}, EmptyFuel: true}, {Name: "Tanker", Design: tanker.Name, At: "Home"}}}}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name, Cargo: Cargo{Colonists: 2500}}, {Name: "Colony", Owner: 1, Position: Vector{180, 0}, Cargo: Cargo{Colonists: 1000}}}})
	p := u.Player(1)
	p.Automation.FleetRoles = map[int]string{u.Fleet("Customer").Num: "freighter", u.Fleet("Tanker").Num: "tanker"}
	u.Run((*turnGenerator).refreshAutomaticFuelIntel)
	return u
}

func TestAutomaticTankerRescuesWithoutCreatingFuelAndRetainsReturnReserve(t *testing.T) {
	u := tankerUniverse(t)
	p, customer, tanker := u.Player(1), u.Fleet("Customer"), u.Fleet("Tanker")
	saveAutomaticFuelRequest(customer, automaticFuelRequest{Destination: customer.Position, Members: []int{customer.Num}, Urgent: true})
	u.Run((*turnGenerator).planAutomaticTankers)
	require.Equal(t, strconv.Itoa(tanker.Num), customer.GetTag(tankerAwaitTag))
	require.Equal(t, customer.Num, readTankerJob(tanker).TargetNum)
	require.Zero(t, customer.Fuel)
	for range 20 {
		u.Run((*turnGenerator).planAutomaticTankers, (*turnGenerator).fleetMove, (*turnGenerator).fleetRefuel)
		before := customer.Fuel + tanker.Fuel
		u.Run((*turnGenerator).resolveAutomaticTankers)
		require.Equal(t, before, customer.Fuel+tanker.Fuel, "transferring fuel must conserve it")
		if customer.GetTag(tankerAwaitTag) == "" {
			break
		}
	}
	require.Positive(t, customer.Fuel)
	require.Empty(t, customer.GetTag(tankerAwaitTag))
	reserve := automaticReturnFuel(&u.Game.Rules, p, tanker, tanker.Position, expeditionStations(p, tanker), false)
	require.GreaterOrEqual(t, tanker.Fuel, reserve)
	require.LessOrEqual(t, customer.Fuel, customer.Spec.FuelCapacity)
	u.Run((*turnGenerator).planAutomaticTankers)
	require.NotNil(t, readTankerJob(tanker))
}

func TestAutomaticTankerDoesNotInventInitialFuelOrChaseLostFleetNumbers(t *testing.T) {
	u := tankerUniverse(t)
	customer, tanker := u.Fleet("Customer"), u.Fleet("Tanker")
	tanker.Fuel = 0
	saveAutomaticFuelRequest(customer, automaticFuelRequest{Destination: customer.Position, Members: []int{customer.Num}, Urgent: true})
	u.Run((*turnGenerator).planAutomaticTankers)
	require.Len(t, tanker.Waypoints, 1)
	require.Zero(t, tanker.Fuel)
	require.Empty(t, customer.GetTag(tankerAwaitTag))
	tanker.Fuel = tanker.Spec.FuelCapacity
	u.Run((*turnGenerator).planAutomaticTankers)
	require.NotEmpty(t, customer.GetTag(tankerAwaitTag))
	u.Game.deleteFleet(tanker)
	u.Run((*turnGenerator).planAutomaticTankers)
	require.Empty(t, customer.GetTag(tankerAwaitTag))
	require.NotNil(t, readAutomaticFuelRequest(customer))
}

func TestAutomaticTankersSpreadIntoNetworkAndPatrolSlowly(t *testing.T) {
	u := tankerUniverse(t)
	p := u.Player(1)
	customer := u.Fleet("Customer")
	delete(p.Automation.FleetRoles, customer.Num)
	tanker := u.Fleet("Tanker")
	second, err := u.turn.buildFleet(p, u.Planet("Home"), ShipToken{DesignNum: tanker.Tokens[0].DesignNum, Quantity: 1, design: tanker.Tokens[0].design}, nil, nil)
	require.NoError(t, err)
	p.Automation.FleetRoles[second.Num] = "tanker"
	u.Run((*turnGenerator).planAutomaticTankers)
	a, b := readTankerJob(tanker), readTankerJob(second)
	require.NotNil(t, a)
	require.NotNil(t, b)
	require.Equal(t, "patrol", a.Stage)
	require.Equal(t, "patrol", b.Stage)
	require.NotEqual(t, a.Anchor, b.Anchor)
	require.Greater(t, a.Anchor.DistanceTo(b.Anchor), 50.0)
	for _, fleet := range []*Fleet{tanker, second} {
		if len(fleet.Waypoints) > 1 {
			require.LessOrEqual(t, fleet.Waypoints[1].WarpSpeed, 4)
		}
		require.Greater(t, fleet.Spec.FuelCapacity/3, 0)
	}
	anchor := a.Anchor
	u.Game.Year++
	u.Run((*turnGenerator).planAutomaticTankers)
	require.Equal(t, anchor, readTankerJob(tanker).Anchor, "do not reassign the patrol area every turn")
}

func TestNetworkTankerApproachesCustomerAndKeepsConvoyTogether(t *testing.T) {
	u := tankerUniverse(t)
	customer, tanker := u.Fleet("Customer"), u.Fleet("Tanker")
	customer.Position = Vector{20, 0}
	customer.Fuel = 70
	for _, token := range customer.Tokens {
		token.design.Spec.Mass = 100
		token.design.Spec.Engine.FuelUsage = []int{0, 5, 5, 5, 5, 5, 100, 100, 100, 1000, 1000}
		token.design.Spec.Engine.FreeSpeed = 0
		token.design.Spec.Engine.MaxSafeSpeed = 8
	}
	customer.Spec.Engine.MaxSafeSpeed = 8
	customer.Spec.CargoCapacity = 0
	tanker.Position = Vector{100, 0}
	tanker.OrbitingPlanetNum = None
	writeTankerJob(tanker, &automaticTankerJob{Stage: "patrol", Anchor: tanker.Position, Position: tanker.Position, PatrolYear: u.Game.Year})
	customer.Waypoints = []Waypoint{{Position: customer.Position}, NewPositionWaypoint(Vector{240, 0}, 5)}
	saveAutomaticFuelRequest(customer, automaticFuelRequest{Destination: Vector{240, 0}, Members: []int{customer.Num}, BaselineTurns: 20})
	u.Run((*turnGenerator).planAutomaticTankers)
	job := readTankerJob(tanker)
	require.Equal(t, customer.Num, job.TargetNum)
	require.NotEqual(t, tanker.Position, job.Position, "a deployed tanker may approach its customer")
	require.Equal(t, job.Position, customer.Waypoints[len(customer.Waypoints)-1].Position)
	require.Greater(t, len(tanker.Waypoints), 1)
	require.Equal(t, job.Position, tanker.Waypoints[len(tanker.Waypoints)-1].Position)
}

func TestFuelPlannerWaitsForActualDockRefillAndIgnoresBareColonies(t *testing.T) {
	u := tankerUniverse(t)
	p, f := u.Player(1), u.Fleet("Customer")
	f.Position = u.Planet("Home").Position
	f.OrbitingPlanetNum = u.Planet("Home").Num
	f.Fuel = 0
	goal := u.Planet("Colony")
	route := planExpeditionRoute(&u.Game.Rules, p, f, goal)
	require.NotNil(t, route)
	require.True(t, route.WaitForFuel)
	applyExpeditionRoute(p, f, route)
	require.Len(t, f.Waypoints, 1)
	require.Zero(t, f.Fuel)
	u.Run((*turnGenerator).fleetRefuel)
	require.Equal(t, f.Spec.FuelCapacity, f.Fuel)
	u.Planet("Home").Spec.DockCapacity = 0
	u.Planet("Home").Spec.HasStarbase = false
	u.Run((*turnGenerator).refreshAutomaticFuelIntel)
	require.Empty(t, expeditionStations(p, f))
	require.Nil(t, planExpeditionRoute(&u.Game.Rules, p, f, goal))
}

func TestFuelPlannerModelsGeneratorsWithoutSpendingFutureFuel(t *testing.T) {
	u := tankerUniverse(t)
	p, f := u.Player(1), u.Fleet("Tanker")
	for _, token := range f.Tokens {
		token.design.Spec.Engine.FreeSpeed = 0
		token.design.Spec.Engine.FuelUsage = []int{0, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100}
	}
	f.Spec.FuelGeneration = 200
	_, _, _, ok := automaticLegFuel(p, f, 5, 400, 0, 0)
	require.False(t, ok, "generation happens after movement")
	charge := f.GetFuelCost(p, 5, 26)
	_, left, _, ok := automaticLegFuel(p, f, 5, 400, charge, 0)
	require.True(t, ok)
	require.Greater(t, left, charge)
}

func TestFuelReserveIncludesCargoAndConvoyUsesIndependentBudgets(t *testing.T) {
	u := logisticsUniverse(t)
	p, f, escort := u.Player(1), u.Fleet("Transport"), u.Fleet("Escort")
	for _, token := range f.Tokens {
		token.design.Spec.Engine.FreeSpeed = 0
		token.design.Spec.Engine.FuelUsage = []int{0, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100}
	}
	goal := Vector{50, 0}
	reserve := automaticReturnFuel(&u.Game.Rules, p, f, goal, expeditionStations(p, f), false)
	empty := *f
	empty.Spec.CargoCapacity = 0
	emptyReserve := automaticReturnFuel(&u.Game.Rules, p, &empty, goal, expeditionStations(p, f), false)
	require.Greater(t, reserve, emptyReserve)
	escort.Fuel = 0
	route := planAutomaticRoute(&u.Game.Rules, p, []*Fleet{f, escort}, goal, MapObjectTarget{}, automaticRoutePolicy{})
	require.NotNil(t, route)
	require.True(t, route.WaitForFuel, "one full transport cannot lend its budget to an empty escort")
	escort.Fuel = escort.Spec.FuelCapacity
	route = planAutomaticRoute(&u.Game.Rules, p, []*Fleet{f, escort}, goal, MapObjectTarget{}, automaticRoutePolicy{})
	require.NotNil(t, route)
	applyExpeditionRoute(p, f, route)
	applyExpeditionRoute(p, escort, route)
	old := escort.Waypoints[0]
	f.Waypoints[0].WarpSpeed = 9
	require.Equal(t, old, escort.Waypoints[0], "convoy orders must not alias")
}

func TestFuelPlannerMineSpeedSafeWarpAndFixedMovingTargets(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Transport")
	p.MinefieldIntels = []*Minefield{{MapObject: MapObject{PlayerNum: 2, Position: Vector{25, 0}}, MinefieldType: MinefieldTypeStandard, NumMines: 100}}
	require.True(t, routeAutomatically(&u.Game.Rules, p, f, Vector{50, 0}, MapObjectTarget{TargetType: MapObjectTypeFleet, TargetNum: 99}, 0))
	require.LessOrEqual(t, f.Waypoints[1].WarpSpeed, 4)
	require.Equal(t, MapObjectTypeNone, f.Waypoints[1].TargetType)
	p.MinefieldIntels[0].PlayerNum = p.Num
	require.True(t, automaticLegSafe(&u.Game.Rules, p, Vector{}, Vector{50, 0}, 8, false))
	p.MinefieldIntels[0].Detonate = true
	require.False(t, routeAutomatically(&u.Game.Rules, p, f, Vector{50, 0}, MapObjectTarget{}, 0))
	require.Len(t, f.Waypoints, 1)
	p.MinefieldIntels = nil
	p.Race.Spec.EngineFailureRate = .1
	p.Race.Spec.EngineReliableSpeed = 4
	require.True(t, routeAutomatically(&u.Game.Rules, p, f, Vector{50, 0}, MapObjectTarget{}, 0))
	require.LessOrEqual(t, f.Waypoints[1].WarpSpeed, 4)
}

func TestTankerJobsSurviveSerializationAndRoleCancellationRestoresMission(t *testing.T) {
	u := tankerUniverse(t)
	p, customer, tanker := u.Player(1), u.Fleet("Customer"), u.Fleet("Tanker")
	original := []Waypoint{{Position: customer.Position}, NewPositionWaypoint(Vector{90, 0}, 5)}
	customer.Waypoints = slices.Clone(original)
	saveAutomaticFuelRequest(customer, automaticFuelRequest{Destination: Vector{90, 0}, Members: []int{customer.Num}, Urgent: true})
	u.Run((*turnGenerator).planAutomaticTankers)
	raw, err := json.Marshal(readTankerJob(tanker))
	require.NoError(t, err)
	var loaded automaticTankerJob
	require.NoError(t, json.Unmarshal(raw, &loaded))
	require.Equal(t, readTankerJob(tanker), &loaded)
	delete(p.Automation.FleetRoles, tanker.Num)
	u.Run((*turnGenerator).planAutomaticTankers)
	require.Empty(t, customer.GetTag(tankerAwaitTag))
	require.Equal(t, Vector{90, 0}, customer.Waypoints[len(customer.Waypoints)-1].Position)
}

func TestAutomaticFuelNetworkThroughRealTurnPipeline(t *testing.T) {
	u := tankerUniverse(t)
	customer := u.Fleet("Customer")
	tanker := u.Fleet("Tanker")
	require.NoError(t, u.turn.generateTurn())
	require.Positive(t, customer.Fuel, "an empty automatic fleet should receive help through the normal turn phases")
	for range 18 {
		require.NoError(t, u.turn.generateTurn())
		for _, fleet := range u.Game.Fleets {
			if !fleet.Delete {
				require.GreaterOrEqual(t, fleet.Fuel, 0)
				require.LessOrEqual(t, fleet.Fuel, fleet.Spec.FuelCapacity)
			}
		}
	}
	require.NotNil(t, readTankerJob(tanker))
	require.Equal(t, "patrol", readTankerJob(tanker).Stage)
}

func TestAutomaticDepartureRechecksPayloadAndStopsWholeConvoy(t *testing.T) {
	u := logisticsUniverse(t)
	p, f, escort := u.Player(1), u.Fleet("Transport"), u.Fleet("Escort")
	p.Automation.FleetRoles[escort.Num] = "escort"
	escort.SetTag(escortTag, strconv.Itoa(f.Num))
	goal := Vector{50, 0}
	f.Waypoints = []Waypoint{{Position: f.Position}, NewPositionWaypoint(goal, 5)}
	escort.Waypoints = slices.Clone(f.Waypoints)
	f.Fuel = 0
	u.Run((*turnGenerator).enforceAutomaticDeparture)
	require.Len(t, f.Waypoints, 1)
	require.Len(t, escort.Waypoints, 1)
	require.NotEmpty(t, f.GetTag(fuelResumeTag))
	require.NotEmpty(t, escort.GetTag(fuelStatusTag))
}

func TestAutomaticFuelRoutingForEveryManagedFleetRole(t *testing.T) {
	for _, role := range []string{"scout", "miner", "freighter", "escort", "tanker", "maintenance", "assembly"} {
		t.Run(role, func(t *testing.T) {
			u := logisticsUniverse(t)
			p, f := u.Player(1), u.Fleet("Transport")
			f.Position = Vector{10, 0}
			f.Fuel = 0
			p.Automation.FleetRoles = map[int]string{}
			switch role {
			case "maintenance":
				writeMaintenance(f, &fleetMaintenance{Group: "test"})
			case "assembly":
				f.SetTag(assemblyTag, "1")
				p.Automation.FleetAssemblies = map[int]FleetAssembly{1: {Revision: 1, LeaderFleetNum: f.Num}}
			default:
				p.Automation.FleetRoles[f.Num] = role
			}
			f.Waypoints = []Waypoint{{Position: f.Position}, NewPositionWaypoint(Vector{100, 0}, 9)}
			u.Run((*turnGenerator).enforceAutomaticDeparture)
			require.Len(t, f.Waypoints, 1)
			require.Zero(t, f.Fuel)
		})
	}
}

func TestFuelPlannerUsesSafeOwnedStargatesWithoutOvergating(t *testing.T) {
	u := tankerUniverse(t)
	p, f := u.Player(1), u.Fleet("Customer")
	home, destination := u.Planet("Home"), u.Planet("Colony")
	f.Position = home.Position
	f.OrbitingPlanetNum = home.Num
	f.Cargo = Cargo{}
	for _, planet := range []*Planet{home, destination} {
		planet.Spec.DockCapacity = -1
		planet.Spec.HasStarbase = true
		planet.Spec.HasStargate = true
		planet.Spec.SafeHullMass = 1000
		planet.Spec.SafeRange = 500
	}
	u.Run((*turnGenerator).refreshAutomaticFuelIntel)
	route := planExpeditionRoute(&u.Game.Rules, p, f, destination)
	require.NotNil(t, route)
	require.Equal(t, StargateWarpSpeed, route.Waypoints[1].WarpSpeed)
	applyExpeditionRoute(p, f, route)
	u.Run((*turnGenerator).enforceAutomaticDeparture)
	require.Len(t, f.Waypoints, 2)
	destination.Spec.SafeHullMass = 1
	u.Run((*turnGenerator).refreshAutomaticFuelIntel)
	route = planExpeditionRoute(&u.Game.Rules, p, f, destination)
	require.NotNil(t, route)
	for _, wp := range route.Waypoints[1:] {
		require.NotEqual(t, StargateWarpSpeed, wp.WarpSpeed, "overweight hulls must use normal safe travel")
	}
}

func TestTankerQueueMetadataCannotBeForged(t *testing.T) {
	for _, key := range []string{tankerJobTag, tankerAwaitTag, fuelAwaitResumeTag, fuelResumeTag, fuelRequestTag} {
		item := ProductionQueueItem{Type: QueueItemTypeShipToken, DesignNum: 1, Quantity: 1, Tags: Tags{key: "forged"}}
		require.Error(t, validateMaintenanceQueueEdits(nil, []ProductionQueueItem{item}))
	}
}

func TestTankerReleasesCustomerWhoseGeneratorAlreadyRefilledIt(t *testing.T) {
	u := tankerUniverse(t)
	customer, tanker := u.Fleet("Customer"), u.Fleet("Tanker")
	saveAutomaticFuelRequest(customer, automaticFuelRequest{Destination: customer.Position, Members: []int{customer.Num}, Urgent: true})
	u.Run((*turnGenerator).planAutomaticTankers)
	require.NotEmpty(t, customer.GetTag(tankerAwaitTag))
	customer.Fuel = customer.Spec.FuelCapacity
	u.Run((*turnGenerator).planAutomaticTankers)
	require.Empty(t, customer.GetTag(tankerAwaitTag))
	require.Zero(t, readTankerJob(tanker).TargetNum)
}

func TestLostFuelDockDoesNotRevealForeignPlanetData(t *testing.T) {
	u := tankerUniverse(t)
	p, home := u.Player(1), u.Planet("Home")
	oldCargo := p.GetPlanetIntel(home.Num).Cargo
	home.PlayerNum = 2
	home.Cargo.Germanium += 9999
	u.Run((*turnGenerator).refreshAutomaticFuelIntel)
	intel := p.GetPlanetIntel(home.Num)
	require.Zero(t, intel.Spec.DockCapacity)
	require.Equal(t, oldCargo, intel.Cargo)
}

func TestIdleEscortCanRescueScoutWithoutAbandoningAssignedConvoy(t *testing.T) {
	u := tankerUniverse(t)
	p, scout, escort := u.Player(1), u.Fleet("Customer"), u.Fleet("Tanker")
	scout.Position = Vector{180, 0}
	p.Automation.FleetRoles[scout.Num] = "scout"
	p.Automation.FleetRoles[escort.Num] = "escort"
	escort.SetTag(escortTag, "99")
	require.False(t, automaticFuelDonor(p, escort))
	delete(escort.Tags, escortTag)
	saveAutomaticFuelRequest(scout, automaticFuelRequest{Destination: scout.Position, Members: []int{scout.Num}, Urgent: true})
	u.Run((*turnGenerator).planAutomaticTankers)
	require.Equal(t, strconv.Itoa(escort.Num), scout.GetTag(tankerAwaitTag))
	for range 20 {
		u.Run((*turnGenerator).planLogisticsAutomation, (*turnGenerator).planAutomaticTankers, (*turnGenerator).fleetMove, (*turnGenerator).fleetRefuel)
		before := scout.Fuel + escort.Fuel
		u.Run((*turnGenerator).resolveAutomaticTankers)
		require.Equal(t, before, scout.Fuel+escort.Fuel)
		if scout.GetTag(tankerAwaitTag) == "" {
			break
		}
	}
	require.Positive(t, scout.Fuel)
	require.Empty(t, scout.GetTag(tankerAwaitTag))
	require.Equal(t, "escort", p.Automation.FleetRoles[escort.Num])
	require.GreaterOrEqual(t, escort.Fuel, automaticReturnFuel(&u.Game.Rules, p, escort, escort.Position, expeditionStations(p, escort), false))
}
