//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestFuelNetworkEdgeCacheMatchesOriginalSearch(t *testing.T) {
	for _, role := range []string{"freighter", "patrol", "scout", "colonizer:best"} {
		for _, fuelPercent := range []int{0, 40, 100} {
			for _, policy := range []automaticRoutePolicy{{}, {OneWay: true}, {AllowThreat: true, MaxTurns: 3}, {RefuelRendezvous: true, MaxWarp: 4}} {
				u, customer, donors, goal := networkFixture(t)
				p := u.Player(1)
				p.Automation.FleetRoles[customer.Num] = role
				customer.Fuel = customer.Spec.FuelCapacity * fuelPercent / 100
				for _, anyDock := range []bool{false, true} {
					want := referenceFuelNetworkRoute1113(&u.Game.Rules, p, []*Fleet{customer}, donors, goal.Position, goal.MapObject.ToTarget(), policy, anyDock)
					got := planFuelNetworkRoute(&u.Game.Rules, p, []*Fleet{customer}, donors, goal.Position, goal.MapObject.ToTarget(), policy, anyDock)
					require.Equal(t, want, got, "role=%s fuel=%d policy=%+v anyDock=%v", role, fuelPercent, policy, anyDock)
				}
			}
		}
	}
}
