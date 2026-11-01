package cs

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"math"
	"slices"
)

const productionBoostCacheTag = "production-boost-plan"

type productionBoostPlan struct {
	Key       string `json:"key"`
	Factories int    `json:"factories"`
	Mines     int    `json:"mines"`
	Alchemy   int    `json:"alchemy"`
}
type productionForecast struct {
	Finish    []int
	Economy   int
	Factories int
	Mines     int
}

func manufacturingItem(item ProductionQueueItem) bool {
	return item.Type == QueueItemTypeShipToken || item.Type == QueueItemTypeStarbase
}

// Run native mining, production and population growth on private copies. Each
// explicit manufacturing queue entry is a separate deadline, including batches.
// deliveryYear is relative to now; mineral cargo is injected on arrival only.
func forecastManufacturing(rules *Rules, player *Player, original *Planet, queue []ProductionQueueItem, deliveryYear int, extra Mineral) (productionForecast, error) {
	p := *original
	p.ProductionQueue = slices.Clone(queue)
	if deliveryYear <= 0 {
		p.Cargo = p.Cargo.AddMineral(extra)
	}
	local := *player
	if local.Spec.Terraform == nil {
		local.Spec = ComputePlayerSpec(&local, rules)
	}
	if p.Starbase != nil {
		copy := *p.Starbase
		copy.Tokens = slices.Clone(copy.Tokens)
		p.Starbase = &copy
	}
	goals := map[int]int{}
	quantities := []int{}
	result := productionForecast{}
	for i := range p.ProductionQueue {
		p.ProductionQueue[i].index = i
		if manufacturingItem(p.ProductionQueue[i]) {
			goals[i] = len(result.Finish)
			result.Finish = append(result.Finish, math.MaxInt)
			quantities = append(quantities, p.ProductionQueue[i].Quantity)
		}
	}
	built := make([]int, len(result.Finish))
	producer := newProducer(slog.Default(), rules, &p, &local)
	producer.estimating = true
	for year := 1; year <= 100 && len(goals) > 0; year++ {
		p.Spec = ComputePlanetSpec(rules, &local, &p)
		if deliveryYear == year {
			p.Cargo = p.Cargo.AddMineral(extra)
		}
		p.mine(rules, p.Spec.MiningOutput, min(p.Mines, p.Spec.MaxPossibleMines))
		production, err := producer.produce()
		if err != nil {
			return productionForecast{}, err
		}
		for _, item := range production.itemsBuilt {
			if goal, ok := goals[item.index]; ok && item.numBuilt > 0 {
				built[goal] += item.numBuilt
				if built[goal] >= quantities[goal] {
					result.Finish[goal] = year
					delete(goals, item.index)
				}
			}
		}
		if len(goals) == 0 {
			break
		}
		p.Spec = ComputePlanetSpec(rules, &local, &p)
		p.grow(&local)
	}
	p.Spec = ComputePlanetSpec(rules, &local, &p)
	result.Factories, result.Mines = p.Factories, p.Mines
	result.Economy = p.Spec.ResourcesPerYearAvailable*4 + p.Spec.MiningOutput.Total()
	return result, nil
}

func productionPlanQueue(original []ProductionQueueItem, plan productionBoostPlan) []ProductionQueueItem {
	q := []ProductionQueueItem{}
	inserted := false
	appendItem := func(item ProductionQueueItem, alchemy bool) {
		if alchemy && (len(q) == 0 || q[len(q)-1].Type != QueueItemTypeAutoMineralAlchemy) {
			q = append(q, ProductionQueueItem{Type: QueueItemTypeAutoMineralAlchemy, Quantity: 1, Tags: Tags{starbaseBoostTag: "production"}})
		}
		q = append(q, item)
	}
	for i, item := range original {
		// Preserve a manually placed alchemy + manufacturing pair.
		before := manufacturingItem(item) || item.Type == QueueItemTypeAutoMineralAlchemy && i+1 < len(original) && manufacturingItem(original[i+1])
		if before && !inserted {
			if plan.Factories > 0 {
				appendItem(autoDevelopmentItem(QueueItemTypeAutoFactories, plan.Factories, starbaseBoostTag), plan.Alchemy == 2)
			}
			if plan.Mines > 0 {
				appendItem(autoDevelopmentItem(QueueItemTypeAutoMines, plan.Mines, starbaseBoostTag), plan.Alchemy == 2)
			}
			inserted = true
		}
		appendItem(item, manufacturingItem(item) && plan.Alchemy > 0)
	}
	return q
}

func manufacturingDatesNoLater(candidate, baseline productionForecast) bool {
	if len(candidate.Finish) != len(baseline.Finish) {
		return false
	}
	for i, year := range candidate.Finish {
		if year > baseline.Finish[i] {
			return false
		}
	}
	return true
}
func manufacturingDatesEarlier(a, b productionForecast) bool {
	for i, year := range a.Finish {
		if year < b.Finish[i] {
			return true
		}
	}
	return false
}
func betterManufacturing(a, b productionForecast) bool {
	// Final completion dominates; earlier queue entries break ties. Economy
	// breaks identical dates, rewarding useful development without delay.
	for i := len(a.Finish) - 1; i >= 0; i-- {
		if a.Finish[i] != b.Finish[i] {
			return a.Finish[i] < b.Finish[i]
		}
	}
	return a.Economy > b.Economy
}

