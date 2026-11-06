package cs

import (
	"maps"
	"math"
	"slices"
)

func (t *turnGenerator) productionMinerals(player *Player, planet *Planet, shipsOnly bool) Mineral {
	need := manufacturingWorkStock(&t.game.Rules, player, planet).Add(t.incomingRebuildMinerals(player, planet))
	calculator := newProducer(t.log, &t.game.Rules, planet, player)
	for _, item := range planet.ProductionQueue {
		if item.Type == QueueItemTypeShipToken && player.DesignMaintenance[item.DesignNum] != 0 {
			continue
		}
		if manufacturingItem(item) {
			continue
		}
		if item.Type.IsAuto() {
			if !shipsOnly && item.Type == QueueItemTypeAutoFactories && item.GetTag(starbaseBoostTag) == "" {
				per := t.game.Rules.FactoryCostGermanium
				if player.Race.FactoriesCostLess {
					per--
				}
				need.Germanium += min(max(0, planet.Spec.MaxFactories-planet.Factories), max(1, item.Quantity)*3) * per
			}
			continue
		}
		if shipsOnly && item.Type != QueueItemTypeShipToken && item.Type != QueueItemTypeStarbase {
			continue
		}
		if cost, err := calculator.getItemCost(&t.game.Rules, player, planet, item); err == nil {
			remaining := MultiplyCost(cost, item.Quantity).Subtract(item.Allocated)
			need = need.Add(remaining.ToMineral())
		}
	}
	return need.Max(automaticBaseMineralStock(planet))
}

