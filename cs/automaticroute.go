package cs

import (
	"math"
	"slices"
)

const fuelStatusTag = "automatic-fuel-status"
const fuelRequestTag = "automatic-fuel-request"

type automaticRoutePolicy struct {
	Fast             bool // Useful combat pursuit only; ordinary movement weighs fuel against travel time.
	HighestWarp      bool // Escape: prefer the highest funded safe warp even on equal-ETA legs.
	EconomicalPatrol bool
	EmergencyReturn  bool
	MaxTurns         int
	MinWarp          int
	ReturnTurnsLimit int
	MaxWarp          int
	AllowThreat      bool
	// Extra fuel to deliver at the destination, in addition to the safe return reserve.
	DeliveryFuel []int
	// Only a tanker mission with a reserved delivery may be a fuel destination.
	RefuelRendezvous bool
	// Colony ships are consumed by the Colonize task at their reserved target.
	OneWay bool
}

func automaticMaxWarp(player *Player, fleet *Fleet) int {
	warp := min(10, fleet.Spec.Engine.MaxSafeSpeed)
	for _, token := range fleet.Tokens {
		if token.design != nil {
			warp = min(warp, token.design.Spec.Engine.MaxSafeSpeed)
		}
	}
	if player.Race.Spec.EngineFailureRate > 0 {
		warp = min(warp, player.Race.Spec.EngineReliableSpeed)
	}
	return max(0, warp)
}

// Collision rules use the field owner's friendship, which is not guaranteed by
// our own diplomatic setting. Non-owned fields are therefore treated conservatively.
func automaticLegSafe(rules *Rules, player *Player, from, to Vector, warp int, allowThreat bool, strategicSpeed ...float64) bool {
	return automaticRouteLegSafe(rules, player, from, to, warp, allowThreat, false, false, strategicSpeed...)
}

func automaticFleetLegSafe(rules *Rules, player *Player, fleet *Fleet, from, to Vector, warp int, allowThreat bool) bool {
	return automaticRouteLegSafe(rules, player, from, to, warp, allowThreat, civilianTraffic(player, fleet), automaticMinesweeper(player, fleet), fleet.StrategicSpeed(warp))
}

// Check the next end-of-year position and the intended destination, not the
// swept corridor. Native battles happen after movement at shared positions.
// Routes are reconsidered next year using fresh public reports.
func automaticRouteLegSafe(rules *Rules, player *Player, from, to Vector, warp int, allowThreat, civilian, sweep bool, strategicSpeed ...float64) bool {
	step := float64(warp * warp)
	if len(strategicSpeed) > 0 {
		step = strategicSpeed[0]
	}
	if !allowThreat {
		if routeThreat(player, from, to, civilian) > 0 {
			return false
		}
		stop := automaticTurnEnd(from, to, step)
		if stop != to && routeThreat(player, from, stop, civilian) > 0 {
			return false
		}
	}
	return automaticMineLegSafe(rules, player, from, to, warp, step, sweep)
}

// Match the native movement engine's final sub-light-year snap and rounding.
func automaticTurnEnd(from, to Vector, step float64) Vector {
	distance := from.DistanceTo(to)
	if distance <= step || distance-step < 1 {
		return to
	}
	if step <= 0 {
		return from
	}
	return from.ToFloat64().Add(to.Subtract(from).Normalized().Scale(step)).ToInt(true)
}
func fullCargoBudget(fleet *Fleet) *Fleet {
	copy := *fleet
	if fleet.Spec.CargoCapacity > fleet.CargoMass() {
		copy.Cargo = Cargo{Ironium: max(0, fleet.Spec.CargoCapacity-readInfrastructure(fleet).mass())}
	}
	return &copy
}

