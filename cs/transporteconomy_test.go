//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"strconv"
	"testing"
)

func TestColonyPopulationPolicyPreventsReexport(t *testing.T) {
	u := passengerUniverse(t)
	p, dest := u.Player(1), u.Planet("Young colony")
	p.Automation.ManualPassengerThresholds = false
	for _, hab := range []int{25, 49, 50, 74} {
		dest.Spec.Habitability = hab
		require.Equal(t, 50, colonyPopulationPercent(p, dest))
		require.Equal(t, populationTarget(dest, 50), sourcePopulationReserve(p, dest))
	}
	dest.Spec.Habitability = 75
	require.Equal(t, 20, colonyPopulationPercent(p, dest))
	p.Automation.ManualPassengerThresholds = true
	p.Automation.SourcePopulationPercent = 40
	dest.Spec.Habitability = 74
	require.Equal(t, populationTarget(dest, 50), sourcePopulationReserve(p, dest), "low-habitability export floor also protects manual thresholds")
	p.Automation.ManualPassengerThresholds = false
	dest.Spec.Habitability = 75
	p.Automation.OccupationSupport = map[int]bool{dest.Num: true}
	require.Equal(t, 75, colonyPopulationPercent(p, dest))
	dest.Cargo.Colonists = populationTarget(dest, 60)
	u.Run((*turnGenerator).updateOccupationSupport)
	require.True(t, p.Automation.OccupationSupport[dest.Num])
	dest.Spec.HasStarbase = true
	u.Run((*turnGenerator).updateOccupationSupport)
	require.False(t, p.Automation.OccupationSupport[dest.Num], "base completion ends the 75-percent policy even below75")
}

func TestCompletedArmyReturnsIndividualEmptyHolds(t *testing.T) {
	u := landingOperationUniverse(t)
	p, f := u.Player(1), u.Fleet("Transport0")
	ships, fuel := f.Spec.TotalShips, f.Fuel
	f.Cargo = Cargo{}
	op := &landingOperation{Target: u.Planet("Target").Num, Phase: "landed"}
	saveLandingOperation(f, op)
	before := len(u.Game.getFleets(1))
	u.Run(func(tg *turnGenerator) { tg.finishLandingOperation(p, f, op) })
	require.Equal(t, before+ships-1, len(u.Game.getFleets(1)))
	require.Equal(t, 1, f.Spec.TotalShips)
	remainingFuel := f.Fuel
	for _, child := range u.Game.getFleets(1)[before:] {
		require.Equal(t, 1, child.Spec.TotalShips)
		require.Equal(t, "freighter", p.Automation.FleetRoles[child.Num])
		require.False(t, landingManaged(child))
		remainingFuel += child.Fuel
	}
	require.Equal(t, fuel, remainingFuel)
}

func TestCapturedColonyReceivesPhysicalFullLoadAboveOldThreshold(t *testing.T) {
	u := passengerUniverse(t)
	p, dest, f := u.Player(1), u.Planet("Young colony"), u.Fleet("Transport")
	p.Automation.OccupationSupport = map[int]bool{dest.Num: true}
	dest.Cargo.Colonists = populationTarget(dest, 65)
	before := dest.Cargo.Colonists
	u.Run((*turnGenerator).planLogisticsAutomation, (*turnGenerator).resolveLogisticsAutomation)
	require.Equal(t, "settle", logisticsWork(f).Kind)
	require.Equal(t, f.Spec.CargoCapacity, f.CargoMass())
	for range 5 {
		u.Run((*turnGenerator).dispatchReadyLogistics, (*turnGenerator).fleetMove, (*turnGenerator).resolveLogisticsAutomation)
	}
	require.Greater(t, dest.Cargo.Colonists, before)
	require.LessOrEqual(t, dest.Cargo.Colonists, populationTarget(dest, 75))
	require.Equal(t, populationTarget(dest, 75), sourcePopulationReserve(p, dest))
}

