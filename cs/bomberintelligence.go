package cs

import (
	"encoding/json"
	"math"
	"slices"
	"sort"
	"strconv"
)

const bomberBorrowTag = "bomber-protection-borrow"

type bomberBorrow struct {
	Role   string      `json:"role"`
	Orders FleetOrders `json:"orders"`
}

// Detection must precede interception by a full movement turn. Use native cloak
// and Tachyon math, conservatively budgeting a 98%-cloaked warp-10 attacker.
// Interval union proves coverage of the entire corridor, without sample gaps.
func bomberScannerCorridor(scanners []scanner, from, to Vector, warningDistance float64, pen bool) bool {
	distance := from.DistanceTo(to)
	direction := to.Subtract(from).Normalized()
	intervals := [][2]float64{}
	for _, s := range scanners {
		radius := s.Range
		if pen {
			radius = s.RangePen
		}
		if radius < 0 {
			continue
		}
		reduction := s.CloakReductionFactor
		r := float64(radius)*getCloakFactor(98, reduction) - warningDistance
		if r < 0 {
			continue
		}
		dx, dy := float64(s.Position.X-from.X), float64(s.Position.Y-from.Y)
		along := dx*direction.X + dy*direction.Y
		cross := max(0, dx*dx+dy*dy-along*along)
		if cross > r*r {
			continue
		}
		width := math.Sqrt(max(0, r*r-cross))
		if distance == 0 {
			if from.DistanceTo(s.Position) <= r {
				return true
			}
			continue
		}
		intervals = append(intervals, [2]float64{along - width, along + width})
	}
	sort.Slice(intervals, func(i, j int) bool { return intervals[i][0] < intervals[j][0] })
	covered := 0.0
	for _, span := range intervals {
		if span[0] > covered+1e-8 {
			return false
		}
		covered = max(covered, span[1])
		if covered >= distance {
			return true
		}
	}
	return false
}
func (t *turnGenerator) bomberUnescortedSafe(p *Player, f *Fleet, to Vector) bool {
	if knownRouteThreat(p, f.Position, to) > 0 || bomberChaseThreat(p, f) > 0 {
		return false
	}
	scan := newPlayerScanner(t.game.Universe, t.game.Players, &t.game.Rules, p)
	scanners := scan.getScanners()
	warning := max(100, f.StrategicSpeed(10))
	for _, enemy := range p.FleetIntels {
		if enemy != nil && !p.IsFriend(enemy.PlayerNum) && enemy.PlayerNum != p.Num {
			warning = max(warning, enemy.observedStrategicSpeed())
		}
	}
	return bomberScannerCorridor(scanners, f.Position, to, warning, false) && bomberScannerCorridor(scanners, to, to, warning, true)
}
func bomberChaseThreat(p *Player, f *Fleet) int {
	power := 0
	for _, enemy := range p.FleetIntels {
		if enemy == nil || enemy.Delete || enemy.PlayerNum == p.Num || p.IsFriend(enemy.PlayerNum) || enemy.ReportAge != 0 || !patrolReportedCombatant(p, enemy) {
			continue
		}
		// The built-in AI targets planets, not passing fleets. A possible
		// change of course is not a pursuit order; actual arrivals are checked
		// separately by interceptionThreat.
		if p.planetOnlyAI[enemy.PlayerNum] {
			continue
		}
		distance := f.Position.DistanceTo(enemy.Position)
		speed := max(0, enemy.observedStrategicSpeed())
		toward := f.Position.Subtract(enemy.Position).Normalized()
		closing := enemy.Heading.X*toward.X + enemy.Heading.Y*toward.Y
		if distance <= speed+3 || closing > .5 && distance <= 2*speed+3 {
			power = min(1_000_000_000, power+patrolReportedPower(p, enemy))
		}
	}
	return power
}
func (t *turnGenerator) automaticBombingTarget(p *Player, f *Fleet) *Planet {
	candidates := []*Planet{}
	for _, planet := range p.PlanetIntels {
		if planet == nil || !planet.Explored() || !planet.Owned() || planet.PlayerNum == p.Num || p.IsFriend(planet.PlayerNum) || !priorityNeedsBombing(planet) && bombingDelegatedToLanding(&t.game.Rules, p, planet, t.game.getFleets(p.Num)) {
			continue
		}
		candidates = append(candidates, planet)
	}
	owned := t.game.getPlanets(p.Num)
	distances := map[int]float64{}
	borderDistance := func(planet *Planet) float64 {
		if cached, ok := distances[planet.Num]; ok {
			return cached
		}
		nearest := math.Inf(1)
		for _, colony := range owned {
			nearest = min(nearest, planet.Position.DistanceTo(colony.Position))
		}
		value := f.Position.DistanceTo(planet.Position) + nearest*.25
		distances[planet.Num] = value
		return value
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := p.Automation.PriorityPlanets[candidates[i].Num], p.Automation.PriorityPlanets[candidates[j].Num]
		if a != b {
			return a
		}
		x, y := borderDistance(candidates[i]), borderDistance(candidates[j])
		if x == y {
			return candidates[i].Num < candidates[j].Num
		}
		return x < y
	})
	for _, target := range candidates[:min(32, len(candidates))] {
		route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, target.Position, target.ToTarget(), automaticRoutePolicy{AllowThreat: true, MinWarp: automaticCruiseWarp(p, f), MaxTurns: 20})
		if route != nil {
			return target
		}
	}
	if len(candidates) > 0 {
		return candidates[0]
	} // Retain an explicit fuel/escort request, never silently disable the role.
	return nil
}