// Budget the movement and generation in their real order: pay for movement
// first, then scoop and annual generator output. A generator cannot pay for a
// departure whose first movement charge exceeds the fuel actually on board.
// Repeated full turns have a constant cost, so long legs need no per-year loop.
func automaticLegFuel(player *Player, fleet *Fleet, warp int, distance float64, initial, endReserve int) (required, left, burned int, ok bool) {
	if warp == StargateWarpSpeed {
		return endReserve, initial, 0, initial >= endReserve
	}
	if distance == 0 {
		return endReserve, initial, 0, initial >= endReserve
	}
	step := fleet.StrategicSpeed(warp)
	if step <= 0 {
		return 0, initial, 0, false
	}
	turns := max(1, int(math.Ceil(distance/step)))
	repeat := turns - 1
	lastDistance := distance - float64(repeat)*step
	charge := func(d float64) int { return max(0, fleet.GetFuelCost(player, warp, math.Ceil(d)+1)) }
	gain := func(d float64) int {
		return max(0, fleet.getFuelGeneration(warp, max(0, math.Floor(d)-1))) + max(0, fleet.Spec.FuelGeneration)
	}
	lastCost, lastGain := charge(lastDistance), gain(lastDistance)
	required = max(lastCost, lastCost+endReserve-lastGain)
	if required > fleet.Spec.FuelCapacity {
		return required, 0, 0, false
	}
	fullCost, fullGain := charge(step), gain(step)
	delta := fullCost - fullGain
	if repeat > 0 {
		if fullCost > fleet.Spec.FuelCapacity {
			return fullCost, 0, 0, false
		}
		if delta >= 0 {
			required = max(fullCost, required+delta) + (repeat-1)*delta
		} else {
			required = max(fullCost, required+repeat*delta)
		}
	}
	if required > initial || required > fleet.Spec.FuelCapacity {
		return required, 0, 0, false
	}
	left = initial
	if repeat > 0 {
		left = min(fleet.Spec.FuelCapacity, left-repeat*delta)
	}
	left = min(fleet.Spec.FuelCapacity, left-lastCost+lastGain)
	return required, left, repeat*fullCost + lastCost, true
}

// A pickup must leave enough fuel even after loading the cargo hold. The reserve
// is a reachable dock, not a guessed percentage of the fuel tank.
func automaticReturnFuel(rules *Rules, player *Player, fleet *Fleet, position Vector, stations []*Planet, allowThreat bool, returnTurns ...int) int {
	defer scopedRouteThreatIndex(player)()
	// A nested reserve calculation reads the same snapshot as its route
	// search. Reuse that cache, but create/release one for standalone calls.
	if player.routeThreatIndex.routes == nil {
		defer scopedRouteThreatCache(player)()
	}
	best := int(^uint(0) >> 2)
	loaded := deliveryReturnBudget(player, fleet, position)
	limit := 0
	if len(returnTurns) > 0 {
		limit = returnTurns[0]
	}
	if automatic := automaticReturnLimit(player, fleet, MapObjectTarget{}); automatic > 0 && (limit == 0 || limit > automatic) {
		limit = automatic
	}
	maxWarp, cruiseWarp := automaticMaxWarp(player, fleet), automaticCruiseWarp(player, fleet)
	role := player.Automation.FleetRoles[fleet.Num]
	cruiseRequired := role == "scout" || role == "patrol" || passengerCruiseFleet(fleet)
	automatic := isAutomaticFleet(player, fleet)
	freeMovement := usefulFreePatrolMovement(player, fleet)
	for _, station := range stations {
		if station.Position == position {
			return 0
		}
		distance := position.DistanceTo(station.Position)
		// Cruise feasibility and fuel budgeting ask about the same speed.
		// Reports and endpoints do not change within this station calculation.
		var safety [11]int8
		safeAt := func(speed int) bool {
			if safety[speed] == 0 {
				safety[speed] = -1
				if automaticFleetLegSafe(rules, player, fleet, position, station.Position, speed, allowThreat) {
					safety[speed] = 1
				}
			}
			return safety[speed] == 1
		}
		cruiseSafe := false
		for speed := cruiseWarp; speed <= maxWarp; speed++ {
			if safeAt(speed) {
				cruiseSafe = true
				break
			}
		}
		for speed := 1; speed <= maxWarp; speed++ {
			if limit > 0 && math.Ceil(distance/fleet.StrategicSpeed(speed)) > float64(limit) {
				continue
			}
			if (cruiseRequired || automatic && cruiseSafe) && speed < cruiseWarp && distance > fleet.StrategicSpeed(speed) && !(freeMovement && speed <= fleet.Spec.Engine.FreeSpeed) {
				continue
			}
			if safeAt(speed) {
				if required, _, _, ok := automaticLegFuel(player, loaded, speed, distance, fleet.Spec.FuelCapacity, 0); ok {
					best = min(best, required)
					// Fuel requirements are nonnegative. Once a safe zero-fuel
					// return exists, no other station can improve the minimum.
					if best == 0 {
						return 0
					}
				}
			}
		}
	}
	if best > fleet.Spec.FuelCapacity && !automaticMinesweeper(player, fleet) {
		best = mineDetourReturnFuel(rules, player, loaded, position, stations, allowThreat, limit, best)
	}
	return best
}