func TestCivilianDeliveryClaimsTransportBeforeLanding(t *testing.T) {
	u := landingOperationUniverse(t)
	p := u.Player(1)
	dest := &Planet{MapObject: MapObject{Num: 3, PlayerNum: 1, Name: "Support", Position: Vector{10, 0}}, Cargo: Cargo{Colonists: 10}, Spec: PlanetSpec{MaxPopulation: 1000000, Habitability: 100}}
	u.Game.Planets = append(u.Game.Planets, dest)
	p.Automation.OccupationSupport = map[int]bool{dest.Num: true}
	u.Run((*turnGenerator).planLogisticsAutomation)
	civilian := 0
	for _, f := range u.Game.getFleets(1) {
		if job := logisticsWork(f); job != nil && job.Kind == "passengers" {
			civilian++
			require.False(t, landingManaged(f))
			require.False(t, landingAvailable(p, f))
		}
	}
	require.Positive(t, civilian)
	require.NotNil(t, landingLeader(u), "remaining holds may still prepare a landing")
}

func TestDefenseExportLeavesEightyPercentAndPhysicallyDelivers(t *testing.T) {
	u := passengerUniverse(t)
	p, f, source, dest := u.Player(1), u.Fleet("Transport"), u.Planet("Home"), u.Planet("Young colony")
	p.Automation.InfrastructureTransport = true
	source.Spec.MaxDefenses, source.Defenses = 100, 100
	dest.Spec.MaxDefenses, dest.Defenses = 100, 0
	source.Factories, source.Mines = 0, 0
	u.Run(func(tg *turnGenerator) { tg.loadOpportunisticInfrastructure(p, f, source, dest, 0) })
	require.Equal(t, 80, source.Defenses)
	require.Equal(t, 20, readInfrastructure(f).Defenses)
	require.Equal(t, 20, f.CargoMass())
	f.Position = dest.Position
	u.Run((*turnGenerator).unloadOpportunisticInfrastructure)
	require.Equal(t, 20, dest.Defenses)
	require.Zero(t, f.CargoMass())
}

func TestSurplusMinesArePackedAndPhysicallyDelivered(t *testing.T) {
	u := passengerUniverse(t)
	p, f, source, dest := u.Player(1), u.Fleet("Transport"), u.Planet("Home"), u.Planet("Young colony")
	p.Automation.InfrastructureTransport = true
	p.Race.NumMines = 10
	source.Cargo.Colonists, dest.Cargo.Colonists = 1000, 200
	source.Factories, source.Mines, source.Defenses = 0, 110, 0
	dest.Mines = 0
	u.Run(func(tg *turnGenerator) { tg.loadOpportunisticInfrastructure(p, f, source, dest, 0) })
	require.Equal(t, 95, source.Mines)
	require.Equal(t, 15, readInfrastructure(f).Mines)
	require.Equal(t, 45, f.CargoMass())
	f.Position = dest.Position
	u.Run((*turnGenerator).unloadOpportunisticInfrastructure)
	require.Equal(t, 15, dest.Mines)
	require.Equal(t, 110, source.Mines+dest.Mines)
}

func TestCivilianAndLandingPopulationReservationsShareSource(t *testing.T) {
	u := landingOperationUniverse(t)
	p, source := u.Player(1), u.Planet("Home")
	a, b := u.Fleet("Transport0"), u.Fleet("Transport1")
	saveLogisticsWork(a, &logisticsJob{Kind: "passengers", TargetNum: source.Num, Passengers: 200})
	saveLogisticsWork(b, &logisticsJob{Kind: "landing-load", TargetNum: source.Num, Passengers: 300})
	u.Run(func(tg *turnGenerator) {
		require.Equal(t, 500, tg.landingReservations(p)[source.Num])
		_, outgoing := tg.passengerReservations(p, a.Num)
		require.Equal(t, 300, outgoing[source.Num])
	})
}

