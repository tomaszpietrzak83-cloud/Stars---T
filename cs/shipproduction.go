package cs

import (
	"fmt"
	"log/slog"
	"maps"
	"slices"
)

const ProductionAutoRoleTag = "new-ship-auto-role"
const shipProductionWindowTag = "ship-production-window"
const productionCandidateTag = "ship-production-preview"
const MaxShipProductionOrder = 100000

type ShipyardProduction struct {
	Planet                           *Planet
	Capacity, ThisYear, Later, Years int
	Reason                           string
}
type EmpireShipProduction struct {
	Yards                                     []ShipyardProduction
	Requested, Assigned, ThisYear, Unassigned int
	AutoRole                                  string
}

func validateProductionAutoRoles(p *Player, queue []ProductionQueueItem) error {
	for _, item := range queue {
		role := item.GetTag(ProductionAutoRoleTag)
		if role == "" {
			continue
		}
		d := p.GetDesign(item.DesignNum)
		if item.Type != QueueItemTypeShipToken || d == nil {
			return fmt.Errorf("post-construction Auto orders require an owned ship design")
		}
		if err := ValidateDesignAutoRole(role, d.Spec); err != nil {
			return err
		}
	}
	return nil
}

func productionPlanetCopy(p *Planet) *Planet {
	c := *p
	c.Tags = maps.Clone(p.Tags)
	c.ProductionQueue = slices.Clone(p.ProductionQueue)
	for i := range c.ProductionQueue {
		c.ProductionQueue[i].Tags = maps.Clone(c.ProductionQueue[i].Tags)
	}
	if p.Starbase != nil {
		base := *p.Starbase
		base.Tokens = slices.Clone(base.Tokens)
		base.Tags = maps.Clone(base.Tags)
		c.Starbase = &base
	}
	return &c
}

// Keep concrete and paid work first, then use capacity ahead of recurring
// development. This is the same placement rule used by fleet assembly orders.
func insertShipProduction(p *Planet, d *ShipDesign, n int, role, window string) {
	item := *NewProductionQueueItemShip(n, d).WithTag(ProductionAutoRoleTag, role)
	item.design = d
	item.Tags[shipProductionWindowTag] = window
	if window != "preview" {
		item.Tags[productionCandidateTag] = window
	}
	at, last := len(p.ProductionQueue), -1
	for i, q := range p.ProductionQueue {
		if !q.Type.IsAuto() {
			last = i
		}
	}
	for i := last + 1; i < len(p.ProductionQueue); i++ {
		if p.ProductionQueue[i].Type.IsAuto() && p.ProductionQueue[i].GetTag(starbaseBoostTag) == "" {
			at = i
			break
		}
	}
	p.ProductionQueue = slices.Insert(p.ProductionQueue, at, item)
}

// A yard with existing concrete orders left after one year is reserved for
// those orders. Recurring Auto entries do not count toward that backlog.
func hasShipyardBacklog(r *Rules, p *Player, original *Planet) (bool, error) {
	concrete := func(item ProductionQueueItem) bool {
		return !item.Type.IsAuto() && item.Quantity > 0
	}
	if !slices.ContainsFunc(original.ProductionQueue, concrete) {
		return false, nil
	}

	planet := productionPlanetCopy(original)
	planet.ProductionQueue = slices.DeleteFunc(planet.ProductionQueue, func(item ProductionQueueItem) bool {
		// Explicit alchemy can supply minerals to the concrete item after it.
		return !concrete(item) &&
			(item.Type != QueueItemTypeAutoMineralAlchemy || item.GetTag(starbaseBoostTag) != "")
	})
	planet.mine(r, planet.Spec.MiningOutput, min(planet.Mines, planet.Spec.MaxPossibleMines))
	producer := newProducer(slog.Default(), r, planet, p)
	producer.estimating = true
	if _, err := producer.produce(); err != nil {
		return false, err
	}
	return slices.ContainsFunc(planet.ProductionQueue, concrete), nil
}

