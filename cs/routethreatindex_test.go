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
