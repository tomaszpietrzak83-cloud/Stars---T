package cs

import (
	"math"
	"slices"
	"strconv"
)

// A reinforcement request is a fresh assessment on the next planning pass,
// not ownership of hunters while no viable attack exists.
func (t *turnGenerator) disperseUnreadyPatrol(p *Player, leader, enemy *Fleet, job *patrolJob) {
	if p.patrolEvaluator.Deferred == nil {
		p.patrolEvaluator.Deferred = map[string]bool{}
	}
	p.patrolEvaluator.Deferred[priorityFleetKey(enemy.PlayerNum, enemy.Num)] = true
	job.Grouped = !t.ordersOnly
	savePatrolJob(leader, *job)
	// Only this pack needs immediate reassignment. Replanning every idle
	// patrol here repeats the global planner for each unsuccessful pack.
	released := map[int]bool{}
	existing := map[int]bool{}
	for _, f := range t.game.getFleets(p.Num) {
		existing[f.Num] = true
		if readPatrolJob(f).Leader == leader.Num {
			released[f.Num] = true
		}
	}
	t.releasePatrol(p, leader)
	if p.patrolEvaluator.PlanningDefence {
		return
	}
	// Split children must receive work in this pass too, even when created after
	// the outer planner took its fleet snapshot.
	// Reassigning our ships does not change enemy fleet/design reports. Share
	// reconnaissance answers only for this batch, never with later phases.
	needsRecon := patrolReconSnapshot(p)
	for _, f := range slices.Clone(t.game.getFleets(p.Num)) {
		if existing[f.Num] && !released[f.Num] {
			continue
		}
		if !patrolAvailable(p, f) || readPatrolJob(f).Leader != 0 || !f.Idle() {
			continue
		}
		if !t.assignSoloPatrolWithRecon(p, f, needsRecon) {
			t.patrolFrontier(p, f)
		}
	}
}

func (t *turnGenerator) assignSoloPatrol(p *Player, f *Fleet) bool {
	return t.assignSoloPatrolWithRecon(p, f, func(enemy *Fleet) bool { return patrolNeedsRecon(p, enemy) })
}

const patrolReturnHuntTag = "patrol-return-hunt"

func (t *turnGenerator) patrolReturnHunt(p *Player, f *Fleet) bool {
	until, _ := strconv.Atoi(f.GetTag(patrolReturnHuntTag))
	if until < t.game.Year || atOwnFuelDock(p, f) {
		delete(f.Tags, patrolReturnHuntTag)
		return false
	}
	if t.assignSoloPatrol(p, f) {
		return true
	}
	return t.returnPatrolToFuel(p, f)
}

func (t *turnGenerator) assignSoloPatrolWithRecon(p *Player, f *Fleet, needsRecon func(*Fleet) bool) bool {
	homeDistance := func(point Vector) float64 {
		distance := math.Inf(1)
		for _, world := range t.game.getPlanets(p.Num) {
			distance = min(distance, point.DistanceTo(world.Position))
		}
		return distance
	}
	homeward := f.GetTag(patrolReturnHuntTag) != ""
	currentHomeDistance := math.Inf(1)
	if homeward {
		currentHomeDistance = homeDistance(f.Position)
	}
	// Assignments stay unchanged until this function selects a target and
	// returns. Build the occupied-target set once, not once per enemy report.
	claimed := make(map[[2]int]bool)
	for _, other := range t.game.getFleets(p.Num) {
		if other.Delete || other == f {
			continue
		}
		job := readPatrolJob(other)
		if job.Leader != 0 {
			claimed[[2]int{job.Enemy, job.Target}] = true
		}
	}
	for _, enemy := range sortedPatrolTargets(p, f) {
		if enemy == nil || enemy.Delete || enemy.ReportAge != 0 || enemy.Starbase || enemy.PlayerNum == p.Num || p.IsFriend(enemy.PlayerNum) || needsRecon(enemy) || declinedPatrolTarget(f, enemy, t.game.Year) || p.patrolEvaluator.Deferred[priorityFleetKey(enemy.PlayerNum, enemy.Num)] {
			continue
		}
		if claimed[[2]int{enemy.PlayerNum, enemy.Num}] {
			continue
		}
		point := patrolInterceptPosition(enemy)
		if homeward && (patrolReportedCombatant(p, enemy) || homeDistance(point) >= currentHomeDistance) {
			continue
		}
		route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, point, MapObjectTarget{}, automaticRoutePolicy{Fast: true, AllowThreat: true, MaxTurns: 2, MinWarp: automaticCruiseWarp(p, f)})
		if route == nil || route.WaitForFuel {
			continue
		}
		result := p.patrolEvaluator.assess(p, []*Fleet{f}, enemy, point, 1)
		if !result.Ready {
			continue
		}
		savePatrolJob(f, patrolJob{Leader: f.Num, Enemy: enemy.PlayerNum, Target: enemy.Num, RequiredShips: 1, Status: "Dispersed hunting: intercepting " + enemy.Name})
		applyPatrolBattlePlan(p, f, result.Tactic)
		applyExpeditionRoute(p, f, route)
		return true
	}
	return false
}

