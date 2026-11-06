//go:build !wasi && !wasm

package cs

import (
	"compress/gzip"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestCopiedPatrolArrivalAudit(t *testing.T) {
	path := os.Getenv("STARS_ARRIVAL_AUDIT")
	if path == "" {
		t.Skip("optional private arrival snapshot")
	}
	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()
	z, err := gzip.NewReader(file)
	require.NoError(t, err)
	defer z.Close()
	var g FullGame
	require.NoError(t, json.NewDecoder(z).Decode(&g))
	g.TechStore = &StaticTechStore
	g.Rules.SetTechStore(&StaticTechStore)
	g.Year++
	tg := newTurnGenerator(&g)
	tg.ordersPlayerNum = 1
	require.NoError(t, g.computeSpecs())
	tg.startPatrolEvaluation()
	tg.enablePlanningThreatIndex()
	defer tg.disablePlanningThreatIndex()
	tg.refreshAutomaticFuelIntel()
	tg.releaseUnreachablePatrolSupport()
	p := g.getPlayer(1)
	tg.reconcilePatrolJobs(p)
	leader := g.getFleet(1, 615)
	require.NotNil(t, leader)
	tg.planUrgentPatrolDefence(p)
	tg.guardAutomaticFuel()
	tg.enforceAutomaticDeparture()
	t.Logf("AFTER work=%d ships=%d job=%+v waypoints=%+v", p.patrolEvaluator.Work, leader.Spec.TotalShips, readPatrolJob(leader), leader.Waypoints)
	require.Equal(t, 53, leader.Spec.TotalShips)
	require.False(t, readPatrolJob(leader).Provisional)
	require.True(t, defencePatrolCommitted(p, leader))
	require.Greater(t, len(leader.Waypoints), 1)
	require.Equal(t, Vector{38, 318}, leader.Waypoints[1].Position)
	require.LessOrEqual(t, p.patrolEvaluator.Work, 4096)
}
