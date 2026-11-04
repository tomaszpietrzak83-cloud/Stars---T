//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestLoadedDeliveryKeepsReceiverDespiteOtherReservations(t *testing.T) {
	u := passengerUniverse(t)
	p, f := u.Player(1), u.Fleet("Transport")
	dest := u.Planet("Young colony")
	p.Automation.ManualPassengerThresholds = false
	dest.Cargo.Colonists = 1950
	f.Cargo.Colonists = 70
	other := *f
	other.Num = 43
	other.Tags = nil
	other.Cargo.Colonists = 70
	u.Game.Fleets = append(u.Game.Fleets, &other)
	p.Automation.FleetRoles[other.Num] = "freighter"
	saveLogisticsWork(&other, &logisticsJob{Kind: "settle", DeliveryNum: dest.Num, TargetNum: dest.Num, Position: dest.Position})
	saveLogisticsWork(f, &logisticsJob{Kind: "settle", DeliveryNum: dest.Num, TargetNum: dest.Num, Position: dest.Position})
	u.Run(func(tg *turnGenerator) { require.Same(t, dest, tg.passengerDestination(p, f, dest.Num)) })
}

func TestPassengerPickupPrioritizesFragileColony(t *testing.T) {
	u := passengerUniverse(t)
	p, f, colony := u.Player(1), u.Fleet("Transport"), u.Planet("Young colony")
	colony.Cargo.Colonists = 500
	better := *colony
	better.Num, better.Name, better.Position = 3, "Established", Vector{25, 0}
	better.Cargo.Colonists = 1500
	better.Hab = u.Planet("Home").Hab
	u.Game.Planets = append(u.Game.Planets, &better)
	u.Run((*turnGenerator).refreshAutomaticFuelIntel, func(tg *turnGenerator) {
		job := tg.findPassengerTransfer(p, f)
		require.NotNil(t, job)
		require.Equal(t, colony.Num, job.DeliveryNum)
	})
}

func TestLoadedDeliveryRepairsInstallationReceiverAndUnloads(t *testing.T) {
	u := passengerUniverse(t)
	p, f := u.Player(1), u.Fleet("Transport")
	dest := u.Planet("Young colony")
	f.Cargo.Colonists = 55
	p.Automation.InfrastructureTransport = true
	saveInfrastructure(f, infrastructureCargo{Factories: 3, Origin: 1, Destination: 1})
	saveLogisticsWork(f, &logisticsJob{Kind: "settle", DeliveryNum: dest.Num, TargetNum: dest.Num, Position: dest.Position})
	before := dest.Cargo.Colonists
	u.Run((*turnGenerator).refreshAutomaticFuelIntel, (*turnGenerator).planLogisticsAutomation)
	require.Equal(t, dest.Num, readInfrastructure(f).Destination)
	for range 4 {
		u.Run((*turnGenerator).fleetMove, (*turnGenerator).resolveLogisticsAutomation, (*turnGenerator).unloadOpportunisticInfrastructure)
		if f.Cargo.Colonists == 0 {
			break
		}
		u.Run((*turnGenerator).planLogisticsAutomation)
	}
	require.Equal(t, before+55, dest.Cargo.Colonists)
	require.Zero(t, f.Cargo.Colonists)
	require.Equal(t, 3, dest.Factories)
	require.False(t, f.carriesInfrastructure())
}

func TestFuelGuardUsesDeliveryTargetInsteadOfIntermediateDock(t *testing.T) {
	u := passengerUniverse(t)
	p, f := u.Player(1), u.Fleet("Transport")
	dest := u.Planet("Young colony")
	home := u.Planet("Home")
	f.Cargo.Colonists = 70
	f.Position = Vector{5, 0}
	f.Waypoints = []Waypoint{{Position: f.Position}, {Position: home.Position, MapObjectTarget: home.ToTarget(), WarpSpeed: 5}}
	saveLogisticsWork(f, &logisticsJob{Kind: "settle", DeliveryNum: dest.Num, TargetNum: dest.Num, Position: dest.Position})
	u.Run((*turnGenerator).refreshAutomaticFuelIntel, (*turnGenerator).guardAutomaticFuel)
	require.Greater(t, len(f.Waypoints), 1)
	last := f.Waypoints[len(f.Waypoints)-1]
	require.Equal(t, dest.Position, last.Position)
	require.Equal(t, dest.Num, last.TargetNum)
	require.Equal(t, 1, p.Num)
}

