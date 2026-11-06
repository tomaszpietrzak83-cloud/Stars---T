//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"math"
	"slices"
	"testing"
)

// Independent pre-pruning search, including the merged minefield detours.
func referenceRouteRound2(rules *Rules, player *Player, fleets []*Fleet, position Vector, target MapObjectTarget, policy automaticRoutePolicy) *expeditionRoute {
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
	if policy.MaxWarp > 0 {
		maxWarp = min(maxWarp, policy.MaxWarp)
	}
	if maxWarp < 1 && !hasGates {
		return nil
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

func TestRoutePruningPreservesRoutesFuelAndTies(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Escort")
	home := u.Planet("Home")
	near, far := *home, *home
	near.Num, near.Position = 2, Vector{X: 100}
	far.Num, far.Position = 3, Vector{X: 450}
	p.PlanetIntels = []*Planet{home, &near, &far}
	policies := []automaticRoutePolicy{
		{OneWay: true}, {OneWay: true, Fast: true, MaxTurns: 2},
		{OneWay: true, Fast: true, HighestWarp: true, MaxTurns: 5},
		{EmergencyReturn: true, MaxTurns: 5}, {MaxTurns: 2, MinWarp: 4},
		{OneWay: true, MaxWarp: 4, MaxTurns: 1},
	}
	for _, gates := range []bool{false, true} {
		for _, station := range p.PlanetIntels {
			station.Spec.HasStargate = gates
			station.Spec.SafeRange = 500
			station.Spec.SafeHullMass = 10000
		}
		for i := 0; i < 16; i++ {
			p.MinefieldIntels = nil
			if i%2 == 1 {
				p.MinefieldIntels = []*Minefield{{MapObject: MapObject{PlayerNum: 2, Position: Vector{X: 25}}, MinefieldType: MinefieldTypeStandard, NumMines: 100}}
			}
			f.Fuel = f.Spec.FuelCapacity
			if i%4 == 0 {
				f.Fuel /= 3
			}
			f.Spec.FuelGeneration = i % 3
			destination := Vector{X: i*31 + 16, Y: i % 3 * 8}
			if i == 15 {
				destination = far.Position
			}
			for _, policy := range policies {
				want := referenceRouteRound2(&u.Game.Rules, p, []*Fleet{f}, destination, MapObjectTarget{}, policy)
				got := planAutomaticRoute(&u.Game.Rules, p, []*Fleet{f}, destination, MapObjectTarget{}, policy)
				require.Equal(t, want, got, "gates=%v case=%d policy=%+v", gates, i, policy)
			}
		}
	}
}
