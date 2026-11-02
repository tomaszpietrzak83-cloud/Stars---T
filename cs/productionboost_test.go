//go:build !wasi && !wasm

package cs

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"math"
	"slices"
	"testing"
)

func manufacturingTestWorld(t *testing.T) (*testUniverse, *Player, *Planet, *ShipDesign) {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Designs: Designs(DesignSpaceStation, DesignTeamster)}}, Planets: []ScenarioPlanet{{Name: "Yard", Owner: 1, Starbase: DesignSpaceStation.Name, Cargo: Cargo{Colonists: 2500, Ironium: 10000, Boranium: 10000, Germanium: 10000}}}})
	p, planet := u.Player(1), u.Planet("Yard")
	p.Race.GrowthRate = 0
	p.Race.FactoryCost = 25
	p.Race.FactoryOutput = 10
	p.Race.NumFactories = 1
	p.Race.MineCost = 8
	p.ResearchAmount = 0
	p.Race.Spec = ComputeRaceSpec(&p.Race, &u.Game.Rules)
	p.Spec = ComputePlayerSpec(p, &u.Game.Rules)
	planet.Factories, planet.Mines = 0, 0
	planet.Hab = Hab{50, 50, 50}
	planet.BaseHab = planet.Hab
	planet.MineralConcentration = Mineral{100, 100, 100}
	planet.Spec = ComputePlanetSpec(&u.Game.Rules, p, planet)
	d := p.GetDesignByName(DesignTeamster.Name)
	cost, err := NewCostCalculator().GetDesignCost(&u.Game.Rules, p.TechLevels, p.Race.Spec, d)
	require.NoError(t, err)
	quantity := max(1, int(math.Ceil(float64(6*planet.Spec.ResourcesPerYearAvailable+planet.Spec.ResourcesPerYearAvailable/3)/float64(max(1, cost.Resources)))))
	planet.ProductionQueue = []ProductionQueueItem{{Type: QueueItemTypeShipToken, DesignNum: d.Num, Quantity: quantity, design: d}}
	return u, p, planet, d
}

func TestManufacturingAllowsUsefulInvestmentAtSameSevenYearCompletion(t *testing.T) {
	u, p, planet, _ := manufacturingTestWorld(t)
	baseline, err := forecastManufacturing(&u.Game.Rules, p, planet, planet.ProductionQueue, 0, Mineral{})
	require.NoError(t, err)
	require.Equal(t, []int{7}, baseline.Finish)
	cargo, concentration := planet.Cargo, planet.MineralConcentration
	playerBefore, err := json.Marshal(p)
	require.NoError(t, err)
	PlanDefaultAuto(&u.Game.Rules, p, planet)
	result, err := forecastManufacturing(&u.Game.Rules, p, planet, planet.ProductionQueue, 0, Mineral{})
	require.NoError(t, err)
	require.Equal(t, baseline.Finish, result.Finish)
	require.Greater(t, result.Economy, baseline.Economy)
	require.NotEmpty(t, planet.ProductionQueue[0].GetTag(starbaseBoostTag))
	require.Equal(t, cargo, planet.Cargo)
	require.Equal(t, concentration, planet.MineralConcentration)
	require.Zero(t, planet.Factories)
	require.Zero(t, planet.Mines)
	playerAfter, err := json.Marshal(p)
	require.NoError(t, err)
	require.Equal(t, string(playerBefore), string(playerAfter))
	t.Logf("Same 7-year manufacturing completion, native projected infrastructure %d factories/%d mines", result.Factories, result.Mines)
	queue, _ := json.Marshal(planet.ProductionQueue)
	PlanDefaultAuto(&u.Game.Rules, p, planet)
	again, _ := json.Marshal(planet.ProductionQueue)
	require.Equal(t, string(queue), string(again), "cached repeated UI estimates must not duplicate or grow boosters")
}

