package cs

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"
)

const tankerJobTag = "automatic-tanker"
const tankerAwaitTag = "awaiting-tanker"
const fuelResumeTag = "automatic-fuel-resume"
const fuelAwaitResumeTag = "automatic-tanker-resume"

type automaticFuelRequest struct {
	OneWay        bool            `json:"oneWay,omitempty"`
	Destination   Vector          `json:"destination"`
	Target        MapObjectTarget `json:"target"`
	Members       []int           `json:"members"`
	Urgent        bool            `json:"urgent"`
	BaselineTurns int             `json:"baselineTurns,omitempty"`
	AllowThreat   bool            `json:"allowThreat,omitempty"`
}
type automaticTankerJob struct {
	Stage        string              `json:"stage,omitempty"`
	Anchor       Vector              `json:"anchor"`
	PatrolIndex  int                 `json:"patrolIndex,omitempty"`
	PatrolYear   int                 `json:"patrolYear,omitempty"`
	TargetNum    int                 `json:"targetNum,omitempty"`
	Members      []int               `json:"members,omitempty"`
	Position     Vector              `json:"position"`
	DeliveryFuel int                 `json:"deliveryFuel,omitempty"`
	AllowThreat  bool                `json:"allowThreat,omitempty"`
	ResumeOrders map[int]FleetOrders `json:"resumeOrders,omitempty"`
	Urgent       bool                `json:"urgent,omitempty"`
	Status       string              `json:"status"`
}

func readTankerJob(fleet *Fleet) *automaticTankerJob {
	var job automaticTankerJob
	if json.Unmarshal([]byte(fleet.GetTag(tankerJobTag)), &job) != nil {
		return nil
	}
	return &job
}
func writeTankerJob(fleet *Fleet, job *automaticTankerJob) {
	data, _ := json.Marshal(job)
	fleet.SetTag(tankerJobTag, string(data))
}
func saveAutomaticFuelRequest(fleet *Fleet, request automaticFuelRequest) {
	data, _ := json.Marshal(request)
	fleet.SetTag(fuelRequestTag, string(data))
}
func readAutomaticFuelRequest(fleet *Fleet) *automaticFuelRequest {
	var value automaticFuelRequest
	if json.Unmarshal([]byte(fleet.GetTag(fuelRequestTag)), &value) != nil {
		return nil
	}
	return &value
}
func fleetNumbers(fleets []*Fleet) []int {
	result := make([]int, len(fleets))
	for i, f := range fleets {
		result[i] = f.Num
	}
	return result
}
func isAutomaticFleet(player *Player, fleet *Fleet) bool {
	return !fleet.Delete && !fleet.Starbase && (fleet.InAutomaticSafety() || player.Automation.FleetRoles[fleet.Num] != "" || player.Automation.RebuildFleets[fleet.Num] || fleet.InDesignMaintenance() || player.FleetInActiveAssembly(fleet))
}
func assemblyLeaderCanAdvance(player *Player, fleet *Fleet) bool {
	if !fleet.Spec.HasWeapons || fleet.GetTag(assemblyRalliedTag) == "" {
		return false
	}
	id, _ := strconv.Atoi(fleet.GetTag(assemblyTag))
	plan, ok := player.Automation.FleetAssemblies[id]
	return ok && plan.LeaderFleetNum == fleet.Num
}
func requestAutomaticRescue(player *Player, fleet *Fleet, force ...bool) {
	// Even a full tank can be stranded outside the direct dock range. A forced
	// request may find intermediate mobile fuel stops rather than a top-up here.
	if fleet.Spec.FuelCapacity > 0 && (fleet.Fuel*4 < fleet.Spec.FuelCapacity || len(force) > 0 && force[0]) {
		saveAutomaticFuelRequest(fleet, automaticFuelRequest{Destination: fleet.Position, Members: []int{fleet.Num}, Urgent: true})
	}
}

