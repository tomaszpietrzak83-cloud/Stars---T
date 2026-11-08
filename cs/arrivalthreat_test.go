//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestArrivalThreatAllowsCrossingButRejectsMeetingForEveryRole(t *testing.T) {
	for _, role := range []string{"freighter", "patrol", "scout", "bomber", "tanker", "colonizer"} {
		t.Run(role, func(t *testing.T) {
			u := logisticsUniverse(t)
			p, f := u.Player(1), u.Fleet("Escort")
			p.Automation.FleetRoles[f.Num] = role
			f.Position = Vector{}
			f.Waypoints = []Waypoint{{Position: f.Position}, {Position: Vector{100, 0}, WarpSpeed: 10}}
			e := patrolTestEnemy(100, p)
			e.Position, e.Heading, e.WarpSpeed = Vector{50, -50}, VectorFloat64{Y: 1}, 10
			p.FleetIntels = []*Fleet{e}
			require.Zero(t, interceptionThreat(p, f), "mid-turn crossing is not a battle")
			require.True(t, automaticLegSafe(&u.Game.Rules, p, Vector{}, Vector{100, 0}, 10, false))
			e.Position = Vector{100, -100}
			require.Positive(t, interceptionThreat(p, f), "a predicted meeting in deep space still matters")
			require.False(t, automaticLegSafe(&u.Game.Rules, p, Vector{}, Vector{100, 0}, 10, false))
		})
	}
}

func TestArrivalThreatChecksAnnualStopAndAllowsPassingStationaryEnemy(t *testing.T) {
	u := logisticsUniverse(t)
	p := u.Player(1)
	e := patrolTestEnemy(100, p)
	e.Position = Vector{25, 0}
	p.FleetIntels = []*Fleet{e}
	require.True(t, automaticLegSafe(&u.Game.Rules, p, Vector{}, Vector{100, 0}, 10, false), "passing the enemy is allowed")
	require.False(t, automaticLegSafe(&u.Game.Rules, p, Vector{}, Vector{100, 0}, 5, false), "the first annual stop is occupied")
	require.True(t, automaticLegSafe(&u.Game.Rules, p, Vector{}, Vector{100, 0}, 5, true), "an assessed attack may allow contact")
	require.Equal(t, Vector{16, 5}, automaticTurnEnd(Vector{}, Vector{16, 5}, 16), "native movement snaps the last fraction of a light year")
	require.Equal(t, Vector{}, automaticTurnEnd(Vector{}, Vector{100, 0}, 0))
}

func TestArrivalPolicyMatchesNativeCrossingAndDeepSpaceBattle(t *testing.T) {
	for _, meeting := range []bool{false, true} {
		u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
			{Designs: Designs(DesignDestroyerDelta), Fleets: []ScenarioFleet{{Name: "Ours", Design: DesignDestroyerDelta.Name}}},
			{Designs: Designs(DesignDestroyerDelta), Fleets: []ScenarioFleet{{Name: "Enemy", Design: DesignDestroyerDelta.Name, Position: Vector{12, -12}}}},
		}})
		f, e := u.Fleet("Ours"), u.Fleet("Enemy")
		destination := Vector{12, 13}
		if meeting {
			e.Position, destination = Vector{25, -25}, Vector{25, 0}
		}
		f.Waypoints = []Waypoint{{Position: f.Position}, {Position: Vector{25, 0}, WarpSpeed: 5}}
		e.Waypoints = []Waypoint{{Position: e.Position}, {Position: destination, WarpSpeed: 5}}
		u.Run((*turnGenerator).fleetMove)
		require.Equal(t, Vector{25, 0}, f.Position)
		require.Equal(t, destination, e.Position)
		u.Run((*turnGenerator).fleetBattle)
		if meeting {
			require.Len(t, u.Player(1).BattleRecords, 1)
		} else {
			require.Empty(t, u.Player(1).BattleRecords)
		}
	}
}
