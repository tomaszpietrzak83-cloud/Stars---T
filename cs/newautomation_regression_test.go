//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"strconv"
	"testing"
)

func TestColonizationIncludesRedPlanetMadeGreenByInstaforming(t *testing.T) {
	p := NewPlayer(1, NewRace().WithPRT(CA).WithSpec(&rules)).WithTechLevels(TechLevel{10, 10, 10, 10, 10, 10}).withSpec(&rules)
	var candidate *Planet
	for grav := 0; grav <= 100; grav++ {
		planet := &Planet{MapObject: MapObject{Num: 1}, Hab: Hab{grav, 50, 50}, BaseHab: Hab{grav, 50, 50}}
		if p.Race.GetPlanetHabitability(planet.Hab) < 0 && prospectivePlanetQuality(p, planet) > 0 {
			candidate = planet
			break
		}
	}
	require.NotNil(t, candidate)
	require.Greater(t, colonyPlanetQuality(p, candidate), 0)
	p.Race.Spec.Instaforming = false
	require.Zero(t, colonyPlanetQuality(p, candidate))
}

func TestPartialPatrolRescueKeepsWaitingForMoreFuel(t *testing.T) {
	u := logisticsUniverse(t)
	p, customer, donor := u.Player(1), u.Fleet("Transport"), u.Fleet("Escort")
	p.Automation.FleetRoles = map[int]string{customer.Num: "scout", donor.Num: "patrol"}
	customer.Position = Vector{60, 0}
	donor.Position = customer.Position
	customer.Fuel = 0
	u.Run((*turnGenerator).refreshAutomaticFuelIntel)
	reserve := automaticReturnFuel(&u.Game.Rules, p, donor, donor.Position, expeditionStations(p, donor), false)
	donor.Fuel = reserve + 20
	writeTankerJob(donor, &automaticTankerJob{TargetNum: customer.Num, Members: []int{customer.Num}, Position: customer.Position, DeliveryFuel: 20})
	customer.SetTag(tankerAwaitTag, strconv.Itoa(donor.Num))
	u.Run((*turnGenerator).resolveAutomaticTankers)
	require.Equal(t, 20, customer.Fuel)
	require.Equal(t, reserve, donor.Fuel)
	require.NotNil(t, readAutomaticFuelRequest(customer))
	require.True(t, readAutomaticFuelRequest(customer).Urgent)
	require.Len(t, customer.Waypoints, 1, "a partial donation must not send the recipient onto an unfunded return")
}

func TestPatrolCanDeliverFuelWithoutLosingItsOwnReturnReserve(t *testing.T) {
	for _, role := range []string{"scout", "freighter", "miner", "terraformer", "escort", "colonizer:auto"} {
		t.Run(role, func(t *testing.T) {
			u := logisticsUniverse(t)
			p, customer, donor := u.Player(1), u.Fleet("Transport"), u.Fleet("Escort")
			customer.Position = Vector{20, 0}
			customer.Fuel = 0
			p.Automation.FleetRoles = map[int]string{customer.Num: role, donor.Num: "patrol"}
			u.Run((*turnGenerator).refreshAutomaticFuelIntel)
			savePatrolJob(donor, patrolJob{Leader: donor.Num, Enemy: 2, Target: 91, Status: "Distant pursuit"})
			saveAutomaticFuelRequest(customer, automaticFuelRequest{Destination: customer.Position, Members: []int{customer.Num}, Urgent: true})
			u.Run((*turnGenerator).planAutomaticTankers)
			require.NotNil(t, readTankerJob(donor))
			for range 24 {
				u.Run((*turnGenerator).planAutomaticTankers, (*turnGenerator).fleetMove)
				fuel := customer.Fuel + donor.Fuel
				u.Run((*turnGenerator).resolveAutomaticTankers)
				require.Equal(t, fuel, customer.Fuel+donor.Fuel)
				if customer.GetTag(tankerAwaitTag) == "" {
					break
				}
			}
			require.Positive(t, customer.Fuel)
			require.Equal(t, "patrol", p.Automation.FleetRoles[donor.Num])
			require.GreaterOrEqual(t, donor.Fuel, automaticReturnFuel(&u.Game.Rules, p, donor, donor.Position, expeditionStations(p, donor), false))
			require.GreaterOrEqual(t, customer.Fuel, automaticReturnFuel(&u.Game.Rules, p, customer, customer.Position, expeditionStations(p, customer), false))
		})
	}
}