// Own station and minefield orders are current knowledge, not last turn's scan.
// Only owned docks are used: another player's friendly stance toward us cannot
// be inferred from our own diplomacy, and actually controls their fuel service.
func (t *turnGenerator) refreshAutomaticFuelIntel() {
	for _, player := range t.automaticPlayers() {
		if len(player.Automation.FleetRoles) == 0 && len(player.Automation.FleetAssemblies) == 0 && len(player.DesignMaintenance) == 0 {
			continue
		}
		t.releaseUnassistedCruiseNetworks(player)
		known := map[int]bool{}
		for i, intel := range player.PlanetIntels {
			if intel == nil {
				continue
			}
			known[intel.Num] = true
			if intel.PlayerNum == player.Num {
				if actual := t.game.getPlanet(intel.Num); actual != nil && actual.OwnedBy(player.Num) {
					copy := *actual
					player.PlanetIntels[i] = &copy
				} else {
					// Remove a lost service without revealing its new owner's data.
					copy := *intel
					copy.Spec.DockCapacity = 0
					copy.Spec.HasStarbase = false
					player.PlanetIntels[i] = &copy
				}
			}
		}
		for _, planet := range t.game.getPlanets(player.Num) {
			if !known[planet.Num] {
				copy := *planet
				player.PlanetIntels = append(player.PlanetIntels, &copy)
			}
		}
		fields := slices.DeleteFunc(slices.Clone(player.MinefieldIntels), func(field *Minefield) bool { return field != nil && field.PlayerNum == player.Num })
		for _, field := range t.game.Minefields {
			if field.PlayerNum == player.Num && !field.Delete {
				copy := *field
				fields = append(fields, &copy)
			}
		}
		player.MinefieldIntels = fields
		for _, fleet := range t.game.getFleets(player.Num) {
			delete(fleet.Tags, fuelRequestTag)
			delete(fleet.Tags, fuelStatusTag)
			if raw := fleet.GetTag(fuelResumeTag); raw != "" {
				var orders FleetOrders
				if json.Unmarshal([]byte(raw), &orders) == nil && len(orders.Waypoints) > 0 && len(fleet.Waypoints) <= 1 {
					orders.Waypoints[0] = Waypoint{Position: fleet.Position}
					fleet.FleetOrders = orders
				}
				delete(fleet.Tags, fuelResumeTag)
			}
		}
	}
}

func convoyMembers(player *Player, fleet *Fleet, all []*Fleet) []*Fleet {
	result := []*Fleet{fleet}
	if player.Automation.FleetRoles[fleet.Num] == "freighter" || player.Automation.FleetRoles[fleet.Num] == "bomber" || player.Automation.FleetRoles[fleet.Num] == "tanker" {
		for _, escort := range all {
			if !escort.Delete && escort.Position == fleet.Position && escort.GetTag(escortTag) == strconv.Itoa(fleet.Num) && player.Automation.FleetRoles[escort.Num] == "escort" {
				result = append(result, escort)
			}
		}
	}
	return result
}

