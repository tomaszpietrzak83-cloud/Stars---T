package cs

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
)

const fuelNetworkPlanTag = "fuel-network-plan"
const fuelNetworkMemberTag = "fuel-network-member"
const fuelNetworkDonorTag = "fuel-network-donor"

type fuelNetworkPlan struct {
	ID          string              `json:"id"`
	Leader      int                 `json:"leader"`
	Members     []int               `json:"members"`
	Roles       map[int]string      `json:"roles"`
	Maintenance map[int]bool        `json:"maintenance"`
	Orders      map[int]FleetOrders `json:"orders"`
	Mission     Vector              `json:"mission"`
	Route       fuelNetworkRoute    `json:"route"`
	Leg         int                 `json:"leg"`
	Started     int                 `json:"started"`
	AllowThreat bool                `json:"allowThreat"`
	Status      string              `json:"status"`
}
type fuelNetworkHold struct {
	ID        string      `json:"id"`
	Leader    int         `json:"leader"`
	Amount    int         `json:"amount"`
	Orders    FleetOrders `json:"orders"`
	TankerJob string      `json:"tankerJob"`
}

func fuelNetworkReserved(fleet *Fleet) bool {
	return fleet != nil && (fuelNetworkManaged(fleet) || fleet.GetTag(fuelNetworkDonorTag) != "")
}
func fuelNetworkManaged(fleet *Fleet) bool {
	return fleet.GetTag(fuelNetworkMemberTag) != "" || fleet.GetTag(fuelNetworkPlanTag) != ""
}
func readFuelNetworkPlan(fleet *Fleet) *fuelNetworkPlan {
	var plan fuelNetworkPlan
	if json.Unmarshal([]byte(fleet.GetTag(fuelNetworkPlanTag)), &plan) != nil {
		return nil
	}
	return &plan
}
func saveFuelNetworkPlan(fleet *Fleet, plan *fuelNetworkPlan) {
	raw, _ := json.Marshal(plan)
	fleet.SetTag(fuelNetworkPlanTag, string(raw))
}
func readFuelNetworkHold(fleet *Fleet) *fuelNetworkHold {
	var hold fuelNetworkHold
	if json.Unmarshal([]byte(fleet.GetTag(fuelNetworkDonorTag)), &hold) != nil {
		return nil
	}
	return &hold
}

