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

// A planning-only export of a copied database; never save or advance its year.
func TestNavigation2477OnExportedCopy(t *testing.T) {
	source := os.Getenv("STARS_NAVIGATION_SNAPSHOT")
	if source == "" {
		t.Skip("optional copied navigation snapshot")
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
	tg.ordersPlayerNum = 1
	require.NoError(t, g.computeSpecs())
	tg.enablePlanningThreatIndex()
	defer tg.disablePlanningThreatIndex()
	p := g.getPlayer(1)
	year := g.Year
	oldCover := 0
	for _, f := range g.getFleets(1) {
		if pack := readBombingPack(f); pack != nil && pack.Target == 1115 {
			for _, token := range f.Tokens {
				if p.GetDesign(token.DesignNum).Spec.HasWeapons {
					oldCover += token.Quantity
				}
			}
		}
	}
	tg.releaseCombatCruiseNetworks()
	tg.planScoutAutomation()
	scoutCount := 0
	for _, f := range g.getFleets(1) {
		if f.GetTag(priorityReconTag) == "1115" && len(f.Waypoints) > 1 {
			scoutCount++
			t.Logf("Covenant scout %d route=%v", f.Num, f.Waypoints)
		}
	}
	require.Equal(t, 1, scoutCount, "exactly one scout must refresh the active bombing target")
	tg.finishBombingPacks()
	newCover := 0
	for _, f := range g.getFleets(1) {
		if pack := readBombingPack(f); pack != nil && pack.Target == 1115 {
			for _, token := range f.Tokens {
				if p.GetDesign(token.DesignNum).Spec.HasWeapons {
					newCover += token.Quantity
				}
			}
		}
	}
	require.Greater(t, oldCover, newCover, "Covenant Reach surplus cover must leave the old mixed pack")
	tg.planBomberEscorts()
	tg.planPatrolAutomation()
	tg.guardAutomaticFuel()
	tg.planAutomaticTankers()
	tg.planFuelNetworks()
	tg.planBombingWolfPacks()
	tg.enforceAutomaticDeparture()
	for _, n := range []int{42, 25, 70, 83, 87, 110, 213} {
		f := g.getFleet(1, n)
		require.NotNil(t, f)
		require.Greater(t, len(f.Waypoints), 1, "patrol %d must receive actual movement", n)
	}
	chosen := patrolEnemy(p, readPatrolJob(g.getFleet(1, 83)))
	if chosen != nil && chosen.Num != 54 {
		require.True(t, patrolReportedTransport(p, chosen) && enemyApproachingColony(p, chosen), "only an incoming landing transport may displace the nearby Scout #54 under the newer defense priority")
	}
	for _, n := range []int{42, 25, 70, 83, 87, 110, 213, 181, 53, 92, 173, 176, 174} {
		f := g.getFleet(1, n)
		if f == nil || f.Delete {
			continue
		}
		t.Logf("Fleet %d fuel=%d role=%s waypoints=%v job=%+v", n, f.Fuel, p.Automation.FleetRoles[n], f.Waypoints, readPatrolJob(f))
	}
	require.Equal(t, year, g.Year)
	t.Logf("Covenant Reach old armed cover=%d retained old-pack cover=%d", oldCover, newCover)
}

func TestBomberPackReleasesSurplusDestroyersWithoutDeletingBombers(t *testing.T) {
	u := automaticBomberUniverse(t)
	p, b, e := u.Player(1), u.Fleet("Bomber"), u.Fleet("Cover")
	p.Automation.FleetRoles[e.Num] = "patrol"
	e.Tokens[0].Quantity = 5
	b.Tokens[0].Quantity = 5
	require.NoError(t, u.Game.computeSpecs())
	pack := bombingPack{Target: u.Planet("Enemy").Num, Parts: append(attackPackParts(p, b), attackPackParts(p, e)...)}
	merged, err := NewOrderer().Merge(&u.Game.Rules, p, []*Fleet{b, e})
	require.NoError(t, err)
	for _, f := range []*Fleet{b, e} {
		if f != merged {
			u.Game.deleteFleet(f)
		}
	}
	raw, _ := json.Marshal(pack)
	merged.SetTag(bombingPackTag, string(raw))
	p.Automation.FleetRoles[merged.Num] = "bomber"
	u.Run((*turnGenerator).finishBombingPacks)
	bombers, patrols := 0, 0
	for _, f := range u.Game.getFleets(p.Num) {
		if f.Delete {
			continue
		}
		switch p.Automation.FleetRoles[f.Num] {
		case "bomber":
			bombers += f.Spec.TotalShips
		case "patrol":
			patrols += f.Spec.TotalShips
		}
	}
	require.Equal(t, 5, bombers)
	require.Equal(t, 5, patrols)
	u.Run((*turnGenerator).planBomberEscorts, (*turnGenerator).planBombingWolfPacks)
	for _, f := range u.Game.getFleets(p.Num) {
		if f.Delete || p.Automation.FleetRoles[f.Num] != "bomber" {
			continue
		}
		armed := 0
		for _, token := range f.Tokens {
			if p.GetDesign(token.DesignNum).Spec.HasWeapons {
				armed += token.Quantity
			}
		}
		require.LessOrEqual(t, armed, 1, "a bomber pool must not recruit one escort for each bomber again")
	}
}
