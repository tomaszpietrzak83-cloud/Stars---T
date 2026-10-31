//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"math"
	"testing"
)

func TestMobileTankersRepairEachDamagedShipWithoutStackingBonuses(t *testing.T) {
	u := tankerUniverse(t)
	p, f, donor := u.Player(1), u.Fleet("Customer"), u.Fleet("Tanker")
	donor.Position = f.Position
	donor.Spec.RepairBonus = .1
	f.OrbitingPlanetNum = 0
	f.PreviousPosition = nil
	f.warped = false
	f.Tokens[0].Quantity = 3
	f.Tokens[0].QuantityDamaged = 3
	armor := f.Tokens[0].design.Spec.Armor
	f.Tokens[0].Damage = float64(armor) * .6
	second := *donor
	second.Num += 100
	second.Tags = nil
	u.Game.Fleets = append(u.Game.Fleets, &second)
	p.Automation.FleetRoles[second.Num] = "tanker"
	before := f.Tokens[0].Damage
	u.Run((*turnGenerator).fleetRepair)
	expected := max(1, int(float64(armor)*(.1+u.Game.Rules.RepairRates[RepairRateStopped]*p.Race.Spec.RepairFactor)))
	require.Equal(t, math.Floor(max(0, before-float64(expected))), f.Tokens[0].Damage)
	require.Zero(t, f.Spec.RepairBonus, "temporary workshop service must not become a permanent hull bonus")
}
func TestDamagedFleetSelectsNearbyWorkshopAndTankerWaitsOnlyForRealPatient(t *testing.T) {
	u := tankerUniverse(t)
	f, donor := u.Fleet("Customer"), u.Fleet("Tanker")
	f.Fuel = f.Spec.FuelCapacity
	f.Position = Vector{200, 0}
	donor.Position = f.Position.Add(Vector{10, 0})
	donor.OrbitingPlanetNum = 0
	donor.Spec.RepairBonus = .1
	f.Tokens[0].Damage = float64(f.Tokens[0].design.Spec.Armor) * .5
	f.Tokens[0].QuantityDamaged = 1
	u.Run((*turnGenerator).planAutomaticSafety)
	require.NotEmpty(t, f.GetTag(repairTankerTag))
	require.Equal(t, donor.Position, f.Waypoints[len(f.Waypoints)-1].Position)
	u.Run((*turnGenerator).planAutomaticTankers)
	require.Len(t, donor.Waypoints, 1)
	f.Tokens[0].Damage = 0
	f.Tokens[0].QuantityDamaged = 0
	u.Run((*turnGenerator).planAutomaticSafety)
	require.Empty(t, f.GetTag(repairTankerTag))
}
