package cs

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
)

const assemblyTag = "fleet-assembly"
const assemblyRalliedTag = "assembly-rallied"

// A composition is a one-shot order. A new revision explicitly requests another
// reconciliation; casualties after completion do not silently order more ships.
type FleetAssembly struct {
	Name              string       `json:"name"`
	RallyPlanetNum    int          `json:"rallyPlanetNum"`
	Designs           map[int]int  `json:"designs"`
	SourceFleets      map[int]bool `json:"sourceFleets"`
	BuildMissing      bool         `json:"buildMissing"`
	Revision          int          `json:"revision"`
	LeaderFleetNum    int          `json:"leaderFleetNum,omitempty"`
	CompletedRevision int          `json:"completedRevision,omitempty"`
	NotifiedRevision  int          `json:"notifiedRevision,omitempty"`
	Status            string       `json:"status,omitempty"`
}

func sameAssemblyComposition(a, b FleetAssembly) bool {
	return a.Name == b.Name && a.RallyPlanetNum == b.RallyPlanetNum && a.BuildMissing == b.BuildMissing && maps.Equal(a.Designs, b.Designs) && maps.Equal(a.SourceFleets, b.SourceFleets)
}

func (p *Player) FleetInActiveAssembly(fleet *Fleet) bool {
	id, _ := strconv.Atoi(fleet.GetTag(assemblyTag))
	plan, ok := p.Automation.FleetAssemblies[id]
	return ok && plan.CompletedRevision != plan.Revision
}

func (p *Player) ValidateFleetAssemblies(next map[int]FleetAssembly, fleets []*Fleet, planets []*Planet) error {
	if len(next) > 32 {
		return fmt.Errorf("at most 32 fleet compositions are supported")
	}
	owned := map[int]*Fleet{}
	for _, f := range fleets {
		if f.OwnedBy(p.Num) && !f.Delete && !f.Starbase {
			owned[f.Num] = f
		}
	}
	yards := map[int]*Planet{}
	for _, planet := range planets {
		if planet.OwnedBy(p.Num) && planet.Spec.HasStarbase {
			yards[planet.Num] = planet
		}
	}
	selected := map[int]int{}
	for id, plan := range next {
		old := p.Automation.FleetAssemblies[id]
		if plan.CompletedRevision != plan.Revision {
			for design := range plan.Designs {
				if p.DesignMaintenance[design] != 0 {
					return fmt.Errorf("edit or cancel the active composition before retiring its requested designs")
				}
			}
		}
		if id <= 0 || len(strings.TrimSpace(plan.Name)) == 0 || len(plan.Name) > 80 || plan.Revision < 1 || plan.Revision < old.Revision || plan.Revision > old.Revision+1 {
			return fmt.Errorf("invalid fleet composition name, id or revision")
		}
		if plan.LeaderFleetNum != old.LeaderFleetNum || plan.CompletedRevision != old.CompletedRevision || plan.NotifiedRevision != old.NotifiedRevision || plan.Status != old.Status {
			return fmt.Errorf("fleet composition progress is controlled by the server")
		}
		if !sameAssemblyComposition(plan, old) && plan.Revision != old.Revision+1 {
			return fmt.Errorf("increment revision when editing a fleet composition")
		}
		// Historical completed orders can refer to a lost station or obsolete design.
		if plan.Revision == old.Revision && sameAssemblyComposition(plan, old) {
			continue
		}
		if yards[plan.RallyPlanetNum] == nil {
			return fmt.Errorf("choose an owned rally station")
		}
		total := 0
		for designNum, quantity := range plan.Designs {
			design := p.GetDesign(designNum)
			if design == nil || design.PlayerNum != p.Num || design.Spec.Starbase || quantity < 1 || quantity > 10000 || p.DesignMaintenance[designNum] != 0 {
				return fmt.Errorf("composition requires active owned ship designs and positive quantities")
			}
			total += quantity
		}
		if total < 1 || total > 10000 {
			return fmt.Errorf("a composition must contain 1 to 10000 ships")
		}
		for num, use := range plan.SourceFleets {
			if !use {
				continue
			}
			fleet := owned[num]
			if fleet == nil {
				if old.SourceFleets[num] {
					continue
				}
				return fmt.Errorf("cannot select a fleet you do not own")
			}
			if previous, ok := selected[num]; ok && previous != id {
				return fmt.Errorf("a source fleet cannot belong to two compositions")
			}
			selected[num] = id
			if fleet.InDesignMaintenance() || fleet.GetTag(assemblyTag) != "" && fleet.GetTag(assemblyTag) != strconv.Itoa(id) {
				return fmt.Errorf("selected fleet is already reserved")
			}
		}
	}
	return nil
}

