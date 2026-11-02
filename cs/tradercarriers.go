package cs

import (
	"maps"
	"slices"
)

func traderLargeFreighter(p *Player, f *Fleet) bool {
	ships := 0
	for _, token := range f.Tokens {
		if token.Quantity <= 0 {
			continue
		}
		design := p.GetDesign(token.DesignNum)
		if design == nil || design.Hull != LargeFreighter.Name {
			return false
		}
		ships += token.Quantity
	}
	return ships > 0
}

// Only automatic freighter replacements are free future carriers. Manual and
// composition orders retain their purpose. Simulate one real production year,
// preserving queue precedence, mineral costs and already allocated resources.
func (t *turnGenerator) queuedTraderLargeFreighterCapacity(p *Player) int {
	capacity := 0
	for _, planet := range t.game.getPlanets(p.Num) {
		eligible := map[int]bool{}
		for i, item := range planet.ProductionQueue {
			design := p.GetDesign(item.DesignNum)
			work := decodeMaintenance(item.GetTag(replacementOrderTag))
			if item.Type == QueueItemTypeShipToken && design != nil && design.Hull == LargeFreighter.Name && item.GetTag(assemblyTag) == "" && work != nil && work.OriginalRole == "freighter" {
				eligible[i] = true
			}
		}
		if len(eligible) == 0 {
			continue
		}
		copy := *planet
		copy.ProductionQueue = slices.Clone(planet.ProductionQueue)
		for i := range copy.ProductionQueue {
			copy.ProductionQueue[i].Tags = maps.Clone(copy.ProductionQueue[i].Tags)
			copy.ProductionQueue[i].index = i
		}
		if planet.Starbase != nil {
			base := *planet.Starbase
			base.Tokens = slices.Clone(base.Tokens)
			copy.Starbase = &base
		}
		local := *p
		if local.Spec.Terraform == nil {
			local.Spec = ComputePlayerSpec(&local, &t.game.Rules)
		}
		copy.Spec = ComputePlanetSpec(&t.game.Rules, &local, &copy)
		copy.mine(&t.game.Rules, copy.Spec.MiningOutput, min(copy.Mines, copy.Spec.MaxPossibleMines))
		producer := newProducer(t.log, &t.game.Rules, &copy, &local)
		producer.estimating = true
		result, err := producer.produce()
		if err != nil {
			continue
		}
		for _, item := range result.itemsBuilt {
			if eligible[item.index] && item.queueItemType == QueueItemTypeShipToken && item.numBuilt > 0 {
				if design := p.GetDesign(item.designNum); design != nil && design.Hull == LargeFreighter.Name {
					capacity += item.numBuilt * design.Spec.CargoCapacity
				}
			}
		}
	}
	return capacity
}
