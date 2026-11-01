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

func TestArchivedPetraEightDestroyersKeepOneYearDeadline(t *testing.T) {
	source := os.Getenv("STARS_PRODUCTION_AUDIT")
	if source == "" {
		t.Skip("optional copied year-2470 production regression")
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
	p, planet := g.getPlayer(1), g.getPlanet(1430)
	require.Equal(t, "Petra", planet.Name)
	d := p.GetDesign(22)
	require.NotNil(t, d)
	batch := ProductionQueueItem{Type: QueueItemTypeShipToken, DesignNum: d.Num, Quantity: 8, design: d}
	base := []ProductionQueueItem{batch}
	planet.ProductionQueue = base
	planet.Spec = ComputePlanetSpec(&g.Rules, p, planet)
	estimate := func() int {
		items, _, err := NewCompletionEstimator().GetProductionWithEstimates(&g.Rules, p, *planet)
		require.NoError(t, err)
		for _, item := range items {
			if manufacturingItem(item) {
				return item.YearsToBuildAll
			}
		}
		return Infinite
	}
	require.Equal(t, 1, estimate())
	planet.ProductionQueue = []ProductionQueueItem{{Type: QueueItemTypeAutoFactories, Quantity: 70}, {Type: QueueItemTypeAutoMines, Quantity: 72}, batch}
	PlanProductionBoost(&g.Rules, p, planet)
	require.Equal(t, 1, estimate(), "the reported 1-to-6-year regression must not return")
	require.True(t, displayedManufacturingNoLater(&g.Rules, p, planet, planet.ProductionQueue, base))
	t.Logf("Copied Petra: 8 destroyers retain ETA %d year; queue %v", estimate(), planet.ProductionQueue)
}
