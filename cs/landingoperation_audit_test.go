//go:build !wasi && !wasm

package cs

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLandingPlannerOnCopiedSnapshot(t *testing.T) {
	path := os.Getenv("STARS_LANDING_SNAPSHOT")
	if path == "" {
		t.Skip("optional private copied snapshot")
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
	var g FullGame
	require.NoError(t, json.NewDecoder(z).Decode(&g))
	g.TechStore = &StaticTechStore
	g.Rules.SetTechStore(&StaticTechStore)
	tg := newTurnGenerator(&g)
	tg.ordersPlayerNum = 1
	require.NoError(t, g.computeSpecs())
	tg.enablePlanningThreatIndex()
	defer tg.disablePlanningThreatIndex()
	tg.refreshAutomaticFuelIntel()
	tg.resumeCompletedEvades()
	tg.migrateLegacyLandings()
	var collector *Fleet
	var collectorOrders []byte
	if value := os.Getenv("STARS_LANDING_COLLECTOR"); value != "" {
		num, err := strconv.Atoi(value)
		require.NoError(t, err)
		collector = g.getFleet(1, num)
		require.NotNil(t, collector)
		require.Zero(t, collector.CargoMass())
		require.NotNil(t, logisticsWork(collector))
		require.Equal(t, "salvage", logisticsWork(collector).Kind)
		collectorOrders, err = json.Marshal(collector.FleetOrders)
		require.NoError(t, err)
	}
	before := 0
	for _, f := range g.getFleets(1) {
		before += f.Cargo.Colonists
	}
	start := time.Now()
	t.Log("starting landing planner")
	tg.planLandingOperations()
	if collector != nil {
		require.False(t, landingManaged(collector), "a real empty collector must keep its pickup")
		require.NotNil(t, logisticsWork(collector))
		require.Equal(t, "salvage", logisticsWork(collector).Kind)
		orders, err := json.Marshal(collector.FleetOrders)
		require.NoError(t, err)
		require.JSONEq(t, string(collectorOrders), string(orders))
		t.Logf("COLLECTOR preserved fleet=%d target=%d", collector.Num, logisticsWork(collector).TargetNum)
	}
	after, ops, members := 0, 0, 0
	for _, f := range g.getFleets(1) {
		after += f.Cargo.Colonists
		if readLandingOperation(f) != nil {
			ops++
		}
		if landingManaged(f) {
			members++
		}
	}
	t.Logf("PLANNER elapsed=%s operations=%d members=%d newlyLoadedPeople=%d", time.Since(start), ops, members, 100*(after-before))
	require.Positive(t, ops, "copied galaxy should prepare landing armies")
}
