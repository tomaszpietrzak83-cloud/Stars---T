//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"strconv"
	"testing"
)

func TestPatrolRallySynchronizesActualArrivalWithoutIdleTurn(t *testing.T) {
	for _, start := range []int{30, 39} {
		t.Run(strconv.Itoa(start), func(t *testing.T) {
			u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Designs: Designs(DesignDestroyerDelta, DesignSpaceStation), Fleets: []ScenarioFleet{
				{Name: "Near", Design: DesignDestroyerDelta.Name, Position: Vector{start, 0}},
				{Name: "Far", Design: DesignDestroyerDelta.Name, At: "Home"},
			}}}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}}})
			p := u.Player(1)
			group := []*Fleet{u.Fleet("Near"), u.Fleet("Far")}
			point := Vector{40, 0}
			rally := &patrolRally{Position: point, GatherTurns: 2}
			for _, f := range group {
				route := planAutomaticRoute(&u.Game.Rules, p, []*Fleet{f}, point, MapObjectTarget{}, automaticRoutePolicy{Fast: true, MinWarp: 6, MaxWarp: 6, MaxTurns: 2})
				require.NotNil(t, route)
				rally.Routes = append(rally.Routes, route)
			}
			require.Equal(t, 1, rally.Routes[0].Turns)
			require.Equal(t, 2, rally.Routes[1].Turns)
			require.True(t, synchronizePatrolRally(&u.Game.Rules, p, group, Vector{60, 0}, rally))
			for i, f := range group {
				applyExpeditionRoute(p, f, rally.Routes[i])
			}
			u.Run((*turnGenerator).fleetMove)
			for _, f := range group {
				require.NotEqual(t, point, f.Position, "no early arrival should trigger cancellation of the pack")
				require.Greater(t, len(f.Waypoints), 1)
			}
			u.Run((*turnGenerator).fleetInit, (*turnGenerator).fleetMove)
			for _, f := range group {
				require.Equal(t, point, f.Position)
			}
		})
	}
}