// Audit every controlled fleet, including an assembly leader's user-issued
// orders and the manual mission an assigned transport is finishing first.
func (t *turnGenerator) guardAutomaticFuel() {
	for _, player := range t.automaticPlayers() {
		for _, fleet := range t.game.getFleets(player.Num) {
			if (readTankerJob(fleet) != nil && readTankerJob(fleet).TargetNum > 0) || (fundedEmergencyReturn(&t.game.Rules, player, fleet) || fundedScoutWaypoints(&t.game.Rules, player, fleet) || fundedLogisticsWaypoints(&t.game.Rules, player, fleet)) || fuelNetworkManaged(fleet) || !isAutomaticFleet(player, fleet) || player.Automation.FleetRoles[fleet.Num] == "tanker" || fleet.GetTag(escortTag) != "" || fleet.GetTag(tankerAwaitTag) != "" {
				continue
			}
			group := convoyMembers(player, fleet, t.game.getFleets(player.Num))
			if len(fleet.Waypoints) < 2 {
				if len(fleet.Waypoints) > 0 && colonizationOneWay(player, fleet, fleet.Waypoints[0]) {
					continue
				}
				if readAutomaticFuelRequest(fleet) == nil {
					requestAutomaticRescue(player, fleet)
				}
				continue
			}
			goal := fleet.Waypoints[1]
			allowThreat := combatAutonomy(player, fleet) || scoutReconActive(player, fleet) || len(group) > 1 || assemblyLeaderCanAdvance(player, fleet) || patrolCanAttack(player, fleet)
			if job := logisticsWork(fleet); job != nil {
				goal.Position = job.Position
				allowThreat = knownRouteThreat(player, fleet.Position, job.Position) > 0 && len(group) > 1
			}
			target := goal.MapObjectTarget
			if target.TargetType == MapObjectTypeFleet {
				target = MapObjectTarget{}
			}
			policy := automaticRoutePolicy{AllowThreat: allowThreat, OneWay: colonizationOneWay(player, fleet, goal)}
			if fleet.GetTag(frontierPatrolTag) != "" || fleet.GetTag(escortStagingTag) != "" {
				policy.EconomicalPatrol = true
				policy.MaxWarp = 4
				policy.MinWarp = 4
				policy.MaxTurns = 20
			}
			// Include cargo that a waypoint-zero loading order may add before movement.
			budgets := make([]*Fleet, len(group))
			for i, f := range group {
				budgets[i] = f
				if len(f.Waypoints) > 0 && f.Waypoints[0].Task == WaypointTaskTransport {
					budgets[i] = fullCargoBudget(f)
				}
			}
			route := planAutomaticRoute(&t.game.Rules, player, budgets, goal.Position, target, policy)
			if route == nil {
				if len(group) == 1 && t.routeEmergencyReturn(player, fleet) {
					continue
				}
				for _, member := range group {
					raw, _ := json.Marshal(member.FleetOrders)
					member.SetTag(fuelResumeTag, string(raw))
					wp0 := member.Waypoints[0]
					holdAutomaticFleet(player, member, "Waiting for fuel or a safe route")
					member.Waypoints[0] = wp0
				}
				saveAutomaticFuelRequest(fleet, automaticFuelRequest{Destination: goal.Position, Target: target, Members: fleetNumbers(group), Urgent: true, AllowThreat: allowThreat, OneWay: policy.OneWay})
				continue
			}
			// Shared automatic routes already contain their final mission destination.
			// Preserve waypoint tasks when auditing a pre-existing manual mission.
			if job := logisticsWork(fleet); job == nil && len(group) == 1 {
				original := fleet.FleetOrders
				original.Waypoints = slices.Clone(fleet.Waypoints)
				applyExpeditionRoute(player, fleet, route)
				if route.WaitForFuel {
					raw, _ := json.Marshal(original)
					fleet.SetTag(fuelResumeTag, string(raw))
					fleet.Waypoints[0] = original.Waypoints[0]
				} else {
					fleet.Waypoints[0] = original.Waypoints[0]
					last := len(fleet.Waypoints) - 1
					if goal.TargetType == MapObjectTypeFleet && goal.Task != WaypointTaskNone {
						fleet.Waypoints = append(fleet.Waypoints, goal)
					} else {
						if scoutReconActive(player, fleet) && goal.TargetType == MapObjectTypeFleet {
							fleet.Waypoints[last].MapObjectTarget = goal.MapObjectTarget
						}
						fleet.Waypoints[last].Task = goal.Task
						fleet.Waypoints[last].TransportTasks = goal.TransportTasks
						fleet.Waypoints[last].WaitAtWaypoint = goal.WaitAtWaypoint
					}
					fleet.Waypoints = append(fleet.Waypoints, original.Waypoints[2:]...)
				}
			} else {
				for _, member := range group {
					applyExpeditionRoute(player, member, route)
				}
			}
			// A tanker is offered only if it can make a real ETA improvement; routine
			// valid economical routes are never stopped merely to top up their tanks.
			full := make([]*Fleet, len(budgets))
			for i, f := range budgets {
				copy := *f
				copy.Fuel = f.Spec.FuelCapacity
				full[i] = &copy
			}
			fast := planAutomaticRoute(&t.game.Rules, player, full, goal.Position, target, policy)
			minimumTurns := max(1, int(math.Ceil(fleet.Position.DistanceTo(goal.Position)/max(1, fleet.StrategicSpeed(automaticMaxWarp(player, fleet))))))
			if !(fleet.GetTag(frontierPatrolTag) != "" || fleet.GetTag(escortStagingTag) != "") && (fast != nil && fast.Turns+1 < route.Turns || route.Turns > minimumTurns) {
				saveAutomaticFuelRequest(fleet, automaticFuelRequest{Destination: goal.Position, Target: target, Members: fleetNumbers(group), BaselineTurns: route.Turns, AllowThreat: allowThreat, OneWay: policy.OneWay})
			}
		}
	}
}

