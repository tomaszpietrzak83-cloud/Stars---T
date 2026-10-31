//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAutomaticPassengerSourceRetainsMaximumNominalGrowth(t *testing.T) {
	u := passengerUniverse(t)
	p, source := u.Player(1), u.Planet("Home")
	p.Automation.ManualPassengerThresholds = false
	source.Hab = Hab{50, 50, 50}
	reserve := sourcePopulationReserve(p, source)
	require.Equal(t, 3334, reserve)
	source.Cargo.Colonists = reserve
	peak := source.GetGrowthAmount(p, source.Spec.MaxPopulation, .04, .12)
	require.Positive(t, peak)
	// Check actual number of people born, not the percentage growth rate.
	for density := 1; density <= 100; density++ {
		source.Cargo.Colonists = source.Spec.MaxPopulation * density / 100 / 100
		require.LessOrEqual(t, source.GetGrowthAmount(p, source.Spec.MaxPopulation, .04, .12), peak)
	}
	source.Cargo.Colonists = 5000
	require.Less(t, source.GetGrowthAmount(p, source.Spec.MaxPopulation, .04, .12), peak)
	p.Automation.ManualPassengerThresholds = true
	p.Automation.SourcePopulationPercent = 45
	require.Equal(t, 4500, sourcePopulationReserve(p, source))
}

func TestPassengerTransportWaitsEmptyThenLoadsFullAndDepartsSamePhase(t *testing.T) {
	u := passengerUniverse(t)
	p, f, source, dest := u.Player(1), u.Fleet("Transport"), u.Planet("Home"), u.Planet("Young colony")
	p.Automation.ManualPassengerThresholds = false
	source.Spec.HasStarbase = true
	source.Spec.DockCapacity = -1
	source.Cargo = Cargo{Colonists: sourcePopulationReserve(p, source) + f.Spec.CargoCapacity - 1}
	dest.Cargo = Cargo{Colonists: 1}
	job := &logisticsJob{Kind: "passengers", TargetNum: source.Num, DeliveryNum: dest.Num, Position: source.Position, Passengers: f.Spec.CargoCapacity - 1}
	saveLogisticsWork(f, job)
	before := source.Cargo
	u.Run((*turnGenerator).resolveLogisticsAutomation, (*turnGenerator).dispatchReadyLogistics)
	require.Equal(t, before, source.Cargo)
	require.Zero(t, f.CargoMass())
	require.Len(t, f.Waypoints, 1)
	require.Equal(t, "passengers", logisticsWork(f).Kind)
	require.Contains(t, logisticsWork(f).Status, "Waiting empty")
	source.Cargo.Colonists++
	u.Run((*turnGenerator).resolveLogisticsAutomation, (*turnGenerator).dispatchReadyLogistics)
	require.Equal(t, f.Spec.CargoCapacity, f.CargoMass())
	require.Equal(t, sourcePopulationReserve(p, source), source.Cargo.Colonists)
	require.Greater(t, len(f.Waypoints), 1)
	require.Equal(t, dest.Position, f.Waypoints[1].Position)
	require.Equal(t, source.Position, f.Waypoints[len(f.Waypoints)-1].Position)
	require.True(t, fundedLogisticsWaypoints(&u.Game.Rules, p, f))
}

func TestPriorityPlanetOverridesColonizationDistanceMode(t *testing.T) {
	u := colonizerUniverse(t)
	p, f := u.Player(1), u.Fleet("Colony Ship")
	p.Automation.FleetRoles[f.Num] = "colonizer:farthest"
	p.Automation.PriorityPlanets = map[int]bool{u.Planet("Near").Num: true}
	u.Run((*turnGenerator).planColonizationAutomation)
	require.Equal(t, u.Planet("Near").Num, readColonizationJob(f).TargetNum)
}

func TestPriorityBombingEndsAtOneHundredThousandPeople(t *testing.T) {
	u := colonizerUniverse(t)
	p, planet := u.Player(1), u.Planet("Near")
	planet.PlayerNum = 2
	p.Automation.PriorityPlanets = map[int]bool{planet.Num: true}
	planet.Cargo.Colonists = 1001
	require.Equal(t, planet, priorityBombingTarget(p, u.Fleet("Colony Ship")))
	planet.Cargo.Colonists = 1000
	require.Nil(t, priorityBombingTarget(p, u.Fleet("Colony Ship")))
	planet.Defenses = 1
	require.Equal(t, planet, priorityBombingTarget(p, u.Fleet("Colony Ship")))
}
