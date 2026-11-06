//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestLoadedDeliveryUsesSafeDetourInsteadOfWaitingForEscort(t *testing.T) {
	for _, invade := range []bool{false, true} {
		owner := 1
		if invade {
			owner = 2
		}
		u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Designs: Designs(DesignTeamster, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Transport", Design: DesignTeamster.Name, At: "Home", Cargo: Cargo{Colonists: 70}}}}, {Designs: Designs(DesignSpaceStation)}}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name, Cargo: Cargo{Colonists: 5000}}, {Name: "Dock", Owner: 1, Position: Vector{50, 80}, Starbase: DesignSpaceStation.Name, Cargo: Cargo{Colonists: 5000}}, {Name: "Target", Owner: owner, Position: Vector{100, 0}, Cargo: Cargo{Colonists: 10}}}})
		p, f, target := u.Player(1), u.Fleet("Transport"), u.Planet("Target")
		p.Automation.FleetRoles = map[int]string{f.Num: "freighter"}
		target.Hab = u.Planet("Home").Hab
		target.BaseHab = target.Hab
		target.Spec.Habitability = 100
		target.Spec.MaxPopulation = 1000000
		p.PlanetIntels = []*Planet{u.Planet("Home"), u.Planet("Dock"), target}
		enemy := patrolTestEnemy(10)
		enemy.Position = Vector{50, 0}
		p.FleetIntels = []*Fleet{enemy}
		p.MinefieldIntels = []*Minefield{{MapObject: MapObject{PlayerNum: 2, Position: enemy.Position}, MinefieldType: MinefieldTypeStandard, NumMines: 100, Detonate: true}}
		kind := "settle"
		if invade {
			kind = "invade"
		}
		job := &logisticsJob{Kind: kind, TargetNum: target.Num, DeliveryNum: target.Num, Position: target.Position}
		saveLogisticsWork(f, job)
		u.Run((*turnGenerator).refreshAutomaticFuelIntel, func(tg *turnGenerator) {
			require.Positive(t, knownRouteThreat(p, f.Position, target.Position))
			route := planAutomaticRoute(&u.Game.Rules, p, []*Fleet{f}, target.Position, target.ToTarget(), automaticRoutePolicy{})
			require.NotNil(t, route, "a funded minefield detour must exist")
			require.False(t, route.WaitForFuel)
			require.Greater(t, len(route.Waypoints), 2)
			for i, wp := range route.Waypoints[1:] {
				require.Greater(t, segmentDistance(enemy.Position, route.Waypoints[i].Position, wp.Position), automaticMineClearance(p.MinefieldIntels[0]))
			}
			tg.prepareConvoyWithMinimum(p, f, job, target.ToTarget(), 1)
			require.Len(t, f.Waypoints, 1, "a mandatory escort cannot be bypassed")
			tg.prepareConvoy(p, f, job, target.ToTarget())
			require.Greater(t, len(f.Waypoints), 1, "a funded safe route needs no unavailable escort")
		})
		for range 10 {
			u.Run((*turnGenerator).planLogisticsAutomation, (*turnGenerator).guardAutomaticFuel, (*turnGenerator).enforceAutomaticDeparture, (*turnGenerator).fleetMove, (*turnGenerator).fleetRefuel, (*turnGenerator).resolveLogisticsAutomation, (*turnGenerator).resolveAutomaticInvasions, (*turnGenerator).resolveColonistDrops)
			if f.Cargo.Colonists == 0 {
				break
			}
		}
		require.Zero(t, f.Cargo.Colonists)
		require.Equal(t, p.Num, target.PlayerNum)
		require.Greater(t, target.Cargo.Colonists, 10)
	}
}