func (t *turnGenerator) releaseFuelNetworkDonor(player *Player, donor *Fleet, id string) {
	hold := readFuelNetworkHold(donor)
	if hold == nil || hold.ID != id {
		return
	}
	delete(donor.Tags, fuelNetworkDonorTag)
	if donor.Delete || donor.InAutomaticSafety() || donor.InDesignMaintenance() || player.Automation.FleetRoles[donor.Num] != "tanker" {
		return
	}
	donor.FleetOrders = hold.Orders
	if len(donor.Waypoints) > 0 {
		donor.Waypoints[0].Position = donor.Position
	}
	if hold.TankerJob != "" {
		donor.SetTag(tankerJobTag, hold.TankerJob)
	}
	delete(donor.Tags, fuelStatusTag)
}
func (t *turnGenerator) closeFuelNetwork(player *Player, leader *Fleet, plan *fuelNetworkPlan, complete bool) {
	for _, donor := range t.game.getFleets(player.Num) {
		t.releaseFuelNetworkDonor(player, donor, plan.ID)
	}
	for _, num := range plan.Members {
		member := t.game.getFleet(player.Num, num)
		if member == nil {
			continue
		}
		delete(member.Tags, fuelNetworkMemberTag)
		delete(member.Tags, fuelNetworkPlanTag)
		if member.Delete || member.InAutomaticSafety() || member.InDesignMaintenance() || player.Automation.FleetRoles[num] != plan.Roles[num] {
			continue
		}
		orders := plan.Orders[num]
		orders.Waypoints = slices.Clone(orders.Waypoints)
		if complete {
			if len(orders.Waypoints) > 1 && orders.Waypoints[1].Position == member.Position && orders.Waypoints[1].TargetType != MapObjectTypeFleet {
				orders.Waypoints = orders.Waypoints[1:]
			}
			if len(orders.Waypoints) > 0 {
				orders.Waypoints[0].Position = member.Position
			}
			member.FleetOrders = orders
			delete(member.Tags, fuelStatusTag)
			delete(member.Tags, fuelRequestTag)
			delete(member.Tags, fuelResumeTag)
		} else {
			raw, _ := json.Marshal(orders)
			member.SetTag(fuelResumeTag, string(raw))
			holdAutomaticFleet(player, member, "Reserved fuel route changed; replanning a safe return")
			saveAutomaticFuelRequest(member, automaticFuelRequest{Destination: member.Position, Members: []int{member.Num}, Urgent: true})
		}
		member.ComputeFuelUsage(player)
	}
	delete(leader.Tags, fuelNetworkPlanTag)
}
func (t *turnGenerator) fuelNetworkMembers(player *Player, plan *fuelNetworkPlan) ([]*Fleet, bool) {
	members := []*Fleet{}
	for _, num := range plan.Members {
		fleet := t.game.getFleet(player.Num, num)
		if fleet == nil || fleet.Delete || fleet.InAutomaticSafety() || fleet.InDesignMaintenance() != plan.Maintenance[num] || fleet.GetTag(fuelNetworkMemberTag) != strconv.Itoa(plan.Leader) || player.Automation.FleetRoles[num] != plan.Roles[num] {
			return nil, false
		}
		if plan.Roles[num] == "" && !plan.Maintenance[num] && !player.FleetInActiveAssembly(fleet) {
			return nil, false
		}
		members = append(members, fleet)
	}
	return members, len(members) > 0
}
func (t *turnGenerator) fuelNetworkReservationsValid(player *Player, plan *fuelNetworkPlan) bool {
	if plan.Leg < 0 || plan.Leg >= len(plan.Route.Legs) {
		return false
	}
	for _, leg := range plan.Route.Legs[plan.Leg:] {
		if len(leg.Required) != len(plan.Members) || len(leg.Reserve) != len(plan.Members) {
			return false
		}
		if leg.RefuelDock {
			dock := player.GetPlanetIntel(leg.Target.TargetNum)
			if dock == nil || !dock.OwnedBy(player.Num) {
				return false
			}
			for _, num := range plan.Members {
				member := t.game.getFleet(player.Num, num)
				if member == nil || !member.CanFuel(player, dock) {
					return false
				}
			}
		}
		for _, donation := range leg.Donations {
			donor := t.game.getFleet(player.Num, donation.FleetNum)
			if donor == nil || donor.Delete || donor.InAutomaticSafety() || donor.InDesignMaintenance() || player.Automation.FleetRoles[donor.Num] != "tanker" || donor.Position != leg.From {
				return false
			}
			hold := readFuelNetworkHold(donor)
			if hold == nil || hold.ID != plan.ID || hold.Amount < donation.Amount {
				return false
			}
			reserve := automaticReturnFuel(&t.game.Rules, player, fullCargoBudget(donor), donor.Position, expeditionStations(player, donor), false)
			if reserve > donor.Fuel || donation.Amount > donor.Fuel-reserve {
				return false
			}
		}
	}
	return true
}
func (t *turnGenerator) advanceFuelNetworks() {
	for _, player := range t.game.Players {
		for _, donor := range t.game.getFleets(player.Num) {
			if hold := readFuelNetworkHold(donor); hold != nil {
				leader := t.game.getFleet(player.Num, hold.Leader)
				if leader == nil || leader.Delete || readFuelNetworkPlan(leader) == nil {
					t.releaseFuelNetworkDonor(player, donor, hold.ID)
				}
			}
		}
		for _, leader := range slices.Clone(t.game.getFleets(player.Num)) {
			plan := readFuelNetworkPlan(leader)
			if plan == nil {
				continue
			}
			if leader.GetTag(frontierPatrolTag) != "" || leader.GetTag(escortStagingTag) != "" {
				mobile := false
				for _, leg := range plan.Route.Legs {
					mobile = mobile || len(leg.Donations) > 0
				}
				if !mobile {
					t.closeFuelNetwork(player, leader, plan, false)
					continue
				}
			}
			members, valid := t.fuelNetworkMembers(player, plan)
			if !valid || plan.Leg >= len(plan.Route.Legs) || !t.fuelNetworkReservationsValid(player, plan) {
				t.closeFuelNetwork(player, leader, plan, false)
				continue
			}
			for transitions := 0; transitions <= len(plan.Route.Legs); transitions++ {
				leg := &plan.Route.Legs[plan.Leg]
				arrived, atFrom := true, true
				for _, member := range members {
					arrived = arrived && member.Position == leg.To
					atFrom = atFrom && member.Position == leg.From
				}
				if atFrom && len(leg.Donations) > 0 {
					if t.armedEnemyAt(player, leg.From) {
						for _, member := range members {
							holdAutomaticFleet(player, member, "Fuel rendezvous waiting for the area to be clear")
						}
						break
					}
					need := 0
					for i, member := range members {
						need += max(0, leg.Required[i]-member.Fuel)
					}
					for _, donation := range leg.Donations {
						donor := t.game.getFleet(player.Num, donation.FleetNum)
						available := min(need, donation.Amount)
						for i, member := range members {
							amount := min(available, max(0, leg.Required[i]-member.Fuel))
							member.Fuel += amount
							donor.Fuel -= amount
							available -= amount
							need -= amount
						}
						t.releaseFuelNetworkDonor(player, donor, plan.ID)
					}
					leg.Donations = nil
					if need > 0 {
						t.closeFuelNetwork(player, leader, plan, false)
						break
					}
				}
				if arrived {
					if leg.DockWait && t.game.Year <= plan.Started {
						for _, member := range members {
							holdAutomaticFleet(player, member, "Waiting for actual dock refuelling")
						}
						break
					}
					plan.Leg++
					if plan.Leg == len(plan.Route.Legs) {
						t.closeFuelNetwork(player, leader, plan, true)
						break
					}
					continue
				}

				valid = true
				for i, member := range members {
					if member.Position != members[0].Position {
						valid = false
						break
					}
					_, _, _, enough := automaticLegFuel(player, fullCargoBudget(member), leg.Warp, member.Position.DistanceTo(leg.To), member.Fuel, leg.Reserve[i])
					if !enough {
						valid = false
						break
					}
				}
				if !valid {
					t.closeFuelNetwork(player, leader, plan, false)
					break
				}
				for _, member := range members {
					member.FleetOrders = FleetOrders{BattlePlanNum: member.BattlePlanNum, Purpose: member.Purpose, Waypoints: []Waypoint{{Position: member.Position}, {Position: leg.To, MapObjectTarget: leg.Target, WarpSpeed: leg.Warp}}}
					member.ComputeFuelUsage(player)
					member.SetTag(fuelStatusTag, fmt.Sprintf("Reserved fuel route: leg %d/%d, next stop %s", plan.Leg+1, len(plan.Route.Legs), leg.To.String()))
				}
				plan.Status = fmt.Sprintf("Leg %d/%d; reserved tanker fuel, expected travel %d turns", plan.Leg+1, len(plan.Route.Legs), plan.Route.Turns)
				saveFuelNetworkPlan(leader, plan)
				break
			}
			if readFuelNetworkPlan(leader) != nil {
				saveFuelNetworkPlan(leader, plan)
			}
		}
	}
}