// Early arrivals cover the perimeter while peers approach. Every leg is funded
// and must permit return by the original deadline. If this is impossible the
// caller abandons the assembly instead of turning it into an indefinite wait.
func (t *turnGenerator) screenPatrolRally(p *Player, f *Fleet, by int) bool {
	j := readPatrolJob(f)
	remaining := by - t.game.Year
	if remaining < 2 {
		return false
	}
	for _, delta := range []Vector{{12, 0}, {0, 12}, {-12, 0}, {0, -12}} {
		point := j.Rally.Add(delta)
		out := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, point, MapObjectTarget{}, automaticRoutePolicy{Fast: true, EconomicalPatrol: true, MaxTurns: 1})
		if out == nil || out.WaitForFuel {
			continue
		}
		arrival := *f
		arrival.Position, arrival.Fuel = point, f.Fuel-out.Fuel
		if len(out.ArrivalFuel) > 0 {
			arrival.Fuel = out.ArrivalFuel[0]
		}
		back := planAutomaticRoute(&t.game.Rules, p, []*Fleet{&arrival}, j.Rally, MapObjectTarget{}, automaticRoutePolicy{Fast: true, EconomicalPatrol: true, MaxTurns: remaining - out.Turns})
		if back == nil || back.WaitForFuel || out.Turns+back.Turns > remaining {
			continue
		}
		applyExpeditionRoute(p, f, out)
		j.Status = "Covering rendezvous perimeter; return before attack deadline"
		savePatrolJob(f, j)
		return true
	}
	return false
}

// Fill the nearest-to-full tanks with fuel left after every ship's return
// reserve. SplitAll distributes fuel first; this step conserves that total.
func (t *turnGenerator) balanceDispersedPatrolFuel(p *Player, ships []*Fleet) {
	if len(ships) < 2 || !patrolFuelAreaClear(p, ships[0], ships[0].Position) {
		return
	}
	ships = slices.Clone(ships)
	reserves := map[*Fleet]int{}
	for _, f := range ships {
		if f.Delete || f.Spec.TotalShips != 1 || f.Position != ships[0].Position || p.Automation.FleetRoles[f.Num] != "patrol" {
			continue
		}
		reserve := automaticReturnFuel(&t.game.Rules, p, f, f.Position, expeditionStations(p, f), false)
		if f.Fuel >= reserve {
			reserves[f] = reserve
		}
	}
	ships = slices.DeleteFunc(ships, func(f *Fleet) bool { _, ok := reserves[f]; return !ok })
	slices.SortStableFunc(ships, func(a, b *Fleet) int { return (a.Spec.FuelCapacity - a.Fuel) - (b.Spec.FuelCapacity - b.Fuel) })
	for i, receiver := range ships {
		need := receiver.Spec.FuelCapacity - receiver.Fuel
		if need <= 0 {
			continue
		}
		surplus := 0
		for _, f := range ships[i+1:] {
			surplus += f.Fuel - reserves[f]
		}
		if surplus < need {
			break
		}
		for j := len(ships) - 1; j > i && need > 0; j-- {
			f := ships[j]
			amount := min(need, f.Fuel-reserves[f])
			receiver.Fuel += amount
			f.Fuel -= amount
			need -= amount
			f.ComputeFuelUsage(p)
		}
		receiver.ComputeFuelUsage(p)
	}
}

const patrolSectorReserveTag = "patrol-sector-reserve"

// After relief, leave half the local force for one additional turn if an
// enemy bomber was recently reported at a nearby friendly colony.
func (t *turnGenerator) retainPatrolSectorReserve(p *Player, stack *Fleet) *Fleet {
	if t.ordersOnly || stack.Spec.TotalShips < 8 || stack.CargoMass() > 0 || fuelNetworkReserved(stack) {
		return nil
	}
	recent := false
	for _, memory := range p.Automation.PatrolMemory {
		if t.game.Year-memory.Seen > 2 || p.IsFriend(memory.Enemy) || stack.Position.DistanceTo(memory.Position) > max(80, 2*stack.StrategicSpeed(patrolFreeWarp(p, stack))) {
			continue
		}
		bomber := false
		for _, num := range memory.Designs {
			if design := p.GetShipDesignIntel(memory.Enemy, num); design != nil {
				if hull := StaticTechStore.GetHull(design.Hull); hull != nil && hull.Class == ShipClassBomber {
					bomber = true
					break
				}
			}
		}
		if !bomber {
			continue
		}
		for _, world := range t.game.getPlanets(p.Num) {
			if world.Position == memory.Position {
				recent = true
				break
			}
		}
		if recent {
			break
		}
	}
	if !recent {
		return nil
	}
	spare := []ShipToken{}
	left := stack.Spec.TotalShips / 2
	for _, token := range stack.Tokens {
		if left == 0 {
			break
		}
		token.Quantity = min(left, token.Quantity)
		spare = append(spare, token)
		left -= token.Quantity
	}
	child, err := (&orders{}).splitFleetTokens(&t.game.Rules, p, t.game.getFleets(p.Num), stack, spare)
	if err != nil {
		return nil
	}
	p.InheritAutomaticRole(stack, child)
	p.Automation.FleetRoles[child.Num] = "patrol"
	stack.SetTag(patrolSectorReserveTag, strconv.Itoa(t.game.Year+1))
	t.game.Fleets = append(t.game.Fleets, child)
	_ = t.game.Universe.addFleet(child)
	return child
}

func localPatrolReserve(f *Fleet, year int) bool {
	until, err := strconv.Atoi(f.GetTag(patrolSectorReserveTag))
	return err == nil && until >= year
}
