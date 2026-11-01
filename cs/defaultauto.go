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
	planet.ProductionQueue = slices.DeleteFunc(slices.Clone(planet.ProductionQueue), func(item ProductionQueueItem) bool { return item.GetTag(matureAlchemyTag) != "" })
	defer planMatureAlchemy(player, planet)
	if player.AIControlled || player.Race.Spec.InnateResources && player.Race.Spec.InnateMining {
		PlanProductionBoost(rules, player, planet)
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
		PlanProductionBoost(rules, player, planet)
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
	hasManufacturing := false
	for _, item := range best {
		hasManufacturing = hasManufacturing || manufacturingItem(item)
	}
	if !hasManufacturing && !player.Race.Spec.InnateResources && !player.Race.Spec.InnateMining {
		producer := newProducer(slog.Default(), rules, planet, player)
		fCost, _ := producer.getItemCost(rules, player, planet, ProductionQueueItem{Type: QueueItemTypeFactory})
		mCost, _ := producer.getItemCost(rules, player, planet, ProductionQueueItem{Type: QueueItemTypeMine})
		f, m := 0, 0
		for _, item := range best {
			if item.Type == QueueItemTypeAutoFactories {
				f = item.Quantity
			}
			if item.Type == QueueItemTypeAutoMines {
				m = item.Quantity
			}
		}
		annual := f*fCost.Resources + m*mCost.Resources
		budget := max(0, planet.Spec.ResourcesPerYearAvailable)
		if annual > 0 && budget > annual {
			for i := range best {
				if best[i].Type == QueueItemTypeAutoFactories || best[i].Type == QueueItemTypeAutoMines {
					best[i].Quantity = max(best[i].Quantity, best[i].Quantity*budget/annual)
				}
			}
		}
	}
	planet.ProductionQueue = best
	PlanProductionBoost(rules, player, planet)
}

func (t *turnGenerator) replanDefaultAuto() {
	for _, planet := range t.game.Planets {
		if planet.Owned() {
			PlanDefaultAuto(&t.game.Rules, t.game.getPlayer(planet.PlayerNum), planet)
		}
	}
}
