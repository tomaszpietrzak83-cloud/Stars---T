//go:build !wasi && !wasm

package cs

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMaintenanceYardPruningMatchesOriginal(t *testing.T) {
	for _, position := range []Vector{{0, 0}, {120, 30}, {500, 600}} {
		for _, fuel := range []int{0, 40, 100} {
			for _, gates := range []bool{false, true} {
				makeUniverse := func() *testUniverse {
					s := maintenanceScenario()
					s.Players[0].Fleets[0].At = ""
					s.Players[0].Fleets[0].Position = position
					for i, point := range []Vector{{900, 900}, {120, 0}, {120, 60}, {500, 600}} {
						s.Planets = append(s.Planets, ScenarioPlanet{Name: []string{"Far", "NearA", "NearB", "End"}[i], Owner: 1, Starbase: DesignSpaceStation.Name, Position: point, Cargo: Cargo{Colonists: 2500, Ironium: 1000, Boranium: 1000, Germanium: 1000}})
					}
					u := newTestUniverse(t, s)
					requestRebuild(u)
					f := u.Fleet("Fleet")
					f.Fuel = f.Spec.FuelCapacity * fuel / 100
					if gates {
						for _, planet := range u.Player(1).PlanetIntels {
							planet.Spec.HasStargate = true
							planet.Spec.SafeRange = 1000
							planet.Spec.SafeHullMass = 1000
						}
					}
					return u
				}
				before, after := makeUniverse(), makeUniverse()
				before.Run((*turnGenerator).referenceFleetDesignMaintenance)
				after.Run((*turnGenerator).fleetDesignMaintenance)
				for _, pair := range [][2]any{{before.Game.Fleets, after.Game.Fleets}, {before.Game.Planets, after.Game.Planets}} {
					a, err := json.Marshal(pair[0])
					require.NoError(t, err)
					b, err := json.Marshal(pair[1])
					require.NoError(t, err)
					require.JSONEq(t, string(a), string(b), "position=%v fuel=%d gates=%v", position, fuel, gates)
				}
			}
		}
	}
}
