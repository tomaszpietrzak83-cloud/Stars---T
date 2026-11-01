//go:build !wasi && !wasm

package cs

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestArchiveVisibilityMatchesOriginalScanning(t *testing.T) {
	source := os.Getenv("STARS_SCANNER_ARCHIVE")
	if source == "" {
		t.Skip("optional copied archive scanner equivalence")
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
	_ = newTurnGenerator(&g)
	require.NoError(t, g.computeSpecs())
	var oldTime, newTime time.Duration
	for _, p := range g.Players {
		scan := newPlayerScanner(g.Universe, g.Players, &g.Rules, p)
		sensors, cargo := scan.getScanners(), scan.getCargoScanners()
		start := time.Now()
		expected := make([]uint8, len(g.Fleets))
		for i, f := range g.Fleets {
			if f.Delete || f.OwnedBy(p.Num) {
				continue
			}
			for _, s := range sensors {
				if scan.fleetInScannerRange(f, s) {
					expected[i] |= 1
					break
				}
			}
			for _, s := range cargo {
				if scan.fleetInScannerRange(f, s) {
					expected[i] |= 2
					break
				}
			}
		}
		oldTime += time.Since(start)
		start = time.Now()
		got := scan.fleetVisibility(sensors, cargo, scannerWorkers())
		newTime += time.Since(start)
		require.Equal(t, expected, got, "visibility for player %d", p.Num)
	}
	t.Logf("Exact fleet visibility for %d players / %d fleets: original=%s indexed=%s", len(g.Players), len(g.Fleets), oldTime, newTime)
}
