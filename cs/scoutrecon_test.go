//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestScoutPrioritizesOnlyUnknownIncomingEnemyFleetAndReportsBeforeBattle(t *testing.T) {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Designs: Designs(DesignLongRangeScout, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Scout", Design: DesignLongRangeScout.Name, At: "Home"}}}, {Designs: Designs(DesignDestroyerDelta), Fleets: []ScenarioFleet{{Name: "Raider", Design: DesignDestroyerDelta.Name, Position: Vector{20, 0}}}}}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}, {Name: "Unexplored", Position: Vector{50, 0}}}})
	p, f, enemy := u.Player(1), u.Fleet("Scout"), u.Fleet("Raider")
	p.Automation.FleetRoles = map[int]string{f.Num: "scout"}
	p.PlanetIntels = []*Planet{u.Planet("Home"), u.Planet("Unexplored")}
	intel := *enemy
	intel.WarpSpeed = 5
	intel.Heading = VectorFloat64{1, 0}
	p.FleetIntels = []*Fleet{&intel}
	require.False(t, enemyApproachingColony(p, &intel))
	intel.Heading = VectorFloat64{-1, 0}
	require.True(t, enemyApproachingColony(p, &intel))
	u.Run((*turnGenerator).planScoutAutomation, (*turnGenerator).guardAutomaticFuel, (*turnGenerator).planAutomaticSafety, (*turnGenerator).enforceAutomaticDeparture)
	require.True(t, scoutReconActive(p, f))
	require.Equal(t, MapObjectTypeFleet, f.Waypoints[len(f.Waypoints)-1].TargetType)
	require.False(t, f.InAutomaticSafety(), "intentional reconnaissance accepts combat risk")
	f.Position = enemy.Position
	u.Run((*turnGenerator).scanScoutReconContacts)
	design := p.GetShipDesignIntel(enemy.PlayerNum, enemy.Tokens[0].DesignNum)
	require.NotNil(t, design)
	require.NotEmpty(t, design.Slots)
	require.False(t, scoutReconActive(p, f))
}

func TestFreighterCannotAcceptSacrificialScoutRecon(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Transport")
	p.Automation.FleetRoles[f.Num] = "scout"
	p.FleetIntels = []*Fleet{{MapObject: MapObject{Num: 91, PlayerNum: 2, Position: Vector{20, 0}}, Heading: VectorFloat64{-1, 0}, WarpSpeed: 5, Spec: FleetSpec{ShipDesignSpec: ShipDesignSpec{HasWeapons: true}}}}
	u.Run(func(tg *turnGenerator) { require.False(t, tg.planScoutRecon(p, f)) })
	require.False(t, scoutReconActive(p, f))
	u.Run((*turnGenerator).repairStaleScoutRoles)
	require.Equal(t, "freighter", p.Automation.FleetRoles[f.Num])
}

func TestReusedFleetNumberDoesNotInheritDestroyedScoutRole(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Transport")
	reusedNum := p.GetNextFleetNum(u.Game.getFleets(p.Num))
	p.Automation.FleetRoles = map[int]string{reusedNum: "scout"}
	p.Automation.RebuildFleets = map[int]bool{reusedNum: true}
	u.Run(func(tg *turnGenerator) {
		built, err := tg.addFleet(p, f.Position, f.Tokens[0], nil)
		require.NoError(t, err)
		require.Equal(t, reusedNum, built.Num)
		require.Empty(t, p.Automation.FleetRoles[built.Num])
		require.False(t, p.Automation.RebuildFleets[built.Num])
	})
}
func TestDepartureRejectsUnarmedFreighterEnemyFleetWaypoint(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Transport")
	p.Automation.FleetRoles[f.Num] = "freighter"
	f.Waypoints = append(f.Waypoints, Waypoint{Position: Vector{20, 0}, WarpSpeed: 5, MapObjectTarget: MapObjectTarget{TargetType: MapObjectTypeFleet, TargetPlayerNum: 2, TargetNum: 91}})
	u.Run((*turnGenerator).enforceAutomaticDeparture)
	for _, wp := range f.Waypoints[1:] {
		require.False(t, wp.TargetType == MapObjectTypeFleet && wp.TargetPlayerNum == 2)
	}
}

func TestScoutStackSplitsBeforeContactReconnaissance(t *testing.T) {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Designs: Designs(DesignLongRangeScout, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Scouts", Design: DesignLongRangeScout.Name, At: "Home"}}}, {Designs: Designs(DesignDestroyerDelta), Fleets: []ScenarioFleet{{Name: "Enemy", Design: DesignDestroyerDelta.Name, Position: Vector{20, 0}}}}}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}}})
	p, f, e := u.Player(1), u.Fleet("Scouts"), u.Fleet("Enemy")
	f.Tokens[0].Quantity = 4
	f.Spec = ComputeFleetSpec(&u.Game.Rules, p, f)
	f.Fuel = f.Spec.FuelCapacity
	p.Automation.FleetRoles = map[int]string{f.Num: "scout"}
	p.PlanetIntels = []*Planet{u.Planet("Home")}
	report := *e
	report.WarpSpeed = 5
	report.Heading = VectorFloat64{-1, 0}
	p.FleetIntels = []*Fleet{&report}
	u.Run(func(tg *turnGenerator) { require.True(t, tg.planScoutRecon(p, f)) })
	require.Equal(t, 1, f.Spec.TotalShips)
	total, contacts := 0, 0
	for _, ship := range u.Game.getFleets(p.Num) {
		if !ship.Delete {
			total += ship.Spec.TotalShips
			if scoutReconActive(p, ship) {
				contacts++
			}
		}
	}
	require.Equal(t, 4, total)
	require.Equal(t, 1, contacts)
}
