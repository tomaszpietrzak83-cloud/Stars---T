//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMinelayerEarlyBoundPreservesOrders(t *testing.T) {
	for _, distance := range []int{10, 63, 64, 100, 200, 1000} {
		for _, gates := range []bool{false, true} {
			for _, onboard := range []bool{false, true} {
				run := func(reference bool) *Fleet {
					layer := ShipDesign{Name: "Layer", Hull: MiniMineLayer.Name, Slots: []ShipDesignSlot{
						{HullSlotIndex: 1, HullComponent: QuickJump5.Name, Quantity: 1},
						{HullSlotIndex: 2, HullComponent: MineDispenser40.Name, Quantity: 1},
					}}
					u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
						{Designs: Designs(DesignDestroyerDelta, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Patrol", Design: DesignDestroyerDelta.Name, Quantity: 3}}},
						{Designs: Designs(layer), Fleets: []ScenarioFleet{{Name: "Layer", Design: layer.Name, Position: Vector{distance, 0}}}},
					}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}}})
					p, f, enemy := u.Player(1), u.Fleet("Patrol"), u.Fleet("Layer")
					p.Automation.FleetRoles = map[int]string{f.Num: "patrol"}
					f.Spec.CanJump = onboard
					if gates {
						home := p.GetPlanetIntel(u.Planet("Home").Num)
						home.Spec.HasStargate, home.Spec.SafeRange, home.Spec.SafeHullMass = true, 10000, 100000
						dock := *home
						dock.Num, dock.Position = 99, enemy.Position
						p.PlanetIntels = append(p.PlanetIntels, &dock)
					}
					p.discoverer.discoverDesign(enemy.Tokens[0].design, true)
					p.discoverer.discoverFleet(enemy, true)
					u.Run((*turnGenerator).startPatrolEvaluation, func(tg *turnGenerator) {
						tg.planUrgentPatrolDefence(p)
						if reference {
							tg.referenceMinelayerInterceptions1113(p)
						} else {
							tg.planMinelayerInterceptions(p)
						}
					})
					return f
				}
				want, got := run(true), run(false)
				require.Equal(t, want.FleetOrders, got.FleetOrders, "distance=%d gates=%v onboard=%v", distance, gates, onboard)
				require.Equal(t, want.Tags, got.Tags)
			}
		}
	}
}
