//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"math"
	"testing"
)

func addExpeditionTestDock(p *Player, position Vector) {
	p.PlanetIntels = append(p.PlanetIntels, &Planet{MapObject: MapObject{Num: 1000, PlayerNum: p.Num, Position: position}, Spec: PlanetSpec{HasStarbase: true, DockCapacity: -1}})
}

func TestTwentyFourScoutsReserveDistinctOutwardTargets(t *testing.T) {
	u := logisticsUniverse(t)
	p := u.Player(1)
	u.Game.Fleets = nil
	p.PlanetIntels = nil
	p.Automation.FleetRoles = map[int]string{}
	for i := range 24 {
		fleet := testLongRangeScout(p)
		fleet.Num = i + 1
		u.Game.Fleets = append(u.Game.Fleets, fleet)
		p.Automation.FleetRoles[fleet.Num] = "scout"
		angle := 2 * math.Pi * float64(i) / 24
		p.PlanetIntels = append(p.PlanetIntels, &Planet{MapObject: MapObject{Num: i + 1, Position: Vector{int(math.Round(50 * math.Cos(angle))), int(math.Round(50 * math.Sin(angle)))}, ReportAge: ReportAgeUnexplored}})
	}
	addExpeditionTestDock(p, Vector{})
	u.Run((*turnGenerator).planScoutAutomation)
	seen := map[int]bool{}
	for _, fleet := range u.Game.Fleets {
		mission := readScoutMission(fleet)
		require.NotZero(t, mission.Target)
		require.False(t, seen[mission.Target])
		seen[mission.Target] = true
	}
	u.Run((*turnGenerator).planScoutAutomation)
	for _, fleet := range u.Game.Fleets {
		require.NotZero(t, readScoutMission(fleet).Target)
	}
}

func TestScoutOnlyTargetsUnexploredReports(t *testing.T) {
	u := logisticsUniverse(t)
	p, fleet := u.Player(1), u.Fleet("Transport")
	p.Automation.FleetRoles[fleet.Num] = "scout"
	p.PlanetIntels = []*Planet{
		{MapObject: MapObject{Num: 1, Position: Vector{1, 0}, ReportAge: 12}},
		{MapObject: MapObject{Num: 2, Name: "Unknown", Position: Vector{20, 0}, ReportAge: ReportAgeUnexplored}},
	}
	addExpeditionTestDock(p, Vector{})
	u.Run((*turnGenerator).planScoutAutomation)
	require.GreaterOrEqual(t, len(fleet.Waypoints), 2)
	require.Equal(t, Vector{20, 0}, fleet.Waypoints[1].Position)
	p.PlanetIntels[1].ReportAge = 0
	u.Run((*turnGenerator).planScoutAutomation)
	require.Len(t, fleet.Waypoints, 1)
}

func TestScoutsDistributeTargetsAndAvoidKnownThreats(t *testing.T) {
	u := logisticsUniverse(t)
	p, a, b := u.Player(1), u.Fleet("Transport"), u.Fleet("Escort")
	p.Automation.FleetRoles = map[int]string{a.Num: "scout", b.Num: "scout"}
	p.PlanetIntels = []*Planet{
		{MapObject: MapObject{Num: 1, Position: Vector{20, 0}, ReportAge: ReportAgeUnexplored}},
		{MapObject: MapObject{Num: 2, Position: Vector{0, 20}, ReportAge: ReportAgeUnexplored}},
	}
	addExpeditionTestDock(p, Vector{})
	u.Run((*turnGenerator).planScoutAutomation)
	require.GreaterOrEqual(t, len(a.Waypoints), 2)
	require.GreaterOrEqual(t, len(b.Waypoints), 2)
	require.NotEqual(t, a.Waypoints[1].Position, b.Waypoints[1].Position)
	p.FleetIntels = []*Fleet{{MapObject: MapObject{PlayerNum: 2, Position: Vector{10, 0}}, Spec: FleetSpec{ShipDesignSpec: ShipDesignSpec{HasWeapons: true}}}}
	u.Run((*turnGenerator).planScoutAutomation)
	require.Len(t, a.Waypoints, 1)
	require.Len(t, b.Waypoints, 1)
}

func TestScoutRetirementWaitsUntilExpeditionEnds(t *testing.T) {
	u := logisticsUniverse(t)
	p, fleet := u.Player(1), u.Fleet("Transport")
	p.Automation.FleetRoles[fleet.Num] = "scout"
	p.DesignMaintenance = DesignMaintenance{fleet.Tokens[0].DesignNum: 99}
	generator := newTurnGenerator(u.Game)
	require.False(t, generator.maintenanceNeeded(p, fleet))
	p.DesignMaintenance[fleet.Tokens[0].DesignNum] = RetireDesign
	require.False(t, generator.maintenanceNeeded(p, fleet))
	saveScoutMission(fleet, &scoutMission{Complete: true})
	require.True(t, generator.maintenanceNeeded(p, fleet))
}

func TestScoutMovesOutwardAndKeepsItsOriginAfterReload(t *testing.T) {
	u := logisticsUniverse(t)
	p, fleet := u.Player(1), u.Fleet("Transport")
	p.Automation.FleetRoles[fleet.Num] = "scout"
	fleet.Position = Vector{20, 0}
	saveScoutMission(fleet, &scoutMission{Origin: Vector{}, Radius: 20})
	p.PlanetIntels = []*Planet{
		{MapObject: MapObject{Num: 1, Position: Vector{19, 0}, ReportAge: ReportAgeUnexplored}},
		{MapObject: MapObject{Num: 2, Position: Vector{40, 0}, ReportAge: ReportAgeUnexplored}},
	}
	addExpeditionTestDock(p, Vector{})
	u.Run((*turnGenerator).planScoutAutomation)
	require.Equal(t, Vector{40, 0}, fleet.Waypoints[1].Position)
	require.Equal(t, Vector{}, readScoutMission(fleet).Origin)
	p.PlanetIntels[1].ReportAge = 0
	u.Run((*turnGenerator).planScoutAutomation)
	require.False(t, readScoutMission(fleet).Complete)
	require.Equal(t, Vector{19, 0}, fleet.Waypoints[1].Position)
	require.Equal(t, Vector{}, fleet.Waypoints[len(fleet.Waypoints)-1].Position)
	p.PlanetIntels[0].ReportAge = 0
	fleet.Position = Vector{}
	u.Run((*turnGenerator).planScoutAutomation)
	require.True(t, readScoutMission(fleet).Complete)
}
