package cs

import (
	"log/slog"
	"maps"
	"slices"
)

func (j patrolJob) reliefActive(year int) bool {
	return j.Leader != 0 && j.ReliefYear != 0 && j.ReliefYear == year
}

// A relief force can assemble at the fight itself when all members can arrive
// this turn. It is still forecast as one native stack, merged before combat.
func (t *turnGenerator) planPatrolRelief(p *Player, enemy *Fleet, leaders []*Fleet) bool {
	if len(leaders) == 0 || patrolAlreadyCoLocated(leaders) {
		return false
	}
	point := patrolInterceptPosition(enemy)
	routes := map[*Fleet]*expeditionRoute{}
	groups := map[int][]*Fleet{}
	planKey := func(f *Fleet) int {
		if isManagedPatrolPlan(f) {
			return 0
		}
		return f.BattlePlanNum
	}
	for _, f := range leaders {
		if f.carriesInfrastructure() || f.Position.DistanceTo(point) > f.StrategicSpeed(automaticMaxWarp(p, f)) {
			continue
		}
		route := t.patrolReliefRoute(p, f, point)
		if route == nil || route.WaitForFuel || route.NetworkRequest != nil || len(route.Waypoints) > 2 {
			continue
		}
		routes[f] = route
		key := planKey(f)
		groups[key] = append(groups[key], f)
	}
	checked := map[int]bool{}
	for _, f := range leaders {
		key := planKey(f)
		if checked[key] || routes[f] == nil {
			continue
		}
		checked[key] = true
		pool := groups[key]
		// Prefer fleets already at the encounter and larger existing stacks. This
		// reduces both the number of arrivals to coordinate and forecast work.
		slices.SortStableFunc(pool, func(a, b *Fleet) int {
			if a.Position == point && b.Position != point {
				return -1
			}
			if b.Position == point && a.Position != point {
				return 1
			}
			return b.Spec.TotalShips - a.Spec.TotalShips
		})
		group := pool[:1]
		result := p.patrolEvaluator.assess(p, group, enemy, point, 1)
		if !result.Ready {
			checks := 0
			group, result = readyPatrolPrefix(p, pool, enemy, point, &checks, 16)
			if group == nil {
				slog.Debug("relief force not ready", "target", enemy.Name, "point", point, "reachableShips", patrolFormationShips(pool), "work", p.patrolEvaluator.Work)
				continue
			}
		}
		if len(group) == 1 && enemy.observedStrategicSpeed() > 0 {
			continue
		}
		leader := group[0]
		slog.Debug("relief force dispatched", "target", enemy.Name, "point", point, "ships", patrolFormationShips(group))
		job := patrolJob{Leader: leader.Num, LeaderID: leader.ID, Enemy: enemy.PlayerNum, Target: enemy.Num, ReliefYear: t.game.Year, Rally: point, RequiredShips: patrolFormationShips(group), Status: "Coordinated patrol interception this turn"}
		t.reservePatrolRelief(p, group, &job)
		for _, member := range group {
			applyExpeditionRoute(p, member, routes[member])
			applyPatrolBattlePlan(p, member, result.Tactic)
			savePatrolJob(member, job)
		}
		return true
	}
	return false
}