// Exports only stock beyond a source's own committed production. Destinations
// are actual shipyard queues, without user-maintained stock thresholds.
func (t *turnGenerator) findShipyardSupply(player *Player, fleet *Fleet, claimed map[string]bool) *logisticsJob {
	return t.findShipyardSupplyMode(player, fleet, claimed, true)
}
func (t *turnGenerator) findShipyardSupplyMode(player *Player, fleet *Fleet, claimed map[string]bool, idleStock bool) *logisticsJob {
	var chosen *logisticsJob
	best := math.Inf(1)
	sources := []*Planet{}
	spareBySource := map[int]Mineral{}
	sites := t.game.getPlanets(player.Num)
	seen := map[int]bool{}
	for _, site := range sites {
		seen[site.Num] = true
	}
	for _, miner := range t.game.getFleets(player.Num) {
		if miner.Delete || miner.Spec.MiningRate <= 0 || player.Automation.FleetRoles[miner.Num] != "miner" {
			continue
		}
		for _, object := range t.game.getMapObjectsAtPosition(miner.Position) {
			if site, ok := object.(*Planet); ok && !site.Owned() && !seen[site.Num] {
				sites = append(sites, site)
				seen[site.Num] = true
			}
		}
	}
	slices.SortFunc(sites, func(a, b *Planet) int { return a.Num - b.Num })
	for _, source := range sites {
		if t.canCollectMiningPlanet(player, source) {
			spare := source.Cargo.ToMineral().ToCost().Subtract(t.productionMinerals(player, source, false).ToCost()).Subtract(t.pendingSupplyMinerals(player, source.Num, true, fleet.Num).ToCost()).MinZero().ToMineral()
			if spare.Total() > 0 {
				sources = append(sources, source)
				spareBySource[source.Num] = spare
			}
		}
	}
	for _, yard := range t.game.getPlanets(player.Num) {
		if !yard.Spec.HasStarbase && !queuedStarbase(yard) {
			continue
		}
		if !idleStock && t.incomingRebuildMinerals(player, yard).Total() == 0 {
			active := false
			for _, item := range yard.ProductionQueue {
				if manufacturingItem(item) && item.Quantity > 0 && (item.Type != QueueItemTypeShipToken || player.DesignMaintenance[item.DesignNum] == 0) {
					active = true
					break
				}
			}
			if !active {
				continue
			}
		}
		need := t.productionMinerals(player, yard, true).ToCost().Subtract(yard.Cargo.ToMineral().ToCost()).Subtract(yard.Spec.MiningOutput.ToCost()).Subtract(t.pendingSupplyMinerals(player, yard.Num, false, fleet.Num).ToCost()).MinZero().ToMineral()
		if need.Total() == 0 {
			continue
		}
		for _, source := range sources {
			if source.Num == yard.Num {
				continue
			}
			room := max(0, fleet.Spec.CargoCapacity-fleet.CargoMass())
			load := Mineral{}
			spare := spareBySource[source.Num]
			for _, kind := range []ResourceType{Ironium, Boranium, Germanium} {
				amount := max(0, min(room, need.GetAmount(kind), spare.GetAmount(kind)))
				load.Set(kind, amount)
				room -= amount
			}
			if load.Total() == 0 {
				continue
			}
			score := fleet.Position.DistanceTo(source.Position) + source.Position.DistanceTo(yard.Position)
			if t.incomingRebuildMinerals(player, yard).Total() > 0 {
				score /= 4
			}
			if score >= best {
				continue
			}
			origin := *source
			origin.Cargo = origin.Cargo.Subtract(t.pendingSupplyMinerals(player, source.Num, true, fleet.Num).ToCargo())
			probe, _ := t.previewShipyardPickup(player, fleet, &origin, yard, load)
			if probe.CargoMass() != fleet.Spec.CargoCapacity {
				continue
			}
			first := planAutomaticRoute(&t.game.Rules, player, []*Fleet{fleet}, source.Position, source.ToTarget(), automaticRoutePolicy{AllowThreat: true, MinWarp: automaticCruiseWarp(player, fleet), MaxTurns: 20})
			if first == nil || first.WaitForFuel {
				continue
			}
			probe.Position = source.Position
			probe.Fuel = first.ArrivalFuel[0]
			if probe.CanFuel(player, source) {
				probe.Fuel = probe.Spec.FuelCapacity
			}
			delivery := planAutomaticRoute(&t.game.Rules, player, []*Fleet{&probe}, yard.Position, yard.ToTarget(), automaticRoutePolicy{AllowThreat: true, MinWarp: automaticCruiseWarp(player, fleet), MaxTurns: 20})
			if delivery == nil || delivery.WaitForFuel {
				continue
			}
			travel := first.Turns + delivery.Turns
			if !manufacturingDeliveryUseful(&t.game.Rules, player, yard, probe.Cargo.ToMineral(), travel) && t.incomingRebuildMinerals(player, yard).Total() == 0 {
				continue
			}
			chosen = &logisticsJob{Kind: "planet", TargetNum: source.Num, Position: source.Position, DeliveryNum: yard.Num, Load: probe.Cargo.ToMineral()}
			best = score
		}
	}
	return chosen
}

// Reservations are quantities, not an exclusive lock on an entire source or
// yard. Several full freighters may serve a large shortage without duplication.
func (t *turnGenerator) pendingSupplyMinerals(p *Player, planet int, pickup bool, except int) Mineral {
	booked := Mineral{}
	for _, f := range t.game.getFleets(p.Num) {
		if f.Delete || f.Num == except {
			continue
		}
		job := logisticsWork(f)
		if job == nil {
			continue
		}
		if job.Kind == "planet" && ((pickup && job.TargetNum == planet) || (!pickup && job.DeliveryNum == planet)) {
			booked = booked.Add(job.Load)
		}
		if !pickup && (job.Kind == "return" || job.Kind == "settle") && job.DeliveryNum == planet {
			booked = booked.Add(f.Cargo.ToMineral())
		}
	}
	return booked
}

