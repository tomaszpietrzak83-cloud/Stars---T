//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPatrolClearsLocalHostileMinesOnlyWhenCapable(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Escort")
	p.Automation.FleetRoles[f.Num] = "patrol"
	p.PlanetIntels = []*Planet{u.Planet("Home")}
	for len(p.Relations) < 2 {
		p.Relations = append(p.Relations, PlayerRelationship{})
	}
	p.Relations[1].Relation = PlayerRelationEnemy
	enemy := NewPlayer(2, NewRace().WithSpec(&u.Game.Rules)).WithNum(2).withSpec(&u.Game.Rules)
	field := newMinefield(enemy, MinefieldTypeStandard, 100, 1, Vector{15, 0})
	p.MinefieldIntels = []*Minefield{field}
	f.Spec.MineSweep = 0
	require.False(t, u.turn.patrolSweepObstacle(p, f))
	f.Spec.MineSweep = 600
	require.True(t, u.turn.patrolSweepObstacle(p, f))
	require.Greater(t, len(f.Waypoints), 1)
	require.Less(t, f.Waypoints[1].Position.DistanceTo(field.Position), field.Radius())
	require.LessOrEqual(t, f.Waypoints[1].WarpSpeed, u.Game.Rules.MinefieldStatsByType[field.MinefieldType].MaxSpeed+p.Race.Spec.MinefieldSafeWarpBonus)
	field.PlayerNum = p.Num
	require.False(t, u.turn.patrolSweepObstacle(p, f))
	field.PlayerNum = enemy.Num
	field.Detonate = true
	require.False(t, u.turn.patrolSweepObstacle(p, f), "patrol must not park inside a detonating field")
}

func TestSweepingPatrolDoesNotWaitOnAFieldAlreadyClearedAtItsPosition(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Escort")
	p.Automation.FleetRoles[f.Num] = "patrol"
	f.Spec.MineSweep = 600
	p.PlanetIntels = []*Planet{u.Planet("Home")}
	p.MinefieldIntels = []*Minefield{{MapObject: MapObject{Num: 99, PlayerNum: 2, Position: f.Position}, MinefieldType: MinefieldTypeStandard, NumMines: 100}}
	require.False(t, u.turn.patrolSweepObstacle(p, f))
	require.Empty(t, p.MinefieldIntels, "a physically surveyed, cleared field must not leave a ghost reservation")
}
