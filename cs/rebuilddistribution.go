package cs

import (
	"fmt"
	"math"
	"slices"
)

func hasShipsToReplace(player *Player, fleet *Fleet) bool {
	for _, token := range fleet.Tokens {
		if player.DesignMaintenance[token.DesignNum] > 0 {
			return true
		}
	}
	return false
}

func hasMaintenanceTargets(player *Player, fleet *Fleet) bool {
	for _, token := range fleet.Tokens {
		if player.DesignMaintenance[token.DesignNum] != 0 {
			return true
		}
	}
	return false
}

func (t *turnGenerator) scatterReplacementFleet(player *Player, fleet *Fleet, work *fleetMaintenance) {
	work.Expected = 0
	for _, token := range fleet.Tokens {
		if player.DesignMaintenance[token.DesignNum] != RetireDesign {
			work.Expected += token.Quantity
		}
	}
	delete(fleet.Tags, maintenanceTag)
	children, err := NewOrderer().SplitAll(&t.game.Rules, player, t.game.getFleets(player.Num), fleet)
	if err != nil {
		writeMaintenance(fleet, work)
		return
	}
	for _, child := range children {
		t.game.Fleets = append(t.game.Fleets, child)
		_ = t.game.Universe.addFleet(child)
	}
	units := append([]*Fleet{fleet}, children...)
	load := t.rebuildYardLoads(player, work.Group)
	for _, planet := range t.game.Planets {
		if !planet.OwnedBy(player.Num) {
			continue
		}
		for _, item := range planet.ProductionQueue {
			if item.Type.IsAuto() {
				continue
			}
			calculator := newProducer(t.log, &t.game.Rules, planet, player)
			if cost, err := calculator.getItemCost(&t.game.Rules, player, planet, item); err == nil {
				load[planet.Num] += float64(max(0, cost.Resources*item.Quantity-item.Allocated.Resources))
			}
		}
	}
	for _, unit := range units {
		assignment := *work
		assignment.Stage = "scrap"
		assignment.PlanetNum = 0
		if player.DesignMaintenance[unit.Tokens[0].DesignNum] == 0 {
			assignment.Stage = "rally"
			writeMaintenance(unit, &assignment)
			unit.SetTag(replacementGroupTag, work.Group)
			continue
		}
		target := player.GetDesign(player.DesignMaintenance[unit.Tokens[0].DesignNum])
		if target == nil {
			target = unit.Tokens[0].design
		}
		var chosen *Planet
		best := math.Inf(1)
		for _, planet := range t.game.Planets {
			if player.Automation.SingleShipyardRebuild && planet.Num != work.RallyPlanetNum {
				continue
			}
			if !planet.OwnedBy(player.Num) || !planet.CanBuild(target.Spec.Mass) || planet.Spec.ResourcesPerYearAvailable <= 0 {
				continue
			}
			route := planAutomaticRoute(&t.game.Rules, player, []*Fleet{unit}, planet.Position, planet.ToTarget(), automaticRoutePolicy{AllowThreat: true, EmergencyReturn: true, MaxTurns: 10000})
			if route == nil {
				continue
			}
			productionYears := (load[planet.Num] + float64(target.Spec.Cost.Resources)) / float64(planet.Spec.ResourcesPerYearAvailable)
			credit := MultiplyCost(unit.Tokens[0].design.Spec.Cost, t.game.Rules.ScrapMineralAmount+player.Race.Spec.ScrapMineralOffsetStarbase)
			for _, kind := range []ResourceType{Ironium, Boranium, Germanium} {
				missing := max(0, target.Spec.Cost.GetAmount(kind)-credit.GetAmount(kind)-planet.Cargo.GetAmount(kind))
				if missing > 0 {
					rate := planet.Spec.MiningOutput.GetAmount(kind)
					if rate <= 0 {
						productionYears += 2
					} else {
						productionYears += min(2, float64(missing)/float64(rate))
					}
				}
			}
			score := float64(route.Turns) + productionYears
			if score < best || score == best && chosen != nil && planet.Num < chosen.Num {
				chosen, best = planet, score
			}
		}
		if chosen != nil {
			assignment.PlanetNum = chosen.Num
			assignment.Status = "Assigned to shipyard " + chosen.Name
			load[chosen.Num] += float64(target.Spec.Cost.Resources)
			if route := planAutomaticRoute(&t.game.Rules, player, []*Fleet{unit}, chosen.Position, chosen.ToTarget(), automaticRoutePolicy{AllowThreat: true, EmergencyReturn: true, MaxTurns: 10000}); route != nil {
				applyExpeditionRoute(player, unit, route)
			}
		} else {
			assignment.Status = "Moving toward a fuel dock while retaining the yard reservation"
			if !t.routeEmergencyReturn(player, unit) {
				holdAutomaticFleet(player, unit, "No physically reachable yard; mobile fuel requested")
			}
			requestAutomaticRescue(player, unit, true)
		}
		writeMaintenance(unit, &assignment)
		unit.SetTag(replacementGroupTag, work.Group)
	}
}