// Native mining/production on private copies, tracking only the added candidate.
// One simulation supplies a cumulative capacity curve; no per-ship forecasting.
func productionCapacity(r *Rules, p *Player, original *Planet, d *ShipDesign, limit, years int, optimize bool) ([]int, error) {
	planet := productionPlanetCopy(original)
	// Adding a manufacturing order replans temporary factory and mine boosts.
	// A boost calculated for the old queue must not consume the new order's capacity.
	planet.ProductionQueue = slices.DeleteFunc(planet.ProductionQueue, func(item ProductionQueueItem) bool {
		return item.Type.IsAuto() && item.GetTag(starbaseBoostTag) != ""
	})
	insertShipProduction(planet, d, limit, "manual", "preview")
	for i := range planet.ProductionQueue {
		if planet.ProductionQueue[i].GetTag(shipProductionWindowTag) == "preview" {
			planet.ProductionQueue[i].Tags[productionCandidateTag] = "candidate"
		}
	}
	if optimize {
		PlanDefaultAuto(r, p, planet)
	}
	index := -1
	for i := range planet.ProductionQueue {
		planet.ProductionQueue[i].index = i
		if planet.ProductionQueue[i].GetTag(productionCandidateTag) == "candidate" {
			index = i
		}
	}
	if index < 0 {
		return nil, fmt.Errorf("production candidate disappeared")
	}
	profile := make([]int, years+1)
	producer := newProducer(slog.Default(), r, planet, p)
	producer.estimating = true
	for year := 1; year <= years; year++ {
		planet.Spec = ComputePlanetSpec(r, p, planet)
		planet.mine(r, planet.Spec.MiningOutput, min(planet.Mines, planet.Spec.MaxPossibleMines))
		result, err := producer.produce()
		if err != nil {
			return nil, err
		}
		profile[year] = profile[year-1]
		for _, built := range result.itemsBuilt {
			if built.index == index && built.queueItemType == QueueItemTypeShipToken {
				profile[year] += built.numBuilt
			}
		}
		if profile[year] >= limit {
			for next := year + 1; next <= years; next++ {
				profile[next] = profile[year]
			}
			break
		}
		planet.grow(p)
	}
	return profile, nil
}

// Distribute available ships up to the same total at each yard, subject to
// its capacity. Give leftover ships to eligible yards in the existing order.
func balanceShipProduction(assigned, capacity []int, remaining int) int {
	low, high := 0, MaxShipProductionOrder
	for low < high {
		level := (low + high + 1) / 2
		needed := 0
		for i, limit := range capacity {
			needed += max(0, min(limit, level)-assigned[i])
		}
		if needed <= remaining {
			low = level
		} else {
			high = level - 1
		}
	}
	for i, limit := range capacity {
		add := max(0, min(limit, low)-assigned[i])
		assigned[i] += add
		remaining -= add
	}
	for i, limit := range capacity {
		if remaining == 0 {
			break
		}
		if assigned[i] == low && assigned[i] < limit {
			assigned[i]++
			remaining--
		}
	}
	return remaining
}

