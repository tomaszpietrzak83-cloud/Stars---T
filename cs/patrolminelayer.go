package cs

import (
	"encoding/json"
	"slices"
)

// Scanner and scout observations share FleetIntels. Use only the reported
// hull/equipment and position, without looking at the opposing fleet's orders.
func exposedReportedMinelayer(p *Player, enemy *Fleet) bool {
	return enemy != nil && enemy.ReportAge == 0 && trackableMinelayer(p, enemy)
}

func trackableMinelayer(p *Player, enemy *Fleet) bool {
	if enemy == nil || enemy.Delete || enemy.Starbase || enemy.OrbitingPlanetNum != 0 || enemy.ReportAge < 0 || enemy.ReportAge > 2 || enemy.PlayerNum == p.Num || p.IsFriend(enemy.PlayerNum) {
		return false
	}
	for _, token := range enemy.Tokens {
		if d := p.GetShipDesignIntel(enemy.PlayerNum, token.DesignNum); d != nil && token.Quantity > 0 {
			for _, slot := range d.Slots {
				if component := StaticTechStore.GetHullComponent(slot.HullComponent); component != nil && slot.Quantity > 0 && component.MineLayingRate > 0 {
					return true
				}
			}
			if hull := StaticTechStore.GetHull(d.Hull); hull != nil && hull.Type == TechHullTypeMineLayer && len(d.Slots) == 0 {
				return true
			}
		}
	}
	return false
}

func (t *turnGenerator) planMinelayerScouts(p *Player, fleets []*Fleet, colonyRecon map[int]bool) map[int]bool {
	used := map[int]bool{}
	watched := map[string]bool{}
	eligible := func(f *Fleet) bool {
		return !used[f.Num] && !colonyRecon[f.Num] && !landingManaged(f) && !f.Delete &&
			p.Automation.FleetRoles[f.Num] == "scout" && scoutReconEligible(p, f) &&
			!f.InDesignMaintenance() && !f.InAutomaticSafety() && !scoutHasRetirement(p, f) &&
			!fuelNetworkManaged(f) && f.GetTag(tankerAwaitTag) == "" && f.GetTag(preyWatchTag) == "" &&
			!p.FleetInActiveAssembly(f) && f.GetTag(priorityReconTag) == ""
	}
	routeFor := func(f, enemy *Fleet) *expeditionRoute {
		if scoutFleetScanRadius(f, enemy) < 4 {
			return nil
		}
		point := scoutFleetScanPoint(f, enemy)
		route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, point, MapObjectTarget{}, automaticRoutePolicy{Fast: true, EconomicalPatrol: true, MaxTurns: 6})
		if route == nil || route.WaitForFuel || route.NetworkRequest != nil {
			return nil
		}
		return route
	}
	assign := func(f, enemy *Fleet, route *expeditionRoute) {
		job := scoutReconJob{Enemy: enemy.PlayerNum, Target: enemy.Num, Watching: true, Minelayer: true, Status: "Shadowing minelayer " + enemy.Name}
		data, _ := json.Marshal(job)
		f.SetTag(scoutReconTag, string(data))
		f.SetTag(scoutTag, job.Status)
		delete(f.Tags, patrolSearchTag)
		applyExpeditionRoute(p, f, route)
		used[f.Num] = true
		watched[priorityFleetKey(enemy.PlayerNum, enemy.Num)] = true
	}
	for _, f := range fleets {
		job := readScoutRecon(f)
		if job == nil || !job.Minelayer {
			continue
		}
		enemy := p.GetFleetIntel(job.Enemy, job.Target)
		if eligible(f) && trackableMinelayer(p, enemy) && !watched[priorityFleetKey(job.Enemy, job.Target)] {
			if route := routeFor(f, enemy); route != nil {
				assign(f, enemy, route)
				continue
			}
		}
		delete(f.Tags, scoutReconTag)
	}
	for _, enemy := range p.FleetIntels {
		if !exposedReportedMinelayer(p, enemy) || watched[priorityFleetKey(enemy.PlayerNum, enemy.Num)] {
			continue
		}
		var scout *Fleet
		var best *expeditionRoute
		for _, f := range fleets {
			if !eligible(f) || readScoutRecon(f) != nil {
				continue
			}
			if route := routeFor(f, enemy); route != nil && (best == nil || route.Turns < best.Turns || route.Turns == best.Turns && route.Fuel < best.Fuel) {
				scout, best = f, route
			}
		}
		if scout != nil {
			assign(scout, enemy, best)
		}
	}
	return used
}

