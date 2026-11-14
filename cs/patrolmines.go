package cs

import (
	"fmt"
	"math"
	"slices"
)

const patrolMineSweepTag = "patrol-mine-sweep"

// Dedicated minesweepers clear reported hostile fields, starting at the rim.
func (t *turnGenerator) patrolSweepObstacle(p *Player, f *Fleet) bool {
	delete(f.Tags, patrolMineSweepTag)
	if !automaticMinesweeper(p, f) {
		return false
	}
	t.surveyMinesweeperField(p, f)
	return t.assignMinesweeperField(p, f, p.MinefieldIntels)
}

func (t *turnGenerator) surveyMinesweeperField(p *Player, f *Fleet) {
	// A sweep-capable ship physically surveys its current known field. This
	// prevents work on a cleared ghost field, or waiting outside its shrunken rim.
	fields := slices.Clone(p.MinefieldIntels)
	p.MinefieldIntels = slices.DeleteFunc(fields, func(field *Minefield) bool {
		if field == nil || field.PlayerNum == p.Num || p.IsFriend(field.PlayerNum) || f.Position.DistanceTo(field.Position) > field.Radius() {
			return false
		}
		actual := t.game.getMinefield(field.PlayerNum, field.Num)
		if actual == nil || actual.Delete || actual.NumMines <= 0 {
			return true
		}
		field.NumMines = actual.NumMines
		return false
	})
}

func (t *turnGenerator) assignMinesweeperField(p *Player, f *Fleet, fields []*Minefield) bool {
	var best *Minefield
	var route *expeditionRoute
	score := math.Inf(1)
	for _, field := range fields {
		candidate := t.minesweeperFieldRoute(p, f, field)
		if candidate == nil {
			continue
		}
		distance := max(0, f.Position.DistanceTo(field.Position)-field.Radius())
		value := float64(candidate.Turns)*1000 + distance
		if value < score {
			best, route, score = field, candidate, value
		}
	}
	if best == nil {
		return false
	}
	t.applyMinesweeperRoute(p, f, best, route)
	return true
}

func (t *turnGenerator) minesweeperFieldRoute(p *Player, f *Fleet, field *Minefield) *expeditionRoute {
	if !reportedHostileMinefield(p, field) || !f.willAttack(p, field.PlayerNum) {
		return nil
	}
	if f.Position.DistanceTo(field.Position)-field.Radius() > 4 {
		return t.minesweeperApproach(p, f, field)
	}
	return t.minesweeperCentreLeg(p, f, field)
}

func minesweeperSafeWarp(rules *Rules, p *Player, f *Fleet, field *Minefield) int {
	return min(automaticMaxWarp(p, f), rules.MinefieldStatsByType[field.MinefieldType].MaxSpeed+p.Race.Spec.MinefieldSafeWarpBonus)
}

func (t *turnGenerator) minesweeperCentreLeg(p *Player, f *Fleet, field *Minefield) *expeditionRoute {
	rules := &t.game.Rules
	goal := field.Position
	if p.getPlanetIntelAt(goal) != nil {
		// A position one light-year from the centre leaves at most one
		// unsweepable mine, below the engine's field removal threshold.
		offsets := []Vector{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}}
		slices.SortStableFunc(offsets, func(a, b Vector) int {
			return f.Position.Subtract(goal.Add(a)).LengthSquared() - f.Position.Subtract(goal.Add(b)).LengthSquared()
		})
		for _, offset := range offsets {
			point := goal.Add(offset)
			if p.getPlanetIntelAt(point) == nil && routeThreat(p, point, point, true) == 0 {
				goal = point
				break
			}
		}
		if goal == field.Position {
			return nil
		}
	}
	if f.Position == goal {
		return planAutomaticRoute(rules, p, []*Fleet{f}, goal, MapObjectTarget{}, automaticRoutePolicy{})
	}
	stations := expeditionStations(p, f)
	for warp := minesweeperSafeWarp(rules, p, f, field); warp > 0; warp-- {
		step := f.StrategicSpeed(warp)
		point := automaticTurnEnd(f.Position, goal, step)
		if f.Position.DistanceTo(point) > step {
			// Map coordinates are integral. Round inward if the native
			// endpoint would put the route budget just over one turn.
			point = automaticTurnEnd(f.Position, goal, max(0, step-1))
		}
		distance := f.Position.DistanceTo(point)
		if distance == 0 || distance > step || p.getPlanetIntelAt(point) != nil || point.DistanceTo(field.Position) >= field.Radius() || !automaticFleetLegSafe(rules, p, f, f.Position, point, warp, false) {
			continue
		}
		reserve := automaticReturnFuel(rules, p, f, point, stations, false)
		if _, remaining, burned, ok := automaticLegFuel(p, f, warp, distance, f.Fuel, reserve); ok {
			return &expeditionRoute{
				Waypoints: []Waypoint{{Position: f.Position}, {Position: point, WarpSpeed: warp}},
				Turns:     1, Fuel: burned, ArrivalFuel: []int{remaining},
			}
		}
	}
	return nil
}

