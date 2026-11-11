//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"slices"
	"testing"
)

func networkFixture(t *testing.T) (*testUniverse, *Fleet, []*Fleet, *Planet) {
	u := tankerUniverse(t)
	p, customer := u.Player(1), u.Fleet("Customer")
	customer.Position = Vector{}
	customer.OrbitingPlanetNum = u.Planet("Home").Num
	customer.Spec.CargoCapacity = 0
	for _, token := range customer.Tokens {
		token.design.Spec.Engine.FreeSpeed = 0
		token.design.Spec.Engine.FuelUsage = []int{0, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100}
	}
	charge := customer.GetFuelCost(p, 5, 20)
	for _, token := range customer.Tokens {
		token.design.Spec.CargoCapacity = 0
		token.design.Spec.FuelCapacity = max(2, charge*3/2)
	}
	customer.Spec = ComputeFleetSpec(&u.Game.Rules, p, customer)
	customer.Fuel = customer.Spec.FuelCapacity
	goal := *u.Planet("Home")
	goal.Num = 3
	goal.Name = "Destination dock"
	goal.Position = Vector{80, 0}
	u.Game.Planets = append(u.Game.Planets, &goal)
	donors := []*Fleet{u.Fleet("Tanker")}
	for i := 1; i < 3; i++ {
		copy := *donors[0]
		copy.Num = 2 + i
		copy.Tags = nil
		copy.Waypoints = nil
		copy.Tokens = slices.Clone(copy.Tokens)
		u.Game.Fleets = append(u.Game.Fleets, &copy)
		p.Automation.FleetRoles[copy.Num] = "tanker"
		donors = append(donors, &copy)
	}
	for i, donor := range donors {
		donor.Position = Vector{20 * (i + 1), 0}
		donor.OrbitingPlanetNum = 0
		donor.Waypoints = []Waypoint{{Position: donor.Position}}
	}
	customer.Waypoints = []Waypoint{{Position: customer.Position}, {Position: goal.Position, MapObjectTarget: goal.MapObject.ToTarget(), WarpSpeed: 5}}
	u.Run((*turnGenerator).refreshAutomaticFuelIntel)
	return u, customer, donors, &goal
}

func TestNetworkPlansAndExecutesThreeFiniteTankerStops(t *testing.T) {
	u, customer, donors, goal := networkFixture(t)
	p := u.Player(1)
	require.Nil(t, planFuelNetworkRoute(&u.Game.Rules, p, []*Fleet{customer}, nil, goal.Position, goal.MapObject.ToTarget(), automaticRoutePolicy{}))
	route := planFuelNetworkRoute(&u.Game.Rules, p, []*Fleet{customer}, donors, goal.Position, goal.MapObject.ToTarget(), automaticRoutePolicy{})
	require.NotNil(t, route)
	stops := 0
	for _, leg := range route.Legs {
		if len(leg.Donations) > 0 {
			stops++
		}
	}
	require.Equal(t, 3, stops)
	require.Equal(t, 4, route.Turns)
	u.Run(func(tg *turnGenerator) {
		require.True(t, tg.commitFuelNetwork(p, []*Fleet{customer}, automaticFuelRequest{Destination: goal.Position}, route))
	})
	for _, donor := range donors {
		require.NotNil(t, readFuelNetworkHold(donor))
	}
	for range 8 {
		fuel := customer.Fuel
		for _, donor := range donors {
			fuel += donor.Fuel
		}
		u.Run((*turnGenerator).advanceFuelNetworks)
		after := customer.Fuel
		for _, donor := range donors {
			after += donor.Fuel
		}
		require.Equal(t, fuel, after, "reserved transfers must conserve fuel")
		u.Run((*turnGenerator).planLogisticsAutomation, (*turnGenerator).enforceAutomaticDeparture, (*turnGenerator).fleetMove, (*turnGenerator).fleetRefuel, (*turnGenerator).advanceFuelNetworks)
		require.GreaterOrEqual(t, customer.Fuel, 0)
		u.Game.Year++
		if !fuelNetworkManaged(customer) {
			break
		}
	}
	require.Equal(t, goal.Position, customer.Position)
	require.False(t, fuelNetworkManaged(customer))
	require.Equal(t, "freighter", p.Automation.FleetRoles[customer.Num])
	for _, donor := range donors {
		require.Nil(t, readFuelNetworkHold(donor))
		require.GreaterOrEqual(t, donor.Fuel, automaticReturnFuel(&u.Game.Rules, p, donor, donor.Position, expeditionStations(p, donor), false))
	}
}

