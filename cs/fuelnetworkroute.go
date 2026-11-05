package cs

import (
	"container/heap"
	"math"
	"slices"
)

type fuelNetworkDonation struct {
	FleetNum int `json:"fleetNum"`
	Amount   int `json:"amount"`
}
type fuelNetworkLeg struct {
	From       Vector                `json:"from"`
	To         Vector                `json:"to"`
	Target     MapObjectTarget       `json:"target"`
	Warp       int                   `json:"warp"`
	RefuelDock bool                  `json:"refuelDock,omitempty"`
	DockWait   bool                  `json:"dockWait,omitempty"`
	Donations  []fuelNetworkDonation `json:"donations,omitempty"`
	Required   []int                 `json:"required"`
	Reserve    []int                 `json:"reserve"`
}
type fuelNetworkRoute struct {
	ArrivalFuel []int            `json:"arrivalFuel"`
	Legs        []fuelNetworkLeg `json:"legs"`
	Turns       int              `json:"turns"`
	Burned      int              `json:"burned"`
}
type fuelNetworkNode struct {
	position Vector
	target   MapObjectTarget
	dock     *Planet
	donors   []*Fleet
	stocks   []int
}
type fuelNetworkLabel struct {
	node, turns, burned int
	fuel                []int
	visited             []int
	legs                []fuelNetworkLeg
	discarded           bool
}
type fuelNetworkQueue []*fuelNetworkLabel

func (q fuelNetworkQueue) Len() int { return len(q) }
func (q fuelNetworkQueue) Less(i, j int) bool {
	if q[i].turns != q[j].turns {
		return q[i].turns < q[j].turns
	}
	return q[i].burned < q[j].burned
}
func (q fuelNetworkQueue) Swap(i, j int) { q[i], q[j] = q[j], q[i] }
func (q *fuelNetworkQueue) Push(x any)   { *q = append(*q, x.(*fuelNetworkLabel)) }
func (q *fuelNetworkQueue) Pop() any {
	old := *q
	item := old[len(old)-1]
	*q = old[:len(old)-1]
	return item
}
func fuelLabelDominates(a, b *fuelNetworkLabel) bool {
	if a.turns > b.turns || a.burned > b.burned {
		return false
	}
	for i, fuel := range a.fuel {
		if fuel < b.fuel[i] {
			return false
		}
	}
	for _, node := range a.visited {
		if !slices.Contains(b.visited, node) {
			return false
		}
	}
	return true
}

