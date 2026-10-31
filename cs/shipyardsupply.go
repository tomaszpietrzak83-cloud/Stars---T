package cs

import (
	"fmt"
	"math"
)

func (t *turnGenerator) productionMinerals(player *Player, planet *Planet, shipsOnly bool) Mineral {
	need := Mineral{}
	calculator := newProducer(t.log, &t.game.Rules, planet, player)
	for _, item := range planet.ProductionQueue {
		if item.Type.IsAuto() {
			if !shipsOnly && item.Type == QueueItemTypeAutoFactories {
				per := t.game.Rules.FactoryCostGermanium
				if player.Race.FactoriesCostLess {
					per--
				}
				need.Germanium += max(0, planet.Spec.MaxFactories-planet.Factories) * per
			}
			continue
		}
		if shipsOnly && item.Type != QueueItemTypeShipToken {
			continue
		}
		if cost, err := calculator.getItemCost(&t.game.Rules, player, planet, item); err == nil {
			remaining := MultiplyCost(cost, item.Quantity).Subtract(item.Allocated)
			need = need.Add(remaining.ToMineral())
		}
	}
	return need
}

// Exports only stock beyond a source's own committed production. Destinations
// are actual shipyard queues, without user-maintained stock thresholds.
func (t *turnGenerator) findShipyardSupply(player *Player, fleet *Fleet, claimed map[string]bool) *logisticsJob {
	var chosen *logisticsJob
	best := math.Inf(1)
	for _, yard := range t.game.Planets {
		if !yard.OwnedBy(player.Num) || !yard.Spec.HasStarbase || yard.Spec.DockCapacity == 0 || claimed[fmt.Sprintf("yard:%d", yard.Num)] {
			continue
		}
		need := t.productionMinerals(player, yard, true).ToCost().Subtract(yard.Cargo.ToMineral().ToCost()).Subtract(yard.Spec.MiningOutput.ToCost()).ToMineral()
		for _, source := range t.game.Planets {
			if source.Num == yard.Num || !t.canCollectMiningPlanet(player, source) || claimed[fmt.Sprintf("planet:%d", source.Num)] {
				continue
			}
			reserved := t.productionMinerals(player, source, false)
			spare := source.Cargo.ToMineral().ToCost().Subtract(reserved.ToCost()).ToMineral()
			room := max(0, fleet.Spec.CargoCapacity-fleet.CargoMass())
			load := Mineral{}
			for _, kind := range []ResourceType{Ironium, Boranium, Germanium} {
				amount := max(0, min(room, need.GetAmount(kind), spare.GetAmount(kind)))
				load.Set(kind, amount)
				room -= amount
			}
			if load.Total() == 0 {
				continue
			}
			score := fleet.Position.DistanceTo(source.Position) + source.Position.DistanceTo(yard.Position)
			if score < best {
				chosen = &logisticsJob{Kind: "planet", TargetNum: source.Num, Position: source.Position, DeliveryNum: yard.Num, Load: load}
				best = score
			}
		}
	}
	return chosen
}

// Own miners physically report their unowned mining site. Foreign colonies
// are never used as a source, even if a mining assignment survives conquest.
func (t *turnGenerator) canCollectMiningPlanet(player *Player, planet *Planet) bool {
	if planet == nil {
		return false
	}
	if planet.OwnedBy(player.Num) {
		return true
	}
	if planet.Owned() {
		return false
	}
	for _, fleet := range t.game.getFleets(player.Num) {
		if !fleet.Delete && fleet.Spec.MiningRate > 0 && fleet.Position == planet.Position && player.Automation.FleetRoles[fleet.Num] == "miner" {
			return true
		}
	}
	return false
}
