//go:build !wasi && !wasm

package cs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompatibleEscortsTravelTogether(t *testing.T) {
	position := Vector{50, 50}
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{
		Designs: Designs(DesignDestroyerDelta),
		Fleets: []ScenarioFleet{
			{Name: "Lead", Position: position, Design: DesignDestroyerDelta.Name},
			{Name: "Wing", Position: position, Design: DesignDestroyerDelta.Name},
			{Name: "Other route", Position: position, Design: DesignDestroyerDelta.Name},
			{Name: "Manual", Position: position, Design: DesignDestroyerDelta.Name},
			{Name: "Followed", Position: position, Design: DesignDestroyerDelta.Name},
		},
	}}})
	p := u.Player(1)
	lead, wing := u.Fleet("Lead"), u.Fleet("Wing")
	other, manual, followed := u.Fleet("Other route"), u.Fleet("Manual"), u.Fleet("Followed")
	p.Automation.FleetRoles = map[int]string{}
	destination := NewPositionWaypoint(Vector{90, 50}, 6)
	for _, fleet := range []*Fleet{lead, wing, other, manual, followed} {
		fleet.Waypoints = []Waypoint{{Position: position}, destination}
		fleet.Fuel = fleet.Spec.FuelCapacity
		if fleet != manual {
			p.Automation.FleetRoles[fleet.Num] = "escort"
		}
	}
	other.Waypoints[1].Position = Vector{100, 50} // Parallel heading, different destination.
	manual.Waypoints[1].MapObjectTarget = followed.ToTarget()
	wing.Tokens[0].Damage = 10
	wing.Tokens[0].QuantityDamaged = 1
	fuel := lead.Fuel + wing.Fuel
	u.Run((*turnGenerator).mergeCompatibleCombatFleets, (*turnGenerator).splitAutomaticCombatPools)
	require.True(t, wing.Delete)
	require.False(t, other.Delete)
	require.False(t, manual.Delete)
	require.False(t, followed.Delete)
	require.Equal(t, 2, lead.Spec.TotalShips)
	require.Equal(t, 1, lead.Tokens[0].QuantityDamaged)
	require.Equal(t, 10.0, lead.Tokens[0].Damage)
	require.Equal(t, fuel, lead.Fuel)
	require.Equal(t, destination.Position, lead.Waypoints[1].Position)
	require.NotContains(t, p.Automation.FleetRoles, wing.Num)
}
