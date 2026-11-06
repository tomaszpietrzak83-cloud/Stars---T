package cs

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"
)

const patrolFrontTag = "patrol-front"
const patrolFrontVisitsTag = "patrol-front-visits"

type patrolFrontVisit struct {
	Owner, Target, Until int
	Type                 MapObjectType
}

// Do not recruit backwater hunters for a long unarmed chase while visible
// combat formations need reinforcement. Nearby kills and imminent landings win.
func patrolShouldAdvanceInstead(p *Player, f, enemy *Fleet) bool {
	if enemy == nil || p.Automation.FleetRoles[f.Num] != "patrol" || priorityFleet(p, enemy) || patrolReportedCombatant(p, enemy) || f.Position.DistanceTo(enemy.Position) <= 150 {
		return false
	}
	if colony := patrolDefendedColony(p, enemy); colony != nil && colony.Position.DistanceTo(enemy.Position) <= max(1, enemy.observedStrategicSpeed())*2 {
		return false
	}
	for _, threat := range p.FleetIntels {
		if threat != nil && !threat.Delete && !threat.Starbase && threat.ReportAge == 0 && threat.PlayerNum != p.Num && !p.IsFriend(threat.PlayerNum) && (reportedFleetShips(threat) >= 8 || patrolReportedCombatant(p, threat)) {
			return true
		}
	}
	return false
}

type patrolFront struct {
	Owner    int           `json:"owner"`
	Target   int           `json:"target"`
	Type     MapObjectType `json:"type"`
	Position Vector        `json:"position"`
}

type patrolFrontCandidate struct {
	patrolFront
	name     string
	center   Vector
	distance float64
	pressure float64
	score    float64
	anchor   *Planet
	support  bool
}