func borrowBomberProtection(p *Player, escort, bomber *Fleet) {
	if escort.GetTag(bomberBorrowTag) == "" {
		orders := escort.FleetOrders
		orders.Waypoints = slices.Clone(orders.Waypoints)
		raw, _ := json.Marshal(bomberBorrow{Role: p.Automation.FleetRoles[escort.Num], Orders: orders})
		escort.SetTag(bomberBorrowTag, string(raw))
	}
	p.Automation.FleetRoles[escort.Num] = "escort"
	delete(escort.Tags, escortStagingTag)
	delete(escort.Tags, frontierPatrolTag)
	escort.SetTag(escortTag, strconv.Itoa(bomber.Num))
}
func restoreBomberProtection(p *Player, escort *Fleet) bool {
	var old bomberBorrow
	if json.Unmarshal([]byte(escort.GetTag(bomberBorrowTag)), &old) != nil {
		return false
	}
	delete(escort.Tags, bomberBorrowTag)
	if old.Role == "" {
		delete(p.Automation.FleetRoles, escort.Num)
	} else {
		p.Automation.FleetRoles[escort.Num] = NormalizeAutomaticRole(old.Role)
	}
	orders := old.Orders
	orders.Waypoints = slices.Clone(orders.Waypoints)
	for len(orders.Waypoints) > 1 && orders.Waypoints[1].Position == escort.Position {
		orders.Waypoints = append(orders.Waypoints[:1], orders.Waypoints[2:]...)
	}
	if len(orders.Waypoints) > 0 {
		orders.Waypoints[0] = Waypoint{Position: escort.Position}
		escort.FleetOrders = orders
		escort.ComputeFuelUsage(p)
	}
	return true
}
func (t *turnGenerator) assignBomberProtection(p *Player, bomber *Fleet, target Vector, emergency bool) {
	needed, cover := 0, patrolPower(bomber)
	if emergency {
		needed = max(interceptionThreat(p, bomber), bomberChaseThreat(p, bomber))*3/2 + 1
		for _, f := range t.game.getFleets(p.Num) {
			if f.GetTag(escortTag) == strconv.Itoa(bomber.Num) && t.fundedPatrolSupport(p, f, bomber) {
				cover += patrolPower(f)
			}
		}
	}
	candidates := slices.Clone(t.game.getFleets(p.Num))
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].Position.DistanceTo(bomber.Position) < candidates[j].Position.DistanceTo(bomber.Position)
	})
	for _, f := range candidates {
		if emergency && cover >= needed {
			break
		}
		role := p.Automation.FleetRoles[f.Num]
		if landingManaged(f) || fleetHasDedicatedMinesweeper(p, f) || patrolNeededForColony(p, f) {
			continue
		}
		if f == bomber || f.Delete || !f.Spec.HasWeapons || f.InDesignMaintenance() || f.InAutomaticSafety() || fuelNetworkManaged(f) || readTankerJob(f) != nil || f.GetTag(tankerAwaitTag) != "" || f.GetTag(escortTag) != "" || f.GetTag("maintenance-pending") != "" || p.FleetInActiveAssembly(f) {
			continue
		}
		if role != "patrol" && role != "combat" {
			continue
		}
		if role == "patrol" && (readPatrolJob(f).Leader != 0 || !emergency) {
			continue
		}
		if emergency {
			if !t.fundedPatrolSupport(p, f, bomber) {
				continue
			}
		} else {
			// Attach to an existing protected mission, retaining its destination.
			if f.Position != bomber.Position || len(f.Waypoints) < 2 || f.Waypoints[len(f.Waypoints)-1].Position != target {
				continue
			}
		}
		borrowBomberProtection(p, f, bomber)
		cover += patrolPower(f)
		if !emergency {
			break
		}
	}
}
func (t *turnGenerator) bomberNearbyFuel(p *Player, f *Fleet) bool {
	if f.Fuel >= f.Spec.FuelCapacity {
		return false
	}
	var best *expeditionRoute
	for _, dock := range expeditionStations(p, f) {
		route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, dock.Position, dock.ToTarget(), automaticRoutePolicy{MaxTurns: 3, MinWarp: automaticCruiseWarp(p, f)})
		if route != nil && (best == nil || route.Turns < best.Turns) {
			best = route
		}
	}
	if best != nil {
		applyExpeditionRoute(p, f, best)
		return true
	}
	// Mobile fuel services approach this area; never send a bomber on a remote
	// warp-1 retreat merely because the nearest permanent base is far away.
	saveAutomaticFuelRequest(f, automaticFuelRequest{Destination: f.Position, Members: []int{f.Num}, Urgent: true})
	return false
}
func (t *turnGenerator) protectEvadingBomber(p *Player, f *Fleet) {
	if p.Automation.FleetRoles[f.Num] != "bomber" && p.Automation.FleetRoles[f.Num] != "tanker" || len(f.Waypoints) < 2 {
		return
	}
	goal := f.Waypoints[len(f.Waypoints)-1]
	for _, escort := range t.game.getFleets(p.Num) {
		if escort.GetTag(escortTag) == strconv.Itoa(f.Num) {
			route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{escort}, goal.Position, goal.MapObjectTarget, automaticRoutePolicy{AllowThreat: true})
			if route != nil {
				applyExpeditionRoute(p, escort, route)
			}
		}
	}
}
