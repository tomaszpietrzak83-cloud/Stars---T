//go:build !wasi && !wasm

package cs

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRebuildWaitingTransportsOnCopiedSnapshot(t *testing.T) {
	source := os.Getenv("STARS_REBUILD_WAIT_SNAPSHOT")
	if source == "" {
		t.Skip("optional copied-game snapshot")
	}
	absolute, err := filepath.Abs(source)
	require.NoError(t, err)
	require.Contains(t, strings.ToLower(filepath.ToSlash(absolute)), "/audit-results/")
	raw, err := os.ReadFile(absolute)
	require.NoError(t, err)
	var g FullGame
	require.NoError(t, json.Unmarshal(raw, &g))
	g.TechStore = &StaticTechStore
	g.Rules.SetTechStore(&StaticTechStore)
	g.Universe.setLogger(testLogger)
	require.NoError(t, g.computeSpecs())
	tg := newTurnGenerator(&g)
	tg.ordersPlayerNum = 1
	p := g.getPlayer(1)
	year := g.Year
	waiting := []*Fleet{}
	for _, num := range []int{20, 53, 54, 55, 56, 57, 59} {
		f := g.getFleet(1, num)
		if f != nil {
			require.True(t, tg.maintenanceNeeded(p, f))
			require.LessOrEqual(t, len(f.Waypoints), 1)
			require.Zero(t, f.CargoMass())
			waiting = append(waiting, f)
		}
	}
	require.NotEmpty(t, waiting)
	tg.fleetDesignMaintenance()
	for _, f := range waiting {
		work := readMaintenance(f)
		require.NotNil(t, work)
		require.Equal(t, "scrap", work.Stage)
		require.Equal(t, 1901, work.PlanetNum)
	}
	for _, owner := range g.Players {
		if owner.Stats == nil {
			owner.Stats = &PlayerStats{}
		}
	}
	require.NoError(t, tg.planetProduction())
	for _, f := range waiting {
		require.True(t, f.Delete, "arrived old ship must actually scrap during production")
	}
	require.GreaterOrEqual(t, tg.maintenanceReport(p).built[7], len(waiting))
	tg.finishReplacementRallies()
	tg.reportShipMaintenance()
	require.Len(t, maintenanceMessages(p), 1)
	t.Logf("%d arrived freighters scrapped; replacement construction and one consolidated summary verified", len(waiting))
	require.Equal(t, year, g.Year)
}
