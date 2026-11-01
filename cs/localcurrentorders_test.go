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

func TestCurrentArchivedCivilianOrders(t *testing.T) {
	source := os.Getenv("STARS_CURRENT_ORDERS_ARCHIVE")
	if source == "" {
		t.Skip("optional exported-state regression")
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
	p := g.getPlayer(1)
	tg.restoreBorrowedManualFleets()
	for _, f := range g.getFleets(1) {
		if f.Spec.TotalShips >= 40 {
			require.Empty(t, p.Automation.FleetRoles[f.Num])
			t.Logf("Recovered manual fleet %d, %d ships: %v", f.Num, f.Spec.TotalShips, f.Waypoints)
		}
	}
	tg.planPatrolAutomation()
	tg.planLogisticsAutomation()
	tg.resolveLogisticsAutomation()
	tg.dispatchReadyLogistics()
	f := g.getFleet(1, 32)
	require.NotNil(t, f)
	t.Logf("Ironium transport: cargo=%v waypoints=%v status=%s", f.Cargo, f.Waypoints, f.GetTag(logisticsTag))
	job := logisticsWork(f)
	require.NotNil(t, job)
	if f.CargoMass() == 0 {
		require.False(t, tg.deliveryEscortReady(p, f, g.getPlanet(1183).Position), "an idle full-funded pickup must have a real protection shortfall")
	}
}
