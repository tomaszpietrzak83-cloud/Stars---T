package cs

import (
	"fmt"
	"math"
	"slices"
)

// Leave a little room for integer movement rounding. Only penetrating scanners
// reveal planets from space; ordinary scanner range must never be used here.
func scoutPlanetScanRadius(f *Fleet) float64 {
	return max(0, float64(f.Spec.ScanRangePen)-2)
}

func scoutScanPoint(from, target Vector, radius float64) Vector {
	if radius <= 0 {
		return target
	}
	if from.DistanceTo(target) <= radius {
		return from
	}
	return target.ToFloat64().Add(from.Subtract(target).Normalized().Scale(radius)).ToInt(true)
}

func scoutFleetScanRadius(f, enemy *Fleet) float64 {
	cloak := getCloakFactor(enemy.Spec.CloakPercent, f.Spec.ReduceCloaking)
	radius := float64(max(0, f.Spec.ScanRangePen)) * cloak
	if !enemy.Orbiting() {
		radius = max(radius, float64(max(0, f.Spec.ScanRange))*cloak)
	}
	return max(0, radius-2)
}

func scoutReportedFleetPosition(enemy *Fleet) Vector {
	// Extrapolate just the public observation, never private enemy orders.
	predicted := enemy.Position
	if !enemy.Orbiting() {
		predicted = predicted.ToFloat64().Add(enemy.Heading.Normalized().Scale(enemy.observedStrategicSpeed() * float64(enemy.ReportAge+1))).ToInt(true)
	}
	return predicted
}

func scoutFleetScanPoint(f, enemy *Fleet) Vector {
	return scoutScanPoint(f.Position, scoutReportedFleetPosition(enemy), scoutFleetScanRadius(f, enemy))
}

func scoutFleetReconPoint(p *Player, f, anchor *Fleet) Vector {
	best := scoutFleetScanPoint(f, anchor)
	radius := scoutFleetScanRadius(f, anchor)
	if radius <= 0 {
		return best
	}
	type report struct {
		position Vector
		radius   float64
	}
	reports := []report{}
	points := []*Planet{}
	for _, enemy := range p.FleetIntels {
		if enemy == nil || !enemyApproachingColony(p, enemy) || !reconCombatFormation(p, enemy) {
			continue
		}
		position := scoutReportedFleetPosition(enemy)
		if position.DistanceTo(scoutReportedFleetPosition(anchor)) > 2*radius {
			continue
		}
		reports = append(reports, report{position, scoutFleetScanRadius(f, enemy)})
		points = append(points, &Planet{MapObject: MapObject{Num: len(points) + 1, Position: position}})
	}
	score := math.Inf(1)
	for _, point := range append([]Vector{best}, scoutScanCandidates(f.Position, points, radius)...) {
		if point.DistanceTo(scoutReportedFleetPosition(anchor)) > radius {
			continue
		}
		count := 0
		for _, enemy := range reports {
			if point.DistanceTo(enemy.position) <= enemy.radius {
				count++
			}
		}
		if count == 0 {
			continue
		}
		value := (1 + f.Position.DistanceTo(point)/max(1, f.StrategicSpeed(automaticCruiseWarp(p, f)))) / float64(count)
		if value < score {
			best, score = point, value
		}
	}
	return best
}

func scoutColonyScanPoint(p *Player, f *Fleet, anchor *Planet) Vector {
	if recaptureEquipmentMissing(p, anchor) {
		return anchor.Position
	}
	radius := scoutPlanetScanRadius(f)
	best := scoutScanPoint(f.Position, anchor.Position, radius)
	if radius <= 0 {
		return best
	}
	targets := []*Planet{anchor}
	for _, planet := range p.PlanetIntels {
		if planet != nil && planet.Num != anchor.Num && planet.Owned() && planet.PlayerNum != p.Num && !p.IsFriend(planet.PlayerNum) && !planet.Unexplored() && planet.ReportAge > 2 && planet.Position.DistanceTo(anchor.Position) <= 2*radius {
			targets = append(targets, planet)
		}
	}
	score := math.Inf(1)
	for _, point := range append([]Vector{best}, scoutScanCandidates(f.Position, targets, radius)...) {
		if point.DistanceTo(anchor.Position) > radius {
			continue
		}
		count := 0
		for _, planet := range targets {
			if point.DistanceTo(planet.Position) <= radius {
				count++
			}
		}
		value := (1 + f.Position.DistanceTo(point)/max(1, f.StrategicSpeed(automaticCruiseWarp(p, f)))) / float64(count)
		if value < score {
			best, score = point, value
		}
	}
	return best
}

