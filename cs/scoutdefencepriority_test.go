//go:build !wasi && !wasm

package cs

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestColonyReconPreemptsDistantDesignReservationAndApproachBonus(t *testing.T) {
	for _, claimed := range []bool{false, true} {
		t.Run(map[bool]string{false: "approach bonus", true: "design reservation"}[claimed], func(t *testing.T) {
			bomber := campaignDesign("Unknown bomber", MiniBomber.Name, "bomber")
			u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
				{Designs: Designs(DesignLongRangeScout, DesignDestroyerDelta, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Near scout", Design: DesignLongRangeScout.Name}, {Name: "Reserved scout", Design: DesignLongRangeScout.Name, Position: Vector{0, -10}}, {Name: "Hunter", Design: DesignDestroyerDelta.Name}}},
				{Designs: Designs(bomber), Fleets: []ScenarioFleet{{Name: "Bombarding", Design: bomber.Name, Position: Vector{20, 0}}, {Name: "Distant", Design: bomber.Name, Position: Vector{200, 0}}}},
			}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}, {Name: "Colony", Owner: 1, Position: Vector{20, 0}, Cargo: Cargo{Colonists: 2000}}}})
			p, f, other, near, far := u.Player(1), u.Fleet("Near scout"), u.Fleet("Reserved scout"), u.Fleet("Bombarding"), u.Fleet("Distant")
			p.Race.Spec.DiscoverDesignOnScan = false
			p.Automation.FleetRoles = map[int]string{f.Num: "scout", other.Num: "scout", u.Fleet("Hunter").Num: "patrol"}
			report := *far
			report.WarpSpeed = 5
			report.Heading = VectorFloat64{-1, 0}
			p.FleetIntels = []*Fleet{near, &report}
			if claimed {
				data, _ := json.Marshal(scoutReconJob{Enemy: 2, Target: far.Num})
				other.SetTag(scoutReconTag, string(data))
			}
			u.Run((*turnGenerator).startPatrolEvaluation)
			// Another known threat ranks first; the second colony must still beat
			// a lower-ranked incoming contact after applying travel bonuses.
			p.patrolQueue = []patrolMission{{Kind: "attack", Enemy: 3, Target: 99}, {Kind: "attack", Enemy: 2, Target: near.Num}, {Kind: "attack", Enemy: 2, Target: far.Num}}
			u.Run(func(tg *turnGenerator) { require.True(t, tg.planScoutRecon(p, f)) })
			require.Equal(t, near.Num, readScoutRecon(f).Target)
			u.Run((*turnGenerator).fleetMove, (*turnGenerator).scanScoutReconContacts)
			require.False(t, unknownFleetEquipment(p, near), "the selected scout must physically report the bombing design")
		})
	}
}
