//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAutomatedCombatPoolSplitsWithoutChangingManualOrActivePacks(t *testing.T) {
	for _, role := range []string{"patrol", "escort", ""} {
		t.Run(role, func(t *testing.T) {
			u := logisticsUniverse(t)
			p, f := u.Player(1), u.Fleet("Escort")
			f.Tokens[0].Quantity = 24
			f.Spec = ComputeFleetSpec(&u.Game.Rules, p, f)
			f.Fuel = f.Spec.FuelCapacity
			p.Automation.FleetRoles[f.Num] = role
			before := len(u.Game.getFleets(1))
			totalFuel := f.Fuel
			u.Run((*turnGenerator).splitAutomaticCombatPools)
			if role == "" {
				require.Len(t, u.Game.getFleets(1), before)
				require.Equal(t, 24, f.Spec.TotalShips)
				return
			}
			count, fuel := 0, 0
			for _, ship := range u.Game.getFleets(1) {
				if p.Automation.FleetRoles[ship.Num] == role {
					require.Equal(t, 1, ship.Spec.TotalShips)
					count++
					fuel += ship.Fuel
				}
			}
			require.Equal(t, 24, count)
			require.Equal(t, totalFuel, fuel)
			// A coordinated mission is not torn apart while it is still active.
			f.Tokens[0].Quantity = 3
			f.Spec = ComputeFleetSpec(&u.Game.Rules, p, f)
			savePatrolJob(f, patrolJob{Leader: f.Num, Grouped: true})
			u.Run((*turnGenerator).splitAutomaticCombatPools)
			require.Equal(t, 3, f.Spec.TotalShips)
		})
	}
}

func TestSplitCombatPoolGetsDifferentFrontierDestinations(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Escort")
	f.Tokens[0].Quantity = 5
	f.Spec = ComputeFleetSpec(&u.Game.Rules, p, f)
	f.Fuel = f.Spec.FuelCapacity
	p.Automation.FleetRoles[f.Num] = "patrol"
	addExpeditionTestDock(p, Vector{})
	for i := 1; i <= 5; i++ {
		p.PlanetIntels = append(p.PlanetIntels, &Planet{MapObject: MapObject{Num: 500 + i, Position: Vector{60 + i*8, i * 15}}})
	}
	u.Run((*turnGenerator).splitAutomaticCombatPools, (*turnGenerator).planPatrolAutomation)
	destinations := map[Vector]bool{}
	for _, ship := range u.Game.getFleets(1) {
		if p.Automation.FleetRoles[ship.Num] == "patrol" && len(ship.Waypoints) > 1 {
			destinations[ship.Waypoints[len(ship.Waypoints)-1].Position] = true
		}
	}
	require.Greater(t, len(destinations), 1, "a fresh patrol pool must spread its coverage")
}