func assemblyCounts(fleets []*Fleet) map[int]int {
	counts := map[int]int{}
	for _, f := range fleets {
		for _, token := range f.Tokens {
			counts[token.DesignNum] += token.Quantity
		}
	}
	return counts
}
func (t *turnGenerator) assemblyMembers(player *Player, id int) []*Fleet {
	var members []*Fleet
	for _, f := range t.game.getFleets(player.Num) {
		if !f.Delete && !f.Starbase && !f.InDesignMaintenance() && f.GetTag(assemblyTag) == strconv.Itoa(id) {
			members = append(members, f)
		}
	}
	slices.SortFunc(members, func(a, b *Fleet) int { return a.Num - b.Num })
	return members
}
func (t *turnGenerator) reserveAssemblyFleet(player *Player, fleet *Fleet, id int) {
	fleet.SetTag(assemblyTag, strconv.Itoa(id))
	delete(player.Automation.FleetRoles, fleet.Num)
	delete(player.Automation.RebuildFleets, fleet.Num)
	t.releaseEscorts(player, fleet)
	for _, key := range []string{logisticsTag, escortTag, scoutTag, scoutMissionTag, "mining-job", colonizationTag, terraformTag} {
		delete(fleet.Tags, key)
	}
	routeAutomatically(&t.game.Rules, player, fleet, fleet.Position, MapObjectTarget{}, 0)
}

// Detach only the required quantities; the normal split preserves damage,
// divides fuel/cargo and records by-hand transfers without creating free ships.
func (t *turnGenerator) detachAssemblyShips(player *Player, source *Fleet, need map[int]int) *Fleet {
	var tokens []ShipToken
	count := 0
	for _, token := range source.Tokens {
		if n := min(token.Quantity, need[token.DesignNum]); n > 0 {
			copy := token
			copy.Quantity = n
			tokens = append(tokens, copy)
			count += n
		}
	}
	if count == 0 {
		return nil
	}
	if count == source.Spec.TotalShips {
		return source
	}
	child, err := (&orders{}).splitFleetTokens(&t.game.Rules, player, t.game.getFleets(player.Num), source, tokens)
	if err != nil {
		return nil
	}
	t.game.Fleets = append(t.game.Fleets, child)
	_ = t.game.Universe.addFleet(child)
	return child
}