func (t *turnGenerator) commitFuelNetwork(player *Player, group []*Fleet, request automaticFuelRequest, route *fuelNetworkRoute) bool {
	leader := group[0]
	plan := &fuelNetworkPlan{ID: fmt.Sprintf("%d:%d:%d", player.Num, leader.Num, t.game.Year), Leader: leader.Num, Members: fleetNumbers(group), Roles: map[int]string{}, Maintenance: map[int]bool{}, Orders: map[int]FleetOrders{}, Mission: request.Destination, Route: *route, Started: t.game.Year, AllowThreat: request.AllowThreat}
	reservations := map[int]int{}
	for _, leg := range route.Legs {
		for _, donation := range leg.Donations {
			reservations[donation.FleetNum] += donation.Amount
		}
	}

	for num, amount := range reservations {
		donor := t.game.getFleet(player.Num, num)
		if donor == nil || donor.Delete || donor.GetTag(fuelNetworkDonorTag) != "" {
			return false
		}
		reserve := automaticReturnFuel(&t.game.Rules, player, fullCargoBudget(donor), donor.Position, expeditionStations(player, donor), false)
		if reserve > donor.Fuel || amount > donor.Fuel-reserve {
			return false
		}
	}
	for num, amount := range reservations {
		donor := t.game.getFleet(player.Num, num)
		hold := fuelNetworkHold{ID: plan.ID, Leader: leader.Num, Amount: amount, Orders: donor.FleetOrders, TankerJob: donor.GetTag(tankerJobTag)}
		hold.Orders.Waypoints = slices.Clone(donor.Waypoints)
		raw, _ := json.Marshal(hold)
		donor.SetTag(fuelNetworkDonorTag, string(raw))
		delete(donor.Tags, tankerJobTag)
		holdAutomaticFleet(player, donor, fmt.Sprintf("Fuel reserved for fleet #%d: %d", leader.Num, amount))
	}
	for _, member := range group {
		orders := member.FleetOrders
		if raw := member.GetTag(fuelResumeTag); raw != "" {
			var saved FleetOrders
			if json.Unmarshal([]byte(raw), &saved) == nil && len(saved.Waypoints) > 0 {
				orders = saved
			}
		}
		orders.Waypoints = slices.Clone(orders.Waypoints)
		plan.Orders[member.Num] = orders
		plan.Roles[member.Num] = player.Automation.FleetRoles[member.Num]
		plan.Maintenance[member.Num] = member.InDesignMaintenance()
		member.SetTag(fuelNetworkMemberTag, strconv.Itoa(leader.Num))
		delete(member.Tags, fuelRequestTag)
	}
	saveFuelNetworkPlan(leader, plan)
	return true
}