// PlanProductionBoost evaluates temporary auto factories/mines and native auto
// alchemy against the actual explicit ship/base queue. It never spends cargo.
func PlanProductionBoost(rules *Rules, player *Player, planet *Planet) {
	if player.AIControlled {
		return
	}
	queue := make([]ProductionQueueItem, 0, len(planet.ProductionQueue))
	tail := []ProductionQueueItem{}
	hasWork := false
	for _, item := range planet.ProductionQueue {
		if manufacturingItem(item) {
			hasWork = true
		}
	}
	for _, item := range planet.ProductionQueue {
		if item.Type.IsAuto() && item.GetTag(starbaseBoostTag) != "" {
			continue
		}
		if hasWork && item.Type.IsAuto() && item.GetTag(defaultAutoTag) != "" {
			tail = append(tail, item)
			continue
		}
		queue = append(queue, item)
	}
	queue = append(queue, tail...)
	planet.ProductionQueue = queue
	if !hasWork {
		return
	}
	// Estimates are deliberately excluded: UI estimate refreshes must not
	// invalidate the plan, whereas cargo, population, tech or work changes must.
	keyItems := make([]ProductionQueueItem, len(queue))
	for i, item := range queue {
		item.QueueItemCompletionEstimate = QueueItemCompletionEstimate{}
		keyItems[i] = item
	}
	input := struct {
		Cargo                             Cargo
		F, M                              int
		Hab                               Hab
		Conc, MineYears                   Mineral
		Research                          int
		Leftovers                         bool
		Tech                              TechLevel
		Race                              Race
		Queue                             []ProductionQueueItem
		Base                              int
		Resources, MaxFactories, MaxMines int
	}{
		planet.Cargo, planet.Factories, planet.Mines, planet.Hab, planet.MineralConcentration, planet.MineYears, player.ResearchAmount, planet.ContributesOnlyLeftoverToResearch, player.TechLevels, player.Race, keyItems, planet.Spec.StarbaseDesignNum, planet.Spec.ResourcesPerYearAvailable, planet.Spec.MaxFactories, planet.Spec.MaxMines}
	encoded, _ := json.Marshal(input)
	key := fmt.Sprintf("v043-%x", sha256.Sum256(encoded))
	var cached productionBoostPlan
	if json.Unmarshal([]byte(planet.GetTag(productionBoostCacheTag)), &cached) == nil && cached.Key == key {
		planet.ProductionQueue = productionPlanQueue(queue, cached)
		return
	}
	baseline, err := forecastManufacturing(rules, player, planet, queue, 0, Mineral{})
	if err != nil || len(baseline.Finish) == 0 {
		return
	}
	best := productionBoostPlan{Key: key}
	bestResult := baseline
	seen := map[[3]int]bool{}
	try := func(f, m, a int) {
		if player.Race.Spec.InnateResources {
			f = 0
		}
		if player.Race.Spec.InnateMining {
			m = 0
		}
		choice := [3]int{f, m, a}
		if seen[choice] {
			return
		}
		seen[choice] = true
		plan := productionBoostPlan{Key: key, Factories: f, Mines: m, Alchemy: a}
		candidate, err := forecastManufacturing(rules, player, planet, productionPlanQueue(queue, plan), 0, Mineral{})
		if err != nil || !manufacturingDatesNoLater(candidate, baseline) {
			return
		}
		// Alchemy burns resources; use it only for an actual speed gain.
		if a > 0 && !manufacturingDatesEarlier(candidate, baseline) {
			return
		}
		if betterManufacturing(candidate, bestResult) {
			best, bestResult = plan, candidate
		}
	}
	maxF := max(0, planet.Spec.MaxFactories-planet.Factories)
	maxM := max(0, planet.Spec.MaxMines-planet.Mines)
	try(min(1, maxF), min(1, maxM), 0) // Start with Factory then Mine, one unit each.
	baseF := min(maxF, max(1, planet.Spec.ResourcesPerYearAvailable/max(1, player.Race.FactoryCost)/2))
	baseM := min(maxM, max(1, planet.Spec.ResourcesPerYearAvailable/max(1, player.Race.MineCost)/2))
	for _, f := range []int{0, min(1, maxF), baseF, min(maxF, baseF*2)} {
		for _, m := range []int{0, min(1, maxM), baseM, min(maxM, baseM*2)} {
			try(f, m, 0)
		}
	}
	// Expand around the best allocation, avoiding a capacity-squared search
	// on enormous colonies. The native forecast still verifies every choice.
	for range 2 {
		for amount := 1; amount <= max(maxF, maxM) && amount <= 512; amount *= 2 {
			if amount <= maxF {
				try(amount, best.Mines, 0)
			}
			if amount <= maxM {
				try(best.Factories, amount, 0)
			}
		}
	}
	try(maxF, best.Mines, 0)
	try(best.Factories, maxM, 0)
	// Refine in exact adjacent units, including paired factory/mine changes.
	// Each proposal runs native production and protects every explicit deadline.
	for range 256 {
		previous := best
		for _, delta := range [][2]int{{1, 0}, {0, 1}, {1, 1}, {-1, 0}, {0, -1}, {-1, -1}, {1, -1}, {-1, 1}} {
			f, m := previous.Factories+delta[0], previous.Mines+delta[1]
			if f >= 0 && f <= maxF && m >= 0 && m <= maxM {
				try(f, m, 0)
			}
		}
		if best.Factories == previous.Factories && best.Mines == previous.Mines {
			break
		}
	}
	try(0, 0, 1)
	try(best.Factories, best.Mines, 1)
	try(best.Factories, best.Mines, 2)
	planet.ProductionQueue = productionPlanQueue(queue, best)
	planet.Tags = maps.Clone(planet.Tags)
	data, _ := json.Marshal(best)
	planet.SetTag(productionBoostCacheTag, string(data))
}