func (t *turnGenerator) planFleetAssemblies() {
	for _, player := range t.game.Players {
		// Cancelling a plan releases ships and removes its remaining queue items.
		for _, fleet := range t.game.getFleets(player.Num) {
			if raw := fleet.GetTag(assemblyTag); raw != "" {
				id, _ := strconv.Atoi(raw)
				if _, ok := player.Automation.FleetAssemblies[id]; !ok {
					delete(fleet.Tags, assemblyTag)
					delete(fleet.Tags, assemblyRalliedTag)
					routeAutomatically(&t.game.Rules, player, fleet, fleet.Position, MapObjectTarget{}, 0)
				}
			}
		}
		for _, planet := range t.game.getPlanets(player.Num) {
			planet.ProductionQueue = slices.DeleteFunc(planet.ProductionQueue, func(item ProductionQueueItem) bool {
				raw := item.Tags[assemblyTag]
				if raw == "" {
					return false
				}
				id, _ := strconv.Atoi(raw)
				_, ok := player.Automation.FleetAssemblies[id]
				return !ok
			})
		}
		ids := slices.Sorted(maps.Keys(player.Automation.FleetAssemblies))
		for _, id := range ids {
			plan := player.Automation.FleetAssemblies[id]
			if plan.CompletedRevision == plan.Revision {
				continue
			}
			members := t.assemblyMembers(player, id)
			// Lowering a composition releases excess ships into manual fleets.
			remaining := maps.Clone(plan.Designs)
			for _, f := range members {
				excess := map[int]int{}
				for _, token := range f.Tokens {
					keep := min(token.Quantity, remaining[token.DesignNum])
					remaining[token.DesignNum] -= keep
					excess[token.DesignNum] = token.Quantity - keep
				}
				if released := t.detachAssemblyShips(player, f, excess); released != nil {
					delete(released.Tags, assemblyTag)
					delete(released.Tags, assemblyRalliedTag)
					routeAutomatically(&t.game.Rules, player, released, released.Position, MapObjectTarget{}, 0)
				}
			}
			members = t.assemblyMembers(player, id)
			counts := assemblyCounts(members)
			for _, num := range slices.Sorted(maps.Keys(plan.SourceFleets)) {
				if !plan.SourceFleets[num] {
					continue
				}
				f := t.game.getFleet(player.Num, num)
				if f == nil || f.Delete {
					delete(plan.SourceFleets, num)
					continue
				}
				if f.Starbase || f.InDesignMaintenance() || f.GetTag(assemblyTag) != "" {
					delete(plan.SourceFleets, num)
					continue
				}
				need := map[int]int{}
				for d, n := range plan.Designs {
					need[d] = max(0, n-counts[d])
				}
				if taken := t.detachAssemblyShips(player, f, need); taken != nil {
					t.reserveAssemblyFleet(player, taken, id)
					for _, token := range taken.Tokens {
						counts[token.DesignNum] += token.Quantity
					}
				}
				// Consume the selection so reused fleet numbers cannot claim future ships.
				delete(plan.SourceFleets, num)
			}
			members = t.assemblyMembers(player, id)
			leader := t.game.getFleet(player.Num, plan.LeaderFleetNum)
			if leader == nil || leader.Delete || leader.GetTag(assemblyTag) != strconv.Itoa(id) || leader.InDesignMaintenance() {
				leader = nil
				plan.LeaderFleetNum = 0
				if len(members) > 0 {
					leader = members[0]
					plan.LeaderFleetNum = leader.Num
					leader.Rename(plan.Name)
				}
			}
			rally := t.game.getPlanet(plan.RallyPlanetNum)
			if rally == nil || !rally.OwnedBy(player.Num) || !rally.Spec.HasStarbase {
				plan.Status = "Waiting for an owned rally station"
				player.Automation.FleetAssemblies[id] = plan
				continue
			}
			plan.Status = "Assembling at " + rally.Name
			if leader != nil {
				leader.Rename(plan.Name)
			}
			if leader != nil && leader.GetTag(assemblyRalliedTag) != strconv.Itoa(plan.RallyPlanetNum) {
				if leader.Position == rally.Position {
					leader.SetTag(assemblyRalliedTag, strconv.Itoa(plan.RallyPlanetNum))
				} else if !t.routeAssemblyFleet(player, leader, rally.Position) {
					plan.Status = "Waiting for a safe fuel route to the rally station"
				}
			}
			for _, f := range members {
				if leader != nil && f != leader {
					if !t.routeAssemblyFleet(player, f, leader.Position) {
						plan.Status = "Reinforcements waiting for a safe fuel route"
					}
				}
			}
			t.queueAssemblyShips(player, id, &plan, assemblyCounts(members), rally)
			player.Automation.FleetAssemblies[id] = plan
		}
	}
}

