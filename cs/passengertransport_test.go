//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func passengerUniverse(t *testing.T) *testUniverse {
	u := logisticsUniverse(t)
	p := u.Player(1)
	p.Automation.PassengerTransport = true
	p.Automation.ManualPassengerThresholds = true
	p.Automation.DisableSalvage = true
	p.Automation.DisableMineralSupply = true
	u.Planet("Home").Cargo.Colonists = 9500
	u.Planet("Home").Spec.MaxPopulation = 1000000
	dest := &Planet{MapObject: MapObject{Type: MapObjectTypePlanet, Num: 2, PlayerNum: 1, Name: "Young colony", Position: Vector{50, 0}}, Cargo: Cargo{Colonists: 1000}, Spec: PlanetSpec{MaxPopulation: 1000000, Habitability: 100}}
	u.Game.Planets = append(u.Game.Planets, dest)
	return u
}

func TestPassengerTransportConservesPeopleAndSourceReserve(t *testing.T) {
	u := passengerUniverse(t)
	source, dest, fleet := u.Planet("Home"), u.Planet("Young colony"), u.Fleet("Transport")
	before := source.Cargo.Colonists + dest.Cargo.Colonists
	for range 3 {
		u.Run((*turnGenerator).planLogisticsAutomation, (*turnGenerator).fleetMove, (*turnGenerator).resolveLogisticsAutomation)
	}
	require.Greater(t, dest.Cargo.Colonists, 1000)
	require.GreaterOrEqual(t, source.Cargo.Colonists, populationTarget(source, 90))
	require.LessOrEqual(t, dest.Cargo.Colonists, populationTarget(dest, 60))
	require.Equal(t, before, source.Cargo.Colonists+dest.Cargo.Colonists+fleet.Cargo.Colonists)
}

func TestPassengerReservationsAvoidOverfillingColony(t *testing.T) {
	u := passengerUniverse(t)
	p := u.Player(1)
	dest := u.Planet("Young colony")
	dest.Spec.MaxPopulation = 10000
	dest.Cargo.Colonists = 55
	second := *u.Fleet("Transport")
	second.Num = 4
	second.Name = "Second transport"
	second.Tags = nil
	u.Game.Fleets = append(u.Game.Fleets, &second)
	p.Automation.FleetRoles[second.Num] = "freighter"
	u.Run((*turnGenerator).planLogisticsAutomation)
	planned := 0
	jobs := 0
	for _, fleet := range u.Game.Fleets {
		if job := logisticsWork(fleet); job != nil && job.Kind == "passengers" {
			jobs++
			planned += job.Passengers
		}
	}
	require.Equal(t, 1, jobs)
	require.Equal(t, 5, planned)
}

func TestPassengerTransportRejectsUnsuitableColonies(t *testing.T) {
	u := passengerUniverse(t)
	u.Planet("Young colony").Spec.Habitability = -50
	u.Run((*turnGenerator).planLogisticsAutomation)
	require.Nil(t, logisticsWork(u.Fleet("Transport")))
}

func TestPassengerTransportKeepsPassengersWhenDestinationFills(t *testing.T) {
	u := passengerUniverse(t)
	fleet := u.Fleet("Transport")
	u.Run((*turnGenerator).planLogisticsAutomation, (*turnGenerator).resolveLogisticsAutomation)
	require.Positive(t, fleet.Cargo.Colonists)
	before := fleet.Cargo.Colonists
	u.Planet("Young colony").Cargo.Colonists = populationTarget(u.Planet("Young colony"), 60)
	u.Run((*turnGenerator).planLogisticsAutomation)
	require.Len(t, fleet.Waypoints, 1)
	require.Equal(t, before, fleet.Cargo.Colonists)
	require.Contains(t, logisticsWork(fleet).Status, "Waiting")
}

func TestAutomationConfigurationValidation(t *testing.T) {
	require.NoError(t, (LogisticsAutomation{}).Validate())
	require.NoError(t, (LogisticsAutomation{SourcePopulationPercent: 85, DestinationPopulationPercent: 40, EscortMarginLY: 150, EscortStrengthPercent: 125}).Validate())
	for _, config := range []LogisticsAutomation{{SourcePopulationPercent: 50, DestinationPopulationPercent: 60}, {SourcePopulationPercent: 101}, {DestinationPopulationPercent: -1}, {EscortMarginLY: 1001}, {EscortStrengthPercent: 10}} {
		require.Error(t, config.Validate())
	}
}

func TestPassengerGrowthModeTargetsNearMaximumAbsoluteGrowth(t *testing.T) {
	a := LogisticsAutomation{}
	require.Equal(t, 35, a.sourcePopulationPercent())
	require.Equal(t, 20, a.destinationPopulationPercent())

}

func TestEveryThirdPassengerTripChoosesAnotherNearbyColony(t *testing.T) {
	u := passengerUniverse(t)
	p, f := u.Player(1), u.Fleet("Transport")
	other := *u.Planet("Young colony")
	other.Num = 3
	other.Position = Vector{60, 0}
	other.Name = "Another colony"
	u.Game.Planets = append(u.Game.Planets, &other)
	f.SetTag(passengerRotationTag, `{"trips":2,"lastDestination":2}`)
	u.Run(func(tg *turnGenerator) {
		job := tg.findPassengerTransfer(p, f)
		require.NotNil(t, job)
		require.Equal(t, 3, job.DeliveryNum)
		tg.loadAutomaticPassengers(p, f, job)
	})
	require.Equal(t, 3, readPassengerRotation(f).Trips)
	require.Equal(t, 3, readPassengerRotation(f).LastDestination)
}
