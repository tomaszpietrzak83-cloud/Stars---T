//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"math"
	"testing"
)

func TestScoutPlansBothPetalArmsAndFundsWholeReturn(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Transport")
	p.Automation.FleetRoles = map[int]string{f.Num: "scout"}
	p.PlanetIntels = nil
	addExpeditionTestDock(p, Vector{})
	home := p.PlanetIntels[len(p.PlanetIntels)-1]
	num := 1
	for _, angle := range []float64{2.1, 2.7} {
		for _, distance := range []float64{20, 50, 80} {
			p.PlanetIntels = append(p.PlanetIntels, &Planet{MapObject: MapObject{Num: num, Name: "Unvisited", Position: Vector{int(math.Round(distance * math.Cos(angle))), int(math.Round(distance * math.Sin(angle)))}, ReportAge: ReportAgeUnexplored}})
			num++
		}
	}
	u.Run((*turnGenerator).planScoutAutomation)
	mission := readScoutMission(f)
	require.NotNil(t, mission)
	require.Greater(t, len(mission.Plan), 2)
	require.Greater(t, mission.ReturnAt, 0)
	require.Less(t, mission.ReturnAt, len(mission.Plan)-1)
	require.Equal(t, home.Num, mission.Plan[len(mission.Plan)-1])
	virtual := *f
	seen := map[int]bool{}
	lastRadius := 0.0
	for i, targetNum := range mission.Plan {
		target := p.GetPlanetIntel(targetNum)
		require.NotNil(t, target)
		if targetNum != home.Num {
			require.False(t, seen[targetNum])
			seen[targetNum] = true
			radius := target.Position.DistanceTo(mission.Origin)
			if i < mission.ReturnAt {
				require.Greater(t, radius, lastRadius)
			} else {
				require.Less(t, radius, lastRadius)
			}
			lastRadius = radius
		}
		route := planExpeditionRoute(&u.Game.Rules, p, &virtual, target)
		require.NotNil(t, route)
		virtual = *scoutVirtualArrival(&virtual, target, route)
		require.GreaterOrEqual(t, virtual.Fuel, 0)
	}
	require.Equal(t, home.Position, virtual.Position)
}
func TestScoutFinishesRefuellingBeforeAnotherExpedition(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Transport")
	p.Automation.FleetRoles = map[int]string{f.Num: "scout"}
	p.PlanetIntels = nil
	addExpeditionTestDock(p, Vector{})
	home := p.PlanetIntels[len(p.PlanetIntels)-1]
	p.PlanetIntels = append(p.PlanetIntels, &Planet{MapObject: MapObject{Num: 1, Name: "Unknown", Position: Vector{10, 0}, ReportAge: ReportAgeUnexplored}})
	saveScoutMission(f, &scoutMission{Home: home.Num, Refuelling: true})
	f.Fuel = f.Spec.FuelCapacity / 2
	u.Run((*turnGenerator).planScoutAutomation)
	require.Len(t, f.Waypoints, 1)
	require.True(t, readScoutMission(f).Refuelling)
	require.Equal(t, "scout", p.Automation.FleetRoles[f.Num])
	f.Fuel = f.Spec.FuelCapacity
	u.Run((*turnGenerator).planScoutAutomation)
	require.False(t, readScoutMission(f).Refuelling)
	require.Greater(t, len(readScoutMission(f).Plan), 1)
}

func TestScoutDoesNotUseSlowMultiYearReturnLegs(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Transport")
	p.Automation.FleetRoles[f.Num] = "scout"
	p.PlanetIntels = nil
	addExpeditionTestDock(p, Vector{})
	f.Position = Vector{100, 0}
	f.OrbitingPlanetNum = 0
	f.Fuel = 1
	for i := range f.Tokens {
		f.Tokens[i].design.Spec.Engine.FuelUsage = []int{0, 0, 0, 0, 100000, 100000, 100000, 100000, 100000, 100000, 100000}
	}
	home := p.PlanetIntels[0]
	require.Nil(t, scoutHomeRoute(&u.Game.Rules, p, f, home), "free warp 1-3 must not justify an eleven-year crawl")
}

func TestFullTankScoutCompletesHomeLegAndStartsAnotherPetal(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Transport")
	p.Automation.FleetRoles = map[int]string{f.Num: "scout"}
	p.PlanetIntels = nil
	addExpeditionTestDock(p, Vector{})
	home := p.PlanetIntels[len(p.PlanetIntels)-1]
	p.PlanetIntels = append(p.PlanetIntels, &Planet{MapObject: MapObject{Num: 1, Name: "Unvisited", Position: Vector{10, 0}, ReportAge: ReportAgeUnexplored}})
	saveScoutMission(f, &scoutMission{Home: home.Num, Plan: []int{home.Num}})
	f.Fuel = f.Spec.FuelCapacity
	u.Run((*turnGenerator).planScoutAutomation)
	require.Equal(t, 1, readScoutMission(f).Cycle)
	u.Run((*turnGenerator).planScoutAutomation)
	require.Greater(t, len(f.Waypoints), 1)
	require.Equal(t, "scout", p.Automation.FleetRoles[f.Num])
}
