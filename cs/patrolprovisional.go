package cs

// Reconnaissance gates an attack, never all preparatory movement. Concentrate
// using reports while a scout determines equipment; no enemy waypoint is set.
func (t *turnGenerator) provisionalPatrolGather(p *Player, leader, enemy *Fleet, job *patrolJob) {
	center := patrolInterceptPosition(enemy)
	var anchor *Planet
	for _, planet := range expeditionStations(p, leader) {
		if anchor == nil || planet.Position.DistanceTo(center) < anchor.Position.DistanceTo(center) {
			anchor = planet
		}
	}
	from := leader.Position
	if anchor != nil {
		from = anchor.Position
	}
	direction := from.Subtract(center).Normalized()
	if direction.LengthSquared() == 0 {
		direction = VectorFloat64{X: 1}
	}
	radius := max(80, enemy.observedStrategicSpeed()+24)
	point := center.ToFloat64().Add(direction.Scale(radius)).ToInt(true)
	job.Provisional = true
	job.Rally = point
	reason := "Concentrating outside reported enemy reach: " + job.Status
	if patrolNeedsRecon(p, enemy) {
		reason = "Concentrating safely while scout reports equipment of " + enemy.Name
	}
	for _, f := range t.game.getFleets(p.Num) {
		if f.Delete || (f != leader && readPatrolJob(f).Leader != leader.Num) || !patrolAvailable(p, f) {
			continue
		}
		if f.Position.DistanceTo(point) > 8 {
			policy := automaticRoutePolicy{Fast: true, EconomicalPatrol: true, MaxTurns: 20, AllowThreat: knownWithdrawalRoute(p, f.Position, point)}
			if route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, point, MapObjectTarget{}, policy); route != nil && !route.WaitForFuel {
				applyExpeditionRoute(p, f, route)
			} else {
				holdAutomaticFleet(p, f, "Provisional concentration needs funded safe route")
			}
		} else {
			holdAutomaticFleet(p, f, "Holding safe concentration while scout reports equipment")
		}
		next := readPatrolJob(f)
		next.Provisional = true
		next.Rally = point
		next.Status = reason
		savePatrolJob(f, next)
	}
	job.Status = reason
	savePatrolJob(leader, *job)
}

func provisionalPatrolLeg(p *Player, f *Fleet) bool {
	job := readPatrolJob(f)
	enemy := patrolEnemy(p, job)
	if !job.Provisional || enemy == nil || len(f.Waypoints) < 2 || f.Waypoints[len(f.Waypoints)-1].Position != job.Rally {
		return false
	}
	for _, wp := range f.Waypoints[1:] {
		if wp.TargetType == MapObjectTypeFleet {
			return false
		}
	}
	return job.Rally.DistanceTo(patrolInterceptPosition(enemy)) >= max(79, enemy.observedStrategicSpeed()+23)
}