func TestNetworkForecastDoesNotSpendAndFailedCommitIsAtomic(t *testing.T) {
	u, customer, donors, goal := networkFixture(t)
	p := u.Player(1)
	fuel := []int{donors[0].Fuel, donors[1].Fuel, donors[2].Fuel}
	route := planFuelNetworkRoute(&u.Game.Rules, p, []*Fleet{customer}, donors, goal.Position, goal.MapObject.ToTarget(), automaticRoutePolicy{})
	require.NotNil(t, route)
	for i, donor := range donors {
		require.Equal(t, fuel[i], donor.Fuel)
		require.Nil(t, readFuelNetworkHold(donor))
	}
	donors[2].Fuel = 0
	u.Run(func(tg *turnGenerator) {
		require.False(t, tg.commitFuelNetwork(p, []*Fleet{customer}, automaticFuelRequest{Destination: goal.Position}, route))
	})
	require.False(t, fuelNetworkManaged(customer))
	for _, donor := range donors {
		require.Nil(t, readFuelNetworkHold(donor), "failed reservation cannot leave partial holds")
	}
}

func TestStrandedFleetCanRequestHelpWithMoreThanQuarterTank(t *testing.T) {
	u, customer, _, _ := networkFixture(t)
	customer.Fuel = customer.Spec.FuelCapacity / 2
	requestAutomaticRescue(u.Player(1), customer)
	require.Nil(t, readAutomaticFuelRequest(customer))
	requestAutomaticRescue(u.Player(1), customer, true)
	require.NotNil(t, readAutomaticFuelRequest(customer))
	require.True(t, readAutomaticFuelRequest(customer).Urgent)
	delete(customer.Tags, fuelRequestTag)
	customer.Fuel = customer.Spec.FuelCapacity
	requestAutomaticRescue(u.Player(1), customer, true)
	require.Nil(t, readAutomaticFuelRequest(customer), "full tanks need route planning, not a local fuel delivery")
}

func TestNetworkOneWayColonizationNeedsNoDestinationDock(t *testing.T) {
	u, customer, _, goal := networkFixture(t)
	p := u.Player(1)
	p.PlanetIntels = nil
	// Tankers remain close enough to their own service; remove only the
	// destination service from the customer's allowed mission endpoint.
	goal.Spec.DockCapacity = 0
	goal.Spec.HasStarbase = false
	u.Run((*turnGenerator).refreshAutomaticFuelIntel)
	goal.PlayerNum = 0
	goal.Hab = Hab{50, 50, 50}
	p.PlanetIntels = append(p.PlanetIntels, goal)
	p.Automation.FleetRoles[customer.Num] = "colonizer:best"
	u.Run(func(tg *turnGenerator) {
		chosen, route := tg.assistedColonyTarget(p, customer, "best", map[int]int{}, true)
		require.NotNil(t, chosen)
		require.Equal(t, goal.Num, chosen.Num)
		require.NotNil(t, route)
		require.NotNil(t, route.NetworkRequest)
		require.True(t, route.NetworkRequest.OneWay)
	})
}

func TestNetworkFuelCannotBePromisedToTwoFleets(t *testing.T) {
	u, first, donors, goal := networkFixture(t)
	p := u.Player(1)
	route := planFuelNetworkRoute(&u.Game.Rules, p, []*Fleet{first}, donors, goal.Position, goal.MapObject.ToTarget(), automaticRoutePolicy{})
	require.NotNil(t, route)
	second := *first
	second.Num = 20
	second.Tags = nil
	u.Game.Fleets = append(u.Game.Fleets, &second)
	p.Automation.FleetRoles[second.Num] = "freighter"
	u.Run(func(tg *turnGenerator) {
		require.True(t, tg.commitFuelNetwork(p, []*Fleet{first}, automaticFuelRequest{Destination: goal.Position}, route))
		require.False(t, tg.commitFuelNetwork(p, []*Fleet{&second}, automaticFuelRequest{Destination: goal.Position}, route))
	})
	require.False(t, fuelNetworkManaged(&second))
}

func TestLostNetworkTankerStopsTheJourneyWithoutClearingTheRole(t *testing.T) {
	u, customer, donors, goal := networkFixture(t)
	p := u.Player(1)
	route := planFuelNetworkRoute(&u.Game.Rules, p, []*Fleet{customer}, donors, goal.Position, goal.MapObject.ToTarget(), automaticRoutePolicy{})
	require.NotNil(t, route)
	u.Run(func(tg *turnGenerator) {
		require.True(t, tg.commitFuelNetwork(p, []*Fleet{customer}, automaticFuelRequest{Destination: goal.Position}, route))
	})
	p.Automation.FleetRoles[donors[1].Num] = ""
	u.Run((*turnGenerator).advanceFuelNetworks)
	require.False(t, fuelNetworkManaged(customer))
	require.Equal(t, "freighter", p.Automation.FleetRoles[customer.Num])
	require.Len(t, customer.Waypoints, 1)
	require.NotNil(t, readAutomaticFuelRequest(customer))
	for _, donor := range donors {
		require.Nil(t, readFuelNetworkHold(donor))
	}
}