// A dedicated mineral shipment may top up the currently deficient mineral
// into a one-hold yard buffer. This prevents a 62/210-kT shortage from keeping
// a freighter empty while thousands of exportable tonnes sit at its source.
func (t *turnGenerator) topUpShipyardBuffer(p *Player, f *Fleet, source, dest *Planet, preferred ...Mineral) {
	if !dest.OwnedBy(p.Num) || !t.canCollectMiningPlanet(p, source) {
		return
	}
	demand := t.productionMinerals(p, dest, true)
	reserve := t.productionMinerals(p, source, false)
	requested := Mineral{}
	if len(preferred) > 0 {
		requested = preferred[0]
	} else if job := logisticsWork(f); job != nil && job.Kind == "planet" {
		requested = job.Load
	}
	for _, kind := range []ResourceType{Ironium, Germanium, Boranium} {
		deficit := demand.GetAmount(kind) - dest.Cargo.GetAmount(kind) - t.pendingSupplyMinerals(p, dest.Num, false, f.Num).GetAmount(kind)
		deficit = max(deficit, requested.GetAmount(kind))
		if deficit <= 0 {
			continue
		}
		amount := max(0, min(f.availableCargoSpace(), deficit+f.Spec.CargoCapacity-f.Cargo.GetAmount(kind), source.Cargo.GetAmount(kind)-reserve.GetAmount(kind)))
		source.Cargo = source.Cargo.SubtractAmount(kind, amount)
		f.Cargo = f.Cargo.AddAmount(kind, amount)
	}
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

// Assigned incoming rebuilds are real future yard demand before the ship is
// scrapped and its replacement enters the queue. Count mineral scrap credit
// and bound reservations to approximately three years of yard output.
func (t *turnGenerator) incomingRebuildMinerals(p *Player, yard *Planet) Mineral {
	need := Mineral{}
	budget := max(1, yard.Spec.ResourcesPerYearAvailable) * 3
	for _, f := range t.game.getFleets(p.Num) {
		if budget <= 0 {
			break
		}
		if f.Delete {
			continue
		}
		w := readMaintenance(f)
		if w == nil || w.Stage != "scrap" || w.PlanetNum != yard.Num {
			continue
		}
		for _, token := range f.Tokens {
			target := p.GetDesign(p.LatestReplacement(p.DesignMaintenance[token.DesignNum]))
			if target == nil {
				continue
			}
			for i := 0; i < token.Quantity && budget > 0; i++ {
				credit := MultiplyCost(token.design.Spec.Cost, t.game.Rules.ScrapMineralAmount+p.Race.Spec.ScrapMineralOffsetStarbase)
				resources := target.Spec.Cost.Resources
				if rapid, ok := rapidUpgradeRecovery(p, token.design, target, token.design.Spec.Cost); ok {
					credit = rapid
					resources = max(0, resources-credit.Resources)
				}
				need = need.Add(target.Spec.Cost.Subtract(credit).MinZero().ToMineral())
				budget -= max(1, resources)
			}
		}
	}
	return need
}

// The same dry-run is used by job selection, stale-job recovery and loading.
// In particular, a rebuild-pending freighter must keep a delivery it can fill.
func (t *turnGenerator) previewShipyardPickup(p *Player, f *Fleet, source, dest *Planet, requested Mineral) (Fleet, Planet) {
	probe, origin := *f, *source
	probe.Tags = maps.Clone(f.Tags)
	spare := origin.Cargo.ToMineral().ToCost().Subtract(t.productionMinerals(p, &origin, false).ToCost()).MinZero().ToMineral()
	cargo := Cargo{Ironium: min(spare.Ironium, requested.Ironium), Boranium: min(spare.Boranium, requested.Boranium), Germanium: min(spare.Germanium, requested.Germanium)}
	before := cargo
	takeMinerals(&probe, &cargo)
	origin.Cargo = origin.Cargo.Subtract(before.Subtract(cargo))
	t.topUpShipyardBuffer(p, &probe, &origin, dest, requested)
	t.topUpLogisticsCargo(p, &probe, &origin, dest)
	if automaticPassengersAllowed(p, f) && suitableColony(p, dest) && !p.Automation.DisablePassengerTransport {
		incoming, _ := t.passengerReservations(p, f.Num)
		pax := max(0, min(probe.availableColonistSpace(), origin.Cargo.Colonists-sourcePopulationReserve(p, &origin), populationTarget(dest, colonyPopulationPercent(p, dest))-dest.Cargo.Colonists-incoming[dest.Num]))
		origin.Cargo.Colonists -= pax
		probe.Cargo.Colonists += pax
	}
	t.loadOpportunisticInfrastructure(p, &probe, &origin, dest, 0)
	return probe, origin
}