func TestLowFuelPatrolReturnsInsteadOfContinuingItsOldRoute(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Escort")
	p.Automation.FleetRoles = map[int]string{f.Num: "patrol"}
	f.Position = Vector{10, 0}
	f.Fuel = f.Spec.FuelCapacity / 5
	u.Run((*turnGenerator).refreshAutomaticFuelIntel, (*turnGenerator).planPatrolAutomation)
	require.Len(t, f.Waypoints, 2)
	require.Equal(t, u.Planet("Home").Position, f.Waypoints[1].Position)
}

func TestAutomaticTransportCapturesWeakValuableEnemyColony(t *testing.T) {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
		{Designs: Designs(DesignTeamster, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Transport", Design: DesignTeamster.Name, At: "Home"}}},
		{}}, Planets: []ScenarioPlanet{
		{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name, Cargo: Cargo{Colonists: 6000}},
		{Name: "Enemy colony", Owner: 2, Position: Vector{20, 0}, Cargo: Cargo{Colonists: 20}},
	}})
	p, f, target := u.Player(1), u.Fleet("Transport"), u.Planet("Enemy colony")
	target.Hab = Hab{50, 50, 50}
	target.BaseHab = target.Hab
	p.Automation.FleetRoles = map[int]string{f.Num: "freighter"}
	p.Automation.DisableSalvage = true
	p.Automation.DisableMineralSupply = true
	intel := *target
	p.PlanetIntels = []*Planet{u.Planet("Home"), &intel}
	u.Run((*turnGenerator).refreshAutomaticFuelIntel, (*turnGenerator).planLogisticsAutomation, (*turnGenerator).resolveLogisticsAutomation)
	require.NotNil(t, logisticsWork(f))
	require.Equal(t, "invade", logisticsWork(f).Kind)
	require.Positive(t, f.Cargo.Colonists)
	require.GreaterOrEqual(t, u.Planet("Home").Cargo.Colonists, populationTarget(u.Planet("Home"), 35))
	f.Position = target.Position
	u.Run((*turnGenerator).resolveAutomaticInvasions, (*turnGenerator).resolveColonistDrops)
	require.Equal(t, p.Num, target.PlayerNum)
	require.Zero(t, f.Cargo.Colonists)
	require.Equal(t, "freighter", p.Automation.FleetRoles[f.Num])
	// Already-arrived transports drop immediately even if the garrison grew.
	target.PlayerNum = 2
	target.Cargo.Colonists = 10000
	f.Cargo.Colonists = 10
	saveLogisticsWork(f, &logisticsJob{Kind: "invade", TargetNum: target.Num, DeliveryNum: target.Num, Position: target.Position})
	u.Run((*turnGenerator).resolveAutomaticInvasions)
	require.Nil(t, logisticsWork(f))
	require.Zero(t, f.Cargo.Colonists)
	require.Equal(t, 2, target.PlayerNum)
}

func TestPassengersCanUseAnotherSourceAndPreferHighGrowth(t *testing.T) {
	u := passengerUniverse(t)
	p, f := u.Player(1), u.Fleet("Transport")
	p.Automation.ManualPassengerThresholds = false
	u.Planet("Home").Cargo.Colonists = 2500
	source := *u.Planet("Home")
	source.Num = 3
	source.Name = "Second source"
	source.Position = Vector{10, 0}
	source.Cargo.Colonists = 9500
	source.Spec.HasStarbase = false
	source.Spec.DockCapacity = 0
	u.Game.Planets = append(u.Game.Planets, &source)
	u.Run(func(tg *turnGenerator) {
		job := tg.findPassengerTransfer(p, f)
		require.NotNil(t, job)
		require.Equal(t, source.Num, job.TargetNum)
	})
	better := *u.Planet("Young colony")
	better.Num = 4
	better.Position = Vector{60, 0}
	better.Spec.Habitability = 100
	u.Planet("Young colony").Spec.Habitability = 10
	u.Game.Planets = append(u.Game.Planets, &better)
	u.Run(func(tg *turnGenerator) {
		job := tg.findPassengerTransfer(p, f)
		require.NotNil(t, job)
		require.Equal(t, better.Num, job.DeliveryNum)
	})
}

func TestPassengersAreNotLoadedWhenTheWholeDeliveryCannotBeFunded(t *testing.T) {
	u := passengerUniverse(t)
	p, f := u.Player(1), u.Fleet("Transport")
	u.Planet("Young colony").Position = Vector{2000, 0}
	u.Run(func(tg *turnGenerator) { require.Nil(t, tg.findPassengerTransfer(p, f)) })
	require.Zero(t, f.Cargo.Colonists)
}