// Validate the shared arrival after the remaining waypoint-zero work, using
// each ship's actual fuel, orders and minefield route. Cancel the group as one.
func (t *turnGenerator) confirmPatrolRelief(p *Player, leader *Fleet, job patrolJob, members []*Fleet) bool {
	if leader == nil || leader.Delete || patrolFormationShips(members) < job.RequiredShips {
		return false
	}
	enemy := patrolEnemy(p, job)
	if enemy == nil || patrolInterceptPosition(enemy) != job.Rally || patrolNeedsRecon(p, enemy) {
		return false
	}
	for _, f := range members {
		if !patrolAvailable(p, f) || f.carriesInfrastructure() || !compatiblePatrolPlans(leader, f) {
			return false
		}
		if f.Position == job.Rally && len(f.Waypoints) == 1 {
			continue
		}
		if len(f.Waypoints) != 2 {
			return false
		}
		wp := f.Waypoints[1]
		if wp.Position != job.Rally || wp.TargetType == MapObjectTypeFleet {
			return false
		}
		distance := f.Position.DistanceTo(wp.Position)
		if wp.WarpSpeed == StargateWarpSpeed {
			if !f.CanJump(p, distance, p.GetPlanetIntel(f.OrbitingPlanetNum), p.GetPlanetIntel(wp.TargetNum)) || !automaticMineLegSafe(&t.game.Rules, p, wp.Position, wp.Position, 1, 0, false) {
				return false
			}
			continue
		}
		if wp.WarpSpeed < 1 || wp.WarpSpeed > automaticMaxWarp(p, f) || distance > f.StrategicSpeed(wp.WarpSpeed) || !automaticFleetLegSafe(&t.game.Rules, p, f, f.Position, wp.Position, wp.WarpSpeed, true) {
			return false
		}
		reserve := automaticReturnFuel(&t.game.Rules, p, f, wp.Position, expeditionStations(p, f), true, automaticReturnLimit(p, f, wp.MapObjectTarget))
		if _, _, _, ok := automaticLegFuel(p, f, wp.WarpSpeed, distance, f.Fuel, reserve); !ok {
			return false
		}
	}
	assessment := patrolFleetEvaluator(p, leader).assess(p, members, enemy, job.Rally, 1)
	if !assessment.Ready {
		return false
	}
	for _, f := range members {
		applyPatrolBattlePlan(p, f, assessment.Tactic)
	}
	return true
}

func (t *turnGenerator) patrolReliefDepartures(p *Player) map[int]bool {
	handled := map[int]bool{}
	groups := map[int][]*Fleet{}
	for _, f := range t.game.getFleets(p.Num) {
		if j := readPatrolJob(f); !f.Delete && j.reliefActive(t.game.Year) {
			groups[j.Leader] = append(groups[j.Leader], f)
		}
	}
	for _, num := range slices.Sorted(maps.Keys(groups)) {
		members := groups[num]
		j := readPatrolJob(members[0])
		ready := t.confirmPatrolRelief(p, t.game.getFleet(p.Num, j.Leader), j, members)
		slog.Debug("relief departure", "target", j.Target, "enemy", j.Enemy, "ready", ready, "ships", patrolFormationShips(members))
		for _, f := range members {
			handled[f.Num] = true
			if !ready {
				holdAutomaticFleet(p, f, "Coordinated interception held: not all patrol ships can arrive safely")
				next := readPatrolJob(f)
				next.ReliefYear = 0
				savePatrolJob(f, next)
			}
		}
	}
	return handled
}

func (t *turnGenerator) mergeArrivedPatrolRelief(p *Player) {
	groups := map[int][]*Fleet{}
	for _, f := range t.game.getFleets(p.Num) {
		if j := readPatrolJob(f); !f.Delete && j.reliefActive(t.game.Year) && f.Position == j.Rally {
			groups[j.Leader] = append(groups[j.Leader], f)
		}
	}
	for _, num := range slices.Sorted(maps.Keys(groups)) {
		members := groups[num]
		leader := members[0]
		job := readPatrolJob(leader)
		for _, f := range members {
			if f.Num == job.Leader {
				leader = f
				break
			}
		}
		if len(members) > 1 {
			ordered := []*Fleet{leader}
			for _, f := range members {
				if f != leader {
					ordered = append(ordered, f)
				}
			}
			if _, err := NewOrderer().Merge(&t.game.Rules, p, ordered); err == nil {
				for _, f := range ordered[1:] {
					t.game.deleteFleet(f)
				}
				job.Grouped = true
			}
		}
		// If movement interrupted a member, it must not later join an obsolete
		// arrival. The survivors can still defend the encounter they reached.
		for _, f := range t.game.getFleets(p.Num) {
			if f != leader && !f.Delete && readPatrolJob(f).Leader == job.Leader {
				delete(f.Tags, patrolTag)
				restorePatrolBattlePlan(p, f)
				if f.GetTag(patrolSupportTag) == "escort" {
					p.Automation.FleetRoles[f.Num] = "escort"
					delete(f.Tags, patrolSupportTag)
				}
			}
		}
		job.Leader, job.ReliefYear, job.RequiredShips = leader.Num, 0, 1
		savePatrolJob(leader, job)
	}
}

