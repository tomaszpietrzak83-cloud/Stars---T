//go:build !wasi && !wasm

package cs

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"log/slog"
	"os"
	"slices"
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
	if os.Getenv("STARS_DEFENCE_RALLY") == "1" {
		p := g.getPlayer(1)
		leader := g.getFleet(1, 475)
		job := readPatrolJob(leader)
		group := []*Fleet{leader}
		for _, f := range g.getFleets(1) {
			if f != leader && readPatrolJob(f).Leader == leader.Num {
				group = append(group, f)
			}
		}
		policies := make([]automaticRoutePolicy, len(group))
		for i := range policies {
			policies[i] = automaticRoutePolicy{Fast: true, MaxTurns: 20}
		}
		chosen := choosePatrolRally(&g.Rules, p, group, patrolInterceptPosition(patrolEnemy(p, job)), []Vector{job.Rally}, policies, true)
		t.Logf("RALLY threat=%d chosen=%+v alreadyHere=%d remaining=%d", knownRouteThreat(p, job.Rally, job.Rally), chosen, len(slices.DeleteFunc(slices.Clone(group), func(f *Fleet) bool { return f.Position != job.Rally })), job.RendezvousBy-g.Year)
		return
	}
	if os.Getenv("STARS_DEFENCE_COMPARE") == "1" {
		p := g.getPlayer(1)
		group := []*Fleet{}
		for _, f := range g.getFleets(1) {
			j := readPatrolJob(f)
			if j.Enemy == 16 && j.Target == 41 {
				group = append(group, f)
			}
		}
		enemy := p.GetFleetIntel(16, 41)
		e := &patrolEvaluator{rules: &g.Rules, cache: map[[32]byte]patrolAssessment{}, Limit: 4096}
		t.Logf("WITH ARRIVALS %+v", e.assess(p, group, enemy, enemy.Position, 1))
		p.FleetIntels = slices.DeleteFunc(p.FleetIntels, func(f *Fleet) bool { return f.PlayerNum == 16 && f.Num == 67 })
		t.Logf("WITHOUT ARRIVAL 67 %+v", e.assess(p, group, enemy, enemy.Position, 1))
		return
	}
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