func TestOrphanedLoadedTransportDefersNewMaintenance(t *testing.T) {
	u := maintenanceUniverse(t)
	requestRebuild(u)
	p, f := u.Player(1), u.Fleet("Fleet")
	p.Automation.FleetRoles = map[int]string{f.Num: "freighter"}
	f.Cargo.Colonists = 10
	u.Run((*turnGenerator).fleetDesignMaintenance)
	require.False(t, f.InDesignMaintenance(), "passengers without a job still need delivery before refit")
	require.NotEmpty(t, f.GetTag("maintenance-pending"))
}

func TestInstallationExportUsesPopulationAfterLoading(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Transport")
	source := u.Planet("Home")
	source.Cargo.Colonists = 1000
	source.Spec.MaxPopulation = 1000000
	source.Factories = 95
	p.Race.NumFactories = 10
	p.Race.NumMines = 10
	p.Automation.InfrastructureTransport = true
	dest := *source
	dest.Num = 2
	dest.Position = Vector{20, 0}
	dest.Cargo.Colonists = 100
	dest.Factories = 0
	dest.Mines = 0
	u.Game.Planets = append(u.Game.Planets, &dest)
	f.Spec.CargoCapacity = 70
	u.Run(func(tg *turnGenerator) { tg.preparePassengerMixedCargo(p, f, source, &dest, 70) })
	packed := readInfrastructure(f)
	require.Positive(t, packed.Factories, "the 95-percent reserve must shrink when passengers leave")
	passengers := min(70, f.availableColonistSpace())
	capacity, _, _ := installationCapacity(p, source, (source.Cargo.Colonists-passengers)*100)
	require.GreaterOrEqual(t, source.Factories, (capacity*95+99)/100)
	require.Equal(t, 95, source.Factories+packed.Factories)
}

func TestLoadedMineralsReachAssignedColonyWithoutStarbase(t *testing.T) {
	u := passengerUniverse(t)
	f := u.Fleet("Transport")
	dest := u.Planet("Young colony")
	f.Cargo = Cargo{Germanium: 70}
	saveLogisticsWork(f, &logisticsJob{Kind: "return", TargetNum: dest.Num, DeliveryNum: dest.Num, Position: dest.Position})
	for range 4 {
		u.Run((*turnGenerator).refreshAutomaticFuelIntel, (*turnGenerator).planLogisticsAutomation, (*turnGenerator).fleetMove, (*turnGenerator).resolveLogisticsAutomation)
		if f.Cargo.Germanium == 0 {
			break
		}
	}
	require.Zero(t, f.Cargo.Germanium)
	require.Equal(t, 70, dest.Cargo.Germanium)
}

func TestLoadedDeliveryChangesReceiverOnlyAfterSustainedNoProgress(t *testing.T) {
	u := passengerUniverse(t)
	p, f, dest := u.Player(1), u.Fleet("Transport"), u.Planet("Young colony")
	f.Cargo.Colonists = 70
	other := *dest
	other.Num, other.Name, other.Position = 3, "Alternative", Vector{20, 0}
	u.Game.Planets = append(u.Game.Planets, &other)
	u.Run(func(tg *turnGenerator) {
		require.Same(t, dest, tg.passengerDestination(p, f, dest.Num))
		tg.game.Year += 5
		require.Same(t, dest, tg.passengerDestination(p, f, dest.Num))
		tg.game.Year++
		require.Same(t, &other, tg.passengerDestination(p, f, dest.Num))
	})
}

func TestMixedInstallationExportsRetainActualOperatingReserve(t *testing.T) {
	for _, capacity := range []int{70, 210, 400} {
		for _, stock := range []int{95, 100, 110} {
			u := logisticsUniverse(t)
			p, f, source := u.Player(1), u.Fleet("Transport"), u.Planet("Home")
			p.Race.NumFactories, p.Race.NumMines = 10, 10
			p.Automation.InfrastructureTransport = true
			source.Cargo.Colonists, source.Factories, source.Mines = 1000, stock, stock
			source.Spec.MaxPopulation = 1000000
			f.Spec.CargoCapacity = capacity
			dest := *source
			dest.Num, dest.Position = 2, Vector{20, 0}
			dest.Factories, dest.Mines = 0, 0
			u.Game.Planets = append(u.Game.Planets, &dest)
			u.Run(func(tg *turnGenerator) { tg.preparePassengerMixedCargo(p, f, source, &dest, capacity) })
			pax := min(capacity, f.availableColonistSpace())
			fc, mc, _ := installationCapacity(p, source, (1000-pax)*100)
			require.GreaterOrEqual(t, source.Factories, (fc*95+99)/100)
			require.GreaterOrEqual(t, source.Mines, (mc*95+99)/100)
			require.Equal(t, stock, source.Factories+readInfrastructure(f).Factories)
			require.Equal(t, stock, source.Mines+readInfrastructure(f).Mines)
			require.LessOrEqual(t, f.CargoMass()+pax, capacity)
		}
	}
}

