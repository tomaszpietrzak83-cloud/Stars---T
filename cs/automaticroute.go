package cs

import (
	"math"
	"slices"
)

const fuelStatusTag = "automatic-fuel-status"
const fuelRequestTag = "automatic-fuel-request"

type automaticRoutePolicy struct {
	Fast             bool // Useful combat pursuit only; ordinary movement weighs fuel against travel time.
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
	step := float64(warp * warp)
	if len(strategicSpeed) > 0 {
		step = strategicSpeed[0]
	}
	if !allowThreat && knownRouteThreat(player, from, to) > 0 && !knownWithdrawalRoute(player, from, to) {
		return false
	}
	for _, field := range player.MinefieldIntels {
		if field == nil || segmentDistance(field.Position, from, to) > field.Radius()+1 {
			continue
		}
		if field.Detonate {
			// Permit escape from a detonating field only if this very turn ends outside it.
			distance := from.DistanceTo(to)
			end := to.ToFloat64()
			if distance > step {
				end = from.ToFloat64().Add(to.Subtract(from).Normalized().Scale(step))
			}
			if from.DistanceTo(field.Position) > field.Radius() || end.DistanceTo(field.Position.ToFloat64()) <= field.Radius()+2 || to.DistanceTo(field.Position) <= field.Radius()+2 {
				return false
			}
		} else if field.PlayerNum != player.Num {
			safe := rules.MinefieldStatsByType[field.MinefieldType].MaxSpeed + player.Race.Spec.MinefieldSafeWarpBonus
			if safe == 0 {
				safe = 3
			}
			if warp > safe {
				return false
			}
		}
	}
	return true
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
	best := int(^uint(0) >> 2)
	loaded := deliveryReturnBudget(player, fleet, position)
	limit := 0
	if len(returnTurns) > 0 {
		limit = returnTurns[0]
	}
	if automatic := automaticReturnLimit(player, fleet, MapObjectTarget{}); automatic > 0 && (limit == 0 || limit > automatic) {
		limit = automatic
	}
	for _, station := range stations {
		if station.Position == position {
			return 0
		}
		cruiseSafe := false
		for speed := automaticCruiseWarp(player, fleet); speed <= automaticMaxWarp(player, fleet); speed++ {
			if automaticLegSafe(rules, player, position, station.Position, speed, allowThreat, fleet.StrategicSpeed(speed)) {
				cruiseSafe = true
				break
			}
		}
		for speed := 1; speed <= automaticMaxWarp(player, fleet); speed++ {
			if limit > 0 && math.Ceil(position.DistanceTo(station.Position)/fleet.StrategicSpeed(speed)) > float64(limit) {
				continue
			}
			if (player.Automation.FleetRoles[fleet.Num] == "scout" || player.Automation.FleetRoles[fleet.Num] == "patrol" || passengerCruiseFleet(fleet) || isAutomaticFleet(player, fleet) && cruiseSafe) && speed < automaticCruiseWarp(player, fleet) && position.DistanceTo(station.Position) > fleet.StrategicSpeed(speed) && !(usefulFreePatrolMovement(player, fleet) && speed <= fleet.Spec.Engine.FreeSpeed) {
				continue
			}
			if automaticLegSafe(rules, player, position, station.Position, speed, allowThreat, fleet.StrategicSpeed(speed)) {
				if required, _, _, ok := automaticLegFuel(player, loaded, speed, position.DistanceTo(station.Position), fleet.Spec.FuelCapacity, 0); ok {
					best = min(best, required)
				}
			}
		}
	}
	return best
}

