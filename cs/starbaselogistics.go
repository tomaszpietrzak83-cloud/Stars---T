package cs

import (
	"log/slog"
	"math"
	"slices"
)

func queuedStarbase(planet *Planet) bool {
	for _, item := range planet.ProductionQueue {
		if item.Type == QueueItemTypeStarbase {
			return true
		}
	}
	return false
}
func starbaseCompletion(rules *Rules, p *Player, planet Planet, extra Mineral) int {
	planet.Cargo = planet.Cargo.AddMineral(extra)
	items, _, err := NewCompletionEstimator().GetProductionWithEstimates(rules, p, planet)
	if err != nil {
		return math.MaxInt
	}
	for _, item := range items {
		if item.Type == QueueItemTypeStarbase {
			if item.YearsToBuildAll == Infinite {
				return math.MaxInt
			}
			return item.YearsToBuildAll
		}
	}
	return math.MaxInt
}

// On good worlds, carry construction stock opportunistically with passengers.
// Predict their own mining/production first; Germanium is left to factories.
func (t *turnGenerator) prefetchStarbaseMinerals(p *Player, dest *Planet) Mineral {
	if dest == nil || !dest.OwnedBy(p.Num) || dest.Spec.HasStarbase || prospectivePlanetQuality(p, dest) < 80 && (!p.Automation.OccupationSupport[dest.Num] || p.Automation.transportPolicy().DisableCapturedSupport) || queuedStarbase(dest) {
		return Mineral{}
	}
	var design *ShipDesign
	for _, d := range p.Designs {
		if !d.Spec.Starbase || d.Spec.SpaceDock == 0 {
			continue
		}
		if design == nil || d.Spec.Cost.Resources < design.Spec.Cost.Resources {
			design = d
		}
	}
	if design == nil {
		return Mineral{}
	}
	future := *dest
	future.ProductionQueue = slices.Clone(dest.ProductionQueue)
	local := *p
	producer := newProducer(slog.Default(), &t.game.Rules, &future, &local)
	producer.estimating = true
	cost, err := producer.getItemCost(&t.game.Rules, &local, &future, ProductionQueueItem{Type: QueueItemTypeStarbase, DesignNum: design.Num, Quantity: 1, design: design})
	if err != nil {
		return Mineral{}
	}
	years := min(10, max(3, int(math.Ceil(float64(cost.Resources)/float64(max(1, future.Spec.ResourcesPerYearAvailable))))))
	for range years {
		future.Spec = ComputePlanetSpec(&t.game.Rules, &local, &future)
		future.mine(&t.game.Rules, future.Spec.MiningOutput, min(future.Mines, future.Spec.MaxPossibleMines))
		if _, err := producer.produce(); err != nil {
			return Mineral{}
		}
		future.Spec = ComputePlanetSpec(&t.game.Rules, &local, &future)
		future.grow(&local)
	}
	return Mineral{Ironium: max(0, cost.Ironium-future.Cargo.Ironium), Boranium: max(0, cost.Boranium-future.Cargo.Boranium)}
}
func (t *turnGenerator) loadStarbasePrefetch(p *Player, f *Fleet, source, dest *Planet) {
	need := t.prefetchStarbaseMinerals(p, dest)
	if need.Total() == 0 {
		return
	}
	reserve := t.productionMinerals(p, source, false)
	room := min(f.availableCargoSpace(), f.Spec.CargoCapacity/5)
	iron := min(need.Ironium, max(0, source.Cargo.Ironium-reserve.Ironium))
	boron := min(need.Boranium, max(0, source.Cargo.Boranium-reserve.Boranium))
	total := iron + boron
	if total == 0 {
		return
	}
	loadI := min(iron, (room*iron+total-1)/total)
	loadB := min(boron, room-loadI)
	// Use any spare mineral share without taking more than its forecast need.
	loadI += min(iron-loadI, room-loadI-loadB)
	load := Mineral{Ironium: loadI, Boranium: loadB}
	source.Cargo = source.Cargo.Subtract(load.ToCargo())
	f.Cargo = f.Cargo.AddMineral(load)
}