func TestManufacturingBoostProtectsEachExplicitOrderAndRemovesItselfAfterWork(t *testing.T) {
	u, p, planet, d := manufacturingTestWorld(t)
	planet.ProductionQueue[0].Quantity *= 6
	planet.ProductionQueue = append(planet.ProductionQueue, ProductionQueueItem{Type: QueueItemTypeShipToken, DesignNum: d.Num, Quantity: 2, design: d})
	baseline, err := forecastManufacturing(&u.Game.Rules, p, planet, planet.ProductionQueue, 0, Mineral{})
	require.NoError(t, err)
	PlanDefaultAuto(&u.Game.Rules, p, planet)
	after, err := forecastManufacturing(&u.Game.Rules, p, planet, planet.ProductionQueue, 0, Mineral{})
	require.NoError(t, err)
	require.True(t, manufacturingDatesNoLater(after, baseline))
	require.Greater(t, baseline.Finish[0], 30)
	require.Greater(t, after.Factories, baseline.Factories, "a long batch should justify factory investment")
	t.Logf("Batch completion %v -> %v; factories %d -> %d", baseline.Finish, after.Finish, baseline.Factories, after.Factories)
	for _, item := range planet.ProductionQueue {
		if manufacturingItem(item) {
			require.Equal(t, d.Num, item.DesignNum)
		}
	}
	queue := planet.ProductionQueue[:0]
	for _, item := range planet.ProductionQueue {
		if !manufacturingItem(item) {
			queue = append(queue, item)
		}
	}
	planet.ProductionQueue = queue
	PlanDefaultAuto(&u.Game.Rules, p, planet)
	for _, item := range planet.ProductionQueue {
		require.Empty(t, item.GetTag(starbaseBoostTag))
	}
}

func TestManufacturingAtCapacityDoesNotAddEmptyInvestmentOrUselessAlchemy(t *testing.T) {
	u, p, planet, _ := manufacturingTestWorld(t)
	planet.Factories = planet.Spec.MaxFactories
	planet.Mines = planet.Spec.MaxMines
	planet.Spec = ComputePlanetSpec(&u.Game.Rules, p, planet)
	PlanDefaultAuto(&u.Game.Rules, p, planet)
	for _, item := range planet.ProductionQueue {
		require.Empty(t, item.GetTag(starbaseBoostTag))
	}
}

func TestManufacturingAlchemyMustReallyAccelerateMineralBlockedWork(t *testing.T) {
	u, p, planet, d := manufacturingTestWorld(t)
	p.Race.NumMines, p.Race.NumFactories = 0, 0
	p.Race.Spec = ComputeRaceSpec(&p.Race, &u.Game.Rules)
	p.Spec = ComputePlayerSpec(p, &u.Game.Rules)
	planet.Cargo = Cargo{Colonists: 2500}
	planet.ProductionQueue[0].Quantity = 1
	_ = d
	planet.Spec = ComputePlanetSpec(&u.Game.Rules, p, planet)
	planet.Factories, planet.Mines = planet.Spec.MaxFactories, planet.Spec.MaxMines
	planet.Spec = ComputePlanetSpec(&u.Game.Rules, p, planet)
	baseline, err := forecastManufacturing(&u.Game.Rules, p, planet, planet.ProductionQueue, 0, Mineral{})
	require.NoError(t, err)
	require.Greater(t, baseline.Finish[0], 1)
	PlanDefaultAuto(&u.Game.Rules, p, planet)
	after, err := forecastManufacturing(&u.Game.Rules, p, planet, planet.ProductionQueue, 0, Mineral{})
	require.NoError(t, err)
	require.Less(t, after.Finish[0], 100)
	require.True(t, slices.ContainsFunc(planet.ProductionQueue, func(item ProductionQueueItem) bool { return item.Type == QueueItemTypeAutoMineralAlchemy }), "queue: %+v; capacity: %d/%d, %d/%d", planet.ProductionQueue, planet.Factories, planet.Spec.MaxFactories, planet.Mines, planet.Spec.MaxMines)
	require.Zero(t, planet.Cargo.Ironium, "forecast must not create real minerals")
}

