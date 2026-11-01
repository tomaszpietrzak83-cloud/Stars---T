//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"strconv"
	"testing"
)

func TestPatrolYearlyChoiceUsesHeadOnEndpoint(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Escort")
	p.Automation.FleetRoles[f.Num] = "patrol"
	addExpeditionTestDock(p, Vector{})
	far := patrolTestEnemy(0)
	far.Num = 80
	far.Position = Vector{100, 0}
	passing := patrolTestEnemy(0)
	passing.Num = 81
	passing.Position = Vector{85, 0}
	passing.WarpSpeed = 9
	passing.Heading = VectorFloat64{X: -1}
	p.FleetIntels = []*Fleet{far, passing}
	savePatrolJob(f, patrolJob{Leader: f.Num, Enemy: 2, Target: far.Num})
	u.Run(func(tg *turnGenerator) { tg.retargetPatrolOpportunity(p, f); tg.advancePatrol(p, f) })
	require.Equal(t, passing.Num, readPatrolJob(f).Target)
	require.Equal(t, Vector{4, 0}, f.Waypoints[1].Position)
	// The same target fleeing away from us is not a nearby interception.
	passing.Heading = VectorFloat64{X: 1}
	savePatrolJob(f, patrolJob{Leader: f.Num, Enemy: 2, Target: far.Num})
	u.Run(func(tg *turnGenerator) { tg.retargetPatrolOpportunity(p, f) })
	require.Equal(t, far.Num, readPatrolJob(f).Target)
}