func (t *turnGenerator) routeAssemblyFleet(player *Player, fleet *Fleet, position Vector) bool {
	if fuelNetworkManaged(fleet) || fleet.InAutomaticSafety() {
		return false
	}
	// A fixed rendezvous is replanned each turn. A moving target must not silently
	// extend a fuel-budgeted leg past the position used to calculate its cost.
	target := &Planet{MapObject: MapObject{Position: position}}
	if route := t.planAutomaticMission(player, []*Fleet{fleet}, target.Position, target.MapObject.ToTarget(), automaticRoutePolicy{}); route != nil {
		applyExpeditionRoute(player, fleet, route)
		return true
	}
	routeAutomatically(&t.game.Rules, player, fleet, fleet.Position, MapObjectTarget{}, 0)
	return false
}

func (t *turnGenerator) queueAssemblyShips(player *Player, id int, plan *FleetAssembly, counts map[int]int, rally *Planet) {
	need := map[int]int{}
	for d, n := range plan.Designs {
		need[d] = max(0, n-counts[d])
	}
	// Keep paid, already queued work first; edits cannot duplicate its allocation.
	for _, planet := range t.game.getPlanets(player.Num) {
		var queue []ProductionQueueItem
		for _, item := range planet.ProductionQueue {
			if item.Tags[assemblyTag] == strconv.Itoa(id) {
				item.Quantity = min(item.Quantity, need[item.DesignNum])
				need[item.DesignNum] -= item.Quantity
				if item.Quantity == 0 {
					continue
				}
			}
			queue = append(queue, item)
		}
		planet.ProductionQueue = queue
	}
	if !plan.BuildMissing {
		plan.Status = "Waiting for selected existing ships"
		return
	}
	load := map[int]float64{}
	for _, planet := range t.game.getPlanets(player.Num) {
		for _, item := range planet.ProductionQueue {
			if item.Type.IsAuto() {
				continue
			}
			producer := newProducer(t.log, &t.game.Rules, planet, player)
			if cost, err := producer.getItemCost(&t.game.Rules, player, planet, item); err == nil {
				load[planet.Num] += float64(max(0, cost.Resources*item.Quantity-item.Allocated.Resources))
			}
		}
	}
	for _, designNum := range slices.Sorted(maps.Keys(need)) {
		design := player.GetDesign(designNum)
		if design == nil || player.DesignMaintenance[designNum] != 0 {
			if need[designNum] > 0 {
				plan.Status = "A requested design is unavailable or obsolete"
			}
			continue
		}
		// Delivery paths depend on the design and dock, not the quantity ordered.
		// Calculate each path once even for a composition of thousands of ships.
		deliveryTurns := map[int]int{}
		for _, planet := range t.game.getPlanets(player.Num) {
			if !planet.CanBuild(design.Spec.Mass) || planet.Spec.ResourcesPerYearAvailable <= 0 {
				continue
			}
			probe := newFleetForDesign(player, design, 1, 1, design.Name, []Waypoint{{Position: planet.Position}})
			probe.Spec = ComputeFleetSpec(&t.game.Rules, player, &probe)
			probe.Fuel = probe.Spec.FuelCapacity
			if route := planExpeditionRoute(&t.game.Rules, player, &probe, rally); route != nil {
				deliveryTurns[planet.Num] = route.Turns
			}
		}
		for range need[designNum] {
			var chosen *Planet
			best := math.Inf(1)
			for _, planet := range t.game.getPlanets(player.Num) {
				turns, reachable := deliveryTurns[planet.Num]
				if !reachable {
					continue
				}
				score := (load[planet.Num]+float64(design.Spec.Cost.Resources))/float64(planet.Spec.ResourcesPerYearAvailable) + float64(turns)
				for _, kind := range []ResourceType{Ironium, Boranium, Germanium} {
					if missing := max(0, design.Spec.Cost.GetAmount(kind)-planet.Cargo.GetAmount(kind)); missing > 0 {
						rate := planet.Spec.MiningOutput.GetAmount(kind)
						if rate == 0 {
							score += 100
						} else {
							score += float64(missing) / float64(rate)
						}
					}
				}
				if score < best || score == best && chosen != nil && planet.Num < chosen.Num {
					chosen, best = planet, score
				}
			}
			if chosen == nil {
				plan.Status = "Waiting for a shipyard with a safe delivery route"
				break
			}
			item := NewProductionQueueItemShip(1, design).WithTag(assemblyTag, strconv.Itoa(id))
			item.design = design
			// Keep explicit work ahead of this order, but insert before recurring
			// auto-factory/mine orders which can otherwise consume every year's budget.
			at := len(chosen.ProductionQueue)
			for i, queued := range chosen.ProductionQueue {
				if queued.Type.IsAuto() {
					at = i
					break
				}
			}
			if at > 0 && chosen.ProductionQueue[at-1].DesignNum == designNum && chosen.ProductionQueue[at-1].Tags[assemblyTag] == strconv.Itoa(id) {
				chosen.ProductionQueue[at-1].Quantity++
			} else {
				chosen.ProductionQueue = slices.Insert(chosen.ProductionQueue, at, *item)
			}
			load[chosen.Num] += float64(design.Spec.Cost.Resources)
		}
	}
}

