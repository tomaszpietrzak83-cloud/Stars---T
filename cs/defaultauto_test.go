//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDefaultAutoReplansAfterPopulationAndMineralChangesWithoutSpending(t *testing.T) {
	u := passengerUniverse(t)
	p, planet := u.Player(1), u.Planet("Young colony")
	planet.Mines = 10
	planet.Factories = 10
	planet.MineralConcentration = Mineral{Ironium: 80, Boranium: 70, Germanium: 40}
	planet.ProductionQueue = []ProductionQueueItem{{Type: QueueItemTypeAutoFactories, Quantity: 250}, {Type: QueueItemTypeAutoMines, Quantity: 250}}
	before := planet.Cargo
	PlanDefaultAuto(&u.Game.Rules, p, planet)
	require.Equal(t, before, planet.Cargo)
	require.Equal(t, "27", planet.ProductionQueue[0].GetTag(defaultAutoTag))
	firstF, firstM := planet.ProductionQueue[0].Quantity, planet.ProductionQueue[len(planet.ProductionQueue)-1].Quantity
	planet.Cargo.Colonists = 6000
	planet.Cargo.Germanium = 1000
	PlanDefaultAuto(&u.Game.Rules, p, planet)
	require.True(t, firstF != planet.ProductionQueue[0].Quantity || firstM != planet.ProductionQueue[len(planet.ProductionQueue)-1].Quantity)
	require.Equal(t, 10, planet.Factories)
	require.Equal(t, 10, planet.Mines)
	require.Equal(t, 6000, planet.Cargo.Colonists)
}
func TestStarbaseBoostMustImproveNativeCompletionAndIsTemporary(t *testing.T) {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Designs: Designs(DesignSpaceStation)}}, Planets: []ScenarioPlanet{{Name: "New colony", Owner: 1, Cargo: Cargo{Colonists: 1000, Ironium: 500, Boranium: 500}}}})
	p, planet := u.Player(1), u.Planet("New colony")
	planet.Mines = 1
	planet.Factories = 10
	planet.MineralConcentration = Mineral{100, 100, 100}
	planet.Spec = ComputePlanetSpec(&u.Game.Rules, p, planet)
	planet.ProductionQueue = []ProductionQueueItem{{Type: QueueItemTypeStarbase, DesignNum: p.Designs[0].Num, Quantity: 1}, {Type: QueueItemTypeAutoFactories, Quantity: 250}, {Type: QueueItemTypeAutoMines, Quantity: 250}}
	require.NoError(t, planet.PopulateProductionQueueDesigns(p))
	original, _, err := NewCompletionEstimator().GetProductionWithEstimates(&u.Game.Rules, p, *planet)
	require.NoError(t, err)
	before := planet.Cargo
	PlanDefaultAuto(&u.Game.Rules, p, planet)
	require.Equal(t, before, planet.Cargo)
	boosted, _, err := NewCompletionEstimator().GetProductionWithEstimates(&u.Game.Rules, p, *planet)
	require.NoError(t, err)
	baseIndex := -1
	for i, item := range boosted {
		if item.Type == QueueItemTypeStarbase {
			baseIndex = i
			break
		}
	}
	require.Greater(t, baseIndex, 0)
	require.NotEqual(t, Infinite, boosted[baseIndex].YearsToBuildAll)
	if original[0].YearsToBuildAll != Infinite {
		require.Less(t, boosted[baseIndex].YearsToBuildAll, original[0].YearsToBuildAll)
	}
	// Model completion: native production removes the concrete base item. The
	// temporary auto limits must then disappear while defaultauto remains.
	queue := planet.ProductionQueue[:0]
	for _, item := range planet.ProductionQueue {
		if item.Type != QueueItemTypeStarbase {
			queue = append(queue, item)
		}
	}
	planet.ProductionQueue = queue
	PlanDefaultAuto(&u.Game.Rules, p, planet)
	for _, item := range planet.ProductionQueue {
		require.Empty(t, item.GetTag(starbaseBoostTag))
	}
	require.Equal(t, "27", planet.ProductionQueue[0].GetTag(defaultAutoTag))
}
