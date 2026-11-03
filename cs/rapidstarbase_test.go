//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func rapidBaseDesigns(t *testing.T) (*Player, *Planet, *ShipDesign, *ShipDesign) {
	p, planet := newProductionTestPlanet(10_000, Mineral{10_000, 10_000, 10_000})
	p.TechLevels = TechLevel{26, 26, 26, 26, 26, 26}
	old := NewShipDesign(p.Num, 1).WithHull(SpaceStation.Name).WithSlots([]ShipDesignSlot{
		{HullComponent: SyncroSapper.Name, HullSlotIndex: 2, Quantity: 16},
		{HullComponent: SyncroSapper.Name, HullSlotIndex: 4, Quantity: 16},
	}).WithSpec(&rules, p)
	next := NewShipDesign(p.Num, 2).WithHull(SpaceStation.Name).WithSlots([]ShipDesignSlot{{HullComponent: MegaDisruptor.Name, HullSlotIndex: 2, Quantity: 10}}).WithSpec(&rules, p)
	old.Version, next.Version = 2, 3
	old.Spec.VersionCreatedYear, next.Spec.VersionCreatedYear = 2440, 2444
	next.Spec.PreviousVersionNum = old.Num
	p.Automation.DesignFamilies = map[int]int{1: -1, 2: -1}
	p.Designs = []*ShipDesign{old, next}
	base := newStarbase(p, planet, old, "Existing base")
	base.Spec = ComputeFleetSpec(&rules, p, &base)
	planet.Starbase = &base
	planet.Spec.PlanetStarbaseSpec = computePlanetStarbaseSpec(planet)
	return p, planet, old, next
}

func TestRapidStarbaseUsesNinetyPercentOldCreditWithoutStacking(t *testing.T) {
	p, _, old, next := rapidBaseDesigns(t)
	c := NewCostCalculator()
	normal, err := c.StarbaseUpgradeCost(&rules, p.TechLevels, p.Race.Spec, old, next)
	require.NoError(t, err)
	oldCost, err := c.GetDesignCost(&rules, p.TechLevels, p.Race.Spec, old)
	require.NoError(t, err)
	newCost, err := c.GetDesignCost(&rules, p.TechLevels, p.Race.Spec, next)
	require.NoError(t, err)
	rapid := newCost.Subtract(Cost{oldCost.Ironium * 9 / 10, oldCost.Boranium * 9 / 10, oldCost.Germanium * 9 / 10, oldCost.Resources * 9 / 10}).MinZero()
	got, err := c.StarbaseUpgradeCostForPlayer(&rules, p, old, next)
	require.NoError(t, err)
	require.Equal(t, Cost{min(normal.Ironium, rapid.Ironium), min(normal.Boranium, rapid.Boranium), min(normal.Germanium, rapid.Germanium), min(normal.Resources, rapid.Resources)}, got)
	require.Less(t, got.Resources, normal.Resources, "qualifying replacement must gain a real discount")
	for _, kind := range CostTypes {
		require.GreaterOrEqual(t, got.GetAmount(kind), 0)
	}
}

func TestRapidStarbaseKeepsBetterComponentReuseAndUnknownHistory(t *testing.T) {
	p, _, old, next := rapidBaseDesigns(t)
	c := NewCostCalculator()
	for _, change := range []func(){
		func() { next.Spec.VersionCreatedYear = 2445 },
		func() { old.Spec.VersionCreatedYear = 0 },
		func() { old.Spec.VersionCreatedYear = 2440; next.Spec.VersionCreatedYear = 2444; next.Version = 4 },
		func() { next.Version = 3; next.Spec.PreviousVersionNum = 99 },
	} {
		change()
		normal, err := c.StarbaseUpgradeCost(&rules, p.TechLevels, p.Race.Spec, old, next)
		require.NoError(t, err)
		got, err := c.StarbaseUpgradeCostForPlayer(&rules, p, old, next)
		require.NoError(t, err)
		require.Equal(t, normal, got)
	}
	next.Version = 3
	next.Spec.PreviousVersionNum = old.Num
	next.Slots = append([]ShipDesignSlot(nil), old.Slots...)
	got, err := c.StarbaseUpgradeCostForPlayer(&rules, p, old, next)
	require.NoError(t, err)
	require.Zero(t, got, "identical parts already have a full reuse credit")
}

func TestRapidStarbaseProductionKeepsOldBaseUntilFullyPaid(t *testing.T) {
	p, planet, old, next := rapidBaseDesigns(t)
	cost, err := NewCostCalculator().StarbaseUpgradeCostForPlayer(&rules, p, old, next)
	require.NoError(t, err)
	require.Positive(t, cost.ToMineral().Total())
	original := planet.Starbase
	planet.Cargo = Cost{}.ToCargo()
	planet.ProductionQueue = []ProductionQueueItem{{Type: QueueItemTypeStarbase, Quantity: 1, DesignNum: next.Num, design: next}}
	producer := newProducer(testLogger, &rules, planet, p)
	_, err = producer.produce()
	require.NoError(t, err)
	require.Same(t, original, planet.Starbase)
	require.Equal(t, old.Num, planet.Starbase.Tokens[0].DesignNum)
	planet.Cargo = cost.ToMineral().ToCargo()
	producer = newProducer(testLogger, &rules, planet, p)
	result, err := producer.produce()
	require.NoError(t, err)
	require.Len(t, result.starbases, 1)
	require.Equal(t, next.Num, planet.Starbase.Tokens[0].DesignNum)
	require.Empty(t, planet.ProductionQueue)
}