func (t *turnGenerator) tankerMembers(player *Player, job *automaticTankerJob) []*Fleet {
	var result []*Fleet
	for _, num := range job.Members {
		fleet := t.game.getFleet(player.Num, num)
		if fleet != nil && !fleet.Delete {
			result = append(result, fleet)
		}
	}
	return result
}
func (t *turnGenerator) releaseTankerCustomers(player *Player, tanker *Fleet, job *automaticTankerJob) {
	for _, fleet := range t.tankerMembers(player, job) {
		if fleet.GetTag(tankerAwaitTag) != strconv.Itoa(tanker.Num) {
			continue
		}
		delete(fleet.Tags, tankerAwaitTag)
		delete(fleet.Tags, fuelAwaitResumeTag)
		delete(fleet.Tags, fuelRequestTag)
		delete(fleet.Tags, fuelStatusTag)
		if orders, ok := job.ResumeOrders[fleet.Num]; ok && len(orders.Waypoints) > 0 && (len(fleet.Waypoints) <= 1 || fleet.Waypoints[len(fleet.Waypoints)-1].Position == job.Position) {
			orders.Waypoints = slices.Clone(orders.Waypoints)
			orders.Waypoints[0] = Waypoint{Position: fleet.Position}
			fleet.FleetOrders = orders
			fleet.ComputeFuelUsage(player)
		}
	}
}

func (t *turnGenerator) routeTankerJob(player *Player, tanker *Fleet, job *automaticTankerJob) bool {
	members := t.tankerMembers(player, job)
	if len(members) == 0 {
		return false
	}
	needsFuel := false
	for _, member := range members {
		needsFuel = needsFuel || member.Fuel < member.Spec.FuelCapacity || member.Position != job.Position
	}
	if !needsFuel {
		return false
	}
	for _, member := range members {
		if member.Position != members[0].Position {
			return false
		}
	}
	receiverRoute := planAutomaticRoute(&t.game.Rules, player, members, job.Position, MapObjectTarget{}, automaticRoutePolicy{AllowThreat: job.AllowThreat || job.Urgent, RefuelRendezvous: true, EmergencyReturn: job.Urgent, MaxTurns: 10000})
	if receiverRoute == nil {
		return false
	}
	for _, member := range members {
		if !isAutomaticFleet(player, member) || member.GetTag(tankerAwaitTag) != strconv.Itoa(tanker.Num) {
			return false
		}
	}
	route := planAutomaticRoute(&t.game.Rules, player, []*Fleet{tanker}, job.Position, MapObjectTarget{}, automaticRoutePolicy{AllowThreat: combatAutonomy(player, tanker), DeliveryFuel: []int{job.DeliveryFuel}})
	if route == nil {
		return false // Release the rendezvous; both ships can withdraw instead of freezing.
	}
	applyExpeditionRoute(player, tanker, route)
	for _, member := range members {
		if member.Position == job.Position {
			holdAutomaticFleet(player, member, "Waiting at rendezvous for tanker")
			continue
		}
		applyExpeditionRoute(player, member, receiverRoute)
		member.SetTag(fuelStatusTag, "Travelling to agreed fuel rendezvous")
	}
	job.Status = fmt.Sprintf("Rendezvous with fleet #%d at %s", job.TargetNum, job.Position.String())
	return true
}

