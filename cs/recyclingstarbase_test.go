//go:build !wasi && !wasm

package cs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func recyclingBaseDesigns(t *testing.T) (*Player, *Planet, *ShipDesign, Cost) {
	p, planet, old, next := rapidBaseDesigns(t)
	p.Race.WithLRT(UR)
	p.Race.Spec = ComputeRaceSpec(&p.Race, &rules)
	next.Slots[0].Quantity = 20
	next = next.WithSpec(&rules, p)
	// Exercise manual orders without the rapid-version discount or family tags.
	p.Automation.DesignFamilies = nil
	old.Spec.VersionCreatedYear = 0
	c := NewCostCalculator()
	normal, err := c.StarbaseUpgradeCost(&rules, p.TechLevels, p.Race.Spec, old, next)
	require.NoError(t, err)
	oldCost, err := c.GetDesignCost(&rules, p.TechLevels, p.Race.Spec, old)
	require.NoError(t, err)
	newCost, err := c.GetDesignCost(&rules, p.TechLevels, p.Race.Spec, next)
	require.NoError(t, err)
	credit := MultiplyCost(oldCost, rules.ScrapMineralAmount+p.Race.Spec.ScrapMineralOffsetStarbase)
	credit.Resources = int(float64(oldCost.Resources)*(rules.ScrapResourceAmount+p.Race.Spec.ScrapResourcesOffsetStarbase) + .5)
	net := newCost.Subtract(credit).MinZero()
	want := Cost{min(normal.Ironium, net.Ironium), min(normal.Boranium, net.Boranium), min(normal.Germanium, net.Germanium), min(normal.Resources, net.Resources)}
	cost, err := c.StarbaseUpgradeCostForPlayer(&rules, p, old, next)
	require.NoError(t, err)
	require.Equal(t, want, cost)
	require.LessOrEqual(t, cost.Resources, normal.Resources)
	planet.ProductionQueue = []ProductionQueueItem{{Type: QueueItemTypeStarbase, Quantity: 1, DesignNum: next.Num, design: next}}
	return p, planet, next, cost
}

func TestRecyclingStarbaseImprovesOrdinaryRefitCost(t *testing.T) {
	p, _, old, next := rapidBaseDesigns(t)
	p.Race.WithLRT(UR)
	p.Race.Spec = ComputeRaceSpec(&p.Race, &rules)
	old.Spec.VersionCreatedYear = 0
	c := NewCostCalculator()
	normal, err := c.StarbaseUpgradeCost(&rules, p.TechLevels, p.Race.Spec, old, next)
	require.NoError(t, err)
	cost, err := c.StarbaseUpgradeCostForPlayer(&rules, p, old, next)
	require.NoError(t, err)
	require.Less(t, cost.Resources, normal.Resources)
}

func TestRecyclingStarbaseCompletesWithRecoveredCost(t *testing.T) {
	p, planet, next, cost := recyclingBaseDesigns(t)
	planet.Cargo = cost.ToCargo()
	planet.Spec.ResourcesPerYearAvailable = cost.Resources
	producer := newProducer(testLogger, &rules, planet, p)
	result, err := producer.produce()
	require.NoError(t, err)
	require.Len(t, result.starbases, 1)
	require.Equal(t, next.Num, planet.Starbase.Tokens[0].DesignNum)
	require.Empty(t, planet.ProductionQueue)
	require.Zero(t, result.leftoverResources)
	require.Zero(t, planet.bonusResources)
}

func TestRecyclingStarbaseAccumulatesAndDoesNotRepeatRefund(t *testing.T) {
	p, planet, next, cost := recyclingBaseDesigns(t)
	original := planet.Starbase
	require.Positive(t, cost.Resources)
	require.Positive(t, cost.ToMineral().Total())
	planet.Cargo = Cargo{}
	planet.Spec.ResourcesPerYearAvailable = max(1, cost.Resources/3)
	producer := newProducer(testLogger, &rules, planet, p)
	for turn := 1; turn <= 4; turn++ {
		result, err := producer.produce()
		require.NoError(t, err)
		require.Empty(t, result.starbases)
		require.Same(t, original, planet.Starbase)
		require.Equal(t, min(cost.Resources, turn*planet.Spec.ResourcesPerYearAvailable), planet.ProductionQueue[0].Allocated.Resources)
	}
	planet.Spec.ResourcesPerYearAvailable = 0
	planet.Cargo = cost.ToCargo()
	result, err := producer.produce()
	require.NoError(t, err)
	require.Len(t, result.starbases, 1)
	require.Equal(t, next.Num, planet.Starbase.Tokens[0].DesignNum)
	require.Empty(t, planet.ProductionQueue)
	result, err = producer.produce()
	require.NoError(t, err)
	require.Empty(t, result.starbases)
	require.Zero(t, result.leftoverResources)
}
