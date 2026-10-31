//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestResearchTaxPopulationAndResourceBoundaries(t *testing.T) {
	policy := ResearchTaxPolicy{Enabled: true, Basis: "population", LeftoverUntil: 50000, MediumThreshold: 250000, LargeThreshold: 750000, SmallPercent: 10, MediumPercent: 20, LargePercent: 30}
	require.NoError(t, policy.Validate())
	for _, test := range []struct {
		value, rate int
		left        bool
	}{{49999, 0, true}, {50000, 10, false}, {249999, 10, false}, {250000, 20, false}, {750000, 30, false}} {
		rate, left := policy.Contribution(test.value)
		require.Equal(t, test.rate, rate)
		require.Equal(t, test.left, left)
	}
	policy.Basis = "resources"
	policy.LeftoverUntil = 50
	policy.MediumThreshold = 250
	policy.LargeThreshold = 1000
	require.Equal(t, 250, policy.value(1000000, 250))
	rate, left := policy.Contribution(policy.value(1000000, 250))
	require.Equal(t, 20, rate)
	require.False(t, left)
	policy.SmallPercent = 0
	require.NoError(t, policy.Validate())
	policy.LargePercent = 101
	require.Error(t, policy.Validate())
}
func TestResearchTaxOverridesBudgetAndReclassifiesGrowingColony(t *testing.T) {
	u := colonizerUniverse(t)
	p, planet := u.Player(1), u.Planet("Home")
	p.Automation.ResearchTax = &ResearchTaxPolicy{Enabled: true, Basis: "population", LeftoverUntil: 50000, MediumThreshold: 250000, LargeThreshold: 750000, SmallPercent: 10, MediumPercent: 20, LargePercent: 30}
	planet.Spec.ResourcesPerYear = 1000
	p.ResearchAmount = 90
	planet.Cargo.Colonists = 2499
	planet.applyResearchTax(p)
	planet.Spec.computeResourcesPerYearAvailable(p, planet)
	require.Equal(t, 100, planet.Spec.ResourcesPerYearResearch)
	require.False(t, planet.ContributesOnlyLeftoverToResearch)
	planet.Cargo.Colonists = 2500
	planet.Spec.computeResourcesPerYearAvailable(p, planet)
	require.Equal(t, 200, planet.Spec.ResourcesPerYearResearch)
	planet.Cargo.Colonists = 499
	planet.applyResearchTax(p)
	planet.Spec.computeResourcesPerYearAvailable(p, planet)
	require.True(t, planet.ContributesOnlyLeftoverToResearch)
	require.Zero(t, planet.Spec.ResourcesPerYearResearch)
	p.Automation.ResearchTax.Basis = "resources"
	p.Automation.ResearchTax.LeftoverUntil = 50
	p.Automation.ResearchTax.MediumThreshold = 250
	p.Automation.ResearchTax.LargeThreshold = 1000
	planet.Spec.computeResourcesPerYearAvailable(p, planet)
	require.Equal(t, 300, planet.Spec.ResourcesPerYearResearch)
}
func TestAutomaticRolesPersistWhileWaitingAfterErrors(t *testing.T) {
	u := colonizerUniverse(t)
	p, f := u.Player(1), u.Fleet("Colony Ship")
	for _, role := range []string{"colonizer:auto", "scout"} {
		p.Automation.FleetRoles[f.Num] = role
		for _, planet := range p.PlanetIntels {
			planet.ReportAge = 0
			planet.PlayerNum = 1
		}
		for range 5 {
			u.Run((*turnGenerator).planColonizationAutomation, (*turnGenerator).planScoutAutomation, (*turnGenerator).pruneAutomationAssignments)
			require.Equal(t, role, p.Automation.FleetRoles[f.Num])
		}
	}
	f.Spec.MiningRate = 0
	p.Automation.FleetRoles[f.Num] = "miner"
	u.Run((*turnGenerator).pruneAutomationAssignments)
	require.Equal(t, "miner", p.Automation.FleetRoles[f.Num])
	require.NoError(t, p.ValidateLogisticsAutomation(p.Automation, u.Game.Fleets))
	delete(p.Automation.FleetRoles, f.Num)
	require.Error(t, p.ValidateLogisticsAutomation(LogisticsAutomation{FleetRoles: map[int]string{f.Num: "miner"}}, u.Game.Fleets))
}
func TestScoutReserveRejectsVerySlowFreeReturn(t *testing.T) {
	u := colonizerUniverse(t)
	p, f := u.Player(1), u.Fleet("Colony Ship")
	p.Automation.FleetRoles[f.Num] = "scout"
	p.Automation.ScoutMaxReturnTurns = 2
	planet := u.Planet("Far")
	planet.Position = Vector{120, 0}
	planet.ReportAge = ReportAgeUnexplored
	route := planExpeditionRoute(&u.Game.Rules, p, f, planet)
	require.Nil(t, route)
	p.Automation.ScoutMaxReturnTurns = 20
	require.NotNil(t, planExpeditionRoute(&u.Game.Rules, p, f, planet))
}
