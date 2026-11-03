//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPriorityFleetDirectsPatrolBeyondOrdinaryLocalRadius(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Escort")
	p.Automation.FleetRoles[f.Num] = "patrol"
	addExpeditionTestDock(p, Vector{})
	f.Spec.FuelCapacity, f.Fuel = 4000, 4000
	near, target := patrolTestEnemy(0), patrolTestEnemy(0)
	near.Num, target.Num = 10, 11
	near.Position, target.Position = Vector{10, 0}, Vector{160, 0}
	p.FleetIntels = []*Fleet{near, target}
	p.Automation.PriorityFleets = map[string]bool{"2:11": true}
	u.Run((*turnGenerator).planPatrolAutomation)
	require.Equal(t, target.Num, readPatrolJob(f).Target)
	require.Greater(t, len(f.Waypoints), 1)
	require.True(t, patrolCanAttack(p, f))
}

func TestPriorityFleetRetargetsExistingPatrolAndSurvivesLostContact(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Escort")
	p.Automation.FleetRoles[f.Num] = "patrol"
	addExpeditionTestDock(p, Vector{})
	current, target := patrolTestEnemy(0), patrolTestEnemy(0)
	current.Num, target.Num = 10, 11
	current.Position, target.Position = Vector{10, 0}, Vector{50, 0}
	p.FleetIntels = []*Fleet{current, target}
	p.Automation.PriorityFleets = map[string]bool{"2:11": true, "3:11": true}
	savePatrolJob(f, patrolJob{Leader: f.Num, Enemy: 2, Target: current.Num})
	u.Run(func(tg *turnGenerator) { tg.retargetPatrolOpportunity(p, f) })
	require.Equal(t, target.Num, readPatrolJob(f).Target)
	p.FleetIntels = nil
	u.Run((*turnGenerator).planPatrolAutomation, (*turnGenerator).pruneAutomationAssignments)
	require.True(t, p.Automation.PriorityFleets["2:11"])
	target.Delete = true
	completeFleetPriorities(p, &BattleRecord{fleets: []*Fleet{target, current}})
	require.NotContains(t, p.Automation.PriorityFleets, "2:11")
	require.True(t, p.Automation.PriorityFleets["3:11"], "another player's same-numbered fleet is a different order")
}

func TestPriorityCannotOverridePatrolStrengthOrFreshReports(t *testing.T) {
	for _, stale := range []bool{false, true} {
		u := logisticsUniverse(t)
		p, f := u.Player(1), u.Fleet("Escort")
		p.Automation.FleetRoles[f.Num] = "patrol"
		e := patrolTestEnemy(f.Spec.PowerRating * 100)
		if stale {
			e.ReportAge = 1
			e.Spec.PowerRating = 0
			e.Spec.HasWeapons = false
		}
		p.FleetIntels = []*Fleet{e}
		p.Automation.PriorityFleets = map[string]bool{priorityFleetKey(2, e.Num): true}
		u.Run((*turnGenerator).planPatrolAutomation)
		require.False(t, patrolCanAttack(p, f))
		require.True(t, priorityFleet(p, e))
	}
}

func TestPlanetPriorityCompletesOnlyWhenOwned(t *testing.T) {
	u := passengerUniverse(t)
	p, home, dest := u.Player(1), u.Planet("Home"), u.Planet("Young colony")
	dest.PlayerNum = 2
	p.Automation.PriorityPlanets = map[int]bool{home.Num: true, dest.Num: true}
	u.Run((*turnGenerator).completePlanetPriorities)
	require.NotContains(t, p.Automation.PriorityPlanets, home.Num)
	require.True(t, p.Automation.PriorityPlanets[dest.Num])
	dest.PlayerNum = p.Num
	u.Run((*turnGenerator).completePlanetPriorities)
	require.Empty(t, p.Automation.PriorityPlanets)
}
