//go:build !wasi && !wasm

package cs

import (
	"compress/gzip"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

// Optional analysis of an exported COPY; neither saves nor advances a turn.
func TestCopiedRecaptureDiagnosis(t *testing.T) {
	path := os.Getenv("STARS_RECAPTURE_SNAPSHOT")
	if path == "" {
		t.Skip("optional copied snapshot")
	}
	f, err := os.Open(path)
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
	p := g.getPlayer(1)
	tg.enablePlanningThreatIndex()
	defer tg.disablePlanningThreatIndex()
	jobs := map[string]int{}
	for _, f := range g.getFleets(1) {
		if p.Automation.FleetRoles[f.Num] != "freighter" {
			continue
		}
		kind := "idle"
		if j := logisticsWork(f); j != nil {
			kind = j.Kind
		}
		if f.InDesignMaintenance() {
			kind += "/maintenance"
		}
		if f.InAutomaticSafety() {
			kind += "/safety"
		}
		if f.GetTag("maintenance-pending") != "" {
			kind += "/pending"
		}
		if f.CargoMass() > 0 {
			kind += "/loaded"
		}
		jobs[kind]++
		if f.CargoMass() == 0 && !f.InDesignMaintenance() && !f.InAutomaticSafety() {
			if j := tg.findInvasionTransfer(p, f, true); j != nil {
				t.Logf("AVAILABLE_INVADER %d pending=%v target=%d", f.Num, f.GetTag("maintenance-pending") != "", j.DeliveryNum)
			}
		}
	}
	t.Logf("FREIGHTER_JOBS %v", jobs)
	for _, i := range p.PlanetIntels {
		if i == nil || !p.Automation.PriorityPlanets[i.Num] {
			continue
		}
		t.Logf("TARGET %d %s age=%d owner=%d quality=%d pop=%d defense=%d base=%v eligible=%v bombing=%v", i.Num, i.Name, i.ReportAge, i.PlayerNum, prospectivePlanetQuality(p, i), i.GetPopulation(), i.Defenses, i.Spec.HasStarbase, invasionColony(p, i), priorityNeedsBombing(i))
	}
	for _, f := range g.getFleets(1) {
		r := p.Automation.FleetRoles[f.Num]
		if r == "bomber" {
			t.Logf("BOMBER %d active=%v maintenance=%v safety=%v pending=%v goal=%d", f.Num, !f.InDesignMaintenance() && !f.InAutomaticSafety(), f.InDesignMaintenance(), f.InAutomaticSafety(), f.GetTag("maintenance-pending") != "", bomberGoal(f))
		}
	}
	tg.planRecaptureReconnaissance(p, g.getFleets(1))
	for _, f := range g.getFleets(1) {
		if n := f.GetTag(priorityReconTag); n != "" {
			t.Logf("RECON %d target=%s wp=%v", f.Num, n, f.Waypoints)
		}
	}
}