func TestManufacturingDeliveryUsesArrivalAndStockWindowIsBounded(t *testing.T) {
	u, p, planet, d := manufacturingTestWorld(t)
	planet.ProductionQueue[0].Quantity = 100
	stock := manufacturingStockTarget(&u.Game.Rules, p, planet)
	require.GreaterOrEqual(t, stock.Ironium, d.Spec.Cost.Ironium)
	require.Less(t, stock.Ironium, 100*d.Spec.Cost.Ironium)
	planet.ProductionQueue[0].Quantity = 1
	planet.Mines = 1
	planet.Cargo = Cargo{Colonists: 2500}
	_ = d
	planet.Spec = ComputePlanetSpec(&u.Game.Rules, p, planet)
	base, err := forecastManufacturing(&u.Game.Rules, p, planet, planet.ProductionQueue, 0, Mineral{})
	require.NoError(t, err)
	require.Less(t, base.Finish[0], 100)
	load := Mineral{10, 10, 10}
	require.True(t, manufacturingDeliveryUseful(&u.Game.Rules, p, planet, load, 1))
	require.True(t, manufacturingDeliveryUseful(&u.Game.Rules, p, planet, load, base.Finish[0]+5), "the new shipyard stock floor remains useful for later orders")
	planet.Cargo = Cargo{Ironium: 300, Boranium: 300, Germanium: 300, Colonists: 2500}
	require.False(t, manufacturingDeliveryUseful(&u.Game.Rules, p, planet, load, base.Finish[0]+5), "a full stock and completed queue must not request late redundant delivery")
}

func TestSevenYearStarbaseCanAddMineWithoutDelayingIt(t *testing.T) {
	u, p, planet, _ := manufacturingTestWorld(t)
	planet.Starbase = nil
	base := p.GetDesignByName(DesignSpaceStation.Name)
	cost, err := NewCostCalculator().GetDesignCost(&u.Game.Rules, p.TechLevels, p.Race.Spec, base)
	require.NoError(t, err)
	resources := (cost.Resources + 14 + 6) / 7
	planet.Cargo.Colonists = max(1, resources*p.Race.PopEfficiency)
	planet.Spec = ComputePlanetSpec(&u.Game.Rules, p, planet)
	planet.Mines = max(0, planet.Spec.MaxMines-1)
	planet.Spec = ComputePlanetSpec(&u.Game.Rules, p, planet)
	planet.ProductionQueue = []ProductionQueueItem{{Type: QueueItemTypeStarbase, DesignNum: base.Num, Quantity: 1, design: base}}
	before, err := forecastManufacturing(&u.Game.Rules, p, planet, planet.ProductionQueue, 0, Mineral{})
	require.NoError(t, err)
	require.Equal(t, []int{7}, before.Finish)
	require.NoError(t, NewOrderer().UpdatePlanetOrders(&u.Game.Rules, p, planet, planet.PlanetOrders))
	after, err := forecastManufacturing(&u.Game.Rules, p, planet, planet.ProductionQueue, 0, Mineral{})
	require.NoError(t, err)
	require.Equal(t, before.Finish, after.Finish)
	require.Greater(t, after.Economy, before.Economy)
	require.NotEmpty(t, planet.ProductionQueue[0].GetTag(starbaseBoostTag))
}

func TestProductionPreviewKeepsCurrentGameStateAndPaidAllocations(t *testing.T) {
	u, p, planet, _ := manufacturingTestWorld(t)
	planet.ProductionQueue[0].Allocated = Cost{Resources: 3, Ironium: 1}
	before, err := json.Marshal(u.Game)
	require.NoError(t, err)
	preview, err := u.Game.PreviewProductionOrders(p.Num)
	require.NoError(t, err)
	after, err := json.Marshal(u.Game)
	require.NoError(t, err)
	require.Equal(t, string(before), string(after))
	require.Equal(t, u.Game.Year, preview.Year)
	for _, change := range preview.Planets {
		found := false
		for _, item := range change.ProductionQueue {
			if manufacturingItem(item) {
				require.Equal(t, planet.ProductionQueue[0].Allocated, item.Allocated)
				found = true
			}
		}
		require.True(t, found)
	}
}

func TestNewAssemblyDoesNotJumpAheadOfPreviouslyBoostedManufacturing(t *testing.T) {
	u := maintenanceUniverse(t)
	p := u.Player(1)
	p.Automation.FleetAssemblies = map[int]FleetAssembly{1: assemblyOrder(u, 100, true)}
	second := assemblyOrder(u, 10, true)
	u.Run((*turnGenerator).planFleetAssemblies)
	second.Name = "Second task force"
	second.SourceFleets = nil
	p.Automation.FleetAssemblies[2] = second
	u.Run((*turnGenerator).planFleetAssemblies)
	firstAt, secondAt := -1, -1
	for i, item := range u.Planet("Yard").ProductionQueue {
		if item.Tags[assemblyTag] == "1" && firstAt < 0 {
			firstAt = i
		}
		if item.Tags[assemblyTag] == "2" && secondAt < 0 {
			secondAt = i
		}
	}
	require.GreaterOrEqual(t, firstAt, 0)
	require.Greater(t, secondAt, firstAt, "temporary auto investment must not make newer fleet orders jump ahead")
}