// Docks can refuel the convoy; minefield bypass points carry forward actual arrival fuel.
func planAutomaticRoute(rules *Rules, player *Player, fleets []*Fleet, position Vector, target MapObjectTarget, policy automaticRoutePolicy) *expeditionRoute {
	defer scopedRouteThreatIndex(player)()
	defer scopedRouteThreatCache(player)()
	if len(fleets) == 0 {
		return nil
	}
	if target.TargetType == MapObjectTypeFleet {
		target = MapObjectTarget{}
	}
	for _, fleet := range fleets {
		if isAutomaticFleet(player, fleet) && policy.MaxTurns == 0 && !policy.EmergencyReturn {
			policy.MaxTurns = 20
			if colonizationMode(player.Automation.FleetRoles[fleet.Num]) != "" {
				policy.MaxTurns = colonizationMaxYears
			}
		}
		if !policy.EmergencyReturn && !policy.EconomicalPatrol && (player.Automation.FleetRoles[fleet.Num] == "scout" || player.Automation.FleetRoles[fleet.Num] == "patrol" || passengerCruiseFleet(fleet) || colonizationMode(player.Automation.FleetRoles[fleet.Num]) != "") {
			policy.MinWarp = max(policy.MinWarp, automaticCruiseWarp(player, fleet))
		}
	}
	if policy.ReturnTurnsLimit == 0 && len(fleets) == 1 {
		policy.ReturnTurnsLimit = automaticReturnLimit(player, fleets[0], target)
	}
	start := fleets[0].Position
	for _, fleet := range fleets {
		if fleet.Position != start {
			return nil
		}
	}
	if start == position {
		for i, f := range fleets {
			if i < len(policy.DeliveryFuel) && policy.DeliveryFuel[i] > 0 {
				reserve := automaticReturnFuel(rules, player, f, position, expeditionStations(player, f), policy.AllowThreat)
				if reserve > f.Fuel || policy.DeliveryFuel[i] > f.Fuel-reserve {
					return nil
				}
			}
		}
		return &expeditionRoute{Waypoints: []Waypoint{{Position: start, MapObjectTarget: target}}, ArrivalFuel: func() []int {
			fuel := make([]int, len(fleets))
			for i, f := range fleets {
				fuel[i] = f.Fuel
			}
			return fuel
		}()}
	}
	stations := expeditionStations(player, fleets[0])
	if len(stations) == 0 && !policy.OneWay && !policy.RefuelRendezvous {
		return nil
	}
	docks := map[Vector]*Planet{}
	hasGates := false
	for _, station := range stations {
		docks[station.Position] = station
		hasGates = hasGates || automaticGateCompatible(player, fleets, station)
	}
	maxWarp := 10
	for _, fleet := range fleets {
		maxWarp = min(maxWarp, automaticMaxWarp(player, fleet))
		// Keep the same convoy speed restriction for unlike engine boosts.
		if fleet.Spec.Engine.strategicFactor() != fleets[0].Spec.Engine.strategicFactor() {
			maxWarp = min(maxWarp, 8)
		}
	}
	if policy.MaxWarp > 0 {
		maxWarp = min(maxWarp, policy.MaxWarp)
	}
	if maxWarp < 1 && !hasGates {
		return nil
	}
	// Without a compatible gate, no detour can beat the straight-line travel
	// bound. Reject impossible deadlines BEFORE planning a return-fuel route,
	// which can itself search many minefield detours. Preserve boundary cases.
	if !hasGates && policy.MaxTurns > 0 && start.DistanceTo(position) > float64(policy.MaxTurns)*fleets[0].StrategicSpeed(maxWarp)+1e-9 {
		return nil
	}
	// A gate elsewhere in the empire does not make every destination reachable
	// this turn. Even with unrestricted gate links, reaching an entry gate,
	// jumping, and leaving an exit gate costs time. Ignore fuel, mines, gate
	// range and connectivity here: this is an optimistic lower bound only.
	if hasGates && policy.MaxTurns > 0 && automaticGateTurnLowerBound(player, fleets, stations, start, position, maxWarp) > policy.MaxTurns {
		return nil
	}
	reserve := make([]int, len(fleets))
	for i, fleet := range fleets {
		if !policy.RefuelRendezvous && !policy.OneWay {
			reserve[i] = automaticReturnFuel(rules, player, fleet, position, stations, policy.AllowThreat, policy.ReturnTurnsLimit)
		}
		if i < len(policy.DeliveryFuel) {
			reserve[i] += policy.DeliveryFuel[i]
		}
		if reserve[i] > fleet.Spec.FuelCapacity {
			return nil
		}
	}
	sweep := len(fleets) == 1 && automaticMinesweeper(player, fleets[0])
	legSafe := func(from, to Vector, warp int) bool {
		return automaticRouteLegSafe(rules, player, from, to, warp, policy.AllowThreat, civilianTraffic(player, fleets[0]), sweep, fleets[0].StrategicSpeed(warp))
	}
	// With no gate alternative a straight leg achieving the geometric minimum
	// ETA cannot be improved by a dock detour. This keeps dense empires cheap.
	if !hasGates && maxWarp > 0 && (policy.Fast || policy.EmergencyReturn) {
		distance := start.DistanceTo(position)
		minimum := max(1, int(math.Ceil(distance/fleets[0].StrategicSpeed(maxWarp))))
		var direct *expeditionRoute
		for speed := 1; speed <= maxWarp; speed++ {
			if speed < policy.MinWarp && distance > fleets[0].StrategicSpeed(speed) {
				continue
			}
			if int(math.Ceil(distance/fleets[0].StrategicSpeed(speed))) != minimum || !legSafe(start, position, speed) {
				continue
			}
			left := make([]int, len(fleets))
			cost := 0
			valid := true
			for i, f := range fleets {
				_, remaining, burned, ok := automaticLegFuel(player, f, speed, distance, f.Fuel, reserve[i])
				if !ok {
					valid = false
					break
				}
				left[i] = remaining
				cost += burned
			}
			if valid && (direct == nil || policy.HighestWarp && speed > direct.Waypoints[1].WarpSpeed || !policy.HighestWarp && cost < direct.Fuel) {
				direct = &expeditionRoute{Waypoints: []Waypoint{{Position: start}, {Position: position, MapObjectTarget: target, WarpSpeed: speed}}, Turns: minimum, Fuel: cost, ArrivalFuel: left}
			}
		}
		if direct != nil {
			if policy.MaxTurns > 0 && direct.Turns > policy.MaxTurns {
				return nil
			}
			return direct
		}
	}
	positions := []Vector{start, position}
	targets := []MapObjectTarget{{}, target}
	for _, station := range stations {
		// Keep a dock at the starting position as an explicit one-turn refuelling
		// alternative. The first leg still uses the fuel actually on board.
		if station.Position != position {
			positions = append(positions, station.Position)
			targets = append(targets, station.MapObject.ToTarget())
		}
	}
	if !sweep {
		for _, point := range mineDetourPoints(player, start, position) {
			if !slices.Contains(positions, point) {
				positions = append(positions, point)
				targets = append(targets, MapObjectTarget{})
			}
		}
	}
	n := len(positions)
	infinity := int(^uint(0) >> 2)
	times, used, previous, speeds := make([]int, n), make([]int, n), make([]int, n), make([]int, n)
	arrival := make([][]int, n)
	arrival[0] = make([]int, len(fleets))
	for i, f := range fleets {
		arrival[0][i] = f.Fuel
	}
	done := make([]bool, n)
	// Scratch belongs to this search; only accepted destinations retain a copy.
	fuelLeft := make([]int, len(fleets))
	choices := make([]int, 0, maxWarp+1)
	for i := range times {
		times[i] = infinity
		previous[i] = -1
	}
	times[0] = 0
	capacity := 0
	for _, f := range fleets {
		capacity += max(1, f.Spec.FuelCapacity)
	}
	score := func(turns, fuel int) float64 {
		if policy.Fast || policy.EmergencyReturn {
			return float64(turns)
		}
		return float64(turns) + 20*float64(fuel)/float64(max(1, capacity))
	}
	for range n {
		at := -1
		for i := range positions {
			if !done[i] && (at < 0 || score(times[i], used[i]) < score(times[at], used[at]) || score(times[i], used[i]) == score(times[at], used[at]) && times[i] < times[at]) {
				at = i
			}
		}
		if at < 0 || times[at] == infinity || at == 1 {
			break
		}
		done[at] = true
		for next := 1; next < n; next++ {
			if done[next] || next == at {
				continue
			}
			distance := positions[at].DistanceTo(positions[next])
			if distance == 0 && at != 0 {
				continue
			}
			choices = choices[:0]
			for speed := 1; speed <= maxWarp; speed++ {
				if speed < policy.MinWarp && distance > fleets[0].StrategicSpeed(speed) {
					continue
				}
				choices = append(choices, speed)
			}
			canGate := docks[positions[next]] != nil
			for _, f := range fleets {
				canGate = canGate && f.CanJump(player, distance, docks[positions[at]], docks[positions[next]])
			}
			if canGate {
				choices = append(choices, StargateWarpSpeed)
			}
			for _, speed := range choices {
				// Reject an over-budget leg before checking threats or fuel. The
				// same bound used below does not depend on those calculations.
				turns := times[at] + 1
				if speed != StargateWarpSpeed {
					turns = times[at] + max(1, int(math.Ceil(distance/fleets[0].StrategicSpeed(speed))))
				}
				if policy.MaxTurns > 0 && turns > policy.MaxTurns {
					continue
				}
				// Fuel burned by an edge is nonnegative. Even a free edge
				// cannot improve this node if its lower bound is already worse.
				// Keep equality: the existing time/warp tie breakers still apply.
				if score(turns, used[at]) > score(times[next], used[next]) {
					continue
				}
				safe := legSafe(positions[at], positions[next], speed)
				if speed == StargateWarpSpeed {
					safe = automaticLegSafe(rules, player, positions[next], positions[next], 1, policy.AllowThreat)
				}
				if !safe {
					continue
				}
				costTotal := 0
				valid := true
				for i, fleet := range fleets {
					available := fleet.Spec.FuelCapacity
					if at == 0 || docks[positions[at]] == nil {
						available = arrival[at][i]
					}
					endReserve := 0
					if next == 1 {
						endReserve = reserve[i]
					}
					_, left, cost, ok := automaticLegFuel(player, fleet, speed, distance, available, endReserve)
					if !ok {
						valid = false
						break
					}
					fuelLeft[i] = left
					costTotal += cost
				}
				if !valid {
					continue
				}
				if score(turns, used[at]+costTotal) < score(times[next], used[next]) || score(turns, used[at]+costTotal) == score(times[next], used[next]) && (turns < times[next] || policy.HighestWarp && speed > speeds[next]) {
					times[next], used[next], previous[next], speeds[next] = turns, used[at]+costTotal, at, speed
					arrival[next] = append(arrival[next][:0], fuelLeft...)
				}
			}
		}
	}
	if previous[1] < 0 || policy.MaxTurns > 0 && times[1] > policy.MaxTurns {
		return nil
	}
	var reverse []Waypoint
	for at := 1; at != 0; at = previous[at] {
		target := targets[at]
		if speeds[at] == StargateWarpSpeed && docks[positions[at]] != nil {
			target = docks[positions[at]].ToTarget()
		}
		reverse = append(reverse, Waypoint{Position: positions[at], MapObjectTarget: target, WarpSpeed: speeds[at]})
	}
	slices.Reverse(reverse)
	return &expeditionRoute{Waypoints: append([]Waypoint{{Position: start}}, reverse...), Turns: times[1], Fuel: used[1], ArrivalFuel: arrival[1], WaitForFuel: len(reverse) > 0 && reverse[0].Position == start}
}

