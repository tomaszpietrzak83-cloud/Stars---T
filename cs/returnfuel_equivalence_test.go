//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"math"
	"testing"
)

// Preserve the pre-optimization calculation as an independent reference.
func referenceReturnFuel2494(rules *Rules, player *Player, fleet *Fleet, position Vector, stations []*Planet, allowThreat bool, returnTurns ...int) int {
	best := int(^uint(0) >> 2)
	loaded := deliveryReturnBudget(player, fleet, position)
	limit := 0
	if len(returnTurns) > 0 {
		limit = returnTurns[0]
	}
	if automatic := automaticReturnLimit(player, fleet, MapObjectTarget{}); automatic > 0 && (limit == 0 || limit > automatic) {
		limit = automatic
	}
	for _, station := range stations {
		if station.Position == position {
			return 0
		}
		cruiseSafe := false
		for speed := automaticCruiseWarp(player, fleet); speed <= automaticMaxWarp(player, fleet); speed++ {
			if automaticFleetLegSafe(rules, player, fleet, position, station.Position, speed, allowThreat) {
				cruiseSafe = true
				break
			}
		}
		for speed := 1; speed <= automaticMaxWarp(player, fleet); speed++ {
			if limit > 0 && math.Ceil(position.DistanceTo(station.Position)/fleet.StrategicSpeed(speed)) > float64(limit) {
				continue
			}
			if (player.Automation.FleetRoles[fleet.Num] == "scout" || player.Automation.FleetRoles[fleet.Num] == "patrol" || passengerCruiseFleet(fleet) || isAutomaticFleet(player, fleet) && cruiseSafe) && speed < automaticCruiseWarp(player, fleet) && position.DistanceTo(station.Position) > fleet.StrategicSpeed(speed) && !(usefulFreePatrolMovement(player, fleet) && speed <= fleet.Spec.Engine.FreeSpeed) {
				continue
			}
			if automaticFleetLegSafe(rules, player, fleet, position, station.Position, speed, allowThreat) {
				if required, _, _, ok := automaticLegFuel(player, loaded, speed, position.DistanceTo(station.Position), fleet.Spec.FuelCapacity, 0); ok {
					best = min(best, required)
				}
			}
		}
	}
	return best
}

func TestReturnFuelOptimizationMatchesReference(t *testing.T) {
	u := logisticsUniverse(t)
	p := u.Player(1)
	for _, name := range []string{"Escort", "Transport", "Combat"} {
		f := u.Fleet(name)
		stations := expeditionStations(p, f)
		require.NotEmpty(t, stations)
		for _, role := range []string{"", "patrol", "scout", "transport"} {
			p.Automation.FleetRoles[f.Num] = role
			for _, x := range []int{0, 10, 100, 500} {
				for _, limit := range []int{0, 1, 5} {
					for _, allow := range []bool{false, true} {
						p.FleetIntels = []*Fleet{patrolTestEnemy(x + 20)}
						position := Vector{X: x, Y: 30}
						want := referenceReturnFuel2494(&u.Game.Rules, p, f, position, stations, allow, limit)
						got := automaticReturnFuel(&u.Game.Rules, p, f, position, stations, allow, limit)
						require.Equal(t, want, got)
						require.Nil(t, p.routeThreatIndex, "standalone reserve calculation must release its snapshot")
						cleanup := scopedRouteThreatIndex(p)
						previous := map[routeThreatKey]int{{from: Vector{X: -100}, to: Vector{X: -50}}: 123}
						p.routeThreatIndex.routes = previous
						require.Equal(t, want, automaticReturnFuel(&u.Game.Rules, p, f, position, stations, allow, limit))
						require.Equal(t, previous, p.routeThreatIndex.routes, "nested calculation must restore caller cache")
						cleanup()
					}
				}
			}
		}
	}
}
