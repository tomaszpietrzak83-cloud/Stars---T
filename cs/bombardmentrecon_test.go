//go:build !wasi && !wasm

package cs

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBombardmentPopulationRecon(t *testing.T) {
	for _, tc := range []struct {
		name            string
		age, population int
		needsScout      bool
	}{
		{"stale", 2, 1000, true},
		{"unknown", 1, 0, true},
		{"fresh", 1, 1000, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bomber := campaignDesign("Bomber design", MiniBomber.Name, "bomber")
			scout := campaignDesign("Scout design", Scout.Name, "scout")
			u := newTestUniverse(t, TestScenario{
				Players: []ScenarioPlayer{
					{Designs: Designs(bomber, scout, DesignSpaceStation), Fleets: []ScenarioFleet{
						{Name: "Bomber", Design: bomber.Name, At: "Enemy"},
						{Name: "Scout", Design: scout.Name, At: "Home"},
					}},
					{},
				},
				Planets: []ScenarioPlanet{
					{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name},
					{Name: "Enemy", Owner: 2, Position: Vector{60, 0}, Cargo: Cargo{Colonists: 120}},
				},
			})
			p, enemy := u.Player(1), u.Planet("Enemy")
			b, s := u.Fleet("Bomber"), u.Fleet("Scout")
			b.OrbitingPlanetNum = enemy.Num
			s.Spec.ScanRangePen = 25
			p.Automation.FleetRoles = map[int]string{b.Num: "bomber", s.Num: "scout"}
			intel := *enemy
			intel.ReportAge, intel.Cargo.Colonists = tc.age, tc.population
			p.PlanetIntels = []*Planet{u.Planet("Home"), &intel}
			u.Game.Rules.PopulationScannerError = 0
			require.Len(t, u.Game.Fleets, 2)
			before := enemy.GetPopulation()
			u.Run((*turnGenerator).fleetBomb)
			require.Less(t, enemy.GetPopulation(), before)
			u.Run(func(tg *turnGenerator) {
				assigned := tg.planRecaptureReconnaissance(p, tg.game.getFleets(p.Num))
				require.Equal(t, tc.needsScout, assigned[s.Num])
			})
			if !tc.needsScout {
				return
			}
			require.Equal(t, strconv.Itoa(enemy.Num), s.GetTag(priorityReconTag))
			require.Greater(t, len(s.Waypoints), 1)
			u.Run((*turnGenerator).fleetMove, (*turnGenerator).scanColonyReconContacts)
			require.NotEqual(t, u.Planet("Home").Position, s.Position)
			require.Equal(t, tc.age, intel.ReportAge, "the scout is still outside penetrating scanner range")
			u.Run((*turnGenerator).fleetMove, (*turnGenerator).scanColonyReconContacts)
			require.Zero(t, intel.ReportAge)
			require.Equal(t, enemy.GetPopulation(), intel.GetPopulation())
			t.Logf("population before=%d after bombing=%d scout report=%d", before, enemy.GetPopulation(), intel.GetPopulation())
		})
	}
}