// A visible gate cannot improve this convoy's route if even its destination
// rejects a hull or the ordinary gate cannot carry its cargo. This deliberately
// overestimates connectivity; every actual edge still uses Fleet.CanJump.
func automaticGateCompatible(player *Player, fleets []*Fleet, station *Planet) bool {
	if !station.Spec.HasStargate || station.Spec.SafeRange <= 0 {
		return false
	}
	for _, f := range fleets {
		if !f.Spec.CanJump && !player.Race.Spec.CanGateCargo && f.CargoMass() > 0 {
			return false
		}
		for _, token := range f.Tokens {
			if token.design != nil && token.design.Spec.Mass > station.Spec.SafeHullMass {
				return false
			}
		}
	}
	return true
}

func holdAutomaticFleet(player *Player, fleet *Fleet, status string) {
	fleet.FleetOrders = FleetOrders{BattlePlanNum: fleet.BattlePlanNum, Purpose: fleet.Purpose, Waypoints: []Waypoint{{Position: fleet.Position}}}
	fleet.WarpSpeed = 0
	fleet.Heading = VectorFloat64{}
	if status != "" {
		fleet.SetTag(fuelStatusTag, status)
	}
	fleet.ComputeFuelUsage(player)
}

func automaticReturnLimit(player *Player, fleet *Fleet, target MapObjectTarget) int {
	// Sweeping requires safe warp within mines. A twelve-turn return deadline
	// must not reject a fuel-feasible outward mission merely because its reserve
	// is conservatively computed at safe warp for the whole trip.
	if automaticMinesweeper(player, fleet) {
		return 0
	}
	if fleet.GetTag(escortStagingTag) != "" {
		return 20
	}
	if player.Automation.FleetRoles[fleet.Num] == "patrol" {
		if fleet.GetTag(frontierPatrolTag) != "" {
			return 20
		}
		return 8
	}
	if player.Automation.FleetRoles[fleet.Num] == "scout" {
		return player.Automation.scoutReturnTurns()
	}
	if isAutomaticFleet(player, fleet) {
		return 12
	}
	return 0
}

