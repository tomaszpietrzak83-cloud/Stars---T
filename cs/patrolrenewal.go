package cs

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
	routes := make([]*expeditionRoute, len(group))
	for i, f := range group {
		routes[i] = planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, point, MapObjectTarget{}, automaticRoutePolicy{AllowThreat: true, EconomicalPatrol: true, MaxWarp: min(6, automaticMaxWarp(p, f)), MinWarp: 1, MaxTurns: 20})
		if routes[i] == nil || routes[i].WaitForFuel {
			return false
		}
	}
	for i, f := range group {
		applyExpeditionRoute(p, f, routes[i])
		j := readPatrolJob(f)
		j.Status = "Moving to shared patrol rendezvous; nearby interception remains under review"
		savePatrolJob(f, j)
	}
	job.Status = "Moving to shared patrol rendezvous; all available groups approach"
	savePatrolJob(leader, *job)
	return true
}

func nearbyPatrolContact(p *Player, f *Fleet, radius float64) bool {
	for _, enemy := range p.FleetIntels {
		if enemy != nil && !enemy.Delete && !enemy.Starbase && enemy.ReportAge == 0 && enemy.PlayerNum != p.Num && !p.IsFriend(enemy.PlayerNum) && f.Position.DistanceTo(enemy.Position) <= radius {
			return true
		}
	}
	return false
}
