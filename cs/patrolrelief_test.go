package cs

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPatrolReliefCombinesZeroFuelDestroyersBeforeBombing(t *testing.T) {
	for _, interrupt := range []bool{false, true} {
		t.Run(fmt.Sprintf("interrupted=%t", interrupt), func(t *testing.T) {
			design := Designs(DesignDestroyerDelta)[0]
			design.Slots[0].HullComponent = RadiatingHydroRamScoop.Name
			bomber := ShipDesign{Name: "Bomber", Hull: MiniBomber.Name, Slots: []ShipDesignSlot{
				{HullComponent: QuickJump5.Name, HullSlotIndex: 1, Quantity: 1},
				{HullComponent: LadyFingerBomb.Name, HullSlotIndex: 2, Quantity: 2},
			}}
			hunters := []ScenarioFleet{}
			for i := range 20 {
				hunters = append(hunters, ScenarioFleet{Name: fmt.Sprintf("Patrol %d", i), Design: design.Name, Position: Vector{4 + i%5, 1 + i/5}})
			}
			u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
				{Designs: Designs(design, DesignSpaceStation), Fleets: hunters},
				{Designs: Designs(DesignDestroyerDelta, bomber), Fleets: []ScenarioFleet{
					{Name: "Escorts", Design: DesignDestroyerDelta.Name, Quantity: 2, At: "Colony"},
					{Name: "Bomber", Design: bomber.Name, At: "Colony"},
				}},
			}, Planets: []ScenarioPlanet{
				{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name},
				{Name: "Colony", Owner: 1, Position: Vector{36, 0}, Cargo: Cargo{Colonists: 1000}},
			}})
			u.Game.Year++
			p := u.Player(1)
			p.Automation.FleetRoles = map[int]string{}
			for _, f := range u.Game.getFleets(1) {
				if !f.Starbase {
					p.Automation.FleetRoles[f.Num] = "patrol"
					f.Fuel = 0
					require.Equal(t, 6, patrolFreeWarp(p, f))
				}
			}
			for _, f := range u.Game.getFleets(2) {
				p.discoverer.discoverDesign(f.Tokens[0].design, true)
				p.discoverer.discoverFleet(f, true)
			}
			u.Run((*turnGenerator).startPatrolEvaluation, func(tg *turnGenerator) { tg.planUrgentPatrolDefence(p) }, (*turnGenerator).guardAutomaticFuel)
			members := []*Fleet{}
			for _, f := range u.Game.getFleets(1) {
				if readPatrolJob(f).reliefActive(u.Game.Year) {
					require.Len(t, f.Waypoints, 2)
					require.Equal(t, 6, f.Waypoints[1].WarpSpeed)
					members = append(members, f)
				}
			}
			require.Greater(t, len(members), 1, "a single patrol ship is insufficient")
			if interrupt {
				holdAutomaticFleet(p, members[len(members)-1], "test changed a member's departure")
			}
			u.Run((*turnGenerator).enforceAutomaticDeparture)
			if interrupt {
				for _, f := range members {
					require.Len(t, f.Waypoints, 1)
				}
				return
			}
			u.Run((*turnGenerator).fleetMove, (*turnGenerator).mergePatrolRendezvous, (*turnGenerator).fleetBattle, (*turnGenerator).fleetBomb)
			require.True(t, u.Fleet("Bomber").Delete)
			for _, m := range p.Messages {
				require.NotEqual(t, PlayerMessagePlanetBombed, m.Type)
			}
			require.NotEmpty(t, p.BattleRecords)
		})
	}
}