func automaticCruiseWarp(player *Player, fleet *Fleet) int {
	if fleet.GetTag(escortStagingTag) != "" || player.Automation.FleetRoles[fleet.Num] == "patrol" || player.Automation.FleetRoles[fleet.Num] == "escort" {
		return min(automaticMaxWarp(player, fleet), 4)
	}
	return min(automaticMaxWarp(player, fleet), 5)
}

// Reserve an empty return only when a known own colony can accept the entire
// committed load. Lost/full destinations continue to require a loaded return.
func deliveryReturnBudget(p *Player, f *Fleet, position Vector) *Fleet {
	job := logisticsWork(f)
	if job == nil {
		return fullCargoBudget(f)
	}
	num := job.DeliveryNum
	if num == 0 {
		num = job.TargetNum
	}
	dest := p.GetPlanetIntel(num)
	if dest == nil || !dest.OwnedBy(p.Num) || dest.Position != position {
		return fullCargoBudget(f)
	}
	if f.Cargo.Colonists > 0 && populationTarget(dest, max(35, colonyPopulationPercent(p, dest)))-dest.Cargo.Colonists < f.Cargo.Colonists {
		return fullCargoBudget(f)
	}
	copy := *f
	copy.Cargo = Cargo{}
	copy.Spec.CargoCapacity = 0
	copy.Tags = make(Tags, len(f.Tags))
	for k, v := range f.Tags {
		copy.Tags[k] = v
	}
	delete(copy.Tags, infrastructureCargoTag)
	return &copy
}

