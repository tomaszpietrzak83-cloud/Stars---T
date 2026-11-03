package cs

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"
)

const patrolFrontTag = "patrol-front"

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
	appendCandidate := func(obj MapObject, center Vector, standOff, pressure float64) {
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
		// Approach from friendly fuel territory, never choose the enemy's orbit.
		position := center
		if nearest <= standOff {
			position = anchor.Position
		} else {
			position = center.ToFloat64().Add(anchor.Position.Subtract(center).Normalized().Scale(standOff)).ToInt(true)
		}
		c := patrolFrontCandidate{patrolFront: patrolFront{Owner: obj.PlayerNum, Target: obj.Num, Type: obj.Type, Position: position}, name: obj.Name, center: center, distance: f.Position.DistanceTo(position), pressure: pressure, anchor: anchor}
		allocated := 0.0
		for _, other := range t.game.getFleets(p.Num) {
			if other == f || !patrolOperational(p, other) || p.Automation.FleetRoles[other.Num] != "patrol" || readPatrolJob(other).Leader != 0 {
				continue
			}
			var front patrolFront
			if json.Unmarshal([]byte(other.GetTag(patrolFrontTag)), &front) == nil && front.Owner == c.Owner && front.Target == c.Target && front.Type == c.Type && (len(other.Waypoints) > 1 || other.Position.DistanceTo(position) < 100) {
				allocated += float64(patrolPower(other)) / float64(unit)
			}
		}
		c.score = (100 + c.distance) * (1 + allocated) / max(1, pressure)
		var previous patrolFront
		if json.Unmarshal([]byte(f.GetTag(patrolFrontTag)), &previous) == nil && previous.Owner == c.Owner && previous.Target == c.Target && previous.Type == c.Type {
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
		appendCandidate(enemy.MapObject, patrolInterceptPosition(enemy), max(80, enemy.observedStrategicSpeed()+24), min(256, pressure))
	}
	// Known hostile colonies provide the next frontier after mobile contacts clear.
	for _, planet := range p.PlanetIntels {
		if planet == nil || !planet.Owned() || planet.PlayerNum == p.Num || p.IsFriend(planet.PlayerNum) || planet.Unexplored() {
			continue
		}
		pressure := 2.0
		if p.Automation.PriorityPlanets[planet.Num] {
			pressure = 12
		}
		appendCandidate(planet.MapObject, planet.Position, 120, pressure)
	}
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
		if f.Position.DistanceTo(position) < 12 {
			// A short observation leg keeps ships active without closing on the enemy.
			direction := c.anchor.Position.Subtract(c.center).Normalized()
			side := 1.0
			if f.Num%2 == 0 {
				side = -1
			}
			position = position.ToFloat64().Add(VectorFloat64{X: -direction.Y * 12 * side, Y: direction.X * 12 * side}).ToInt(true)
		}
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
			policy := economicalContainmentPolicy(p, f, f.Position.DistanceTo(dest))
			route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, dest, MapObjectTarget{}, policy)
			if route == nil || route.WaitForFuel {
				continue
			}
			data, _ := json.Marshal(c.patrolFront)
			f.SetTag(patrolFrontTag, string(data))
			f.SetTag(patrolSectorTag, strconv.Itoa(c.anchor.Num))
			applyExpeditionRoute(p, f, route)
			f.SetTag(frontierPatrolTag, fmt.Sprintf("Advancing to frontier near %s; gathering outside enemy reach, return fuel reserved", c.name))
			return true
		}
	}
	delete(f.Tags, patrolFrontTag)
	return false
}
