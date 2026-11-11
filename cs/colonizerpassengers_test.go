//go:build !wasi && !wasm

package cs

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestColonizerCarriesOnlyPassengersAndResumesColonization(t *testing.T) {
	u := colonizerUniverse(t)
	p, f := u.Player(1), u.Fleet("Colony Ship")
	for _, name := range []string{"Near", "Far"} {
		u.Planet(name).PlayerNum = 1
		u.Planet(name).Cargo.Colonists = 10
	}
	u.Recompute()
	u.Run((*turnGenerator).planColonizationAutomation, (*turnGenerator).planLogisticsAutomation, (*turnGenerator).resolveLogisticsAutomation, (*turnGenerator).dispatchReadyLogistics)
	require.Equal(t, "colonizer:auto", p.Automation.FleetRoles[f.Num])
	require.Equal(t, "passengers", readColonizationJob(f).Kind)
	job := logisticsWork(f)
	require.NotNil(t, job)
	require.Equal(t, "settle", job.Kind)
	require.Greater(t, len(f.Waypoints), 1)
	require.Zero(t, f.Cargo.ToMineral().Total())
	require.False(t, f.carriesInfrastructure())
	u.Planet("Far").PlayerNum = Unowned
	u.Planet("Far").Cargo.Colonists = 0
	u.Run((*turnGenerator).planColonizationAutomation)
	require.Equal(t, "passengers", readColonizationJob(f).Kind, "finish the current delivery first")
	delete(f.Tags, logisticsTag) // completed delivery releases the current job
	u.Run((*turnGenerator).planColonizationAutomation)
	require.Equal(t, "colonize", readColonizationJob(f).Kind)
	require.Nil(t, logisticsWork(f))
	require.Equal(t, WaypointTaskColonize, f.Waypoints[len(f.Waypoints)-1].Task)
}

func TestSplittingManualFleetDoesNotInheritStaleScoutAssignments(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Escort")
	f.Tokens[0].Quantity = 6
	f.Spec = ComputeFleetSpec(&u.Game.Rules, p, f)
	f.Fuel = f.Spec.FuelCapacity
	for n := 1; n <= 15; n++ {
		p.Automation.FleetRoles[n] = "scout"
	}
	delete(p.Automation.FleetRoles, f.Num)
	children, err := NewOrderer().SplitAll(&u.Game.Rules, p, u.Game.getFleets(1), f)
	require.NoError(t, err)
	require.Len(t, children, 5)
	for _, child := range children {
		require.Empty(t, p.Automation.FleetRoles[child.Num])
	}
	p.Automation.FleetRoles[f.Num] = "scout"
	f.Spec.HasWeapons = true
	f.Spec.Scanner = false
	u.Run((*turnGenerator).repairStaleScoutRoles)
	require.Equal(t, "patrol", p.Automation.FleetRoles[f.Num])
}

func TestColonizerStagesOnFriendlyWorldForForecastBombingClearance(t *testing.T) {
	u := colonizerUniverse(t)
	p, f := u.Player(1), u.Fleet("Colony Ship")
	target := u.Planet("Far")
	target.PlayerNum = 2
	target.Cargo.Colonists = 10
	target.ReportAge = 0
	bomber := &Fleet{MapObject: MapObject{Num: 99, PlayerNum: 1, Position: target.Position}, Spec: FleetSpec{Bombs: []Bomb{{Quantity: 1, MinKillRate: 10000}}}}
	p.Automation.FleetRoles[bomber.Num] = "bomber"
	bomber.SetTag(bomberTag, fmt.Sprintf(`{"orders":{"waypoints":[{}, {"targetNum":%d}]}}`, target.Num))
	u.Game.Fleets = append(u.Game.Fleets, bomber)
	before := target.Cargo.Colonists
	job := &colonizationJob{Mode: "best"}
	u.Run(func(tg *turnGenerator) {
		require.Equal(t, 1, tg.estimatedBombingClearance(p, target))
		require.True(t, tg.stageColonizerForBombing(p, f, job))
	})
	require.Equal(t, "preposition", job.Kind)
	require.Equal(t, target.Num, job.TargetNum)
	require.Equal(t, before, target.Cargo.Colonists, "prediction must never mutate the target")
	for _, wp := range f.Waypoints {
		require.NotEqual(t, WaypointTaskColonize, wp.Task, "an occupied world cannot be colonized in advance")
	}
}