func automaticGateTurnLowerBound(player *Player, fleets []*Fleet, stations []*Planet, from, to Vector, maxWarp int) int {
	infinity := int(^uint(0) >> 2)
	step := fleets[0].StrategicSpeed(maxWarp)
	travel := func(a, b Vector) int {
		if a == b {
			return 0
		}
		if step <= 0 {
			return infinity
		}
		return max(0, int(math.Ceil((a.DistanceTo(b)-1e-9)/step)))
	}
	best := travel(from, to)
	entry, exit := infinity, infinity
	selfJumping := true
	for _, fleet := range fleets {
		selfJumping = selfJumping && fleet.Spec.CanJump
	}
	if selfJumping {
		entry = 0 // onboard jump gates need no departure station
	}
	for _, station := range stations {
		if automaticGateCompatible(player, fleets, station) {
			exit = min(exit, travel(station.Position, to))
		}
		// In a mixed convoy only ordinary ships need the source gate's
		// mass/cargo limits; onboard jump gates use the destination alone.
		canEnter := station.Spec.HasStargate && station.Spec.SafeRange > 0
		for _, fleet := range fleets {
			if !fleet.Spec.CanJump {
				canEnter = canEnter && automaticGateCompatible(player, []*Fleet{fleet}, station)
			}
		}
		if canEnter {
			entry = min(entry, travel(from, station.Position))
		}
	}
	if entry < infinity && exit < infinity {
		best = min(best, entry+1+exit)
	}
	return best
}
