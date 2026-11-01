//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPatrolSwitchesDistantRecedingChaseForPassingTransport(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Escort")
	p.Automation.FleetRoles[f.Num] = "patrol"
	addExpeditionTestDock(p, Vector{})
	distant := patrolTestEnemy(0)
	distant.Num = 90
	distant.Position = Vector{130, 0}
	distant.WarpSpeed = 6
	distant.Heading = VectorFloat64{X: 1}
	nearby := patrolTestEnemy(0)
	nearby.Num = 213
	nearby.Name = "Super-Fuel Xport"
	nearby.Position = Vector{10, 0}
	p.FleetIntels = []*Fleet{distant, nearby}
	savePatrolJob(f, patrolJob{Leader: f.Num, Enemy: 2, Target: distant.Num})
	u.Run(func(tg *turnGenerator) { tg.retargetPatrolOpportunity(p, f) })
	require.Equal(t, nearby.Num, readPatrolJob(f).Target)
	u.Run((*turnGenerator).planPatrolAutomation)
	require.Greater(t, len(f.Waypoints), 1)
	require.Equal(t, nearby.Position, f.Waypoints[1].Position)
}
