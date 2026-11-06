package server

import (
	"crypto/sha256"
	"encoding/json"
	"github.com/sirgwain/craig-stars/ai"
	"github.com/sirgwain/craig-stars/config"
	"github.com/sirgwain/craig-stars/cs"
	"github.com/sirgwain/craig-stars/db"
	"github.com/stretchr/testify/require"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestParallelAIEquivalentOnCopiedGame(t *testing.T) {
	source := os.Getenv("STARS_AI_COMPARE_DB")
	if source == "" {
		t.Skip("optional copied-save AI equivalence")
	}
	absolute, err := filepath.Abs(source)
	require.NoError(t, err)
	require.Contains(t, strings.ToLower(filepath.ToSlash(absolute)), "/audit-results/")
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer slog.SetDefault(old)
	cfg := config.Config{}
	cfg.Database.Filename = absolute
	cfg.Database.SkipUpgrade = true
	conn := db.NewConn()
	require.NoError(t, conn.Connect(t.Context(), &cfg))
	defer conn.Close()
	runner := NewGameRunner(conn, cfg).(*gameRunner)
	run := func(workers int) ([32]byte, time.Duration) {
		g, err := conn.NewReadClient().GetFullGame(t.Context(), 10)
		require.NoError(t, err)
		require.NoError(t, cs.NewGamer().ComputeSpecs(g))
		start := time.Now()
		require.NoError(t, runner.processAITurnsWorkers(g, workers))
		elapsed := time.Since(start)
		// Timestamps on new designs are bookkeeping, not AI decisions.
		raw, err := json.Marshal(g)
		require.NoError(t, err)
		var value any
		require.NoError(t, json.Unmarshal(raw, &value))
		var clean func(any)
		clean = func(v any) {
			switch x := v.(type) {
			case map[string]any:
				delete(x, "createdAt")
				delete(x, "updatedAt")
				for _, c := range x {
					clean(c)
				}
			case []any:
				for _, c := range x {
					clean(c)
				}
			}
		}
		clean(value)
		raw, err = json.Marshal(value)
		require.NoError(t, err)
		return sha256.Sum256(raw), elapsed
	}
	a, serial := run(1)
	runtime.GC()
	b, parallel := run(runtime.GOMAXPROCS(0))
	require.Equal(t, a, b, "all gameplay objects, player orders and reports must match")
	t.Logf("AI planning: serial=%s parallel=%s workers=%d; identical game digest", serial, parallel, runtime.GOMAXPROCS(0))
}

func TestParallelAIFixtureKeepsOrdersDeterministic(t *testing.T) {
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer slog.SetDefault(old)
	conn, cfg := connectSubmitTestDB(t)
	runner := NewGameRunner(conn, cfg).(*gameRunner)
	user, err := conn.NewReadWriteClient().CreateUser(t.Context(), cs.NewUser("parallel-ai-fixture", "", "", cs.RoleUser))
	require.NoError(t, err)
	settings := cs.NewGameSettings().WithHost(cs.Humanoids())
	for i := 0; i < 8; i++ {
		settings = settings.WithAIPlayerRace(ai.Races[i], cs.AIDifficultyNormal, i%4)
	}
	original, err := runner.HostGame(user.ID, settings)
	require.NoError(t, err)
	raw, err := json.Marshal(original)
	require.NoError(t, err)
	run := func(workers int) []byte {
		universe := cs.NewUniverse(slog.Default(), nil)
		g := cs.FullGame{Universe: &universe}
		require.NoError(t, json.Unmarshal(raw, &g))
		g.TechStore = &cs.StaticTechStore
		g.Rules.SetTechStore(&cs.StaticTechStore)
		require.NoError(t, cs.NewGamer().ComputeSpecs(&g))
		require.NoError(t, runner.processAITurnsWorkers(&g, workers))
		out := []any{}
		for _, p := range g.Players {
			if p.AIControlled {
				require.True(t, p.SubmittedTurn)
				out = append(out, p.PlayerOrders)
			}
		}
		for _, f := range g.Fleets {
			out = append(out, f.FleetOrders)
		}
		for _, planet := range g.Planets {
			out = append(out, planet.ProductionQueue)
		}
		encoded, err := json.Marshal(out)
		require.NoError(t, err)
		return encoded
	}
	require.JSONEq(t, string(run(1)), string(run(8)))
	original.TechStore = nil
	require.Panics(t, func() { runner.processAITurnsWorkers(original, 8) }, "worker failures must propagate to the parent turn handler instead of terminating the server")
}
