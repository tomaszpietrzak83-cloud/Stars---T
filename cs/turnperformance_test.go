//go:build !wasi && !wasm

package cs

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Diagnostic only: archives omit AI reports, so this is not an exact replay.
// All simulation mutations remain in memory, never in a personal database.
func TestCopiedTurnPerformance(t *testing.T) {
	path := os.Getenv("STARS_TURN_PROFILE_ARCHIVE")
	if path == "" {
		t.Skip("optional copied archive profile")
	}
	absolute, err := filepath.Abs(path)
	require.NoError(t, err)
	require.Contains(t, strings.ToLower(filepath.ToSlash(absolute)), "/audit-results/")
	f, err := os.Open(absolute)
	require.NoError(t, err)
	defer f.Close()
	z, err := gzip.NewReader(f)
	require.NoError(t, err)
	defer z.Close()
	var wrapper struct{ Game *FullGame }
	require.NoError(t, json.NewDecoder(z).Decode(&wrapper))
	g := wrapper.Game
	require.NotNil(t, g)
	g.TechStore = &StaticTechStore
	g.Rules.SetTechStore(&StaticTechStore)
	g.Rules.ResetSeed(42)
	for _, p := range g.Players {
		if len(p.ScoreIntels) == 0 {
			p.ScoreIntels = make([]ScoreIntel, len(g.Players))
		}
		if len(p.PlayerIntels) == 0 {
			p.PlayerIntels = p.defaultPlayerIntels(g.Players)
		}
		if len(p.PlanetIntels) == 0 {
			require.NoError(t, p.initDefaultPlanetIntels(g.Planets))
		}
	}
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer slog.SetDefault(old)
	profile, err := os.Create(filepath.Join(filepath.Dir(absolute), "simulation.cpu"))
	require.NoError(t, err)
	defer profile.Close()
	require.NoError(t, pprof.StartCPUProfile(profile))
	defer pprof.StopCPUProfile()
	timer := time.AfterFunc(45*time.Second, pprof.StopCPUProfile)
	defer timer.Stop()
	year := g.Year
	started := time.Now()
	require.NoError(t, NewGamer().GenerateTurn(g.Game, g.Universe, g.Players))
	t.Logf("Simulation: %s, players=%d fleets=%d planets=%d", time.Since(started), len(g.Players), len(g.Fleets), len(g.Planets))
	require.Equal(t, year+1, g.Year)
}
