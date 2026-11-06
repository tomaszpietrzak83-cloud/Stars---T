//go:build !wasi && !wasm

package cs

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestCopiedPatrolDefencePlanning(t *testing.T) {
	path := os.Getenv("STARS_DEFENCE_AUDIT")
	if path == "" {
		t.Skip("private audit")
	}
	require.True(t, strings.Contains(strings.ReplaceAll(path, "\\", "/"), "/audit-results/"))
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	var g FullGame
	require.NoError(t, json.Unmarshal(raw, &g))
	g.TechStore = &StaticTechStore
	g.Rules.SetTechStore(&StaticTechStore)
	if os.Getenv("STARS_DEFENCE_ADVANCE") == "1" {
		g.Year++
	}
	tg := newTurnGenerator(&g)
	require.NoError(t, g.computeSpecs())
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(old)
	tg.startPatrolEvaluation()
	tg.fleetDesignMaintenance()
	tg.splitAutomaticCombatPools()
	if os.Getenv("STARS_DEFENCE_ONLY") != "1" {
		tg.planPatrolAutomation()
	}
	p := g.getPlayer(1)
	t.Logf("WORK %d DEFERRED %v", p.patrolEvaluator.Work, p.patrolEvaluator.Deferred)
	ships := 0
	for _, f := range g.getFleets(1) {
		j := readPatrolJob(f)
		if !f.Delete && j.Enemy == 16 && j.Target == 41 {
			ships += f.Spec.TotalShips
			t.Logf("DEFENCE fleet=%d ships=%d pos=%v job=%+v", f.Num, f.Spec.TotalShips, f.Position, j)
		}
	}
	require.Positive(t, ships, "the copied state must produce an actual defence assignment")
}