// All intermediate nodes are actual known fuel docks. Separate budgets and
// return reserves for every member prevent a convoy from spending its escort's fuel.
func planAutomaticRoute(rules *Rules, player *Player, fleets []*Fleet, position Vector, target MapObjectTarget, policy automaticRoutePolicy) *expeditionRoute {
	defer scopedRouteThreatIndex(player)()
	if len(fleets) == 0 {
		return nil
	}
	if target.TargetType == MapObjectTypeFleet {
		target = MapObjectTarget{}
	}
	for _, fleet := range fleets {
		if isAutomaticFleet(player, fleet) && policy.MaxTurns == 0 && !policy.EmergencyReturn {
			policy.MaxTurns = 20
		}
		if !policy.EmergencyReturn && !policy.EconomicalPatrol && (player.Automation.FleetRoles[fleet.Num] == "scout" || player.Automation.FleetRoles[fleet.Num] == "patrol" || passengerCruiseFleet(fleet) || colonizationMode(player.Automation.FleetRoles[fleet.Num]) != "") {
			policy.MinWarp = max(policy.MinWarp, automaticCruiseWarp(player, fleet))
			if colonizationMode(player.Automation.FleetRoles[fleet.Num]) != "" {
				policy.MaxTurns = 50
			}
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
	reserve := make([]int, len(fleets))
	for i, fleet := range fleets {
		maxWarp = min(maxWarp, automaticMaxWarp(player, fleet))
		// Separate convoy members receive identical waypoints. Unlike ships in
		// one fleet, they do not share an engine minimum; use a common ordinary
		// speed when their boosts differ so escorts never leave cargo behind.
		if fleet.Spec.Engine.strategicFactor() != fleets[0].Spec.Engine.strategicFactor() {
			maxWarp = min(maxWarp, 8)
		}
		reserve[i] = automaticReturnFuel(rules, player, fleet, position, stations, policy.AllowThreat, policy.ReturnTurnsLimit)
		if policy.RefuelRendezvous || policy.OneWay {
			reserve[i] = 0
		}
		if i < len(policy.DeliveryFuel) {
			reserve[i] += policy.DeliveryFuel[i]
		}
		if reserve[i] > fleet.Spec.FuelCapacity {
			return nil
		}
	}
	if policy.MaxWarp > 0 {
		maxWarp = min(maxWarp, policy.MaxWarp)
	}
	if maxWarp < 1 && !hasGates {
		return nil
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
			if int(math.Ceil(distance/fleets[0].StrategicSpeed(speed))) != minimum || !automaticLegSafe(rules, player, start, position, speed, policy.AllowThreat, fleets[0].StrategicSpeed(speed)) {
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
			if valid && (direct == nil || cost < direct.Fuel) {
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
	n := len(positions)
	infinity := int(^uint(0) >> 2)
	times, used, previous, speeds := make([]int, n), make([]int, n), make([]int, n), make([]int, n)
	arrival := make([][]int, n)
	arrival[0] = make([]int, len(fleets))
	for i, f := range fleets {
		arrival[0][i] = f.Fuel
	}
	done := make([]bool, n)
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
			choices := make([]int, 0, maxWarp+1)
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
				safe := automaticLegSafe(rules, player, positions[at], positions[next], speed, policy.AllowThreat, fleets[0].StrategicSpeed(speed))
				if speed == StargateWarpSpeed {
					safe = automaticLegSafe(rules, player, positions[next], positions[next], 1, policy.AllowThreat)
				}
				if !safe {
					continue
				}
				fuelLeft := make([]int, len(fleets))
				costTotal := 0
				valid := true
				for i, fleet := range fleets {
					available := fleet.Spec.FuelCapacity
					if at == 0 {
						available = fleet.Fuel
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
				turns := times[at] + 1
				if speed != StargateWarpSpeed {
					turns = times[at] + max(1, int(math.Ceil(distance/fleets[0].StrategicSpeed(speed))))
				}
				if policy.MaxTurns > 0 && turns > policy.MaxTurns {
					continue
				}
				if score(turns, used[at]+costTotal) < score(times[next], used[next]) || score(turns, used[at]+costTotal) == score(times[next], used[next]) && turns < times[next] {
					times[next], used[next], previous[next], speeds[next] = turns, used[at]+costTotal, at, speed
					arrival[next] = fuelLeft
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
	if f.Cargo.Colonists > 0 && populationTarget(dest, max(35, p.Automation.destinationPopulationPercent()))-dest.Cargo.Colonists < f.Cargo.Colonists {
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