// PlanEmpireShipProduction never changes the supplied player or planets. First
// exhaust this year's real capacity across bases, then distribute the remainder
// by native completion year. Current-year batches stay separate from later work
// so automatic development cannot delay an already confirmed one-year deadline.
func PlanEmpireShipProduction(r *Rules, player *Player, planets []*Planet, designNum, quantity int, role string, fillThisYear bool) (EmpireShipProduction, error) {
	result := EmpireShipProduction{Requested: quantity}
	if quantity < 1 || quantity > MaxShipProductionOrder {
		return result, fmt.Errorf("quantity must be between 1 and %d", MaxShipProductionOrder)
	}
	local := *player
	local.Race.Spec = ComputeRaceSpec(&local.Race, r)
	local.Spec = ComputePlayerSpec(&local, r)
	d := local.GetDesign(designNum)
	if d == nil || d.PlayerNum != local.Num || d.OriginalPlayerNum != 0 || d.Spec.Starbase || local.DesignMaintenance[designNum] != 0 {
		return result, fmt.Errorf("choose a current owned ship design")
	}
	if role == "" {
		role = effectiveDesignAutoRole(d.Spec)
		if role == "" {
			role = "manual"
		}
	}
	if err := ValidateDesignAutoRole(role, d.Spec); err != nil {
		return result, err
	}
	role = NormalizeAutomaticRole(role)
	result.AutoRole = role
	for _, source := range planets {
		if !source.OwnedBy(local.Num) || !source.Spec.HasStarbase {
			continue
		}
		planet := productionPlanetCopy(source)
		yard := ShipyardProduction{Planet: planet}
		if err := planet.PopulateStarbaseDesign(&local); err != nil {
			yard.Reason = err.Error()
		} else if err := planet.PopulateProductionQueueDesigns(&local); err != nil {
			yard.Reason = err.Error()
		} else {
			if planet.Starbase != nil {
				planet.Starbase.Spec = ComputeFleetSpec(r, &local, planet.Starbase)
			}
			planet.Spec = ComputePlanetSpec(r, &local, planet)
			if !planet.CanBuild(d.Spec.Mass) {
				yard.Reason = "This base cannot build this hull mass"
			} else if busy, err := hasShipyardBacklog(r, &local, planet); err != nil {
				yard.Reason = err.Error()
			} else if busy {
				yard.Reason = "Existing non-Auto orders extend beyond this year"
			} else {
				profile, err := productionCapacity(r, &local, planet, d, MaxShipProductionOrder, 1, false)
				if err != nil {
					yard.Reason = err.Error()
				} else {
					yard.Capacity = profile[1]
				}
			}
		}
		result.Yards = append(result.Yards, yard)
	}
	slices.SortStableFunc(result.Yards, func(a, b ShipyardProduction) int {
		if a.Planet.Spec.ResourcesPerYearAvailable != b.Planet.Spec.ResourcesPerYearAvailable {
			return b.Planet.Spec.ResourcesPerYearAvailable - a.Planet.Spec.ResourcesPerYearAvailable
		}
		return a.Planet.Num - b.Planet.Num
	})
	assigned := make([]int, len(result.Yards))
	capacity := make([]int, len(result.Yards))
	for i, yard := range result.Yards {
		capacity[i] = yard.Capacity
	}
	remaining := balanceShipProduction(assigned, capacity, quantity)
	result.ThisYear = quantity - remaining
	for i := range result.Yards {
		yard := &result.Yards[i]
		yard.ThisYear = assigned[i]
		if yard.ThisYear > 0 {
			insertShipProduction(yard.Planet, d, yard.ThisYear, role, "current")
		}
	}
	if remaining > 0 && !fillThisYear {
		profiles := make([][]int, len(result.Yards))
		for i := range result.Yards {
			yard := &result.Yards[i]
			if yard.Reason != "" {
				continue
			}
			profile, err := productionCapacity(r, &local, yard.Planet, d, remaining, 100, true)
			if err != nil {
				yard.Reason = err.Error()
				continue
			}
			profiles[i] = profile
		}
		for year := 1; year <= 100 && remaining > 0; year++ {
			for i, profile := range profiles {
				capacity[i] = result.Yards[i].ThisYear
				if len(profile) != 0 {
					capacity[i] += profile[year]
				}
			}
			remaining = balanceShipProduction(assigned, capacity, remaining)
		}
		for i := range result.Yards {
			yard := &result.Yards[i]
			yard.Later = assigned[i] - yard.ThisYear
			if yard.Later > 0 {
				insertShipProduction(yard.Planet, d, yard.Later, role, "later")
			}
		}
	}
	for i := range result.Yards {
		yard := &result.Yards[i]
		if yard.ThisYear+yard.Later == 0 {
			if yard.Reason == "" {
				yard.Reason = "No remaining capacity for this order this year"
			}
			continue
		}
		PlanDefaultAuto(r, &local, yard.Planet)
		if err := yard.Planet.PopulateProductionQueueEstimates(r, &local); err != nil {
			return result, err
		}
		yard.Years = 0
		for index := range yard.Planet.ProductionQueue {
			item := &yard.Planet.ProductionQueue[index]
			window := item.GetTag(productionCandidateTag)
			if item.DesignNum != designNum || window == "" {
				continue
			}
			if window == "current" && item.YearsToBuildAll != 1 {
				return result, fmt.Errorf("the current-year forecast changed; refresh production")
			}
			yard.Years = max(yard.Years, item.YearsToBuildAll)
			delete(item.Tags, productionCandidateTag)
		}
	}
	result.Assigned = quantity - remaining
	result.Unassigned = remaining
	if fillThisYear {
		result.Requested = result.Assigned
		result.Unassigned = 0
	}
	return result, nil
}