func TestBaseMineralFloorProtectsSourcesAndCountsIncoming(t *testing.T) {
	u := passengerUniverse(t)
	p, f, source, dest := u.Player(1), u.Fleet("Transport"), u.Planet("Home"), u.Planet("Young colony")
	p.Automation.DisablePassengerTransport, p.Automation.DisableMineralSupply = true, false
	source.ProductionQueue = nil
	dest.ProductionQueue = nil
	source.Spec.MiningOutput, dest.Spec.MiningOutput = Mineral{}, Mineral{}
	dest.Spec.HasStarbase, dest.Spec.DockCapacity = true, -1
	u.Run(func(tg *turnGenerator) {
		require.Equal(t, Mineral{Ironium: 500, Boranium: 300, Germanium: 500}, tg.productionMinerals(p, dest, false))
		source.Cargo = Cargo{Ironium: 500, Boranium: 300, Germanium: 500}
		require.Nil(t, tg.findShipyardSupply(p, f, map[string]bool{}), "a donor's own reserve cannot be exported")
		source.Cargo.Ironium, source.Cargo.Germanium = 5000, 5000
		dest.Cargo = Cargo{Ironium: 450, Boranium: 300, Germanium: 450}
		require.NotNil(t, tg.findShipyardSupply(p, f, map[string]bool{}))
		second := *f
		second.Num, second.Tags = 88, nil
		second.Cargo = Cargo{Ironium: 50, Germanium: 50}
		u.Game.Fleets = append(u.Game.Fleets, &second)
		require.NoError(t, u.Game.addFleet(&second))
		saveLogisticsWork(&second, &logisticsJob{Kind: "return", TargetNum: dest.Num, DeliveryNum: dest.Num, Position: dest.Position})
		require.Nil(t, tg.findShipyardSupply(p, f, map[string]bool{}), "in-flight cargo already covers the floor")
	})
	dest.Spec.DockCapacity = 0
	require.Equal(t, Mineral{Ironium: 500, Germanium: 500}, automaticBaseMineralStock(dest), "a base without a shipyard still receives the requested I/G reserves")
}

func TestLoadedInvasionArrivesAndCapturesWithNativeGroundCombat(t *testing.T) {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Designs: Designs(DesignTeamster, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Landing", Design: DesignTeamster.Name, At: "Home", Cargo: Cargo{Colonists: 100}}}}, {Designs: Designs(DesignSpaceStation)}}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name, Cargo: Cargo{Colonists: 8000}}, {Name: "Target", Owner: 2, Position: Vector{80, 0}, Cargo: Cargo{Colonists: 10}}}})
	p, f, target := u.Player(1), u.Fleet("Landing"), u.Planet("Target")
	p.Automation.FleetRoles = map[int]string{f.Num: "freighter"}
	p.Automation.DisableSalvage = true
	p.Automation.DisableMineralSupply = true
	target.Spec.Habitability = 100
	target.Hab = u.Planet("Home").Hab
	target.BaseHab = target.Hab
	target.Spec.MaxPopulation = 1000000
	target.Spec.HasStarbase = false
	target.Defenses = 0
	p.PlanetIntels = []*Planet{u.Planet("Home"), target}
	saveLogisticsWork(f, &logisticsJob{Kind: "invade", TargetNum: target.Num, DeliveryNum: target.Num, Position: target.Position})
	// Recover a loaded invasion whose saved waypoint still points at the fuel
	// dock. Its final position and target identity must both describe the enemy.
	f.Position = Vector{5, 0}
	f.Waypoints = []Waypoint{{Position: f.Position}, {Position: u.Planet("Home").Position, MapObjectTarget: u.Planet("Home").ToTarget(), WarpSpeed: 5}}
	u.Run((*turnGenerator).refreshAutomaticFuelIntel, (*turnGenerator).guardAutomaticFuel)
	require.Greater(t, len(f.Waypoints), 1)
	require.Equal(t, target.Num, f.Waypoints[len(f.Waypoints)-1].TargetNum)
	for range 5 {
		u.Run((*turnGenerator).refreshAutomaticFuelIntel, (*turnGenerator).planLogisticsAutomation, (*turnGenerator).guardAutomaticFuel, (*turnGenerator).enforceAutomaticDeparture, (*turnGenerator).fleetMove, (*turnGenerator).resolveAutomaticInvasions, (*turnGenerator).resolveColonistDrops)
		if target.PlayerNum == p.Num {
			break
		}
	}
	require.Equal(t, p.Num, target.PlayerNum)
	require.Positive(t, target.Cargo.Colonists)
	require.Zero(t, f.Cargo.Colonists)
}
