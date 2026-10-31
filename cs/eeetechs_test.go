//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestEEEPricesStayComparableToStandardParts(t *testing.T) {
	// Compare price per unit of the primary benefit, allowing at most a 25%
	// premium for slot density or earlier availability. Prices are base costs;
	// normal racial discounts and miniaturisation still apply afterwards.
	cases := []struct {
		name     string
		standard TechHullComponent
		benefit  func(TechHullComponent) int
	}{
		{"EEE Cryogenic Tank", SuperFuelTank, func(p TechHullComponent) int { return p.FuelBonus }},
		{"EEE Deep Reservoir", SuperFuelTank, func(p TechHullComponent) int { return p.FuelBonus }},
		{"EEE Modular Cargo", SuperCargoPod, func(p TechHullComponent) int { return p.CargoBonus }},
		{"EEE Gravity Cargo", SuperCargoPod, func(p TechHullComponent) int { return p.CargoBonus }},
		{"EEE Adaptive Miner", RoboMaxiMiner, func(p TechHullComponent) int { return p.MiningRate }},
		{"EEE Bio Miner", RoboSuperMiner, func(p TechHullComponent) int { return p.MiningRate }},
		{"EEE Ceramic Armor", Kelarium, func(p TechHullComponent) int { return p.Armor }},
		{"EEE Phase Barrier", ElephantHideFortress, func(p TechHullComponent) int { return p.Shield }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			part := StaticTechStore.GetHullComponent(tc.name)
			require.NotNil(t, part)
			require.LessOrEqual(t, part.Cost.Resources*tc.benefit(tc.standard)*100, tc.standard.Cost.Resources*tc.benefit(*part)*125)
		})
	}
}

func TestEEETechnologiesUnlockAndRespectRacialRestrictions(t *testing.T) {
	seen := map[string]bool{}
	for _, component := range EEETechHullComponents() {
		require.False(t, seen[component.Name])
		seen[component.Name] = true
		require.NotNil(t, StaticTechStore.GetHullComponent(component.Name))
		player := NewPlayer(1, NewRace().WithSpec(&rules))
		require.False(t, player.HasTech(&component.Tech), component.Name)
		player.TechLevels = component.Requirements.TechLevel
		require.True(t, player.HasTech(&component.Tech), component.Name)
		if component.MiningRate > 0 {
			player.Race.WithLRT(OBRM)
			require.False(t, player.HasTech(&component.Tech))
		}
		if component.ScanRangePen > 0 {
			player.Race.WithLRT(NAS)
			require.False(t, player.HasTech(&component.Tech))
		}
	}
	require.Len(t, seen, 14)
	player := NewPlayer(1, NewRace().WithLRT(NRSE).WithSpec(&rules)).WithTechLevels(TechLevel{26, 26, 26, 26, 26, 26})
	require.False(t, player.HasTech(&StaticTechStore.GetHullComponent("EEE Survey Drive").Tech))
}

func TestFleetCapabilityTasksAreEquipmentBased(t *testing.T) {
	fleet := &Fleet{Waypoints: []Waypoint{{Task: WaypointTaskColonize}, {Task: WaypointTaskLayMinefield}, {Task: WaypointTaskRemoteMining}}}
	for _, task := range []WaypointTask{WaypointTaskColonize, WaypointTaskLayMinefield, WaypointTaskRemoteMining} {
		require.False(t, fleet.CanPerformWaypointTask(task))
	}
	fleet.Spec.MiningRate = 10
	require.True(t, fleet.CanPerformWaypointTask(WaypointTaskRemoteMining))
	fleet.clearUnusableTasks()
	require.Equal(t, WaypointTaskNone, fleet.Waypoints[0].Task)
	require.Equal(t, WaypointTaskNone, fleet.Waypoints[1].Task)
	require.Equal(t, WaypointTaskRemoteMining, fleet.Waypoints[2].Task)
	fleet.Spec.Colonizer = true
	fleet.Spec.CanLayMines = true
	require.True(t, fleet.CanPerformWaypointTask(WaypointTaskColonize))
	require.True(t, fleet.CanPerformWaypointTask(WaypointTaskLayMinefield))
}
