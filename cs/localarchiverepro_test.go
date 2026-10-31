//go:build !wasi && !wasm

package cs

import (
	"compress/gzip"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Optional diagnosis on an exported analysis archive copied into audit-results.
// No database is opened and no turn is generated or written back.
func TestLocalArchivedAutomationOrders(t *testing.T) {
	path := os.Getenv("STARS_AUTOMATION_ARCHIVE")
	if path == "" {
		t.Skip("optional copied analysis-archive regression")
	}
	abs, err := filepath.Abs(path)
	require.NoError(t, err)
	require.Contains(t, strings.ToLower(filepath.ToSlash(abs)), "/audit-results/")
	source, err := os.Open(abs)
	require.NoError(t, err)
	defer source.Close()
	zipped, err := gzip.NewReader(source)
	require.NoError(t, err)
	defer zipped.Close()
	var archive struct{ Game *FullGame }
	require.NoError(t, json.NewDecoder(zipped).Decode(&archive))
	g := archive.Game
	require.NotNil(t, g)
	g.TechStore = &StaticTechStore
	g.Rules.SetTechStore(&StaticTechStore)
	tg := newTurnGenerator(g)
	require.NoError(t, g.computeSpecs())
	p := g.getPlayer(1)
	year := g.Year
	repaired := 0
	for _, f := range g.getFleets(1) {
		if p.Automation.FleetRoles[f.Num] == "scout" && !scoutReconEligible(p, f) && readScoutRecon(f) != nil {
			repaired++
		}
	}
	tg.repairStaleScoutRoles()
	for _, f := range g.getFleets(1) {
		if p.Automation.FleetRoles[f.Num] == "freighter" {
			require.False(t, scoutReconActive(p, f))
			for _, wp := range f.Waypoints[1:] {
				require.False(t, wp.TargetType == MapObjectTypeFleet && wp.TargetPlayerNum != 1 && !p.IsFriend(wp.TargetPlayerNum))
			}
		}
	}
	before := len(g.getFleets(1))
	tg.mergePatrolRendezvous()
	merged := 0
	for _, f := range g.getFleets(1) {
		if !f.Delete && readPatrolJob(f).Grouped && f.Spec.TotalShips > 1 {
			merged++
			t.Logf("combat stack %d: %d ships at %v", f.Num, f.Spec.TotalShips, f.Position)
		}
	}
	t.Logf("year %d: %d invalid reconnaissance fleets recovered; %d merged combat stacks from %d source fleets", year, repaired, merged, before)
	require.Positive(t, repaired)
	require.Positive(t, merged)
	require.Equal(t, year, g.Year)
}
