package cs

import (
	"log/slog"
	"math"
)

const automaticShipyardMineralStock = 300

// Stock follows committed work: cover at least its next unit and roughly three
// production years, rather than hoarding the minerals for a decades-long queue.
func manufacturingStockTarget(rules *Rules, p *Player, planet *Planet) Mineral {
	need := Mineral{}
	for _, item := range planet.ProductionQueue {
		if item.Type == QueueItemTypeAutoFactories && item.GetTag(starbaseBoostTag) != "" {
			per := p.Race.Spec.Costs[QueueItemTypeAutoFactories].Germanium
			need.Germanium += min(max(0, planet.Spec.MaxFactories-planet.Factories), max(1, item.Quantity)*3) * per
		}
	}
	budget := max(1, planet.Spec.ResourcesPerYearAvailable) * 3
	calculator := newProducer(slog.Default(), rules, planet, p)
	first := true
	for _, item := range planet.ProductionQueue {
		if !manufacturingItem(item) || item.Type == QueueItemTypeShipToken && p.DesignMaintenance[item.DesignNum] != 0 {
			continue
		}
		cost, err := calculator.getItemCost(rules, p, planet, item)
		if err != nil || item.Quantity <= 0 {
			continue
		}
		quantity := min(item.Quantity, max(0, (budget+max(1, cost.Resources)-1)/max(1, cost.Resources)))
		if first {
			quantity = max(1, quantity)
			first = false
		}
		if quantity == 0 {
			break
		}
		remaining := MultiplyCost(cost, quantity).Subtract(item.Allocated).MinZero()
		need = need.Add(remaining.ToMineral())
		budget -= remaining.Resources
		if budget <= 0 {
			break
		}
	}
	if planet.Spec.HasStarbase && planet.Spec.DockCapacity != 0 {
		need = need.Max(Mineral{Ironium: automaticShipyardMineralStock, Boranium: automaticShipyardMineralStock, Germanium: automaticShipyardMineralStock})
	}
	return need
}

func manufacturingDeliveryUseful(rules *Rules, p *Player, planet *Planet, load Mineral, arrival int) bool {
	base, err := forecastManufacturing(rules, p, planet, planet.ProductionQueue, 0, Mineral{})
	if err != nil {
		return false
	}
	if planet.Spec.HasStarbase && planet.Spec.DockCapacity != 0 {
		short := Mineral{Ironium: automaticShipyardMineralStock, Boranium: automaticShipyardMineralStock, Germanium: automaticShipyardMineralStock}.ToCost().Subtract(planet.Cargo.ToMineral().ToCost()).MinZero().ToMineral()
		if min(short.Ironium, load.Ironium)+min(short.Boranium, load.Boranium)+min(short.Germanium, load.Germanium) > 0 {
			return true
		}
	}
	if len(base.Finish) == 0 {
		return false
	}
	with, err := forecastManufacturing(rules, p, planet, planet.ProductionQueue, max(1, arrival), load)
	if err != nil || !manufacturingDatesNoLater(with, base) {
		return false
	}
	if manufacturingDatesEarlier(with, base) {
		return true
	}
	// A full load can be one of several necessary deliveries to an otherwise
	// mineral-blocked order. It must still reduce the forecast stock shortage.
	if slicesContainInfinite(base.Finish) {
		short := manufacturingStockTarget(rules, p, planet).ToCost().Subtract(planet.Cargo.ToMineral().ToCost()).MinZero().ToMineral()
		return min(short.Ironium, load.Ironium)+min(short.Boranium, load.Boranium)+min(short.Germanium, load.Germanium) > 0
	}
	return false
}
func slicesContainInfinite(years []int) bool {
	for _, year := range years {
		if year == math.MaxInt {
			return true
		}
	}
	return false
}

func committedManufacturingMinerals(rules *Rules, p *Player, planet *Planet) Mineral {
	need := Mineral{}
	calculator := newProducer(slog.Default(), rules, planet, p)
	for _, item := range planet.ProductionQueue {
		if !manufacturingItem(item) || item.Type == QueueItemTypeShipToken && p.DesignMaintenance[item.DesignNum] != 0 {
			continue
		}
		if cost, err := calculator.getItemCost(rules, p, planet, item); err == nil {
			need = need.Add(MultiplyCost(cost, item.Quantity).Subtract(item.Allocated).MinZero().ToMineral())
		}
	}
	return need
}