func TestNetworkKeepsLaterArrivalWithMoreFuel(t *testing.T) {
	u, customer, donors, goal := networkFixture(t)
	p := u.Player(1)
	goal.Position = Vector{100, 0}
	customer.Spec.FuelCapacity = customer.GetFuelCost(p, 5, 60)
	customer.Fuel = customer.Spec.FuelCapacity
	dock := *u.Planet("Home")
	dock.Num = 4
	dock.Position = Vector{40, 10}
	u.Game.Planets = append(u.Game.Planets, &dock)
	donors[0].Position = Vector{50, 0}
	u.Run((*turnGenerator).refreshAutomaticFuelIntel)
	// Refresh the fixture's actual goal as well as its scanned position.
	u.Game.getPlanet(goal.Num).Position = goal.Position
	u.Run((*turnGenerator).refreshAutomaticFuelIntel)
	donor := donors[0]
	donor.Fuel = automaticReturnFuel(&u.Game.Rules, p, donor, donor.Position, expeditionStations(p, donor), false) + customer.Spec.FuelCapacity/3
	route := planFuelNetworkRoute(&u.Game.Rules, p, []*Fleet{customer}, []*Fleet{donor}, goal.Position, goal.MapObject.ToTarget(), automaticRoutePolicy{})
	require.NotNil(t, route)
	require.Equal(t, 3, route.Turns)
	require.Equal(t, dock.Position, route.Legs[0].To)
	require.Equal(t, donor.Position, route.Legs[1].To)
}

func TestNetworkUrgentRescueReturnsToADockAndPreservesTheMission(t *testing.T) {
	u, customer, donors, goal := networkFixture(t)
	customer.Position = Vector{60, 0}
	customer.Fuel = 0
	customer.Waypoints = []Waypoint{{Position: customer.Position}, {Position: Vector{200, 0}, WarpSpeed: 5}}
	saveAutomaticFuelRequest(customer, automaticFuelRequest{Destination: customer.Position, Members: []int{customer.Num}, Urgent: true})
	u.Run((*turnGenerator).planFuelNetworks)
	require.True(t, fuelNetworkManaged(customer))
	for range 5 {
		u.Run((*turnGenerator).advanceFuelNetworks, (*turnGenerator).enforceAutomaticDeparture, (*turnGenerator).fleetMove, (*turnGenerator).fleetRefuel, (*turnGenerator).advanceFuelNetworks)
		u.Game.Year++
		if !fuelNetworkManaged(customer) {
			break
		}
	}
	require.Equal(t, goal.Position, customer.Position)
	require.Len(t, customer.Waypoints, 2)
	require.Equal(t, Vector{200, 0}, customer.Waypoints[1].Position)
	for _, donor := range donors {
		require.Nil(t, readFuelNetworkHold(donor))
	}
}

func TestNetworkConvoyWaitsForActualRefillOfAnEmptyMember(t *testing.T) {
	u, customer, donors, goal := networkFixture(t)
	p := u.Player(1)
	second := *customer
	second.Num = 20
	second.Tags = nil
	second.Fuel = 0
	u.Game.Fleets = append(u.Game.Fleets, &second)
	p.Automation.FleetRoles[second.Num] = "escort"
	route := planFuelNetworkRoute(&u.Game.Rules, p, []*Fleet{customer, &second}, donors, goal.Position, goal.MapObject.ToTarget(), automaticRoutePolicy{})
	require.NotNil(t, route)
	require.True(t, route.Legs[0].DockWait)
	u.Run(func(tg *turnGenerator) {
		require.True(t, tg.commitFuelNetwork(p, []*Fleet{customer, &second}, automaticFuelRequest{Destination: goal.Position}, route))
	})
	u.Run((*turnGenerator).advanceFuelNetworks)
	require.Len(t, customer.Waypoints, 1)
	require.Zero(t, second.Fuel)
	u.Run((*turnGenerator).fleetRefuel)
	u.Game.Year++
	u.Run((*turnGenerator).advanceFuelNetworks)
	require.Len(t, customer.Waypoints, 2)
	require.Len(t, second.Waypoints, 2)
	require.Equal(t, customer.Waypoints[1].Position, second.Waypoints[1].Position)
}

func TestNetworkCancelsWhenAFutureDockIsLost(t *testing.T) {
	u, customer, donors, goal := networkFixture(t)
	p := u.Player(1)
	route := planFuelNetworkRoute(&u.Game.Rules, p, []*Fleet{customer}, donors, goal.Position, goal.MapObject.ToTarget(), automaticRoutePolicy{})
	require.NotNil(t, route)
	u.Run(func(tg *turnGenerator) {
		require.True(t, tg.commitFuelNetwork(p, []*Fleet{customer}, automaticFuelRequest{Destination: goal.Position}, route))
	})
	goal.Spec.HasStarbase = false
	goal.Spec.DockCapacity = 0
	u.Run((*turnGenerator).refreshAutomaticFuelIntel, (*turnGenerator).advanceFuelNetworks)
	require.False(t, fuelNetworkManaged(customer))
	require.NotNil(t, readAutomaticFuelRequest(customer))
}