func (t *turnGenerator) patrolReliefRoute(p *Player, f *Fleet, point Vector) *expeditionRoute {
	policy := automaticRoutePolicy{Fast: true, AllowThreat: true, MaxTurns: 1}
	if warp := patrolFreeWarp(p, f); warp > 0 && f.Position.DistanceTo(point) <= f.StrategicSpeed(warp) {
		free := policy
		free.MinWarp, free.MaxWarp, free.EconomicalPatrol = warp, warp, true
		if route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, point, MapObjectTarget{}, free); route != nil && !route.WaitForFuel && route.NetworkRequest == nil {
			return route
		}
	}
	return planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, point, MapObjectTarget{}, policy)
}

func (t *turnGenerator) reservePatrolRelief(p *Player, group []*Fleet, job *patrolJob) {
	// Detach everybody before assigning the new job, since transferring old
	// leadership can change the tags of other selected members.
	for _, member := range group {
		t.detachPatrolMember(p, member)
	}
	for _, member := range group {
		if p.Automation.FleetRoles[member.Num] == "escort" {
			p.Automation.FleetRoles[member.Num] = "patrol"
			member.SetTag(patrolSupportTag, "escort")
		}
		if member.GetTag(patrolSupportTag) == "escort" {
			if job.SupportDesigns == nil {
				job.SupportDesigns = map[int]int{}
			}
			for _, token := range member.Tokens {
				job.SupportDesigns[token.DesignNum] += token.Quantity
			}
		}
		delete(member.Tags, patrolFrontTag)
		delete(member.Tags, frontierPatrolTag)
		delete(member.Tags, fuelRequestTag)
		p.patrolEvaluator.DefenceMembers[member.Num] = true
	}
}

type patrolReliefStaging struct {
	point  Vector
	ships  int
	fleets []*Fleet
	routes []*expeditionRoute
}