func (t *turnGenerator) resolveFleetAssemblies() {
	for _, player := range t.game.Players {
		for _, id := range slices.Sorted(maps.Keys(player.Automation.FleetAssemblies)) {
			plan := player.Automation.FleetAssemblies[id]
			if plan.CompletedRevision == plan.Revision {
				continue
			}
			members := t.assemblyMembers(player, id)
			leader := t.game.getFleet(player.Num, plan.LeaderFleetNum)
			if leader == nil || leader.Delete || leader.GetTag(assemblyTag) != strconv.Itoa(id) {
				if len(members) == 0 {
					continue
				}
				leader = members[0]
				plan.LeaderFleetNum = leader.Num
				leader.Rename(plan.Name)
			}
			if rally := t.game.getPlanet(plan.RallyPlanetNum); rally != nil && leader.Position == rally.Position {
				leader.SetTag(assemblyRalliedTag, strconv.Itoa(plan.RallyPlanetNum))
			}
			for _, member := range members {
				if fuelNetworkManaged(leader) || fuelNetworkManaged(member) || leader.InAutomaticSafety() || member.InAutomaticSafety() || member == leader || member.Position != leader.Position {
					continue
				}
				if _, err := NewOrderer().Merge(&t.game.Rules, player, []*Fleet{leader, member}); err == nil {
					t.game.deleteFleet(member)
				}
			}
			counts := assemblyCounts([]*Fleet{leader})
			ready, total := 0, 0
			for d, n := range plan.Designs {
				ready += min(n, counts[d])
				total += n
			}
			if total > 0 && ready*100 >= total*70 && plan.NotifiedRevision != plan.Revision {
				player.Messages = append(player.Messages, PlayerMessage{Type: PlayerMessageInfo, Text: fmt.Sprintf("%s: at least 70%% of the ordered composition has assembled.", plan.Name)})
				plan.NotifiedRevision = plan.Revision
			}
			plan.Status = fmt.Sprintf("Assembled %d/%d ships; reinforcements follow fleet #%d", ready, total, leader.Num)
			if total > 0 && ready == total && leader.GetTag(assemblyRalliedTag) == strconv.Itoa(plan.RallyPlanetNum) {
				plan.CompletedRevision = plan.Revision
				plan.Status = "Complete — one-shot order fulfilled"
				player.Messages = append(player.Messages, PlayerMessage{Type: PlayerMessageInfo, Text: fmt.Sprintf("%s: 100%% of the ordered composition has assembled.", plan.Name)})
			}
			player.Automation.FleetAssemblies[id] = plan
		}
	}
}