func TestNetworkCannotBeDuplicatedThroughSplitOrMerge(t *testing.T) {
	u, customer, donors, goal := networkFixture(t)
	p := u.Player(1)
	route := planFuelNetworkRoute(&u.Game.Rules, p, []*Fleet{customer}, donors, goal.Position, goal.MapObject.ToTarget(), automaticRoutePolicy{})
	u.Run(func(tg *turnGenerator) {
		require.True(t, tg.commitFuelNetwork(p, []*Fleet{customer}, automaticFuelRequest{Destination: goal.Position}, route))
	})
	_, err := NewOrderer().SplitAll(&u.Game.Rules, p, u.Game.getFleets(p.Num), customer)
	require.ErrorContains(t, err, "reserved fuel")
	_, err = NewOrderer().Merge(&u.Game.Rules, p, []*Fleet{customer, donors[0]})
	require.ErrorContains(t, err, "reserved fuel")
}

func TestNetworkSurvivesRealTurnPhasesAcrossThreeTankers(t *testing.T) {
	tender := ShipDesign{Name: "Tender", Hull: SuperFuelXport.Name, Slots: []ShipDesignSlot{{HullComponent: LongHump6.Name, HullSlotIndex: 1, Quantity: 2}}}
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Designs: Designs(DesignTeamster, tender, DesignSpaceStation), Fleets: []ScenarioFleet{
		{Name: "Customer", Design: DesignTeamster.Name, At: "Home"},
		{Name: "Tender 1", Design: tender.Name, Position: Vector{20, 0}},
		{Name: "Tender 2", Design: tender.Name, Position: Vector{40, 0}},
		{Name: "Tender 3", Design: tender.Name, Position: Vector{60, 0}},
	}}}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name, Cargo: Cargo{Colonists: 2500}}, {Name: "Dock", Owner: 1, Position: Vector{80, 0}, Starbase: DesignSpaceStation.Name, Cargo: Cargo{Colonists: 2500}}}})
	p, customer, goal := u.Player(1), u.Fleet("Customer"), u.Planet("Dock")
	// A per-game test ruleset, never a mutation of the shared production techs.
	store := StaticTechStore
	store.HullComponents = slices.Clone(store.HullComponents)
	store.Hulls = slices.Clone(store.Hulls)
	for i := range store.HullComponents {
		if store.HullComponents[i].Name == LongHump6.Name {
			store.HullComponents[i].Engine.FuelUsage = []int{0, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100}
			store.HullComponents[i].Engine.FreeSpeed = 0
		}
	}
	for i := range store.Hulls {
		if store.Hulls[i].Name == MediumFreighter.Name {
			store.Hulls[i].CargoCapacity = 0
			store.Hulls[i].FuelCapacity = 19
		}
	}
	store.Init()
	u.Game.Rules.SetTechStore(&store)
	u.Recompute()
	customer.Fuel = customer.Spec.FuelCapacity
	donors := []*Fleet{u.Fleet("Tender 1"), u.Fleet("Tender 2"), u.Fleet("Tender 3")}
	p.Automation.FleetRoles = map[int]string{customer.Num: "freighter"}
	for _, donor := range donors {
		p.Automation.FleetRoles[donor.Num] = "tanker"
	}
	u.Run((*turnGenerator).refreshAutomaticFuelIntel)
	customer.Waypoints = []Waypoint{{Position: customer.Position}, {Position: goal.Position, MapObjectTarget: goal.MapObject.ToTarget(), WarpSpeed: 5}}
	route := planFuelNetworkRoute(&u.Game.Rules, p, []*Fleet{customer}, donors, goal.Position, goal.MapObject.ToTarget(), automaticRoutePolicy{})
	require.NotNil(t, route)
	require.Equal(t, 4, route.Turns)
	u.Run(func(tg *turnGenerator) {
		require.True(t, tg.commitFuelNetwork(p, []*Fleet{customer}, automaticFuelRequest{Destination: goal.Position}, route))
	})
	for range 4 {
		require.NoError(t, u.turn.generateTurn())
		require.GreaterOrEqual(t, customer.Fuel, 0)
	}
	require.Equal(t, goal.Position, customer.Position)
	require.False(t, fuelNetworkManaged(customer))
	require.Equal(t, "freighter", p.Automation.FleetRoles[customer.Num])
	for _, donor := range donors {
		require.Nil(t, readFuelNetworkHold(donor))
	}
}