// Choose a safe local rally before selecting a formation. Recruits that cannot
// reach it in time must not turn a winning forecast into an impossible order.
func (t *turnGenerator) stagePatrolRelief(p *Player, enemy *Fleet, candidates []*Fleet) bool {
	if !urgentPatrolTarget(p, enemy) || len(candidates) < 2 || patrolAlreadyCoLocated(candidates) || !patrolReportedBomber(p, enemy) && !patrolReportedCombatant(p, enemy) {
		return false
	}
	point := patrolInterceptPosition(enemy)
	recon := patrolNeedsRecon(p, enemy)
	bombing, _ := patrolColonyDeadline(p, enemy)
	if recon && !bombing {
		return false
	}
	worlds := slices.Clone(t.game.getPlanets(p.Num))
	slices.SortStableFunc(worlds, func(a, b *Planet) int {
		da, db := point.DistanceTo(a.Position), point.DistanceTo(b.Position)
		if da < db {
			return -1
		}
		if da > db {
			return 1
		}
		return a.Num - b.Num
	})
	stages := []patrolReliefStaging{}
	for _, world := range worlds {
		if len(stages) >= 4 || point.DistanceTo(world.Position) > 175 {
			break
		}
		if knownRouteThreat(p, world.Position, world.Position) > 0 || recon && world.Position.DistanceTo(point) < max(80, enemy.observedStrategicSpeed()+24) {
			continue
		}
		stage := patrolReliefStaging{point: world.Position}
		plans := map[int]int{}
		for _, f := range candidates {
			if f.carriesInfrastructure() || f.Position.DistanceTo(world.Position) > 2*f.StrategicSpeed(automaticMaxWarp(p, f)) {
				continue
			}
			route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, world.Position, world.ToTarget(), automaticRoutePolicy{Fast: true, MaxTurns: 2, AllowThreat: knownWithdrawalRoute(p, f.Position, world.Position)})
			if route == nil || route.WaitForFuel || route.NetworkRequest != nil || slices.ContainsFunc(route.Waypoints, func(wp Waypoint) bool { return wp.WarpSpeed == StargateWarpSpeed }) {
				continue
			}
			stage.fleets = append(stage.fleets, f)
			stage.routes = append(stage.routes, route)
			key := f.BattlePlanNum
			if isManagedPatrolPlan(f) {
				key = 0
			}
			plans[key] += f.Spec.TotalShips
			stage.ships = max(stage.ships, plans[key])
		}
		stages = append(stages, stage)
	}
	slices.SortStableFunc(stages, func(a, b patrolReliefStaging) int { return b.ships - a.ships })
	for _, stage := range stages {
		if len(stage.fleets) < 2 {
			continue
		}
		for i, leader := range stage.fleets[:min(3, len(stage.fleets))] {
			group := []*Fleet{leader}
			routes := []*expeditionRoute{stage.routes[i]}
			for j, f := range stage.fleets {
				if f != leader && compatiblePatrolPlans(leader, f) {
					group = append(group, f)
					routes = append(routes, stage.routes[j])
				}
			}
			if len(group) < 2 {
				continue
			}
			checks := 0
			var ready []*Fleet
			if recon {
				limit := t.reconPatrolReservation(p)
				ships := 0
				for j, f := range group {
					ships += f.Spec.TotalShips
					if ships > limit {
						break
					}
					ready = group[:j+1]
				}
				if len(ready) < 2 {
					continue
				}
			} else {
				ready, _ = readyPatrolPrefix(p, group, enemy, point, &checks, 14)
			}
			if ready == nil {
				continue
			}
			routes = routes[:len(ready)]
			// Verify the assembled force can get from its rally to the encounter.
			merged := patrolMergedFormation(&t.game.Rules, p, ready)
			merged.Position, merged.Fuel = stage.point, 0
			turns := 0
			for _, route := range routes {
				turns = max(turns, route.Turns)
				merged.Fuel += route.ArrivalFuel[0]
			}
			merged.Fuel = min(merged.Spec.FuelCapacity, merged.Fuel)
			if !recon {
				attack := planAutomaticRoute(&t.game.Rules, p, []*Fleet{merged}, point, MapObjectTarget{}, automaticRoutePolicy{Fast: true, AllowThreat: true, MaxTurns: 2})
				if turns == 0 || attack == nil || attack.WaitForFuel || attack.NetworkRequest != nil {
					continue
				}
			} else {
				turns = max(1, turns)
			}
			job := patrolJob{Leader: leader.Num, LeaderID: leader.ID, Enemy: enemy.PlayerNum, Target: enemy.Num, RendezvousBy: t.game.Year + turns, RequiredShips: patrolFormationShips(ready), ReliefStaging: true}
			t.reservePatrolRelief(p, ready, &job)
			for _, f := range ready {
				savePatrolJob(f, job)
			}
			slog.Debug("relief force staging", "target", enemy.Name, "rally", stage.point, "ships", job.RequiredShips, "turns", turns)
			return applyPatrolRally(p, leader, &job, ready, routes)
		}
	}
	return false
}

func patrolAlreadyCoLocated(fleets []*Fleet) bool {
	return len(fleets) > 1 && !slices.ContainsFunc(fleets[1:], func(f *Fleet) bool { return f.Position != fleets[0].Position })
}

func (t *turnGenerator) continuePatrolRelief(p *Player, enemy *Fleet, leaders []*Fleet) bool {
	for _, leader := range leaders {
		j := readPatrolJob(leader)
		if !j.ReliefStaging || j.Leader != leader.Num || j.Enemy != enemy.PlayerNum || j.Target != enemy.Num || j.RendezvousBy < t.game.Year {
			continue
		}
		if t.movingPatrolRally(p, leader, &j) {
			for _, f := range t.game.getFleets(p.Num) {
				if !f.Delete && readPatrolJob(f).Leader == leader.Num {
					p.patrolEvaluator.DefenceMembers[f.Num] = true
				}
			}
			return true
		}
	}
	return false
}