func (t *turnGenerator) applyMinesweeperRoute(p *Player, f *Fleet, best *Minefield, route *expeditionRoute) {
	applyExpeditionRoute(p, f, route)
	distance := max(0, f.Position.DistanceTo(best.Position)-best.Radius())
	status := fmt.Sprintf("Approaching %s: %.0f ly to reported edge; planned arrival in %d turns; no mines removed by this order yet", best.Name, distance, route.Turns)
	if distance == 0 {
		status = "Inside reported field " + best.Name + ": advancing toward its centre; actual removals are reported after movement"
	}
	f.SetTag(minesweeperTargetTag, minefieldAssignmentKey(best))
	f.SetTag(patrolMineSweepTag, status)
	f.SetTag(fuelStatusTag, status)
}

// Cruise efficiently outside mines; cross the rim on a separate safe-warp leg.
// Both legs budget actual fuel and the final return, with no fictional top-up.
func (t *turnGenerator) minesweeperApproach(p *Player, f *Fleet, field *Minefield) *expeditionRoute {
	direction := f.Position.Subtract(field.Position).Normalized()
	for _, angle := range []float64{0, math.Pi / 4, -math.Pi / 4, math.Pi / 2, -math.Pi / 2, math.Pi} {
		sine, cosine := math.Sincos(angle)
		offset := VectorFloat64{X: direction.X*cosine - direction.Y*sine, Y: direction.X*sine + direction.Y*cosine}
		outside := field.Position.ToFloat64().Add(offset.Scale(field.Radius() + 2)).ToInt(true)
		if p.getPlanetIntelAt(outside) != nil {
			continue
		}
		if route := t.minesweeperApproachVia(p, f, field, outside); route != nil {
			return route
		}
	}
	return nil
}

func (t *turnGenerator) minesweeperApproachVia(p *Player, f *Fleet, field *Minefield, outside Vector) *expeditionRoute {
	approach := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, outside, MapObjectTarget{}, automaticRoutePolicy{EconomicalPatrol: true, MaxTurns: 20})
	if approach == nil || approach.WaitForFuel {
		return nil
	}
	for i, waypoint := range approach.Waypoints[1:] {
		from := approach.Waypoints[i].Position
		if waypoint.WarpSpeed == StargateWarpSpeed || waypoint.WarpSpeed <= 0 {
			return nil
		}
		for from != waypoint.Position {
			end := automaticTurnEnd(from, waypoint.Position, f.StrategicSpeed(waypoint.WarpSpeed))
			if end == from || p.getPlanetIntelAt(end) != nil {
				return nil
			}
			from = end
		}
	}
	arrival := *f
	arrival.Position = outside
	arrival.Fuel = f.Fuel - approach.Fuel
	if len(approach.ArrivalFuel) > 0 {
		arrival.Fuel = approach.ArrivalFuel[0]
	}
	entry := t.minesweeperCentreLeg(p, &arrival, field)
	if entry == nil || entry.WaitForFuel {
		return nil
	}
	return &expeditionRoute{Waypoints: append(slices.Clone(approach.Waypoints), entry.Waypoints[1:]...), Turns: approach.Turns + entry.Turns, Fuel: approach.Fuel + entry.Fuel, ArrivalFuel: entry.ArrivalFuel}
}