func TestPatrolSectorStaysAtBusyBirthBaseAndSurplusCoversOtherBase(t *testing.T) {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Designs: Designs(DesignDestroyerDelta, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Local", Design: DesignDestroyerDelta.Name, At: "Home"}, {Name: "Spare", Design: DesignDestroyerDelta.Name, At: "Home"}}}}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}, {Name: "Border", Owner: 1, Starbase: DesignSpaceStation.Name, Position: Vector{100, 0}}}})
	p, a, b := u.Player(1), u.Fleet("Local"), u.Fleet("Spare")
	p.Automation.FleetRoles = map[int]string{a.Num: "patrol", b.Num: "patrol"}
	p.PlanetIntels = []*Planet{u.Planet("Home"), u.Planet("Border")}
	p.FleetIntels = []*Fleet{patrolTestEnemy(0)}
	u.Run(func(tg *turnGenerator) { require.True(t, tg.patrolFrontier(p, a)) })
	require.Equal(t, u.Planet("Home").Num, mustSector(a))
	p.FleetIntels = nil
	u.Run(func(tg *turnGenerator) { require.True(t, tg.patrolFrontier(p, b)) })
	require.Equal(t, u.Planet("Border").Num, mustSector(b))
	require.NotEqual(t, a.Waypoints[len(a.Waypoints)-1].Position, b.Waypoints[len(b.Waypoints)-1].Position)
}
func mustSector(f *Fleet) int { n, _ := strconv.Atoi(f.GetTag(patrolSectorTag)); return n }

func TestMovingBomberGuardHasNarrowObservedCorridor(t *testing.T) {
	u := logisticsUniverse(t)
	p := u.Player(1)
	p.PlanetIntels = []*Planet{u.Planet("Home")}
	f := patrolTestEnemy(100)
	f.Position = Vector{60, 0}
	f.Spec.Bomber = true
	f.WarpSpeed = 6
	f.Heading = VectorFloat64{X: -1}
	p.FleetIntels = []*Fleet{f}
	require.True(t, observedBombardmentGuard(p, f))
	require.Zero(t, knownRouteThreat(p, Vector{30, 20}, Vector{60, 20}))
	require.Positive(t, knownRouteThreat(p, Vector{30, -10}, Vector{30, 10}))
	f.Spec.Bomber = false
	require.Positive(t, knownRouteThreat(p, Vector{30, 20}, Vector{60, 20}))
}

func TestAutomaticGateCompatibilityRejectsHeavyHullsAndCargo(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Escort")
	gate := u.Planet("Home")
	gate.Spec.HasStargate = true
	gate.Spec.SafeRange = 250
	gate.Spec.SafeHullMass = f.Tokens[0].design.Spec.Mass - 1
	require.False(t, automaticGateCompatible(p, []*Fleet{f}, gate))
	gate.Spec.SafeHullMass += 1
	require.True(t, automaticGateCompatible(p, []*Fleet{f}, gate))
	f.Cargo.Ironium = 1
	p.Race.Spec.CanGateCargo = false
	require.False(t, automaticGateCompatible(p, []*Fleet{f}, gate))
	p.Race.Spec.CanGateCargo = true
	require.True(t, automaticGateCompatible(p, []*Fleet{f}, gate))
}

func traderTestUniverse(t *testing.T) (*testUniverse, *MysteryTrader) {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Designs: Designs(DesignTeamster, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Offer A", At: "Home", Tokens: []ScenarioShipToken{{Design: DesignTeamster.Name, Quantity: 20}}}, {Name: "Offer B", At: "Home", Tokens: []ScenarioShipToken{{Design: DesignTeamster.Name, Quantity: 20}}}}}}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name, Cargo: Cargo{Ironium: 10000, Colonists: 2500}}}})
	p := u.Player(1)
	p.Automation.FleetRoles = map[int]string{}
	for _, f := range u.Game.Fleets {
		if !f.Starbase {
			p.Automation.FleetRoles[f.Num] = "freighter"
		}
	}
	p.PlanetIntels = []*Planet{u.Planet("Home")}
	mt := &MysteryTrader{MapObject: MapObject{Type: MapObjectTypeMysteryTrader, Num: 1, Position: Vector{5, 0}}, RequestedBoon: 5000, WarpSpeed: 1, Destination: Vector{-100, 0}, Heading: VectorFloat64{X: -1}}
	p.MysteryTraderIntels = []*MysteryTrader{mt}
	p.Automation.PriorityTraders = map[int]bool{1: true}
	return u, mt
}

func TestTraderConvoyReservesLoadsMergesAndTargetsNativeMeeting(t *testing.T) {
	u, mt := traderTestUniverse(t)
	p := u.Player(1)
	before := u.Planet("Home").Cargo.Ironium
	u.Run((*turnGenerator).planTraderAutomation, (*turnGenerator).planLogisticsAutomation, (*turnGenerator).guardAutomaticFuel, (*turnGenerator).enforceAutomaticDeparture)
	var offering *Fleet
	total := 0
	for _, f := range u.Game.Fleets {
		if !f.Starbase && !f.Delete {
			total += f.Cargo.Ironium
			if readTraderConvoy(f) != nil {
				offering = f
			}
		}
	}
	require.Equal(t, 5000, total)
	require.Equal(t, before-5000, u.Planet("Home").Cargo.Ironium)
	require.NotNil(t, offering)
	require.True(t, readTraderConvoy(offering).Offering)
	require.Equal(t, 5000, offering.Cargo.Ironium)
	require.True(t, offering.targetingMysteryTrader(mt))
	require.Greater(t, len(offering.Waypoints), 1)
	// Once dispatched, do not send the offering back to the rally every year.
	offering.Position = Vector{2, 0}
	u.Run((*turnGenerator).planTraderAutomation)
	require.True(t, readTraderConvoy(offering).Offering)
	require.NotEqual(t, u.Planet("Home").Position, offering.Waypoints[len(offering.Waypoints)-1].Position)
	delete(p.Automation.PriorityTraders, 1)
	u.Run((*turnGenerator).planTraderAutomation)
	require.Empty(t, offering.GetTag(traderConvoyTag))
	require.Equal(t, "freighter", p.Automation.FleetRoles[offering.Num])
}

func TestRadiatingEscortTravelsSeparatelyWithoutPassengerDieoff(t *testing.T) {
	u := logisticsUniverse(t)
	p, f, e := u.Player(1), u.Fleet("Transport"), u.Fleet("Escort")
	p.Automation.FleetRoles[e.Num] = "escort"
	p.Race.Spec.HabCenter.Rad = 50
	f.Cargo.Colonists = 100
	e.Spec.Radiating = true
	job := &logisticsJob{Kind: "settle", Position: Vector{20, 0}}
	u.Run(func(tg *turnGenerator) { tg.prepareConvoyWithMinimum(p, f, job, MapObjectTarget{}, 1) }, (*turnGenerator).fleetMove, (*turnGenerator).fleetRadiatingEngineDieoff)
	require.False(t, f.Delete)
	require.False(t, e.Delete)
	require.Equal(t, 100, f.Cargo.Colonists)
	require.Equal(t, f.Position, e.Position)
	require.NotEqual(t, Vector{}, f.Position)
	require.False(t, f.Spec.Radiating)
}

func TestIncomingRebuildReservesMineralsBeforeScrapping(t *testing.T) {
	u := maintenanceUniverse(t)
	requestRebuild(u)
	p, f := u.Player(1), u.Fleet("Fleet")
	yard := u.Planet("Yard")
	target := p.Designs[1]
	target.Spec.Cost.Ironium = 1000
	f.Position = Vector{10, 0}
	u.Run((*turnGenerator).fleetDesignMaintenance)
	require.Equal(t, yard.Num, readMaintenance(f).PlanetNum)
	require.Greater(t, u.turn.incomingRebuildMinerals(p, yard).Ironium, 0)
	need := u.turn.productionMinerals(p, yard, true)
	require.GreaterOrEqual(t, need.Ironium, u.turn.incomingRebuildMinerals(p, yard).Ironium)
	reserved := readMaintenance(f).PlanetNum
	u.Run((*turnGenerator).fleetDesignMaintenance)
	require.Equal(t, reserved, readMaintenance(f).PlanetNum)
}

func TestTraderNativeMeetingAbsorbsOnlyTheCompletedOffering(t *testing.T) {
	u, mt := traderTestUniverse(t)
	mt.PlayersRewarded = map[int]bool{}
	mt.RewardType = MysteryTraderRewardResearch
	u.Game.addMysteryTrader(mt)
	u.Game.Rules.MysteryTraderRules.MaxWarp = mt.WarpSpeed
	u.Run((*turnGenerator).planTraderAutomation, (*turnGenerator).guardAutomaticFuel, (*turnGenerator).enforceAutomaticDeparture, (*turnGenerator).mysteryTraderMove, (*turnGenerator).fleetMove)
	var offering *Fleet
	for _, f := range u.Game.Fleets {
		if !f.Delete && readTraderConvoy(f) != nil {
			offering = f
		}
	}
	require.NotNil(t, offering)
	require.Equal(t, mt.Position, offering.Position)
	u.Run(func(tg *turnGenerator) { require.NoError(t, tg.mysteryTraderMeet()) })
	require.True(t, mt.PlayersRewarded[1])
	require.True(t, offering.Delete)
}

func TestTraderInsufficientCapacityDoesNotReserveCargoOrBorrowManualShips(t *testing.T) {
	u, mt := traderTestUniverse(t)
	p := u.Player(1)
	p.Automation.FleetRoles = map[int]string{u.Fleet("Offer A").Num: "freighter"}
	before := u.Planet("Home").Cargo
	u.Run((*turnGenerator).planTraderAutomation)
	require.Equal(t, before, u.Planet("Home").Cargo)
	for _, f := range u.Game.Fleets {
		require.Empty(t, f.GetTag(traderConvoyTag))
	}
	require.True(t, p.Automation.PriorityTraders[mt.Num])
}

func TestPatrolMergesEnoughLocalShipsForThreeUnarmedHulls(t *testing.T) {
	u := logisticsUniverse(t)
	p, a, b := u.Player(1), u.Fleet("Escort"), u.Fleet("Combat")
	b.Position = a.Position
	p.Automation.FleetRoles = map[int]string{a.Num: "patrol", b.Num: "patrol"}
	enemy := patrolTestEnemy(0)
	enemy.Spec.TotalShips = 3
	p.FleetIntels = []*Fleet{enemy}
	addExpeditionTestDock(p, Vector{})
	u.Run((*turnGenerator).planPatrolAutomation)
	var leader *Fleet
	for _, f := range u.Game.Fleets {
		if !f.Delete && readPatrolJob(f).Leader == f.Num {
			leader = f
		}
	}
	require.NotNil(t, leader)
	require.Equal(t, 3, leader.Spec.TotalShips)
	require.True(t, patrolCanAttack(p, leader))
}

func TestPassengerTripBudgetsAnEmptyReturnAfterDeliveringAFullLoad(t *testing.T) {
	u := passengerUniverse(t)
	p, f, source, dest := u.Player(1), u.Fleet("Transport"), u.Planet("Home"), u.Planet("Young colony")
	p.PlanetIntels = []*Planet{source, dest}
	source.Spec.HasStarbase = true
	source.Spec.DockCapacity = -1
	dest.Cargo.Colonists = 1
	dest.Spec.MaxPopulation = 1000000
	for _, token := range f.Tokens {
		token.design.Spec.Engine.FuelUsage = []int{0, 100, 100, 100, 100, 100, 150, 300, 500, 1000, 1500}
	}
	found := false
	for distance := 20; distance <= 300; distance += 5 {
		dest.Position = Vector{distance, 0}
		job := &logisticsJob{Kind: "passengers", TargetNum: source.Num, DeliveryNum: dest.Num, Position: source.Position, Passengers: f.Spec.CargoCapacity}
		budget := *f
		budget.Tags = Tags{}
		budget.Cargo.Colonists = f.Spec.CargoCapacity
		budget.Fuel = f.Spec.FuelCapacity
		saveLogisticsWork(&budget, &logisticsJob{Kind: "passengers"})
		old := planAutomaticRoute(&u.Game.Rules, p, []*Fleet{&budget}, dest.Position, dest.ToTarget(), automaticRoutePolicy{MinWarp: automaticCruiseWarp(p, f), MaxTurns: 20})
		if old == nil && u.turn.passengerTripSafe(p, f, job) {
			found = true
			break
		}
	}
	require.True(t, found, "a full-load round trip rejected by the legacy full-hold return budget must be available after unloading")
}

func TestPriorityPlanetMovesIdleEscortCoverageToItsNearestBase(t *testing.T) {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Designs: Designs(DesignDestroyerDelta, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Cover", Design: DesignDestroyerDelta.Name, At: "Home"}}}}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}, {Name: "Border", Owner: 1, Position: Vector{100, 0}, Starbase: DesignSpaceStation.Name}}})
	p, f := u.Player(1), u.Fleet("Cover")
	p.PlanetIntels = []*Planet{u.Planet("Home"), u.Planet("Border")}
	p.Automation.FleetRoles = map[int]string{f.Num: "escort"}
	p.Automation.PriorityPlanets = map[int]bool{u.Planet("Border").Num: true}
	u.Run((*turnGenerator).planEscortStaging)
	require.Equal(t, u.Planet("Border").Num, mustSector(f))
	require.Equal(t, u.Planet("Border").Position, f.Waypoints[len(f.Waypoints)-1].Position)
}

func TestAutomaticGateEndpointKeepsItsPlanetTargetThroughDeparture(t *testing.T) {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Designs: Designs(DesignDestroyerDelta, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Hunter", Design: DesignDestroyerDelta.Name, At: "Home"}}}}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}, {Name: "Far", Owner: 1, Position: Vector{200, 0}, Starbase: DesignSpaceStation.Name}}})
	p, f := u.Player(1), u.Fleet("Hunter")
	p.PlanetIntels = []*Planet{u.Planet("Home"), u.Planet("Far")}
	p.Automation.FleetRoles = map[int]string{f.Num: "patrol"}
	for _, planet := range p.PlanetIntels {
		planet.Spec.HasStargate = true
		planet.Spec.SafeRange = 500
		planet.Spec.SafeHullMass = 1000
	}
	route := planAutomaticRoute(&u.Game.Rules, p, []*Fleet{f}, u.Planet("Far").Position, MapObjectTarget{}, automaticRoutePolicy{Fast: true, AllowThreat: true, MaxTurns: 1})
	require.NotNil(t, route)
	require.Equal(t, StargateWarpSpeed, route.Waypoints[1].WarpSpeed)
	require.Equal(t, u.Planet("Far").Num, route.Waypoints[1].TargetNum)
	applyExpeditionRoute(p, f, route)
	u.Run((*turnGenerator).enforceAutomaticDeparture)
	require.Greater(t, len(f.Waypoints), 1)
	require.Equal(t, StargateWarpSpeed, f.Waypoints[1].WarpSpeed)
}

func TestHuntersAlreadyAssignedElsewhereMergeForNearbyThreeShipTarget(t *testing.T) {
	u := logisticsUniverse(t)
	p, a, b := u.Player(1), u.Fleet("Escort"), u.Fleet("Combat")
	b.Position = a.Position
	p.Automation.FleetRoles = map[int]string{a.Num: "patrol", b.Num: "patrol"}
	far := patrolTestEnemy(0)
	far.Num = 80
	far.Position = Vector{140, 0}
	nearby := patrolTestEnemy(0)
	nearby.Num = 81
	nearby.Position = Vector{10, 0}
	nearby.Spec.TotalShips = 3
	p.FleetIntels = []*Fleet{far, nearby}
	addExpeditionTestDock(p, Vector{})
	savePatrolJob(a, patrolJob{Leader: a.Num, Enemy: 2, Target: far.Num})
	savePatrolJob(b, patrolJob{Leader: b.Num, Enemy: 2, Target: nearby.Num})
	u.Run(func(tg *turnGenerator) { tg.retargetPatrolOpportunity(p, a) })
	require.True(t, a.Delete)
	require.Equal(t, 3, b.Spec.TotalShips)
	require.Equal(t, nearby.Num, readPatrolJob(b).Target)
}

func TestWaitingPassengerTransportSwitchesToAFullSource(t *testing.T) {
	u := passengerUniverse(t)
	p, f, home, dest := u.Player(1), u.Fleet("Transport"), u.Planet("Home"), u.Planet("Young colony")
	p.Automation.ManualPassengerThresholds = false
	home.Cargo.Colonists = sourcePopulationReserve(p, home) + 1
	source := *home
	source.Num = 3
	source.Name = "Full source"
	source.Position = Vector{10, 0}
	source.Cargo.Colonists = sourcePopulationReserve(p, &source) + f.Spec.CargoCapacity*2
	u.Game.Planets = append(u.Game.Planets, &source)
	p.PlanetIntels = []*Planet{home, dest, &source}
	saveLogisticsWork(f, &logisticsJob{Kind: "passengers", TargetNum: home.Num, DeliveryNum: dest.Num, Position: home.Position, Passengers: 1, Status: "Waiting empty at source for a full safe load"})
	u.Run((*turnGenerator).planLogisticsAutomation)
	require.NotNil(t, logisticsWork(f))
	require.Equal(t, source.Num, logisticsWork(f).TargetNum)
	require.Greater(t, len(f.Waypoints), 1)
	require.Zero(t, f.CargoMass())
}

func TestIncomingRebuildAtSecondYardReceivesRealMineralShipment(t *testing.T) {
	scenario := maintenanceScenario()
	scenario.Players[0].Designs = append(scenario.Players[0].Designs, DesignTeamster)
	scenario.Players[0].Fleets = append(scenario.Players[0].Fleets, ScenarioFleet{Name: "Supplier", Design: DesignTeamster.Name, At: "Yard"})
	second := scenario.Planets[0]
	second.Name = "Second yard"
	second.Position = Vector{10, 0}
	scenario.Planets = append(scenario.Planets, second)
	u := newTestUniverse(t, scenario)
	p, f := u.Player(1), u.Fleet("Supplier")
	yard := u.Planet("Second yard")
	p.DesignMaintenance = DesignMaintenance{p.Designs[0].Num: p.Designs[1].Num}
	p.Designs[1].Spec.Cost.Ironium = 2000
	writeMaintenance(u.Fleet("Fleet"), &fleetMaintenance{Group: "second-yard", Stage: "scrap", PlanetNum: yard.Num})
	yard.Cargo.Ironium = 0
	u.Planet("Yard").Cargo.Ironium = 10000
	p.Automation.FleetRoles = map[int]string{f.Num: "freighter"}
	p.Automation.DisablePassengerTransport = true
	p.Automation.DisableSalvage = true
	p.PlanetIntels = []*Planet{u.Planet("Yard"), yard}
	before := u.Planet("Yard").Cargo.Ironium
	u.Run((*turnGenerator).planLogisticsAutomation, (*turnGenerator).resolveLogisticsAutomation, (*turnGenerator).dispatchReadyLogistics)
	require.NotNil(t, logisticsWork(f))
	require.Equal(t, yard.Num, logisticsWork(f).DeliveryNum)
	require.Equal(t, f.Spec.CargoCapacity, f.Cargo.Ironium)
	require.Equal(t, before-f.Cargo.Ironium, u.Planet("Yard").Cargo.Ironium)
	require.Equal(t, yard.Position, f.Waypoints[1].Position)
}
