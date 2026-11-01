//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMatureEconomySplitsResearchAndAlchemyUntilShipsAreOrdered(t *testing.T) {
	u, p, planet, d := manufacturingTestWorld(t)
	planet.ProductionQueue = nil
	planet.Factories, planet.Mines = planet.Spec.MaxFactories, planet.Spec.MaxMines
	p.ResearchAmount = 20
	planet.Spec = ComputePlanetSpec(&u.Game.Rules, p, planet)
	require.Equal(t, planet.Spec.ResourcesPerYear/2, planet.Spec.ResourcesPerYearResearch)
	PlanDefaultAuto(&u.Game.Rules, p, planet)
	require.Len(t, planet.ProductionQueue, 1)
	require.Equal(t, QueueItemTypeAutoMineralAlchemy, planet.ProductionQueue[0].Type)
	rate := 75
	p.Automation.ResearchTax = &ResearchTaxPolicy{MatureResearchPercent: &rate}
	planet.Spec = ComputePlanetSpec(&u.Game.Rules, p, planet)
	require.Equal(t, planet.Spec.ResourcesPerYear*75/100, planet.Spec.ResourcesPerYearResearch)
	planet.ProductionQueue = append([]ProductionQueueItem{{Type: QueueItemTypeShipToken, DesignNum: d.Num, Quantity: 1, design: d}}, planet.ProductionQueue...)
	planet.Spec = ComputePlanetSpec(&u.Game.Rules, p, planet)
	require.Equal(t, planet.Spec.ResourcesPerYear*20/100, planet.Spec.ResourcesPerYearResearch)
	PlanDefaultAuto(&u.Game.Rules, p, planet)
	for _, item := range planet.ProductionQueue {
		require.Empty(t, item.GetTag(matureAlchemyTag))
	}
	planet.ProductionQueue = nil
	p.Automation.ResearchTax.DisableMatureEconomy = true
	planet.Spec = ComputePlanetSpec(&u.Game.Rules, p, planet)
	require.Equal(t, planet.Spec.ResourcesPerYear*20/100, planet.Spec.ResourcesPerYearResearch)
}

func TestMatureEconomyAcceptsZeroResearchAndRejectsInvalidPercent(t *testing.T) {
	zero := 0
	policy := ResearchTaxPolicy{MatureResearchPercent: &zero}
	require.NoError(t, policy.Validate())
	invalid := 101
	policy.MatureResearchPercent = &invalid
	require.Error(t, policy.Validate())
}
