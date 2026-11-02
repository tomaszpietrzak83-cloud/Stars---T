//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestStarbaseFamilyFollowsLatestAndKeepsExistingStationUntilComplete(t *testing.T) {
	p, planet := newTestPlayerPlanet()
	first := NewShipDesign(p.Num, 2).WithHull(SpaceStation.Name).WithSpec(&rules, p)
	second := NewShipDesign(p.Num, 3).WithHull(SpaceStation.Name).WithSlots([]ShipDesignSlot{{HullComponent: MassDriver5.Name, HullSlotIndex: 1, Quantity: 1}}).WithSpec(&rules, p)
	third := NewShipDesign(p.Num, 4).WithHull(SpaceStation.Name).WithSlots([]ShipDesignSlot{{HullComponent: MassDriver6.Name, HullSlotIndex: 1, Quantity: 1}}).WithSpec(&rules, p)
	p.Designs = append(p.Designs, first, second)
	require.NoError(t, p.RegisterDesignUpgrade(first, second))
	require.NoError(t, p.RegisterDesignUpgrade(second, third))
	p.Designs = append(p.Designs, third)
	require.NoError(t, p.ValidateDesignMaintenance(p.DesignMaintenance))
	base := newStarbase(p, planet, first, "Existing base")
	base.Spec = ComputeFleetSpec(&rules, p, &base)
	planet.Starbase = &base
	planet.Spec = ComputePlanetSpec(&rules, p, planet)
	planet.Spec.ResourcesPerYearAvailable = 0
	planet.Cargo = Cargo{Colonists: 1000}
	planet.ProductionQueue = []ProductionQueueItem{{Type: QueueItemTypeStarbase, DesignNum: second.Num, Quantity: 1, design: second, Allocated: Cost{Resources: 10}}}
	producer := newProducer(testLogger, &rules, planet, p)
	_, err := producer.produce()
	require.NoError(t, err)
	require.Same(t, &base, planet.Starbase, "old station stays operational while new construction is unfunded")
	require.Len(t, planet.ProductionQueue, 1)
	require.Equal(t, third.Num, planet.ProductionQueue[0].DesignNum)
	require.GreaterOrEqual(t, planet.ProductionQueue[0].Allocated.Resources, 10)
	planet.Cargo = Cargo{Ironium: 10000, Boranium: 10000, Germanium: 10000, Colonists: 10000}
	planet.Spec = ComputePlanetSpec(&rules, p, planet)
	planet.Spec.ResourcesPerYearAvailable = 10000
	producer = newProducer(testLogger, &rules, planet, p)
	result, err := producer.produce()
	require.NoError(t, err)
	require.Len(t, result.starbases, 1)
	require.Equal(t, third.Num, planet.Starbase.Tokens[0].DesignNum)
	require.Same(t, &base, result.replacedStarbase)
}

func TestStarbaseFamiliesRejectForeignAndDifferentHullSuccessors(t *testing.T) {
	p, _ := newTestPlayerPlanet()
	first := NewShipDesign(p.Num, 2).WithHull(SpaceStation.Name).WithSpec(&rules, p)
	next := NewShipDesign(p.Num, 3).WithHull(SpaceDock.Name).WithSpec(&rules, p)
	require.Error(t, p.RegisterDesignUpgrade(first, next))
	next.Hull = first.Hull
	next.PlayerNum = p.Num + 1
	require.Error(t, p.RegisterDesignUpgrade(first, next))
}

func TestRetiredQueuedShipsDoNotBlockUnrelatedStarbaseOrder(t *testing.T) {
	p, _ := newTestPlayerPlanet()
	p.DesignMaintenance = DesignMaintenance{2: 3}
	existing := []ProductionQueueItem{{Type: QueueItemTypeShipToken, DesignNum: 2, Quantity: 1, Allocated: Cost{Resources: 5}}}
	incoming := append([]ProductionQueueItem{{Type: QueueItemTypeStarbase, DesignNum: 4, Quantity: 1}}, existing...)
	require.NoError(t, validateRetiredQueueEdits(p, existing, incoming))
	incoming[1].Quantity = 2
	require.Error(t, validateRetiredQueueEdits(p, existing, incoming))
	require.Error(t, validateRetiredQueueEdits(p, existing, append(existing, existing[0])))
}