func (t *turnGenerator) planAutomaticTankers() {
	for _, player := range t.automaticPlayers() {
		fleets := slices.Clone(t.game.getFleets(player.Num))
		slices.SortFunc(fleets, func(a, b *Fleet) int { return a.Num - b.Num })
		for _, tanker := range fleets {
			if job := readTankerJob(tanker); job != nil && (tanker.Delete || !automaticFuelDonor(player, tanker) || tanker.InDesignMaintenance()) {
				t.releaseTankerCustomers(player, tanker, job)
				delete(tanker.Tags, tankerJobTag)
			}
		}
		// Deleted tankers disappear from the universe map. Release orphaned waits,
		// including a fleet number reused later by an unrelated ship.
		for _, customer := range fleets {
			if raw := customer.GetTag(tankerAwaitTag); raw != "" {
				num, _ := strconv.Atoi(raw)
				donor := t.game.getFleet(player.Num, num)
				valid := false
				if donor != nil && !donor.Delete && automaticFuelDonor(player, donor) {
					if job := readTankerJob(donor); job != nil {
						valid = slices.Contains(job.Members, customer.Num)
					}
				}
				if !valid {
					if backup := customer.GetTag(fuelAwaitResumeTag); backup != "" && len(customer.Waypoints) <= 1 {
						var orders FleetOrders
						if json.Unmarshal([]byte(backup), &orders) == nil && len(orders.Waypoints) > 0 {
							orders.Waypoints[0] = Waypoint{Position: customer.Position}
							customer.FleetOrders = orders
						}
					}
					delete(customer.Tags, tankerAwaitTag)
					delete(customer.Tags, fuelAwaitResumeTag)
					requestAutomaticRescue(player, customer)
				}
			}
		}
		var requests []*Fleet
		for _, fleet := range fleets {
			if !fuelNetworkManaged(fleet) && isAutomaticFleet(player, fleet) && player.Automation.FleetRoles[fleet.Num] != "tanker" && fleet.GetTag(tankerAwaitTag) == "" && readAutomaticFuelRequest(fleet) != nil {
				requests = append(requests, fleet)
			}
		}
		slices.SortStableFunc(requests, func(a, b *Fleet) int {
			ra, rb := readAutomaticFuelRequest(a), readAutomaticFuelRequest(b)
			if ra.Urgent != rb.Urgent {
				if ra.Urgent {
					return -1
				}
				return 1
			}
			return a.Fuel*max(1, b.Spec.FuelCapacity) - b.Fuel*max(1, a.Spec.FuelCapacity)
		})
		for _, tanker := range fleets {
			if tanker.GetTag(fuelNetworkDonorTag) != "" || fuelNetworkManaged(tanker) || tanker.InAutomaticSafety() || tanker.Delete || tanker.Starbase || tanker.InDesignMaintenance() || !automaticFuelDonor(player, tanker) {
				continue
			}
			if player.Automation.FleetRoles[tanker.Num] == "tanker" && t.holdTankerForRepairs(player, tanker) {
				continue
			}
			if player.Automation.FleetRoles[tanker.Num] == "patrol" && readTankerJob(tanker) == nil {
				patrol := readPatrolJob(tanker)
				if patrol.Leader != 0 {
					enemy := patrolEnemy(player, patrol)
					if patrol.Grouped || patrol.Leader != tanker.Num || enemy != nil && tanker.Position.DistanceTo(enemy.Position) <= 30 {
						continue
					}
				}
			}
			if job := readTankerJob(tanker); job != nil {
				if job.TargetNum > 0 && t.routeTankerJob(player, tanker, job) {
					writeTankerJob(tanker, job)
					continue
				}
				if job.TargetNum > 0 {
					t.releaseTankerCustomers(player, tanker, job)
				}
				if tanker.Fuel < tanker.Spec.FuelCapacity && job.Stage != "patrol" {
					if !expeditionRefuel(&t.game.Rules, player, tanker) {
						holdAutomaticFleet(player, tanker, "No reachable fuel dock")
					}
					writeTankerJob(tanker, &automaticTankerJob{Status: "Returning or waiting at dock to refill"})
					continue
				}
				if job.Stage != "patrol" {
					delete(tanker.Tags, tankerJobTag)
				}
			}
			var chosen *automaticTankerJob
			best := math.Inf(1)
			for _, customer := range requests {
				if customer == tanker || customer.GetTag(tankerAwaitTag) != "" {
					continue
				}
				request := readAutomaticFuelRequest(customer)
				if player.Automation.FleetRoles[tanker.Num] != "tanker" && (!request.Urgent || tanker.Position.DistanceTo(customer.Position) > max(100, 4*tanker.StrategicSpeed(automaticMaxWarp(player, tanker)))) {
					continue
				}
				var group []*Fleet
				valid := true
				for _, num := range request.Members {
					f := t.game.getFleet(player.Num, num)
					if f == nil || f.Delete || !isAutomaticFleet(player, f) || f.Position != customer.Position || f.GetTag(tankerAwaitTag) != "" {
						valid = false
						break
					}
					group = append(group, f)
				}
				if !valid || len(group) == 0 {
					continue
				}
				// A fuel dock will refill the customer naturally at the end of this turn.
				atDock := false
				for _, dock := range expeditionStations(player, customer) {
					if dock.Position == customer.Position {
						atDock = true
						break
					}
				}
				if atDock {
					continue
				}
				positions := []Vector{customer.Position, {(customer.Position.X + tanker.Position.X) / 2, (customer.Position.Y + tanker.Position.Y) / 2}, tanker.Position}
				networkNode := false
				if hub := readTankerJob(tanker); hub != nil && hub.Stage == "patrol" {
					networkNode = true
					if request.Urgent {
						positions = append(positions, hub.Anchor)
					} else {
						positions = []Vector{tanker.Position, hub.Anchor, customer.Position, {(customer.Position.X + tanker.Position.X) / 2, (customer.Position.Y + tanker.Position.Y) / 2}}
					}
				}
				if !request.Urgent && !networkNode && len(customer.Waypoints) > 1 {
					wp := customer.Waypoints[1]
					distance := customer.Position.DistanceTo(wp.Position)
					if distance > 0 {
						step := min(distance, customer.StrategicSpeed(wp.WarpSpeed))
						positions = append(positions, customer.Position.ToFloat64().Add(wp.Position.Subtract(customer.Position).Normalized().Scale(step)).ToInt(true))
					}
				}
				for _, position := range positions {
					receiverRoute := planAutomaticRoute(&t.game.Rules, player, group, position, MapObjectTarget{}, automaticRoutePolicy{AllowThreat: request.AllowThreat || request.Urgent, RefuelRendezvous: true, EmergencyReturn: request.Urgent, MaxTurns: 10000})
					if receiverRoute == nil {
						continue
					}
					want := 0
					for i, f := range group {
						want += max(0, f.Spec.FuelCapacity-receiverRoute.ArrivalFuel[i])
					}
					if want == 0 {
						continue
					}
					minimumDelivery := 0
					for i, member := range group {
						need := automaticReturnFuel(&t.game.Rules, player, member, position, expeditionStations(player, member), request.AllowThreat)
						minimumDelivery += min(max(0, need-receiverRoute.ArrivalFuel[i]), max(10, member.Spec.FuelCapacity/4))
					}
					var deliveryRoute *expeditionRoute
					donation := 0
					for _, amount := range []int{want, min(want, max(1, tanker.Spec.FuelCapacity/2)), min(want, max(1, tanker.Spec.FuelCapacity/4)), min(want, max(1, minimumDelivery)), 1} {
						if amount < minimumDelivery || !request.Urgent && amount < want {
							continue
						}
						if route := planAutomaticRoute(&t.game.Rules, player, []*Fleet{tanker}, position, MapObjectTarget{}, automaticRoutePolicy{AllowThreat: combatAutonomy(player, tanker), DeliveryFuel: []int{amount}}); route != nil && !route.WaitForFuel {
							deliveryRoute, donation = route, amount
							break
						}
					}
					if deliveryRoute == nil {
						continue
					}
					eta := max(receiverRoute.Turns, deliveryRoute.Turns)
					if !request.Urgent {
						full := make([]*Fleet, len(group))
						for i, f := range group {
							copy := *f
							copy.Position = position
							copy.Fuel = f.Spec.FuelCapacity
							full[i] = &copy
						}
						after := planAutomaticRoute(&t.game.Rules, player, full, request.Destination, request.Target, automaticRoutePolicy{AllowThreat: request.AllowThreat, OneWay: request.OneWay})
						if after == nil || eta+after.Turns >= request.BaselineTurns {
							continue
						}
					}
					score := float64(eta) + float64(receiverRoute.Turns)*0.5 + float64(want-min(want, donation))/float64(max(1, want))
					if score < best {
						chosen = &automaticTankerJob{Urgent: request.Urgent, TargetNum: customer.Num, Members: fleetNumbers(group), Position: position, DeliveryFuel: donation, AllowThreat: request.AllowThreat, ResumeOrders: map[int]FleetOrders{}}
						best = score
					}
				}
				if chosen != nil && request.Urgent {
					break
				}
			}
			if chosen != nil {
				if player.Automation.FleetRoles[tanker.Num] == "patrol" && readTankerJob(tanker) == nil {
					t.releasePatrol(player, tanker)
				}
				if hub := readTankerJob(tanker); hub != nil && hub.Stage == "patrol" {
					chosen.Stage = "delivery"
					chosen.Anchor = hub.Anchor
					chosen.PatrolYear = hub.PatrolYear
					chosen.PatrolIndex = hub.PatrolIndex
				}
				for _, member := range t.tankerMembers(player, chosen) {
					orders := member.FleetOrders
					orders.Waypoints = slices.Clone(member.Waypoints)
					chosen.ResumeOrders[member.Num] = orders
					backup, _ := json.Marshal(orders)
					member.SetTag(fuelAwaitResumeTag, string(backup))
					member.SetTag(tankerAwaitTag, strconv.Itoa(tanker.Num))
				}
				if t.routeTankerJob(player, tanker, chosen) {
					writeTankerJob(tanker, chosen)
				} else {
					t.releaseTankerCustomers(player, tanker, chosen)
				}
			} else {
				if player.Automation.FleetRoles[tanker.Num] != "tanker" {
					continue
				}
				if t.patrolAutomaticTanker(player, tanker, readTankerJob(tanker)) {
					continue
				}
				if tanker.Fuel < tanker.Spec.FuelCapacity {
					if !expeditionRefuel(&t.game.Rules, player, tanker) {
						holdAutomaticFleet(player, tanker, "Waiting for a reachable refuelling dock")
					}
				} else {
					holdAutomaticFleet(player, tanker, "")
				}
				writeTankerJob(tanker, &automaticTankerJob{Status: "Available; refilling or awaiting a useful fuel request"})
			}
		}
	}
}

