//go:build !wasi && !wasm

package cs

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPatrolNativeStrongEnemyMergeKillDisperseCycle(t *testing.T) {
	fleets := []ScenarioFleet{}
	for i := 0; i < 16; i++ {
		fleets = append(fleets, ScenarioFleet{Name: fmt.Sprintf("Hunter %d", i), Design: DesignDestroyerDelta.Name, Position: Vector{i % 4 * 4, i / 4 * 4}})
	}
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
		{Designs: Designs(DesignDestroyerDelta, DesignSpaceStation), Fleets: fleets},
		{Designs: Designs(DesignDestroyerDelta), Fleets: []ScenarioFleet{{Name: "Armed enemy", Design: DesignDestroyerDelta.Name, Quantity: 2, Position: Vector{50, 0}}}},
	}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}}})
	p, e := u.Player(1), u.Fleet("Armed enemy")
	p.Automation.FleetRoles = map[int]string{}
	for _, f := range u.Game.getFleets(1) {
		if !f.Starbase {
			p.Automation.FleetRoles[f.Num] = "patrol"
		}
	}
	p.discoverer.discoverDesign(e.Tokens[0].design, true)
	p.discoverer.discoverFleet(e, true)
	u.Run((*turnGenerator).startPatrolEvaluation)
	require.False(t, p.patrolEvaluator.assess(p, []*Fleet{u.Fleet("Hunter 0")}, p.GetFleetIntel(2, e.Num), e.Position, 1).Ready, "one hunter cannot safely attack this armed force")
	merged, killed, dispersed := false, false, false
	for turn := 0; turn < 20; turn++ {
		u.Game.Year++
		u.Run((*turnGenerator).fleetInit)
		u.Run((*turnGenerator).startPatrolEvaluation, (*turnGenerator).splitAutomaticCombatPools, (*turnGenerator).planPatrolAutomation, (*turnGenerator).planAutomaticSafety)
		// Match the native turn pipeline: relief fleets first arrive, then
		// physically merge at the encounter before any combat damage is dealt.
		u.Run((*turnGenerator).fleetMove, (*turnGenerator).mergePatrolRendezvous, (*turnGenerator).mergeCompatibleCombatFleets)
		for _, f := range u.Game.getFleets(1) {
			if !f.Delete && !f.Starbase && f.Spec.TotalShips > 1 {
				merged = true
			}
		}
		u.Run((*turnGenerator).fleetBattle, (*turnGenerator).fleetRepair)
		for _, b := range p.BattleRecords {
			if b.Stats.ShipsDestroyedByPlayer[2] >= 2 {
				killed = true
			}
		}
		if e.Delete {
			p.FleetIntels = nil
		}
		if killed {
			u.Run((*turnGenerator).startPatrolEvaluation, (*turnGenerator).splitAutomaticCombatPools, (*turnGenerator).planPatrolAutomation, (*turnGenerator).planAutomaticSafety)
			dispersed = true
			for _, f := range u.Game.getFleets(1) {
				if !f.Delete && !f.Starbase && f.Spec.TotalShips > 1 {
					dispersed = false
				}
			}
			if dispersed {
				break
			}
		}
	}
	require.True(t, merged, "native merge before the armed encounter")
	require.True(t, killed, "real battle destroys the armed target, not merely a favorable forecast")
	for _, f := range u.Game.getFleets(1) {
		if !f.Delete && !f.Starbase && f.Spec.TotalShips > 1 {
			t.Logf("retained %s ships=%d damage=%d fuel=%d job=%+v tags=%v", f.Name, f.Spec.TotalShips, fleetDamagePercent(f), f.Fuel, readPatrolJob(f), f.Tags)
		}
	}
	require.True(t, dispersed, "survivors return to individual hunters after the target is gone")
}

func TestPatrolBlockedDepartureReleasesMissionNextTurn(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Escort")
	p.Automation.FleetRoles[f.Num] = "patrol"
	enemy := patrolTestEnemy(0, p)
	p.FleetIntels = []*Fleet{enemy}
	p.PlanetIntels = append(p.PlanetIntels, &Planet{MapObject: MapObject{Num: 500, Position: Vector{-60, 0}, ReportAge: ReportAgeUnexplored}})
	savePatrolJob(f, patrolJob{Leader: f.Num, Enemy: enemy.PlayerNum, Target: enemy.Num})
	u.Run(func(tg *turnGenerator) { tg.rememberBlockedPatrolDeparture(p, f) })
	u.Game.Year++
	u.Run(func(tg *turnGenerator) { tg.advancePatrol(p, f) })
	require.Zero(t, readPatrolJob(f).Leader, "a rejected attack cannot retain the patrol for another idle year")
	require.True(t, declinedPatrolTarget(f, enemy, u.Game.Year))
	u.Run((*turnGenerator).planPatrolAutomation)
	require.NotEqual(t, enemy.Num, readPatrolJob(f).Target)
	require.Greater(t, len(f.Waypoints), 1, "choose another funded patrol route")
}

func TestSmallCollectorActuallyPicksUpAndDeliversTwoDebrisFields(t *testing.T) {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{
		Designs:  Designs(DesignSmallFreighter, DesignTeamster, DesignSpaceStation),
		Fleets:   []ScenarioFleet{{Name: "Small collector", Design: DesignSmallFreighter.Name, At: "Home"}, {Name: "Large collector", Design: DesignTeamster.Name, At: "Home"}},
		Salvages: []Salvage{{MapObject: MapObject{Position: Vector{25, 0}}, Cargo: Cargo{Ironium: 12}}, {MapObject: MapObject{Position: Vector{-25, 0}}, Cargo: Cargo{Germanium: 8}}},
	}}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}}})
	p, small, large := u.Player(1), u.Fleet("Small collector"), u.Fleet("Large collector")
	p.Automation.FleetRoles = map[int]string{small.Num: "freighter", large.Num: "freighter"}
	p.Automation.DisablePassengerTransport = true
	p.Automation.DisableMineralSupply = true
	p.SalvageIntels = u.Game.Salvages
	loaded := false
	for step := 0; step < 12; step++ {
		u.Run((*turnGenerator).fleetInit, (*turnGenerator).planPatrolSalvage, (*turnGenerator).planLogisticsAutomation, (*turnGenerator).resolveLogisticsAutomation, (*turnGenerator).fleetMove, (*turnGenerator).resolveLogisticsAutomation)
		loaded = loaded || small.Cargo.ToMineral().Total() > 0
		if j := logisticsWork(large); j != nil {
			require.NotEqual(t, "salvage", j.Kind)
		}
	}
	require.True(t, loaded, "minerals really enter the small collector's hold")
	require.Equal(t, 12, u.Planet("Home").Cargo.Ironium)
	require.Equal(t, 8, u.Planet("Home").Cargo.Germanium)
	for _, s := range u.Game.Salvages {
		require.True(t, s.Delete)
	}
}
