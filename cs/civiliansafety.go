package cs

import "math"

// Civilian vulnerability survives splitting for refit: an escort which belongs
// to a bomber replacement group must not keep following an unsafe old rally.
func civilianFleet(p *Player, f *Fleet) bool {
	role := p.Automation.FleetRoles[f.Num]
	if f.Spec.Bomber || !f.Spec.HasWeapons || role == "minesweeper" || role == "bomber" || role == "freighter" {
		return true
	}
	if work := readMaintenance(f); work != nil {
		return work.OriginalRole == "bomber" || work.OriginalRole == "freighter" || work.OriginalRole == "minesweeper"
	}
	return false
}

func (t *turnGenerator) civilianReports(p *Player, point Vector, horizon int) []*Fleet {
	if p.AIControlled {
		return nil
	}
	key := civilianWarningKey{Player: p.Num, Point: point, Horizon: horizon}
	if t.civilianWarnings == nil {
		t.civilianWarnings = map[civilianWarningKey][]*Fleet{}
	}
	if reports, ok := t.civilianWarnings[key]; ok {
		return reports
	}
	var reports []*Fleet
	for _, e := range bomberWarningReports(p, &Fleet{MapObject: MapObject{Position: point}}) {
		if horizon <= 1 && e.Position.DistanceTo(point) > reportedMaximumSpeed(p, e)+3 {
			continue
		}
		reports = append(reports, e)
	}
	t.civilianWarnings[key] = reports
	return reports
}

type civilianWarningKey struct {
	Player, Horizon int
	Point           Vector
}

func (t *turnGenerator) civilianDefense(p *Player, seeds []*Fleet, point Vector) []*Fleet {
	group := append([]*Fleet(nil), seeds...)
	for _, f := range t.game.getFleets(p.Num) {
		if f.Delete || f.Starbase || f.Position != point {
			continue
		}
		present := false
		for _, seed := range seeds {
			present = present || seed == f
		}
		if present || len(f.Waypoints) > 1 && f.Waypoints[1].Position != point {
			continue
		}
		group = append(group, f)
	}
	for _, planet := range t.game.getPlanets(p.Num) {
		if planet.Position == point && planet.Starbase != nil && !planet.Starbase.Delete {
			base := *planet.Starbase
			base.Num = -100000 - planet.Num
			group = append(group, &base)
			break
		}
	}
	return group
}

func (t *turnGenerator) defensiveEvaluator(p *Player) *patrolEvaluator {
	if t.bombardmentEvaluation == nil {
		t.bombardmentEvaluation = map[int]*patrolEvaluator{}
	}
	e := t.bombardmentEvaluation[p.Num]
	if e == nil {
		e = &patrolEvaluator{rules: &t.game.Rules, cache: map[[32]byte]patrolAssessment{}, Limit: 2048}
		t.bombardmentEvaluation[p.Num] = e
	}
	return e
}

func (t *turnGenerator) civilianSiteSafe(p *Player, f *Fleet, point Vector, horizon int) bool {
	reports := t.civilianReports(p, point, horizon)
	if len(reports) == 0 {
		return true
	}
	seed := *f
	seed.Position = point
	seed.Waypoints = []Waypoint{{Position: point}}
	seeds := []*Fleet{&seed}
	group := t.civilianDefense(p, nil, point)
	for i, member := range group {
		if member == f {
			group = append(group[:i], group[i+1:]...)
			break
		}
	}
	group = append(seeds, group...)
	return t.defensiveEvaluator(p).assessFormations(p, group, reports, point, float64(horizon), false).Ready
}

func (t *turnGenerator) civilianConstructionSafe(p *Player, planet *Planet, d *ShipDesign) bool {
	if p.AIControlled || d == nil || d.Spec.Starbase || !d.Spec.Bomber && d.Spec.HasWeapons && !dedicatedMinesweeperSpec(d.Spec) {
		return true
	}
	if len(t.civilianReports(p, planet.Position, 1)) == 0 {
		return true
	}
	f := newFleetForToken(p, -2, ShipToken{DesignNum: d.Num, Quantity: 1, design: d}, []Waypoint{{Position: planet.Position}})
	f.Position = planet.Position
	f.Spec = ComputeFleetSpec(&t.game.Rules, p, &f)
	f.Fuel = f.Spec.FuelCapacity
	return t.civilianSiteSafe(p, &f, planet.Position, 1)
}

func (t *turnGenerator) civilianFirstLegSafe(p *Player, f *Fleet, route *expeditionRoute) bool {
	if route == nil || route.WaitForFuel {
		return false
	}
	if len(route.Waypoints) < 2 {
		return true
	}
	wp := route.Waypoints[1]
	point := wp.Position
	if wp.WarpSpeed != StargateWarpSpeed {
		if distance := f.Position.DistanceTo(point); distance > f.StrategicSpeed(wp.WarpSpeed) {
			point = f.Position.ToFloat64().Add(point.Subtract(f.Position).Normalized().Scale(f.StrategicSpeed(wp.WarpSpeed))).ToInt(true)
		}
	}
	probe := *f
	closest, ending, _ := reportedEscapeSeparation(p, &probe, point)
	return closest > 3 && ending > 3
}

// Relocate the whole maintenance rendezvous, including server-owned queue
// metadata, without canceling paid replacements or the saved original mission.
func (t *turnGenerator) safeMaintenanceRally(p *Player, f *Fleet, work *fleetMaintenance) *Planet {
	current := t.game.getPlanet(work.RallyPlanetNum)
	if current != nil && current.OwnedBy(p.Num) && current.Spec.HasStarbase && t.civilianSiteSafe(p, f, current.Position, 2) {
		return current
	}
	var members []*Fleet
	for _, m := range t.game.getFleets(p.Num) {
		if !m.Delete && m.GetTag(replacementGroupTag) == work.Group {
			members = append(members, m)
		}
	}
	if len(members) == 0 {
		members = []*Fleet{f}
	}
	var chosen *Planet
	score := math.Inf(1)
	for _, dock := range expeditionStations(p, f) {
		if !t.civilianSiteSafe(p, f, dock.Position, 2) {
			continue
		}
		time := 0
		valid := true
		for _, member := range members {
			route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{member}, dock.Position, dock.ToTarget(), automaticRoutePolicy{AllowThreat: true, Fast: true, HighestWarp: true, MaxTurns: 20})
			if !t.civilianFirstLegSafe(p, member, route) {
				valid = false
				break
			}
			time = max(time, route.Turns)
		}
		if valid && (float64(time) < score || float64(time) == score && (chosen == nil || dock.Num < chosen.Num)) {
			chosen, score = dock, float64(time)
		}
	}
	if chosen == nil {
		return nil
	}
	work.RallyPlanetNum = chosen.Num
	for _, member := range members {
		if job := readMaintenance(member); job != nil {
			job.RallyPlanetNum = chosen.Num
			job.Status = "Rendezvous relocated to " + chosen.Name + " before a reported raid"
			writeMaintenance(member, job)
		}
	}
	for _, planet := range t.game.getPlanets(p.Num) {
		for i := range planet.ProductionQueue {
			item := &planet.ProductionQueue[i]
			if job := decodeMaintenance(item.Tags[replacementOrderTag]); job != nil && job.Group == work.Group {
				job.RallyPlanetNum = chosen.Num
				item.Tags[replacementOrderTag] = encodeMaintenance(job)
			}
		}
	}
	return chosen
}
