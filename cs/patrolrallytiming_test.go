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
			}}, {Designs: Designs(DesignDestroyerDelta), Fleets: []ScenarioFleet{{Name: "Enemy", Design: DesignDestroyerDelta.Name, Position: Vector{300, 0}}}}}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}}})
			p := u.Player(1)
			enemy := u.Fleet("Enemy")
			p.discoverer.discoverDesign(enemy.Tokens[0].design, true)
			p.discoverer.discoverFleet(enemy, true)
			p.Automation.FleetRoles = map[int]string{}
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
				p.Automation.FleetRoles[f.Num] = "patrol"
				savePatrolJob(f, patrolJob{Provisional: true, Leader: group[0].Num, Enemy: 2, Target: enemy.Num, Rally: point, RendezvousBy: u.Game.Year + 2})
				applyExpeditionRoute(p, f, rally.Routes[i])
				require.True(t, fundedPatrolRallyWaypoints(&u.Game.Rules, p, f, u.Game.Year))
				fuel := f.Fuel
				f.Fuel = 0
				require.False(t, fundedPatrolRallyWaypoints(&u.Game.Rules, p, f, u.Game.Year))
				f.Fuel = fuel
			}
			u.Run((*turnGenerator).startPatrolEvaluation, (*turnGenerator).guardAutomaticFuel, (*turnGenerator).enforceAutomaticDeparture, (*turnGenerator).fleetMove)
			for _, f := range group {
				require.NotEqual(t, point, f.Position, "no early arrival should trigger cancellation of the pack")
				require.Greater(t, len(f.Waypoints), 1)
			}
			u.Game.Year++
			u.Run((*turnGenerator).fleetInit, (*turnGenerator).guardAutomaticFuel, (*turnGenerator).enforceAutomaticDeparture, (*turnGenerator).fleetMove)
			for _, f := range group {
				require.Equal(t, point, f.Position)
			}
		})
	}
}
