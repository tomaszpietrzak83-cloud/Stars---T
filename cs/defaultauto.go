package cs

import (
	"log/slog"
	"math"
	"slices"
)

const defaultAutoTag = "defaultauto"
const starbaseBoostTag = "starbase-boost"
const developmentHorizon = 27

func autoDevelopmentItem(kind QueueItemType, quantity int, tag string) ProductionQueueItem {
	return ProductionQueueItem{Type: kind, Quantity: max(1, quantity), Tags: Tags{tag: "27"}}
}

// Forecasts run the same mining, production, growth and mineral depletion as
// actual turns, on private copies. Incoming transports cannot be predicted;
// their effects enter the next replanning pass before actual production.
func developmentForecast(rules *Rules, player *Player, original *Planet, queue []ProductionQueueItem, targetF, targetM int) (float64, error) {
	p := *original
	p.ProductionQueue = slices.Clone(queue)
	local := *player
	producer := newProducer(slog.Default(), rules, &p, &local)
	producer.estimating = true
	finishF, finishM := 31, 31
	for year := 1; year <= 30; year++ {
		p.Spec = ComputePlanetSpec(rules, &local, &p)
		p.mine(rules, p.Spec.MiningOutput, min(p.Mines, p.Spec.MaxPossibleMines))
		if _, err := producer.produce(); err != nil {
			return math.Inf(1), err
		}
		if p.Factories >= targetF && finishF == 31 {
			finishF = year
		}
		if p.Mines >= targetM && finishM == 31 {
			finishM = year
		}
		if year == developmentHorizon {
			gapF := float64(max(0, targetF-p.Factories)) / float64(max(1, targetF))
			gapM := float64(max(0, targetM-p.Mines)) / float64(max(1, targetM))
			// Reward balanced completion, not factory starvation of Germanium/mines.
			return max(gapF, gapM)*100 + math.Abs(gapF-gapM)*20 + math.Abs(float64(finishF-finishM))*.05, nil
		}
		p.Spec = ComputePlanetSpec(rules, &local, &p)
		p.grow(&local)
	}
	return math.Inf(1), nil
}

func PlanDefaultAuto(rules *Rules, player *Player, planet *Planet) {
	if player.AIControlled || player.Race.Spec.InnateResources && player.Race.Spec.InnateMining {
		return
	}
	queue := make([]ProductionQueueItem, 0, len(planet.ProductionQueue))
	for _, item := range planet.ProductionQueue {
		// Concrete partially paid installations are retained; only temporary auto
		// limits disappear. They never contain allocated minerals or resources.
		if item.Type.IsAuto() && item.GetTag(starbaseBoostTag) != "" {
			continue
		}
		queue = append(queue, item)
	}
	managed, first := false, -1
	for i, item := range queue {
		if item.Type == QueueItemTypeAutoFactories || item.Type == QueueItemTypeAutoMines {
			if first < 0 {
				first = i
			}
			managed = managed || item.GetTag(defaultAutoTag) != "" || item.Quantity == 250
		}
	}
	if first < 0 && len(queue) > 0 && queue[0].Type == QueueItemTypeStarbase {
		managed = true
		first = len(queue)
	}
	if !managed || first < 0 {
		planet.ProductionQueue = queue
		return
	}
	projection := *planet
	local := *player
	for range developmentHorizon {
		projection.Spec = ComputePlanetSpec(rules, &local, &projection)
		projection.grow(&local)
	}
	projection.Spec = ComputePlanetSpec(rules, &local, &projection)
	targetF, targetM := projection.Spec.MaxFactories, projection.Spec.MaxMines
	baseF := max(1, (max(0, targetF-planet.Factories)+developmentHorizon-1)/developmentHorizon)
	baseM := max(1, (max(0, targetM-planet.Mines)+developmentHorizon-1)/developmentHorizon)
	prefix := slices.Clone(queue[:first])
	middle := []ProductionQueueItem{}
	for _, item := range queue[first:] {
		if item.Type != QueueItemTypeAutoFactories && item.Type != QueueItemTypeAutoMines {
			middle = append(middle, item)
		}
	}
	makeQueue := func(f, m int) []ProductionQueueItem {
		q := slices.Clone(prefix)
		if !player.Race.Spec.InnateResources {
			q = append(q, autoDevelopmentItem(QueueItemTypeAutoFactories, f, defaultAutoTag))
		}
		q = append(q, middle...)
		if !player.Race.Spec.InnateMining {
			q = append(q, autoDevelopmentItem(QueueItemTypeAutoMines, m, defaultAutoTag))
		}
		return q
	}
	best, bestScore := makeQueue(baseF, baseM), math.Inf(1)
	for _, f := range []int{max(1, baseF/2), baseF, baseF * 2, baseF * 4} {
		for _, m := range []int{max(1, baseM/2), baseM, baseM * 2, baseM * 4} {
			candidate := makeQueue(f, m)
			score, err := developmentForecast(rules, player, planet, candidate, targetF, targetM)
			score += float64(f+m) * .00001
			if err == nil && score < bestScore {
				best, bestScore = candidate, score
			}
		}
	}
	planet.ProductionQueue = best
	planStarbaseBoost(rules, player, planet, baseF, baseM)
}

func planStarbaseBoost(rules *Rules, player *Player, planet *Planet, baseF, baseM int) {
	if len(planet.ProductionQueue) == 0 || planet.ProductionQueue[0].Type != QueueItemTypeStarbase {
		return
	}
	// Evaluate the actual starbase completion, including Germanium allocation,
	// mine depletion, research allocation, factory growth and its existing payments.
	eta := func(queue []ProductionQueueItem, starbaseIndex int) int {
		copy := *planet
		copy.ProductionQueue = queue
		estimates, _, err := NewCompletionEstimator().GetProductionWithEstimates(rules, player, copy)
		if err != nil || starbaseIndex >= len(estimates) {
			return math.MaxInt
		}
		years := estimates[starbaseIndex].YearsToBuildAll
		if years == Infinite {
			return math.MaxInt
		}
		return years
	}
	original := planet.ProductionQueue
	best, bestETA := original, eta(original, 0)
	for _, f := range []int{0, 1, baseF, baseF * 2} {
		for _, m := range []int{0, 1, baseM, baseM * 2} {
			if f == 0 && m == 0 {
				continue
			}
			q := []ProductionQueueItem{}
			if f > 0 && !player.Race.Spec.InnateResources {
				q = append(q, autoDevelopmentItem(QueueItemTypeAutoFactories, f, starbaseBoostTag))
			}
			if m > 0 && !player.Race.Spec.InnateMining {
				q = append(q, autoDevelopmentItem(QueueItemTypeAutoMines, m, starbaseBoostTag))
			}
			index := len(q)
			q = append(q, original...)
			years := eta(q, index)
			if years < bestETA {
				best, bestETA = q, years
			}
		}
	}
	planet.ProductionQueue = best
}

func (t *turnGenerator) replanDefaultAuto() {
	for _, planet := range t.game.Planets {
		if planet.Owned() {
			PlanDefaultAuto(&t.game.Rules, t.game.getPlayer(planet.PlayerNum), planet)
		}
	}
}
