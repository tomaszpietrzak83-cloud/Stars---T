//go:build !wasi && !wasm

package cs

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBoundedLandingCopiedCampaign(t *testing.T) {
	path := os.Getenv("STARS_BOUNDED_LANDING_COPY")
	if path == "" {
		t.Skip("optional private campaign copy")
	}
	abs, err := filepath.Abs(path)
	require.NoError(t, err)
	require.Contains(t, strings.ToLower(filepath.ToSlash(abs)), "/audit-results/")
	f, err := os.Open(abs)
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
	p := g.getPlayer(1)
	totals := func() (people, fuel, ships, reserved int) {
		for _, f := range g.getFleets(1) {
			if f.Delete {
				continue
			}
			people += f.Cargo.Colonists
			fuel += f.Fuel
			ships += f.Spec.TotalShips
			if landingManaged(f) {
				reserved += f.Cargo.Colonists
			}
		}
		return
	}
	people, fuel, ships, reserved := totals()
	tg.releaseBlockedLandings()
	afterPeople, afterFuel, afterShips, afterReserved := totals()
	require.Equal(t, people, afterPeople)
	require.Equal(t, fuel, afterFuel)
	require.Equal(t, ships, afterShips)
	require.Less(t, afterReserved, reserved)
	t.Logf("RELEASE beforePeople=%d afterPeople=%d civilianPeople=%d preservedShips=%d preservedFuel=%d", reserved*100, afterReserved*100, (reserved-afterReserved)*100, ships, fuel)
	tg.planLogisticsAutomation()
	movingCivilian, ops := 0, 0
	for _, f := range g.getFleets(1) {
		if f.Delete {
			continue
		}
		if op := readLandingOperation(f); op != nil {
			ops++
			require.LessOrEqual(t, tg.landingReservedTroops(p, f), landingArmyLimit)
			require.True(t, tg.landingPrepared(p, p.GetPlanetIntel(op.Target)))
		}
		if !landingManaged(f) && f.Cargo.Colonists > 0 && len(f.Waypoints) > 1 && logisticsWork(f) != nil {
			movingCivilian += f.Cargo.Colonists
		}
	}
	require.Positive(t, movingCivilian)
	t.Logf("PLANNED boundedOperations=%d civilianPeopleWithRoutes=%d", ops, movingCivilian*100)
}
