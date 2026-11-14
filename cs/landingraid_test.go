//go:build !wasi && !wasm

package cs

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLandingEscortRaidKeepsOneGuardAtRally(t *testing.T) {
	hunter := campaignDesign("Escort hunter", Cruiser.Name, "patrol")
	for i := range hunter.Slots {
		if StaticTechStore.GetHullComponent(hunter.Slots[i].HullComponent).HullSlotType&HullSlotTypeEngine != 0 {
			hunter.Slots[i].HullComponent = FuelMizer.Name
		}
	}
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
		{Player: proposalTestPlayer(), Designs: Designs(hunter, DesignTeamster, DesignSpaceStation),
			Fleets: []ScenarioFleet{{Name: "Hunter", Design: hunter.Name}, {Name: "Guard", Design: hunter.Name}, {Name: "Transport", Design: DesignTeamster.Name}}},
		{Designs: Designs(DesignLongRangeScout), Fleets: []ScenarioFleet{{Name: "Contact", Design: DesignLongRangeScout.Name, Position: Vector{16, 0}}}},
	}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}}})
	p, leader, f, guard, enemy := u.Player(1), u.Fleet("Transport"), u.Fleet("Hunter"), u.Fleet("Guard"), u.Fleet("Contact")
	p.Automation.FleetRoles = map[int]string{f.Num: "patrol", guard.Num: "escort", leader.Num: "freighter"}
	p.discoverer.discoverDesign(enemy.Tokens[0].design, true)
	p.discoverer.discoverFleet(enemy, true)
	op := &landingOperation{Phase: "gather", Rally: leader.Position}
	for _, escort := range []*Fleet{f, guard} {
		escort.SetTag(landingMemberTag, fmt.Sprint(leader.Num))
	}
	u.Run((*turnGenerator).startPatrolEvaluation, func(tg *turnGenerator) { tg.landingEscortRaid(p, leader, op) })
	require.NotEmpty(t, f.GetTag(landingRaidTag))
	require.Empty(t, guard.GetTag(landingRaidTag))
	require.True(t, landingManaged(f))
	require.Equal(t, enemy.Position, f.Waypoints[1].Position)
}