// Candidate stops include shared scan areas between neighbouring worlds. Score
// real end-of-leg observations, not swept circles along a multi-year journey:
// native scanning runs at year end, not continuously during movement.
func scoutScanCandidates(from Vector, targets []*Planet, radius float64) []Vector {
	near := slices.Clone(targets)
	slices.SortFunc(near, func(a, b *Planet) int {
		da, db := from.DistanceSquaredTo(a.Position), from.DistanceSquaredTo(b.Position)
		if da != db {
			if da < db {
				return -1
			}
			return 1
		}
		return a.Num - b.Num
	})
	near = near[:min(24, len(near))]
	points := []Vector{}
	seen := map[Vector]bool{}
	add := func(v Vector) {
		if !seen[v] {
			seen[v] = true
			points = append(points, v)
		}
	}
	for i, a := range near {
		add(scoutScanPoint(from, a.Position, radius))
		add(a.Position)
		for _, b := range near[i+1:] {
			d := a.Position.DistanceTo(b.Position)
			if d == 0 || d > 2*radius {
				continue
			}
			mid := a.Position.ToFloat64().Add(b.Position.ToFloat64()).Scale(.5)
			add(mid.ToInt(true))
			offset := b.Position.Subtract(a.Position).Normalized()
			h := math.Sqrt(max(0, radius*radius-d*d/4))
			perp := VectorFloat64{-offset.Y * h, offset.X * h}
			add(mid.Add(perp).ToInt(true))
			add(mid.Subtract(perp).ToInt(true))
		}
	}
	return points
}

func (t *turnGenerator) planScannerExploration(p *Player, f *Fleet, claimed map[int]int, quota int) bool {
	radius := scoutPlanetScanRadius(f)
	if radius <= 0 {
		return false
	}
	m := readScoutMission(f)
	if m == nil {
		m = &scoutMission{Origin: f.Position}
	}
	home := scoutHome(p, f, m)
	if home == nil {
		return false
	}
	m.Home = home.Num
	if m.Refuelling || scoutHasRetirement(p, f) {
		if f.Position != home.Position || f.Fuel < f.Spec.FuelCapacity {
			t.advanceScoutPetal(p, f, m, claimed, quota)
			saveScoutMission(f, m)
			return true
		}
		m.Complete = true
		if scoutHasRetirement(p, f) {
			saveScoutMission(f, m)
			return false
		}
		m.Refuelling = false
	}
	virtual := *f
	waypoints := []Waypoint{{Position: f.Position}}
	used := map[int]bool{}
	m.Plan = nil
	m.Leg = 0
	m.Complete = false
	turns := 0
	for range min(8, max(1, quota)) {
		targets := []*Planet{}
		for _, planet := range p.PlanetIntels {
			if planet != nil && planet.Unexplored() && !used[planet.Num] && (claimed[planet.Num] == 0 || claimed[planet.Num] == f.Num) {
				targets = append(targets, planet)
			}
		}
		var best *expeditionRoute
		var position Vector
		var covered []*Planet
		score := math.Inf(1)
		type scanStop struct {
			point    Vector
			observed []*Planet
			estimate float64
		}
		stops := []scanStop{}
		for _, point := range scoutScanCandidates(virtual.Position, targets, radius) {
			observed := []*Planet{}
			for _, planet := range targets {
				if point.DistanceTo(planet.Position) <= radius {
					observed = append(observed, planet)
				}
			}
			if len(observed) == 0 {
				continue
			}
			stops = append(stops, scanStop{point, observed, (1 + virtual.Position.DistanceTo(point)/max(1, virtual.StrategicSpeed(automaticCruiseWarp(p, &virtual)))) / float64(len(observed))})
		}
		slices.SortStableFunc(stops, func(a, b scanStop) int {
			if a.estimate < b.estimate {
				return -1
			}
			if a.estimate > b.estimate {
				return 1
			}
			return 0
		})
		for _, stop := range stops[:min(16, len(stops))] {
			point, observed := stop.point, stop.observed
			proxy := &Planet{MapObject: MapObject{Position: point}}
			route := scoutEconomicalLeg(&t.game.Rules, p, &virtual, proxy)
			if route == nil || turns+route.Turns > p.Automation.scoutReturnTurns() {
				continue
			}
			after := scoutVirtualArrival(&virtual, proxy, route)
			back := scoutHomeRoute(&t.game.Rules, p, after, home)
			if back == nil || back.WaitForFuel || back.Turns > p.Automation.scoutReturnTurns() {
				continue
			}
			value := (float64(route.Turns) + .002*float64(route.Fuel) + .1*float64(back.Turns)) / float64(len(observed))
			if value < score {
				best, position, covered, score = route, point, observed, value
			}
		}
		if best == nil {
			break
		}
		// A scan stop is a coordinate, never an order to enter the anchor's orbit.
		for _, wp := range best.Waypoints[1:] {
			wp.MapObjectTarget = MapObjectTarget{}
			waypoints = append(waypoints, wp)
		}
		virtual.Position, virtual.OrbitingPlanetNum = position, 0
		virtual.Fuel = best.ArrivalFuel[0]
		turns += best.Turns
		for _, planet := range covered {
			used[planet.Num] = true
			claimed[planet.Num] = f.Num
			m.Plan = append(m.Plan, planet.Num)
		}
	}
	back := scoutHomeRoute(&t.game.Rules, p, &virtual, home)
	if back == nil || back.WaitForFuel {
		return false
	}
	if len(used) == 0 && f.Position == home.Position {
		return false
	}
	waypoints = append(waypoints, back.Waypoints[1:]...)
	m.Plan = append(m.Plan, home.Num)
	m.Refuelling = len(used) == 0
	m.Target = m.Plan[0]
	applyExpeditionRoute(p, f, &expeditionRoute{Waypoints: waypoints})
	f.SetTag(scoutTag, fmt.Sprintf("Scanner exploration: %d worlds covered by scan stops; funded return to %s", len(used), home.Name))
	saveScoutMission(f, m)
	return true
}