func (t *turnGenerator) rallyReplacementFleet(player *Player, fleet *Fleet, work *fleetMaintenance) {
	rally := t.game.getPlanet(work.RallyPlanetNum)
	if rally == nil || !rally.OwnedBy(player.Num) || !rally.Spec.HasStarbase {
		rally = nil
		for _, planet := range t.game.Planets {
			if planet.OwnedBy(player.Num) && planet.Spec.HasStarbase {
				rally = planet
				break
			}
		}
	}
	if rally == nil {
		work.Status = "Waiting for an owned rally station"
		writeMaintenance(fleet, work)
		return
	}
	work.RallyPlanetNum = rally.Num
	work.Status = "Regrouping at " + rally.Name
	if route := planAutomaticRoute(&t.game.Rules, player, []*Fleet{fleet}, rally.Position, rally.ToTarget(), automaticRoutePolicy{AllowThreat: true, EmergencyReturn: true, MaxTurns: 10000}); route != nil {
		applyExpeditionRoute(player, fleet, route)
	} else {
		t.routeEmergencyReturn(player, fleet)
		requestAutomaticRescue(player, fleet, true)
	}
	writeMaintenance(fleet, work)
	if fleet.Position != rally.Position {
		return
	}
	readyCount := 0
	for _, member := range t.game.getFleets(player.Num) {
		if !member.Delete && member.Position == rally.Position && member.GetTag(replacementGroupTag) == work.Group && !hasMaintenanceTargets(player, member) {
			readyCount += member.Spec.TotalShips
		}
	}
	progressKey := "rebuild-70:" + work.Group
	if work.Expected > 0 && readyCount*100 >= work.Expected*70 && rally.GetTag(progressKey) == "" {
		player.Messages = append(player.Messages, PlayerMessage{Type: PlayerMessageInfo, Text: fmt.Sprintf("%s: at least 70%% of the fleet has regrouped after rebuilding.", work.BaseName)})
		rally.SetTag(progressKey, "sent")
	}
	if t.pendingReplacements(player, work) {
		return
	}
	var group []*Fleet
	for _, member := range t.game.getFleets(player.Num) {
		if member.Delete || member.GetTag(replacementGroupTag) != work.Group {
			continue
		}
		if member.Position != rally.Position || hasMaintenanceTargets(player, member) {
			return
		}
		group = append(group, member)
	}
	if len(group) == 0 {
		return
	}
	slices.SortFunc(group, func(a, b *Fleet) int { return a.Num - b.Num })
	for _, member := range group {
		delete(member.Tags, maintenanceTag)
	}
	leader := group[0]
	if len(group) > 1 {
		merged, err := NewOrderer().Merge(&t.game.Rules, player, group)
		if err != nil {
			for _, member := range group {
				writeMaintenance(member, work)
			}
			return
		}
		leader = merged
		for _, member := range group[1:] {
			t.game.deleteFleet(member)
		}
	}
	leader.Rename(work.BaseName)
	leader.FleetOrders = work.OriginalOrders
	if len(leader.Waypoints) > 0 {
		leader.Waypoints[0] = Waypoint{Position: leader.Position}
	}
	delete(leader.Tags, replacementGroupTag)
	delete(leader.Tags, maintenanceTag)
	text := fmt.Sprintf("%s: rebuilding and regrouping complete.", work.BaseName)
	if readyCount >= work.Expected {
		text = fmt.Sprintf("%s: 100%% of the rebuilt fleet has regrouped.", work.BaseName)
	}
	player.Messages = append(player.Messages, PlayerMessage{Type: PlayerMessageInfo, Text: text})
	delete(rally.Tags, progressKey)
	if work.OriginalRole != "" {
		if player.Automation.FleetRoles == nil {
			player.Automation.FleetRoles = map[int]string{}
		}
		player.Automation.FleetRoles[leader.Num] = work.OriginalRole
		leader.Waypoints = []Waypoint{{Position: leader.Position}}
	}
}

// Incoming ships reserve production space before they reach the yard.
func (t *turnGenerator) rebuildYardLoads(p *Player, exclude string) map[int]float64 {
	loads := map[int]float64{}
	for _, f := range t.game.getFleets(p.Num) {
		w := readMaintenance(f)
		if f.Delete || w == nil || w.Group == exclude || w.Stage != "scrap" {
			continue
		}
		for _, token := range f.Tokens {
			if d := p.GetDesign(p.LatestReplacement(p.DesignMaintenance[token.DesignNum])); d != nil {
				loads[w.PlanetNum] += float64(d.Spec.Cost.Resources * token.Quantity)
			}
		}
	}
	return loads
}
func (t *turnGenerator) finishReplacementRallies() {
	for _, p := range t.automaticPlayers() {
		for _, f := range slices.Clone(t.game.getFleets(p.Num)) {
			if f.Delete {
				continue
			}
			if w := readMaintenance(f); w != nil && w.Stage == "rally" && !hasMaintenanceTargets(p, f) {
				t.rallyReplacementFleet(p, f, w)
			}
		}
	}
}