func (t *turnGenerator) resolveAutomaticTankers() {
	for _, player := range t.automaticPlayers() {
		for _, tanker := range t.game.getFleets(player.Num) {
			t.refuelRepairPatients(player, tanker)
			job := readTankerJob(tanker)
			if tanker.Delete || job == nil || job.TargetNum == 0 || !automaticFuelDonor(player, tanker) || tanker.Position != job.Position {
				continue
			}
			members := t.tankerMembers(player, job)
			if len(members) == 0 || t.armedEnemyAt(player, job.Position) {
				continue
			}
			ready := true
			for _, member := range members {
				if !isAutomaticFleet(player, member) || member.Position != job.Position || member.GetTag(tankerAwaitTag) != strconv.Itoa(tanker.Num) {
					ready = false
					break
				}
			}
			if !ready {
				continue
			}
			reserve := automaticReturnFuel(&t.game.Rules, player, tanker, tanker.Position, expeditionStations(player, tanker), false)
			available := max(0, tanker.Fuel-reserve)
			delivered := 0
			for _, member := range members {
				amount := min(available, max(0, member.Spec.FuelCapacity-member.Fuel))
				member.Fuel += amount
				tanker.Fuel -= amount
				available -= amount
				delivered += amount
				member.ComputeFuelUsage(player)
			}
			if delivered == 0 {
				continue
			}
			t.releaseTankerCustomers(player, tanker, job)
			for _, member := range members {
				reserve := automaticReturnFuel(&t.game.Rules, player, member, member.Position, expeditionStations(player, member), false)
				if member.Fuel < reserve {
					holdAutomaticFleet(player, member, "Partial rescue fuel received; waiting for enough fuel to return safely")
					saveAutomaticFuelRequest(member, automaticFuelRequest{Destination: member.Position, Members: []int{member.Num}, Urgent: true})
				}
			}
			next := &automaticTankerJob{Stage: "return", Anchor: job.Anchor, PatrolYear: job.PatrolYear, PatrolIndex: job.PatrolIndex, Status: "Fuel delivered; returning to refill"}
			if tanker.Spec.FuelGeneration > 0 && tanker.Fuel >= tanker.Spec.FuelCapacity/3 && job.PatrolYear > 0 {
				next.Stage = "patrol"
				next.Position = job.Anchor
				next.Status = "Returning to network area and regenerating fuel"
			}
			writeTankerJob(tanker, next)
			player.Messages = append(player.Messages, PlayerMessage{Type: PlayerMessageInfo, Text: fmt.Sprintf("%s delivered %d fuel to %s and retained fuel to reach a dock.", tanker.Name, delivered, members[0].Name)})
		}
	}
}

func automaticFuelDonor(player *Player, fleet *Fleet) bool {
	role := player.Automation.FleetRoles[fleet.Num]
	return role == "tanker" || role == "patrol" || role == "escort" && fleet.GetTag(escortTag) == "" && fleet.GetTag(bomberBorrowTag) == "" && !player.FleetInActiveAssembly(fleet)
}
