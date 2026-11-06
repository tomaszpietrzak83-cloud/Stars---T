//go:build !wasi && !wasm

package cs

import (
	"compress/gzip"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestCopiedRallyDepartureAudit(t *testing.T) {
	path := os.Getenv("STARS_RALLY_AUDIT")
	if path == "" {
		t.Skip("optional copied rally diagnostics")
	}
	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()
	reader, err := gzip.NewReader(file)
	require.NoError(t, err)
	defer reader.Close()
	var game FullGame
	require.NoError(t, json.NewDecoder(reader).Decode(&game))
	game.TechStore = &StaticTechStore
	game.Rules.SetTechStore(&StaticTechStore)
	game.Universe.setLogger(testLogger)
	require.NoError(t, game.computeSpecs())
	tg := newTurnGenerator(&game)
	tg.ordersPlayerNum = 1
	tg.startPatrolEvaluation()
	p := game.getPlayer(1)

	f := game.getFleet(1, 57)
	require.NotNil(t, f)
	p.Automation.FleetRoles[f.Num] = "patrol"
	point := Vector{244, 374}
	route := planAutomaticRoute(&game.Rules, p, []*Fleet{f}, point, MapObjectTarget{}, automaticRoutePolicy{Fast: true, EconomicalPatrol: true, MaxTurns: 2, AllowThreat: knownWithdrawalRoute(p, f.Position, point)})
	require.NotNil(t, route)
	route = pacedPatrolRoute(&game.Rules, p, f, point, route)
	require.NotNil(t, route)
	applyExpeditionRoute(p, f, route)
	savePatrolJob(f, patrolJob{Leader: 319, Provisional: true, Rally: point, Enemy: 16, Target: 25, RendezvousBy: 2491})
	contact := p.GetFleetIntel(16, 135)
	require.NotNil(t, contact)
	result := p.patrolEvaluator.assess(p, []*Fleet{f}, contact, contact.Position, 1)
	require.False(t, result.Ready, "incidental three-transport encounter is not a useful victory")
	require.True(t, tg.patrolDepartureConfirmed(p, f), "harmless incidental traffic must not stop the gathering leg")
	t.Logf("COPIED_RALLY: D57 from %v to %v, warp %d, incidental 16:135 cannot stop transit: %s", f.Position, point, f.Waypoints[1].WarpSpeed, result.Reason)
}
