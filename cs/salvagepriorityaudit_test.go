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

func TestDebrisAssignmentOnCopied2444Snapshot(t *testing.T) {
	source := os.Getenv("STARS_SALVAGE_SNAPSHOT")
	if source == "" {
		t.Skip("optional private copied snapshot")
	}
	absolute, err := filepath.Abs(source)
	require.NoError(t, err)
	require.Contains(t, strings.ToLower(filepath.ToSlash(absolute)), "/audit-results/")
	raw, err := os.ReadFile(absolute)
	require.NoError(t, err)
	var game FullGame
	require.NoError(t, json.Unmarshal(raw, &game))
	game.TechStore = &StaticTechStore
	game.Rules.SetTechStore(&StaticTechStore)
	game.Universe.setLogger(testLogger)
	require.NoError(t, game.computeSpecs())
	tg := newTurnGenerator(&game)
	tg.ordersPlayerNum = 1
	year := game.Year
	require.Equal(t, 2444, year)
	p := game.getPlayer(1)
	require.NotNil(t, p.GetSalvageIntel(3))
	tg.planPatrolSalvage()
	collectors := 0
	for _, f := range game.getFleets(1) {
		if job := logisticsWork(f); job != nil && job.Kind == "salvage" && job.TargetNum == 3 {
			collectors++
			require.Empty(t, f.GetTag(anticipatedSalvageTag))
			t.Logf("Known debris #3 assigned to %s (%d); no time advanced", f.Name, f.Num)
		}
	}
	require.Equal(t, 1, collectors)
	require.Equal(t, year, game.Year)
}
