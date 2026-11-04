package cs

import "slices"

// A direct cruise is not a tanker reservation. It must not lock a patrol to
// last year's fixed endpoint and suppress fresh interception opportunities.
func (t *turnGenerator) releaseCombatCruiseNetworks() {
	for _, p := range t.automaticPlayers() {
		for _, f := range t.game.getFleets(p.Num) {
			plan := readFuelNetworkPlan(f)
			if plan == nil || (!combatAutonomy(p, f) && p.Automation.FleetRoles[f.Num] != "bomber" && !scoutReconActive(p, f) && f.GetTag(priorityReconTag) == "") {
				continue
			}
			mobile := false
			for _, leg := range plan.Route.Legs {
				mobile = mobile || len(leg.Donations) > 0
			}
			if !mobile {
				t.closeFuelNetwork(p, f, plan, true)
			}
		}
	}
}

// All actual members move toward the same reachable point, rather than chasing
// a stationary leader. Once co-located they still merge through native orders.
func (t *turnGenerator) movingPatrolRally(p *Player, leader *Fleet, job *patrolJob) bool {
	group := []*Fleet{leader}
	sum := leader.Position
	for _, f := range t.game.getFleets(p.Num) {
		if f != leader && !f.Delete && readPatrolJob(f).Leader == leader.Num {
			group = append(group, f)
			sum = sum.Add(f.Position)
		}
	}
	if len(group) < 2 {
		return false
	}
	point := Vector{sum.X / len(group), sum.Y / len(group)}
	if p.patrolEvaluator == nil {
		// Keep legacy isolated planner fixtures on their established policy.
		return t.applyLegacyPatrolRally(p, leader, job, group, point)
	}
	enemy := patrolEnemy(p, *job)
	if enemy == nil {
		return false
	}
	target := patrolInterceptPosition(enemy)
	candidates := []Vector{point}
	for _, f := range group {
		candidates = append(candidates, f.Position)
	}
	for n := 0; n <= 12; n++ {
		candidates = append(candidates, point.ToFloat64().Add(target.Subtract(point).ToFloat64().Scale(float64(n)/12)).ToInt(true))
	}
	for _, dock := range expeditionStations(p, leader) {
		if dock.Position.DistanceTo(point) < 200 {
			candidates = append(candidates, dock.Position)
		}
	}
	policies := make([]automaticRoutePolicy, len(group))
	for i := range policies {
		policies[i] = automaticRoutePolicy{Fast: true, EconomicalPatrol: true, MaxTurns: 20}
	}
	chosen := choosePatrolRally(&t.game.Rules, p, group, target, candidates, policies)
	if chosen == nil {
		return false
	}
	if job.RendezvousBy == 0 {
		job.RendezvousBy = t.game.Year + chosen.Turns
	}
	ok := applyPatrolRally(p, leader, job, group, chosen.Routes)
	if ok {
		t.patrolGatheringCleanup(p, leader, enemy, job, group)
	}
	return ok
}

func (t *turnGenerator) applyLegacyPatrolRally(p *Player, leader *Fleet, job *patrolJob, group []*Fleet, point Vector) bool {
	routes := make([]*expeditionRoute, len(group))
	for i, f := range group {
		routes[i] = planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, point, MapObjectTarget{}, automaticRoutePolicy{AllowThreat: true, EconomicalPatrol: true, MaxWarp: min(6, automaticMaxWarp(p, f)), MinWarp: 1, MaxTurns: 20})
		if routes[i] == nil || routes[i].WaitForFuel {
			return false
		}
	}
	return applyPatrolRally(p, leader, job, group, routes)
}

func applyPatrolRally(p *Player, leader *Fleet, job *patrolJob, group []*Fleet, routes []*expeditionRoute) bool {
	for i, f := range group {
		applyExpeditionRoute(p, f, routes[i])
		j := readPatrolJob(f)
		if p.patrolEvaluator != nil {
			j.Provisional = true
			j.Rally = routes[i].Waypoints[len(routes[i].Waypoints)-1].Position
		}
		j.Status = "Moving to shared patrol rendezvous; nearby interception remains under review"
		savePatrolJob(f, j)
	}
	job.Status = "Moving to shared patrol rendezvous; all available groups approach"
	if p.patrolEvaluator != nil {
		job.Provisional = true
		job.Rally = routes[0].Waypoints[len(routes[0].Waypoints)-1].Position
	}
	savePatrolJob(leader, *job)
	return true
}

type patrolRally struct {
	Position    Vector
	Turns, Fuel int
	Routes      []*expeditionRoute
}

func choosePatrolRally(rules *Rules, p *Player, group []*Fleet, target Vector, candidates []Vector, policies []automaticRoutePolicy) *patrolRally {
	var best *patrolRally
	seen := map[Vector]bool{}
	for _, point := range candidates {
		if seen[point] || knownRouteThreat(p, point, point) > 0 {
			continue
		}
		seen[point] = true
		merged := patrolMergedFormation(rules, p, group)
		if merged == nil {
			return nil
		}
		merged.Position, merged.Fuel = point, 0
		candidate := &patrolRally{Position: point}
		feasible := true
		for i, f := range group {
			policy := policies[i]
			policy.AllowThreat = knownWithdrawalRoute(p, f.Position, point)
			route := planAutomaticRoute(rules, p, []*Fleet{f}, point, MapObjectTarget{}, policy)
			if route == nil || route.WaitForFuel {
				feasible = false
				break
			}
			candidate.Routes = append(candidate.Routes, route)
			candidate.Turns = max(candidate.Turns, route.Turns)
			candidate.Fuel += route.Fuel
			arrival := f.Fuel - route.Fuel
			if len(route.ArrivalFuel) > 0 {
				arrival = route.ArrivalFuel[0]
			}
			merged.Fuel += max(0, arrival)
		}
		if !feasible {
			continue
		}
		merged.Fuel = min(merged.Spec.FuelCapacity, merged.Fuel)
		attack := planAutomaticRoute(rules, p, []*Fleet{merged}, target, MapObjectTarget{}, automaticRoutePolicy{Fast: true, AllowThreat: true, MaxTurns: 20, MinWarp: automaticCruiseWarp(p, merged)})
		if attack == nil || attack.WaitForFuel {
			continue
		}
		candidate.Turns += attack.Turns
		candidate.Fuel += attack.Fuel
		if best == nil || candidate.Turns < best.Turns || candidate.Turns == best.Turns && candidate.Fuel < best.Fuel {
			best = candidate
		}
	}
	if best != nil {
		best.Routes = slices.Clone(best.Routes)
	}
	return best
}

func nearbyPatrolContact(p *Player, f *Fleet, radius float64) bool {
	for _, enemy := range p.FleetIntels {
		if enemy != nil && !enemy.Delete && !enemy.Starbase && enemy.ReportAge == 0 && enemy.PlayerNum != p.Num && !p.IsFriend(enemy.PlayerNum) && f.Position.DistanceTo(enemy.Position) <= radius {
			return true
		}
	}
	return false
}