func TestUnitRefinementDoesNotLeaveAnImprovingAdjacentShipBatchInvestment(t *testing.T) {
	u, p, planet, d := manufacturingTestWorld(t)
	second := *d
	second.Num = 90
	second.Name = "Second queued ship"
	p.Designs = append(p.Designs, &second)
	original := []ProductionQueueItem{{Type: QueueItemTypeShipToken, DesignNum: d.Num, Quantity: 30, design: d}, {Type: QueueItemTypeShipToken, DesignNum: second.Num, Quantity: 12, design: &second}}
	planet.ProductionQueue = original
	baseline, err := forecastManufacturing(&u.Game.Rules, p, planet, original, 0, Mineral{})
	require.NoError(t, err)
	PlanDefaultAuto(&u.Game.Rules, p, planet)
	result, err := forecastManufacturing(&u.Game.Rules, p, planet, planet.ProductionQueue, 0, Mineral{})
	require.NoError(t, err)
	require.True(t, manufacturingDatesNoLater(result, baseline))
	plan := productionBoostPlan{}
	for _, item := range planet.ProductionQueue {
		if item.GetTag(starbaseBoostTag) != "" {
			switch item.Type {
			case QueueItemTypeAutoFactories:
				plan.Factories = item.Quantity
			case QueueItemTypeAutoMines:
				plan.Mines = item.Quantity
			}
		}
	}
	for _, delta := range [][2]int{{1, 0}, {0, 1}, {1, 1}, {-1, 0}, {0, -1}, {-1, -1}, {1, -1}, {-1, 1}} {
		f, m := plan.Factories+delta[0], plan.Mines+delta[1]
		if f < 0 || m < 0 || f > planet.Spec.MaxFactories-planet.Factories || m > planet.Spec.MaxMines-planet.Mines {
			continue
		}
		candidate, err := forecastManufacturing(&u.Game.Rules, p, planet, productionPlanQueue(original, productionBoostPlan{Factories: f, Mines: m}), 0, Mineral{})
		require.NoError(t, err)
		if manufacturingDatesNoLater(candidate, baseline) {
			require.False(t, betterManufacturing(candidate, result), "an adjacent one-unit improvement was missed: %d/%d", f, m)
		}
	}
}

func TestEightShipsCannotBeDelayedByAutomaticDevelopmentOrCache(t *testing.T) {
	u, p, planet, d := manufacturingTestWorld(t)
	planet.Cargo.Colonists = 9000
	p.Race.NumFactories = 15
	p.Race.Spec = ComputeRaceSpec(&p.Race, &u.Game.Rules)
	planet.Factories = 1000
	planet.Spec = ComputePlanetSpec(&u.Game.Rules, p, planet)
	batch := ProductionQueueItem{Type: QueueItemTypeShipToken, DesignNum: d.Num, Quantity: 8, design: d}
	baseline := []ProductionQueueItem{batch}
	planet.ProductionQueue = []ProductionQueueItem{{Type: QueueItemTypeAutoFactories, Quantity: 70}, {Type: QueueItemTypeAutoMines, Quantity: 72}, batch}
	PlanProductionBoost(&u.Game.Rules, p, planet)
	require.True(t, displayedManufacturingNoLater(&u.Game.Rules, p, planet, planet.ProductionQueue, baseline))
	first := planet.ProductionQueue
	PlanProductionBoost(&u.Game.Rules, p, planet)
	require.True(t, displayedManufacturingNoLater(&u.Game.Rules, p, planet, planet.ProductionQueue, baseline))
	require.Equal(t, first, planet.ProductionQueue)
}

func TestAutomaticAlchemyIsForbiddenBelowInstallationCapacity(t *testing.T) {
	u, p, planet, _ := manufacturingTestWorld(t)
	planet.Cargo.Ironium, planet.Cargo.Boranium, planet.Cargo.Germanium = 0, 0, 0
	require.Less(t, planet.Factories, planet.Spec.MaxFactories)
	PlanDefaultAuto(&u.Game.Rules, p, planet)
	for _, item := range planet.ProductionQueue {
		require.False(t, item.Type == QueueItemTypeAutoMineralAlchemy && item.GetTag(starbaseBoostTag) != "")
	}
}
