//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestExpeditionChoosesEconomicalSafeWarpAndReturnReserve(t *testing.T) {
	u := logisticsUniverse(t)
	p, fleet := u.Player(1), u.Fleet("Transport")
	for i := range fleet.Tokens {
		fleet.Tokens[i].design.Spec.Engine.FuelUsage = []int{0, 10, 10, 10, 10, 10, 10, 10, 10, 10000, 10000}
	}
	fleet.Spec.Engine.MaxSafeSpeed = 9
	p.PlanetIntels = nil
	addExpeditionTestDock(p, fleet.Position)
	destination := &Planet{MapObject: MapObject{Num: 2, Position: Vector{100, 0}}}
	stations := expeditionStations(p, fleet)
	reserve := expeditionReserve(&u.Game.Rules, p, fleet, destination.Position, stations)
	fleet.Fuel = expeditionFuel(p, fleet, 5, 100) + reserve
	fleet.Spec.FuelCapacity = fleet.Fuel
	route := planExpeditionRoute(&u.Game.Rules, p, fleet, destination)
	require.NotNil(t, route)
	require.LessOrEqual(t, route.Waypoints[1].WarpSpeed, 8)
	require.LessOrEqual(t, route.Fuel+reserve, fleet.Fuel)
	fleet.Fuel = 0
	wait := planExpeditionRoute(&u.Game.Rules, p, fleet, destination)
	require.NotNil(t, wait)
	require.True(t, wait.WaitForFuel)
	applyExpeditionRoute(p, fleet, wait)
	require.Len(t, fleet.Waypoints, 1)
	require.Zero(t, fleet.Fuel)
}

func TestExpeditionUsesKnownFuelStopAndDoesNotInventInitialFuel(t *testing.T) {
	u := logisticsUniverse(t)
	p, fleet := u.Player(1), u.Fleet("Transport")
	for i := range fleet.Tokens {
		fleet.Tokens[i].design.Spec.Engine.FuelUsage = []int{0, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100}
	}
	fleet.Spec.Engine.MaxSafeSpeed = 9
	p.PlanetIntels = nil
	addExpeditionTestDock(p, Vector{})
	addExpeditionTestDock(p, Vector{100, 0})
	addExpeditionTestDock(p, Vector{200, 0})
	destination := p.PlanetIntels[2]
	fleet.Fuel = expeditionFuel(p, fleet, 9, 100) + 1
	fleet.Spec.FuelCapacity = fleet.Fuel
	route := planExpeditionRoute(&u.Game.Rules, p, fleet, destination)
	require.NotNil(t, route)
	require.Len(t, route.Waypoints, 3)
	require.Equal(t, Vector{100, 0}, route.Waypoints[1].Position)
	fleet.Fuel = 0
	wait := planExpeditionRoute(&u.Game.Rules, p, fleet, destination)
	require.NotNil(t, wait)
	require.True(t, wait.WaitForFuel)
	applyExpeditionRoute(p, fleet, wait)
	require.Len(t, fleet.Waypoints, 1)
	require.Zero(t, fleet.Fuel)
}