func TestBlockedBaseArmyRetargetsWithoutDroppingBlind(t *testing.T) {
	u, leader, op := landingBaseUniverse(t, true)
	p := u.Player(1)
	leader.Tokens[0].Quantity = 10
	leader.Spec = ComputeFleetSpec(&u.Game.Rules, p, leader)
	leader.Fuel = leader.Spec.FuelCapacity
	leader.Cargo.Colonists = leader.Spec.CargoCapacity
	loaded := leader.Cargo.Colonists
	weak := *p.GetPlanetIntel(op.Target)
	weak.Num, weak.Name, weak.Position = 3, "Opportunity", Vector{-50, 0}
	weak.Spec.HasStarbase = false
	weak.Spec.MaxPopulation = 100000
	weak.Cargo.Colonists = 1
	p.PlanetIntels = append(p.PlanetIntels, &weak)
	u.Run(func(tg *turnGenerator) { tg.prepareLandingBaseAssault(p, leader, op, p.GetPlanetIntel(op.Target), nil) })
	require.Equal(t, weak.Num, op.Target)
	require.Equal(t, "gather", op.Phase)
	require.Zero(t, op.ScanYear)
	require.False(t, op.Base)
	require.Equal(t, loaded, leader.Cargo.Colonists)
	require.Empty(t, leader.GetTag(recaptureAssaultTag))
	require.NotEqual(t, weak.Position, leader.Position)
}

func TestUsefulAttritionDropReducesRealDefenders(t *testing.T) {
	u := landingOperationUniverse(t)
	p, target := u.Player(1), u.Planet("Target")
	target.Cargo.Colonists = 10000
	require.True(t, usefulAttritionLanding(&u.Game.Rules, p, target, 2000))
	require.False(t, usefulAttritionLanding(&u.Game.Rules, p, target, 1999))
	f := u.Fleet("Transport0")
	f.Tokens[0].Quantity = 10
	f.Spec = ComputeFleetSpec(&u.Game.Rules, p, f)
	f.Fuel = f.Spec.FuelCapacity
	f.Position, f.Cargo.Colonists = Vector{35, 0}, f.Spec.CargoCapacity
	scout, cover := u.Fleet("Scout"), u.Fleet("Cover")
	scout.Position, cover.Position = f.Position, f.Position
	scout.SetTag(landingMemberTag, strconv.Itoa(f.Num))
	cover.SetTag(landingMemberTag, strconv.Itoa(f.Num))
	p.GetPlanetIntel(target.Num).Cargo.Colonists = 10000
	op := &landingOperation{Target: target.Num, Owner: target.PlayerNum, Phase: "gather", Need: 20000, Scout: scout.Num, Rally: f.Position, EscortDesigns: map[int]bool{}}
	saveLandingOperation(f, op)
	plan := func(tg *turnGenerator) {
		op = readLandingOperation(f)
		tg.advanceLandingOperation(p, f, op, p.GetPlanetIntel(target.Num))
		saveLandingOperation(f, op)
	}
	u.Run(plan)
	require.Equal(t, "scan", readLandingOperation(f).Phase)
	u.Run((*turnGenerator).fleetMove, (*turnGenerator).scanLandingContacts, plan)
	require.Equal(t, "assault", readLandingOperation(f).Phase)
	u.Run((*turnGenerator).enforceAutomaticDeparture, (*turnGenerator).fleetMove, (*turnGenerator).resolveLandingDrops, (*turnGenerator).resolveColonistDrops)
	require.Equal(t, 2, target.PlayerNum, "the attack is useful without capturing the planet")
	require.Less(t, target.Cargo.Colonists, 10000)
	require.Zero(t, f.Cargo.Colonists)
	target.Spec.DefenseCoverage = 1
	require.False(t, usefulAttritionLanding(&u.Game.Rules, p, target, 2000), "do not repeatedly feed troops into overwhelming ground defenses")
}