// A mobile node has finite, currently available fuel after its own cruise return
// reserve. No future generation or uncommitted moving tanker is a fuel source.
// Pareto labels retain slower arrivals with more fuel; one label per node is
// insufficient when a tanker cannot refill every convoy member completely.
func planFuelNetworkRoute(rules *Rules, player *Player, fleets, donors []*Fleet, destination Vector, target MapObjectTarget, policy automaticRoutePolicy, anyDock ...bool) *fuelNetworkRoute {
	if len(fleets) == 0 {
		return nil
	}
	endAtDock := len(anyDock) > 0 && anyDock[0]
	start := fleets[0].Position
	budgets := make([]*Fleet, len(fleets))
	initial := make([]int, len(fleets))
	reserve := make([]int, len(fleets))
	maxWarp, minWarp := 10, 5
	stations := expeditionStations(player, fleets[0])
	for i, fleet := range fleets {
		if fleet.Delete || fleet.Position != start {
			return nil
		}
		budgets[i] = fullCargoBudget(fleet)
		initial[i] = fleet.Fuel
		maxWarp = min(maxWarp, automaticMaxWarp(player, fleet))
		if fleet.Spec.Engine.strategicFactor() != fleets[0].Spec.Engine.strategicFactor() {
			maxWarp = min(maxWarp, 8)
		}
		if !policy.OneWay && !policy.RefuelRendezvous && !endAtDock {
			reserve[i] = automaticReturnFuel(rules, player, budgets[i], destination, stations, policy.AllowThreat, automaticReturnLimit(player, fleet, target))
		}
		if reserve[i] > fleet.Spec.FuelCapacity {
			return nil
		}
	}
	if policy.MaxWarp > 0 {
		maxWarp = min(maxWarp, policy.MaxWarp)
	}
	minWarp = max(policy.MinWarp, min(minWarp, maxWarp))
	maxTurns := policy.MaxTurns
	if maxTurns == 0 {
		maxTurns = 50
	}
	nodes := []fuelNetworkNode{{position: start}, {position: destination, target: target}}
	byPosition := map[Vector]int{}
	for _, dock := range stations {
		usable := true
		for _, fleet := range fleets {
			usable = usable && fleet.CanFuel(player, dock)
		}
		if !usable {
			continue
		}
		if dock.Position == destination {
			nodes[1].dock = dock
			if nodes[1].target.TargetType == MapObjectTypeNone {
				nodes[1].target = dock.MapObject.ToTarget()
			}
			byPosition[dock.Position] = 1
			continue
		}
		if _, exists := byPosition[dock.Position]; exists {
			continue
		}
		byPosition[dock.Position] = len(nodes)
		nodes = append(nodes, fuelNetworkNode{position: dock.Position, target: dock.MapObject.ToTarget(), dock: dock})
	}
	for _, donor := range donors {
		if donor.Delete || donor.InAutomaticSafety() || donor.InDesignMaintenance() || slices.Contains(fleets, donor) {
			continue
		}
		stock := max(0, donor.Fuel-automaticReturnFuel(rules, player, fullCargoBudget(donor), donor.Position, expeditionStations(player, donor), false))
		if stock == 0 {
			continue
		}
		at, exists := byPosition[donor.Position]
		if exists && nodes[at].dock != nil {
			continue
		}
		if !exists {
			at = len(nodes)
			byPosition[donor.Position] = at
			nodes = append(nodes, fuelNetworkNode{position: donor.Position})
		}
		nodes[at].donors = append(nodes[at].donors, donor)
		nodes[at].stocks = append(nodes[at].stocks, stock)
	}
	first := &fuelNetworkLabel{node: 0, fuel: initial, visited: []int{0}}
	labels := make([][]*fuelNetworkLabel, len(nodes))
	labels[0] = []*fuelNetworkLabel{first}
	queue := &fuelNetworkQueue{first}
	heap.Init(queue)
	for expanded := 0; queue.Len() > 0 && expanded < 20000; expanded++ {
		current := heap.Pop(queue).(*fuelNetworkLabel)
		if current.discarded {
			continue
		}
		if !endAtDock && current.node == 1 || endAtDock && nodes[current.node].dock != nil {
			return &fuelNetworkRoute{Legs: current.legs, Turns: current.turns, Burned: current.burned, ArrivalFuel: slices.Clone(current.fuel)}
		}
		from := nodes[current.node]
		for next, to := range nodes {
			if endAtDock && next == 1 && to.dock == nil || next == 0 || next == current.node || slices.Contains(current.visited, next) {
				continue
			}
			distance := from.position.DistanceTo(to.position)
			for speed := 1; speed <= maxWarp+1; speed++ {
				warp := speed
				if speed == maxWarp+1 {
					warp = StargateWarpSpeed
					canGate := from.dock != nil && to.dock != nil
					for _, fleet := range budgets {
						canGate = canGate && fleet.CanJump(player, distance, from.dock, to.dock)
					}
					if !canGate {
						continue
					}
				} else if speed < minWarp && distance > fleets[0].StrategicSpeed(speed) {
					continue
				}
				if warp == StargateWarpSpeed {
					if !automaticLegSafe(rules, player, to.position, to.position, 1, policy.AllowThreat) {
						continue
					}
				} else if !automaticFleetLegSafe(rules, player, fleets[0], from.position, to.position, warp, policy.AllowThreat) {
					continue
				}
				turns := 1
				if distance == 0 && to.dock == nil {
					turns = 0
				} else if warp != StargateWarpSpeed {
					turns = max(1, int(math.Ceil(distance/fleets[0].StrategicSpeed(warp))))
				}
				if current.turns+turns > maxTurns {
					continue
				}
				leg := fuelNetworkLeg{From: from.position, To: to.position, Target: to.target, Warp: warp, RefuelDock: to.dock != nil, DockWait: distance == 0 && to.dock != nil, Required: make([]int, len(fleets)), Reserve: make([]int, len(fleets))}
				fuel := make([]int, len(fleets))
				missing, cost := 0, 0
				valid := true
				for i, fleet := range budgets {
					if next == 1 {
						leg.Reserve[i] = reserve[i]
					}
					required, _, _, ok := automaticLegFuel(player, fleet, warp, distance, fleet.Spec.FuelCapacity, leg.Reserve[i])
					if !ok {
						valid = false
						break
					}
					leg.Required[i] = required
					available := current.fuel[i]
					if from.dock != nil {
						available = fleet.Spec.FuelCapacity
					}
					if required > available {
						if len(from.donors) == 0 {
							valid = false
							break
						}
						missing += required - available
						available = required
					}
					_, left, burned, ok := automaticLegFuel(player, fleet, warp, distance, available, leg.Reserve[i])
					if !ok {
						valid = false
						break
					}
					fuel[i] = left
					cost += burned
				}
				if !valid {
					continue
				}
				for i, donor := range from.donors {
					amount := min(missing, from.stocks[i])
					if amount > 0 {
						leg.Donations = append(leg.Donations, fuelNetworkDonation{donor.Num, amount})
						missing -= amount
					}
				}
				if missing > 0 {
					continue
				}
				candidate := &fuelNetworkLabel{node: next, turns: current.turns + turns, burned: current.burned + cost, fuel: fuel, visited: append(slices.Clone(current.visited), next), legs: append(slices.Clone(current.legs), leg)}
				dominated := false
				for _, old := range labels[next] {
					if !old.discarded && fuelLabelDominates(old, candidate) {
						dominated = true
						break
					}
				}
				if dominated {
					continue
				}
				for _, old := range labels[next] {
					if !old.discarded && fuelLabelDominates(candidate, old) {
						old.discarded = true
					}
				}
				labels[next] = append(labels[next], candidate)
				heap.Push(queue, candidate)
			}
		}
	}
	return nil
}