// Local interception is planned first. Surplus hunters move toward reported
// enemy concentrations instead of reserving one ship at every quiet home base.
func (t *turnGenerator) advancePatrolFront(p *Player, f *Fleet) bool {
	if !patrolOperational(p, f) || readPatrolJob(f).Leader != 0 || f.GetTag(escortTag) != "" {
		return false
	}
	docks := expeditionStations(p, f)
	if usefulFreePatrolMovement(p, f) {
		for _, planet := range t.game.getPlanets(p.Num) {
			if !planet.Spec.HasStarbase {
				docks = append(docks, planet)
			}
		}
	}
	if len(docks) == 0 {
		return false
	}
	candidates := []patrolFrontCandidate{}
	unit := max(1, patrolPower(f)/max(1, f.Spec.TotalShips))
	type frontKey struct {
		owner, target int
		kind          MapObjectType
	}
	type allocation struct {
		fleet *Fleet
		power float64
	}
	// Candidate scoring does not modify fleets. Read each assignment once,
	// preserving fleet order within a front so scores remain unchanged.
	allocations := map[frontKey][]allocation{}
	for _, other := range t.game.getFleets(p.Num) {
		if other == f || p.Automation.FleetRoles[other.Num] != "patrol" || !patrolOperational(p, other) || readPatrolJob(other).Leader != 0 {
			continue
		}
		var front patrolFront
		raw := other.GetTag(patrolFrontTag)
		if raw != "" && json.Unmarshal([]byte(raw), &front) == nil {
			key := frontKey{front.Owner, front.Target, front.Type}
			allocations[key] = append(allocations[key], allocation{other, float64(patrolPower(other)) / float64(unit)})
		}
	}
	var previous patrolFront
	hasPrevious := json.Unmarshal([]byte(f.GetTag(patrolFrontTag)), &previous) == nil
	visits := []patrolFrontVisit{}
	_ = json.Unmarshal([]byte(f.GetTag(patrolFrontVisitsTag)), &visits)
	visits = slices.DeleteFunc(visits, func(v patrolFrontVisit) bool { return v.Until < t.game.Year })
	homeDistance := math.Inf(1)
	for _, dock := range docks {
		homeDistance = min(homeDistance, f.Position.DistanceTo(dock.Position))
	}
	appendCandidate := func(obj MapObject, center Vector, standOff, pressure float64, support bool) {
		var anchor *Planet
		nearest := math.Inf(1)
		for _, dock := range docks {
			if d := dock.Position.DistanceTo(center); d < nearest {
				nearest = d
				anchor = dock
			}
		}
		if anchor == nil {
			return
		}
		for _, visit := range visits {
			if visit.Owner == obj.PlayerNum && visit.Target == obj.Num && visit.Type == obj.Type {
				return
			}
		}
		// Approach from the ship's current side. A fuel dock funds the route;
		// it must not pull an advanced ship back to the friendly side of a front.
		position := center
		if standOff > 0 {
			direction := f.Position.Subtract(center)
			if direction == (Vector{}) {
				direction = anchor.Position.Subtract(center)
			}
			position = center.ToFloat64().Add(direction.Normalized().Scale(standOff)).ToInt(true)
		}
		if f.Position.DistanceTo(position) < 12 {
			// Reaching an observation area completes this advance. If the native
			// combat planner cannot attack, seek another objective, not a tiny orbit.
			visits = append(visits, patrolFrontVisit{obj.PlayerNum, obj.Num, t.game.Year + 3, obj.Type})
			return
		}
		c := patrolFrontCandidate{patrolFront: patrolFront{Owner: obj.PlayerNum, Target: obj.Num, Type: obj.Type, Position: position}, name: obj.Name, center: center, distance: f.Position.DistanceTo(position), pressure: pressure, anchor: anchor, support: support}
		allocated := 0.0
		for _, assignment := range allocations[frontKey{c.Owner, c.Target, c.Type}] {
			if len(assignment.fleet.Waypoints) > 1 || assignment.fleet.Position.DistanceTo(position) < 100 {
				allocated += assignment.power
			}
		}
		c.score = (100 + c.distance) * (1 + allocated) / max(1, pressure)
		// A deep force increasingly favors useful conquest support. Nearby kills
		// and urgent colony relief were already considered by the tactical pass.
		if support {
			c.score /= 2 + min(4, homeDistance/100)
		}
		if nearest+80 < homeDistance && !support {
			c.score *= 2
		}
		if hasPrevious && previous.Owner == c.Owner && previous.Target == c.Target && previous.Type == c.Type {
			c.score *= .8
		}
		candidates = append(candidates, c)
	}
	for _, enemy := range p.FleetIntels {
		if enemy == nil || enemy.Delete || enemy.Starbase || enemy.ReportAge < 0 || enemy.ReportAge > 2 || enemy.PlayerNum == p.Num || p.IsFriend(enemy.PlayerNum) || !patrolReportedCombatant(p, enemy) && reportedFleetShips(enemy) < 8 {
			continue
		}
		pressure := float64(max(reportedFleetShips(enemy), (patrolReportedPower(p, enemy)*3/2+unit-1)/unit))
		if priorityFleet(p, enemy) {
			pressure *= 3
		}
		appendCandidate(enemy.MapObject, patrolInterceptPosition(enemy), max(80, enemy.observedStrategicSpeed()+24), min(256, pressure), false)
	}
	// Transport operations own recruitment and native base-assault assessment.
	// Route surplus hunters into their reachable assembly area without reserving
	// them or changing the user's planet priorities. A retarget is read next turn.
	landingTargets := map[int]bool{}
	for _, leader := range t.game.getFleets(p.Num) {
		op := readLandingOperation(leader)
		if leader.Delete || op == nil || op.Need <= 0 || landingTargets[op.Target] {
			continue
		}
		target := p.GetPlanetIntel(op.Target)
		if target == nil || !target.Owned() || target.PlayerNum != op.Owner || target.PlayerNum == p.Num || p.IsFriend(target.PlayerNum) {
			continue
		}
		landingTargets[target.Num] = true
		appendCandidate(target.MapObject, op.Rally, 0, 12, true)
	}
	// Known hostile colonies provide the next frontier after mobile contacts clear.
	for _, planet := range p.PlanetIntels {
		if planet == nil || !planet.Owned() || planet.PlayerNum == p.Num || p.IsFriend(planet.PlayerNum) || planet.Unexplored() || landingTargets[planet.Num] {
			continue
		}
		pressure := 2 + min(6, homeDistance/100)
		if p.Automation.PriorityPlanets[planet.Num] {
			pressure = 12
		}
		appendCandidate(planet.MapObject, planet.Position, 80, pressure, false)
	}
	// When a front is cleared or temporarily cannot be attacked, expand contact
	// coverage at unexplored or stale neutral worlds instead of circling home.
	for _, planet := range p.PlanetIntels {
		if planet == nil || planet.Owned() || !planet.Unexplored() && planet.ReportAge < 4 {
			continue
		}
		appendCandidate(planet.MapObject, planet.Position, 0, 1, false)
	}
	if len(visits) > 16 {
		visits = visits[len(visits)-16:]
	}
	data, _ := json.Marshal(visits)
	f.SetTag(patrolFrontVisitsTag, string(data))
	slices.SortStableFunc(candidates, func(a, b patrolFrontCandidate) int {
		if a.score < b.score {
			return -1
		}
		if a.score > b.score {
			return 1
		}
		if a.Owner != b.Owner {
			return a.Owner - b.Owner
		}
		return a.Target - b.Target
	})
	for _, c := range candidates[:min(8, len(candidates))] {
		position := c.Position
		// Try the frontier itself, then funded forward legs. Far contacts must not
		// freeze a ship which can already advance to the next useful observation area.
		for _, limit := range []float64{math.Inf(1), 120, 60, 30} {
			dest := position
			distance := f.Position.DistanceTo(position)
			if distance > limit {
				dest = f.Position.ToFloat64().Add(position.Subtract(f.Position).Normalized().Scale(limit)).ToInt(true)
			}
			if dest == f.Position {
				continue
			}
			policy := automaticRoutePolicy{Fast: true, MaxTurns: 8}
			route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, dest, MapObjectTarget{}, policy)
			if route == nil || route.WaitForFuel {
				continue
			}
			data, _ := json.Marshal(c.patrolFront)
			f.SetTag(patrolFrontTag, string(data))
			f.SetTag(patrolSectorTag, strconv.Itoa(c.anchor.Num))
			applyExpeditionRoute(p, f, route)
			status := fmt.Sprintf("Offensive advance toward %s; return fuel reserved", c.name)
			if c.support {
				status = fmt.Sprintf("Advancing to support the invasion of %s; return fuel reserved", c.name)
			}
			f.SetTag(frontierPatrolTag, status)
			return true
		}
	}
	delete(f.Tags, patrolFrontTag)
	return false
}
