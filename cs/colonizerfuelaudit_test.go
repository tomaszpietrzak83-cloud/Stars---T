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

func TestArchivedColonizerReleasesOptionalStationaryTanker(t *testing.T) {
	source := os.Getenv("STARS_COLONIZER_FUEL_ARCHIVE")
	if source == "" {
		t.Skip("optional copied year 2475 fuel regression")
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
	tg.refreshAutomaticFuelIntel()
	p, f, donor := g.getPlayer(1), g.getFleet(1, 54), g.getFleet(1, 62)
	require.NotNil(t, readFuelNetworkPlan(f))
	require.NotNil(t, readFuelNetworkHold(donor))
	fuel, year := f.Fuel, g.Year
	tg.releaseColonizerFuelDetours()
	require.False(t, fuelNetworkManaged(f))
	require.Nil(t, readFuelNetworkHold(donor))
	require.Equal(t, fuel, f.Fuel)
	tg.planColonizationAutomation()
	tg.guardAutomaticFuel()
	tg.planFuelNetworks()
	tg.planAutomaticTankers()
	t.Logf("Santa Maria: fuel %d/%d, job %s, route %v", f.Fuel, f.Spec.FuelCapacity, f.GetTag(colonizationTag), f.Waypoints)
	t.Logf("Tanker 62: job %s, route %v", donor.GetTag(tankerJobTag), donor.Waypoints)
	require.False(t, fuelNetworkManaged(f), "routine acceleration must use a moving rendezvous")
	require.Equal(t, "colonizer:best", p.Automation.FleetRoles[f.Num])
	require.Equal(t, year, g.Year)
}
