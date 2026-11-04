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

// Read-only exported archive; diagnostic planning never advances or saves a turn.
func TestCurrentPriorityBombersAndManufacturing(t *testing.T) {
	source := os.Getenv("STARS_PRIORITY_AUDIT")
	if source == "" {
		t.Skip("optional copied-state audit")
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
	tg := newTurnGenerator(&g)
	require.NoError(t, g.computeSpecs())
	tg.enablePlanningThreatIndex()
	p := g.getPlayer(1)
	year := g.Year
	for _, planet := range p.PlanetIntels {
		if planet != nil && p.Automation.PriorityPlanets[planet.Num] {
			t.Logf("Priority %s owner=%d pop=%d defenses=%d base=%v bombing=%v quality=%d", planet.Name, planet.PlayerNum, planet.Cargo.Colonists, planet.Defenses, planet.Spec.HasStarbase, priorityBombingTarget(&g.Rules, p, g.getFleet(1, 21)) == planet, prospectivePlanetQuality(p, planet))
		}
	}
	for _, f := range g.getFleets(1) {
		if p.Automation.FleetRoles[f.Num] != "bomber" {
			continue
		}
		target := tg.automaticBombingTarget(p, f)
		t.Logf("Bomber %d: safety=%s chase=%d target=%v", f.Num, f.GetTag(safetyTag), bomberChaseThreat(p, f), target)
		for _, enemy := range p.FleetIntels {
			if enemy != nil && enemy.ReportAge == 0 && enemy.PlayerNum != p.Num && patrolReportedCombatant(p, enemy) && f.Position.DistanceTo(enemy.Position) < 210 {
				t.Logf("Nearby armed report %d/%d %s at %v warp=%d heading=%v power=%d bomber=%v", enemy.PlayerNum, enemy.Num, enemy.Name, enemy.Position, enemy.WarpSpeed, enemy.Heading, patrolReportedPower(p, enemy), patrolReportedBomber(p, enemy))
			}
		}
	}
	tg.resumeCompletedEvades()
	tg.splitAutomaticCombatPools()
	tg.planBomberEscorts()
	tg.planPatrolAutomation()
	tg.planBombingWolfPacks()
	tg.planAutomaticSafety()
	for _, f := range g.getFleets(1) {
		if p.Automation.FleetRoles[f.Num] == "bomber" {
			t.Logf("Result bomber %d: safety=%s mission=%s route=%v", f.Num, f.GetTag(safetyTag), f.GetTag(bomberTag), f.Waypoints)
		}
	}
	require.Equal(t, year, g.Year)
}
