//go:build !wasi && !wasm

package cs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBomberReserveCoversPriorityBasesAndLaunchesOnClearance(t *testing.T) {
	design := campaignDesign("Reserve bomber", B17Bomber.Name, "bomber")
	u := newTestUniverse(t, TestScenario{
		Players: []ScenarioPlayer{
			{Designs: Designs(design, DesignSpaceStation), Fleets: []ScenarioFleet{
				{Name: "West", Design: design.Name, At: "West dock", Quantity: 2},
				{Name: "East", Design: design.Name, At: "East dock"},
				{Name: "Main force", Design: design.Name, At: "West dock", Quantity: 7},
			}},
			{Designs: Designs(DesignSpaceStation)},
		},
		Planets: []ScenarioPlanet{
			{Name: "West dock", Owner: 1, Starbase: DesignSpaceStation.Name},
			{Name: "East dock", Owner: 1, Starbase: DesignSpaceStation.Name, Position: Vector{80, 0}},
			{Name: "West priority", Owner: 2, Starbase: DesignSpaceStation.Name, Position: Vector{20, 0}, Cargo: Cargo{Colonists: 2001}},
			{Name: "East priority", Owner: 2, Starbase: DesignSpaceStation.Name, Position: Vector{65, 0}, Cargo: Cargo{Colonists: 2001}},
			{Name: "Alternative", Owner: 2, Position: Vector{0, 35}, Cargo: Cargo{Colonists: 2001}},
		},
	})
	p := u.Player(1)
	west, east, main := u.Fleet("West"), u.Fleet("East"), u.Fleet("Main force")
	westTarget, eastTarget, alternative := u.Planet("West priority"), u.Planet("East priority"), u.Planet("Alternative")
	p.PlanetIntels = []*Planet{u.Planet("West dock"), u.Planet("East dock"), westTarget, eastTarget, alternative}
	for _, planet := range p.PlanetIntels {
		planet.ReportAge = 0
		if planet.OwnedBy(p.Num) {
			planet.Scanner = true
			planet.Spec.ScanRange = 10000
			planet.Spec.ScanRangePen = 10000
		}
	}
	p.Automation.PriorityPlanets = map[int]bool{westTarget.Num: true, eastTarget.Num: true}
	p.Automation.FleetRoles = map[int]string{west.Num: "bomber", east.Num: "bomber", main.Num: "bomber"}

	u.Run((*turnGenerator).planBomberEscorts)
	require.Equal(t, westTarget.Num, readBomberOrders(west).ReserveTarget)
	require.Equal(t, eastTarget.Num, readBomberOrders(east).ReserveTarget)
	require.Len(t, west.Waypoints, 1)
	require.Len(t, east.Waypoints, 1)
	require.Equal(t, alternative.Num, bomberGoal(main))
	require.Greater(t, len(main.Waypoints), 1)

	westTarget.Spec.HasStarbase = false
	u.Run((*turnGenerator).planBomberEscorts)
	require.Zero(t, readBomberOrders(west).ReserveTarget)
	require.Equal(t, westTarget.Num, bomberGoal(west))
	require.Greater(t, len(west.Waypoints), 1)
	require.Equal(t, westTarget.Position, west.Waypoints[len(west.Waypoints)-1].Position)
	require.Equal(t, eastTarget.Num, readBomberOrders(east).ReserveTarget)

	eastTarget.Spec.HasStarbase = false
	u.Run((*turnGenerator).planBomberEscorts)
	require.Zero(t, readBomberOrders(east).ReserveTarget)
	require.Equal(t, eastTarget.Num, bomberGoal(east))
	require.Greater(t, len(east.Waypoints), 1)
}