func (t *turnGenerator) planFuelNetworks() {
	for _, player := range t.game.Players {
		fleets := slices.Clone(t.game.getFleets(player.Num))
		slices.SortFunc(fleets, func(a, b *Fleet) int { return a.Num - b.Num })
		for _, customer := range fleets {
			request := readAutomaticFuelRequest(customer)
			if !fuelAssistanceUseful(player, customer, request) {
				continue
			}
			if request == nil || customer.Delete || customer.InAutomaticSafety() || fuelNetworkManaged(customer) || customer.GetTag(tankerAwaitTag) != "" {
				continue
			}
			group := []*Fleet{}
			valid := true
			for _, num := range request.Members {
				member := t.game.getFleet(player.Num, num)
				if member == nil || member.Delete || !isAutomaticFleet(player, member) || fuelNetworkManaged(member) || member.Position != customer.Position || member.GetTag(tankerAwaitTag) != "" {
					valid = false
					break
				}
				group = append(group, member)
			}
			if !valid || len(group) == 0 {
				continue
			}
			donors := []*Fleet{}
			for _, donor := range fleets {
				if donor.Delete || donor.Starbase || player.Automation.FleetRoles[donor.Num] != "tanker" || donor.InAutomaticSafety() || donor.InDesignMaintenance() || donor.GetTag(fuelNetworkDonorTag) != "" || fuelNetworkManaged(donor) || slices.Contains(group, donor) {
					continue
				}
				job := readTankerJob(donor)
				if job != nil && job.Stage != "patrol" {
					continue
				}
				donors = append(donors, donor)
			}

			if !request.Urgent && len(donors) == 0 {
				continue
			} // No phantom acceleration without mobile fuel.
			policy := automaticRoutePolicy{AllowThreat: request.AllowThreat, OneWay: request.OneWay, MaxTurns: 50}
			returning := request.Urgent && request.Destination == customer.Position
			if returning {
				policy.MaxTurns = 20
			}
			if player.Automation.FleetRoles[customer.Num] == "patrol" {
				policy.MaxTurns = 8
			}
			if returning && player.Automation.FleetRoles[customer.Num] == "scout" {
				policy.MaxTurns = player.Automation.scoutReturnTurns()
			}
			chosen := planFuelNetworkRoute(&t.game.Rules, player, group, donors, request.Destination, request.Target, policy, returning)
			if chosen != nil && !request.Urgent && chosen.Turns >= request.BaselineTurns {
				chosen = nil
			}

			if chosen != nil && combatAutonomy(player, customer) {
				mobile := false
				for _, leg := range chosen.Legs {
					mobile = mobile || len(leg.Donations) > 0
				}
				if !mobile {
					chosen = nil
				}
			}
			if chosen != nil {
				t.commitFuelNetwork(player, group, *request, chosen)
			}
		}
	}
	t.advanceFuelNetworks()
}

func (t *turnGenerator) fuelNetworkDepartureReserve(player *Player, fleet *Fleet) (int, bool) {
	if !fuelNetworkManaged(fleet) {
		return 0, false
	}
	num, _ := strconv.Atoi(fleet.GetTag(fuelNetworkMemberTag))
	leader := t.game.getFleet(player.Num, num)
	if leader == nil {
		return 0, false
	}
	plan := readFuelNetworkPlan(leader)
	if plan == nil || plan.Leg >= len(plan.Route.Legs) || !t.fuelNetworkReservationsValid(player, plan) {
		return 0, false
	}
	for i, member := range plan.Members {
		if member == fleet.Num {
			return plan.Route.Legs[plan.Leg].Reserve[i], true
		}
	}
	return 0, false
}

func (t *turnGenerator) releaseUnassistedCruiseNetworks(p *Player) {
	for _, f := range t.game.getFleets(p.Num) {
		plan := readFuelNetworkPlan(f)
		if plan == nil || f.GetTag(frontierPatrolTag) == "" && f.GetTag(escortStagingTag) == "" {
			continue
		}
		mobile := false
		for _, leg := range plan.Route.Legs {
			mobile = mobile || len(leg.Donations) > 0
		}
		if !mobile {
			t.closeFuelNetwork(p, f, plan, false)
			for _, n := range plan.Members {
				if m := t.game.getFleet(p.Num, n); m != nil {
					delete(m.Tags, fuelResumeTag)
					delete(m.Tags, fuelRequestTag)
				}
			}
		}
	}
}
