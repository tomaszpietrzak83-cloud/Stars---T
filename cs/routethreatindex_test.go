//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"math/rand"
	"testing"
)

func TestPlanningThreatIndexPreservesExactThreatValues(t *testing.T) {
	u := logisticsUniverse(t)
	p := u.Player(1)
	rng := rand.New(rand.NewSource(44))
	for n := 1; n <= 200; n++ {
		f := patrolTestEnemy(n % 3 * 100)
		f.Num = n
		f.Position = Vector{rng.Intn(300), rng.Intn(300)}
		f.ReportAge = n % 2
		p.FleetIntels = append(p.FleetIntels, f)
	}
	u.Run((*turnGenerator).enablePlanningThreatIndex)
	for _, margin := range []int{0, 20, 80, 150} {
		p.Automation.EscortMarginLY = margin
		for n := 0; n < 300; n++ {
			from, to := Vector{rng.Intn(300), rng.Intn(300)}, Vector{rng.Intn(300), rng.Intn(300)}
			require.Equal(t, originalKnownRouteThreat(p, from, to), knownRouteThreat(p, from, to))
		}
	}
	u.Run((*turnGenerator).disablePlanningThreatIndex)
	require.Nil(t, p.routeThreatIndex)
}

func TestStandaloneRouteIndexIsScopedAndWithdrawalMatchesReference(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Escort")
	p.Automation.FleetRoles[f.Num] = "patrol"
	route := planAutomaticRoute(&u.Game.Rules, p, []*Fleet{f}, Vector{10, 0}, MapObjectTarget{}, automaticRoutePolicy{})
	require.NotNil(t, route)
	require.Nil(t, p.routeThreatIndex)
	p.FleetIntels = []*Fleet{patrolTestEnemy(100)}
	for _, to := range []Vector{{-40, 0}, {40, 0}, {0, 40}, {0, -40}} {
		expected := knownWithdrawalRoute(p, Vector{}, to)
		cleanup := scopedRouteThreatIndex(p)
		_ = knownRouteThreat(p, Vector{}, to)
		require.Equal(t, expected, knownWithdrawalRoute(p, Vector{}, to))
		cleanup()
		require.Nil(t, p.routeThreatIndex)
	}
	p.FleetIntels[0].Position = Vector{1000, 1000}
	require.Zero(t, knownRouteThreat(p, Vector{}, Vector{40, 0}), "a completed route search must not freeze an old report position")
}