func (t *turnGenerator) planMinelayerInterceptions(p *Player) {
	if p.patrolEvaluator == nil {
		return
	}
	evaluator := p.patrolEvaluator
	limit := evaluator.Limit
	evaluator.Limit = max(limit, evaluator.Work+384)
	defer func() { evaluator.Limit = limit }()
	fleets := slices.Clone(t.game.getFleets(p.Num))
	claimed := map[string]bool{}
	watched := map[string]bool{}
	for _, f := range fleets {
		if f.Delete {
			continue
		}
		if scout := readScoutRecon(f); scout != nil && scout.Minelayer && scoutReconActive(p, f) {
			watched[priorityFleetKey(scout.Enemy, scout.Target)] = true
		}
		j := readPatrolJob(f)
		if j.Leader != f.Num {
			continue
		}
		key := priorityFleetKey(j.Enemy, j.Target)
		claimed[key] = true
		if !j.Minelayer || defencePatrolCommitted(p, f) {
			continue
		}
		enemy := p.GetFleetIntel(j.Enemy, j.Target)
		if !trackableMinelayer(p, enemy) || !patrolAvailable(p, f) {
			t.releasePatrol(p, f)
			continue
		}
		if enemy.ReportAge != 0 || patrolNeedsRecon(p, enemy) {
			for _, member := range fleets {
				if !member.Delete && readPatrolJob(member).Leader == f.Num {
					holdAutomaticFleet(p, member, "Minelayer pursuit: waiting for a fresh scout report")
				}
			}
			continue
		}
		t.advanceAssessedPatrol(p, f, enemy, &j)
	}
	for _, enemy := range p.FleetIntels {
		if !exposedReportedMinelayer(p, enemy) || patrolNeedsRecon(p, enemy) {
			continue
		}
		key := priorityFleetKey(enemy.PlayerNum, enemy.Num)
		if claimed[key] {
			continue
		}
		point := patrolInterceptPosition(enemy)
		slices.SortStableFunc(fleets, func(a, b *Fleet) int {
			return a.Position.Subtract(point).LengthSquared() - b.Position.Subtract(point).LengthSquared()
		})
		for _, f := range fleets {
			if p.Automation.FleetRoles[f.Num] != "patrol" || !patrolAvailable(p, f) || readPatrolJob(f).Leader != 0 ||
				defencePatrolCommitted(p, f) || localPatrolReserve(f, t.game.Year) || f.carriesInfrastructure() {
				continue
			}
			// Without a watching scout, the existing check below rejects every
			// route taking more than one turn. An optimistic gate/travel bound
			// can reject those fleets before expensive mine-detour return plans.
			if !watched[key] && automaticGateTurnLowerBound(p, []*Fleet{f}, expeditionStations(p, f), f.Position, point, automaticMaxWarp(p, f)) > 1 {
				continue
			}
			route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, point, MapObjectTarget{}, automaticRoutePolicy{Fast: true, AllowThreat: true, MaxTurns: 6, MinWarp: automaticCruiseWarp(p, f)})
			if route == nil || route.WaitForFuel || route.NetworkRequest != nil || route.Turns > 1 && !watched[key] {
				continue
			}
			j := patrolJob{Leader: f.Num, Enemy: enemy.PlayerNum, Target: enemy.Num, Minelayer: true, RequiredShips: 1, GatheringSince: t.game.Year}
			if route.Turns <= 1 && len(route.Waypoints) <= 2 {
				group := []*Fleet{f}
				if result := p.patrolEvaluator.assess(p, group, enemy, point, 1); result.Ready {
					j.ReliefYear, j.Rally, j.RequiredShips = t.game.Year, point, f.Spec.TotalShips
					j.Status = "Intercepting reported minelayer: " + enemy.Name
					t.reservePatrolRelief(p, group, &j)
					applyExpeditionRoute(p, f, route)
					applyPatrolBattlePlan(p, f, result.Tactic)
					savePatrolJob(f, j)
					break
				}
			}
			if !watched[key] {
				continue
			}
			savePatrolJob(f, j)
			t.advanceAssessedPatrol(p, f, enemy, &j)
			if slices.ContainsFunc(t.game.getFleets(p.Num), func(member *Fleet) bool {
				active := readPatrolJob(member)
				return !member.Delete && active.Leader == member.Num && active.Enemy == enemy.PlayerNum && active.Target == enemy.Num
			}) {
				break
			}
		}
	}
}
