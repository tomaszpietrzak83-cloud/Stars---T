package cs

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
)

// Positive values order a scrap-and-rebuild replacement on the same hull.
// RetireDesign scraps the old ships without building replacements.
type DesignMaintenance map[int]int

const RetireDesign = -1
const maintenanceTag = "design-maintenance"
const replacementOrderTag = "replacement-order"
const replacementGroupTag = "replacement-group"

type fleetMaintenance struct {
	Expected       int         `json:"expected,omitempty"`
	OriginalRole   string      `json:"originalRole,omitempty"`
	RallyPlanetNum int         `json:"rallyPlanetNum,omitempty"`
	Stage          string      `json:"stage,omitempty"`
	OriginalOrders FleetOrders `json:"originalOrders"`
	Group          string      `json:"group"`
	BaseName       string      `json:"baseName"`
	PlanetNum      int         `json:"planetNum,omitempty"`
	Status         string      `json:"status,omitempty"`
}

func (p *Player) ValidateDesignMaintenance(policies DesignMaintenance) error {
	for source, target := range policies {
		old := p.GetDesign(source)
		if old == nil || old.PlayerNum != p.Num || old.OriginalPlayerNum != None {
			return fmt.Errorf("maintenance requires an owned, non-transferred design")
		}
		if target == RetireDesign {
			if old.Spec.Starbase {
				return fmt.Errorf("starbases require a successor design instead of automatic scrapping")
			}
			continue
		}
		next := p.GetDesign(target)
		if next == nil || source == target || next.PlayerNum != p.Num || next.Spec.Starbase != old.Spec.Starbase || next.OriginalPlayerNum != None || old.Hull != next.Hull {
			return fmt.Errorf("replacement must be a different owned design on the same hull")
		}
		if _, chained := policies[target]; chained {
			return fmt.Errorf("replacement cannot itself be retired or replaced")
		}
	}
	return nil
}
func (f *Fleet) InDesignMaintenance() bool       { return f.GetTag(maintenanceTag) != "" }
func readMaintenance(f *Fleet) *fleetMaintenance { return decodeMaintenance(f.GetTag(maintenanceTag)) }
func decodeMaintenance(raw string) *fleetMaintenance {
	work := &fleetMaintenance{}
	if json.Unmarshal([]byte(raw), work) != nil {
		return nil
	}
	return work
}
func encodeMaintenance(work *fleetMaintenance) string {
	data, _ := json.Marshal(work)
	return string(data)
}
func writeMaintenance(f *Fleet, work *fleetMaintenance) {
	f.SetTag(maintenanceTag, encodeMaintenance(work))
}

// Existing server-issued rebuild orders may be moved, reduced, or removed, but
// their metadata and paid allocation cannot be forged or duplicated by a client.
func validateMaintenanceQueueEdits(existing, incoming []ProductionQueueItem) error {
	used := map[int]bool{}
	keys := []string{frontierPatrolTag, emergencyReturnTag, bombingPackTag, bombingRallyTag, repairTankerTag, patrolSupportTag, scoutReconTag, bomberBorrowTag, logisticsReturnTag, "maintenance-pending", maintenanceTag, replacementGroupTag, replacementOrderTag, logisticsTag, escortTag, assemblyTag, fuelStatusTag, fuelRequestTag, fuelResumeTag, fuelAwaitResumeTag, tankerJobTag, tankerAwaitTag, colonizationTag, terraformTag, safetyTag, bomberTag, automaticPacketTag, patrolTag, passengerRotationTag, fuelNetworkPlanTag, fuelNetworkMemberTag, fuelNetworkDonorTag}
	for _, item := range incoming {
		reserved := false
		for _, key := range keys {
			reserved = reserved || item.Tags[key] != ""
		}
		if !reserved {
			continue
		}
		matched := false
		for index, old := range existing {
			if used[index] || item.DesignNum != old.DesignNum || item.Type != old.Type || item.Quantity > old.Quantity || item.Allocated != old.Allocated {
				continue
			}
			equal := true
			for _, key := range keys {
				equal = equal && item.Tags[key] == old.Tags[key]
			}
			if equal {
				used[index] = true
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("cannot forge or duplicate reserved maintenance metadata")
		}
	}
	return nil
}
func (t *turnGenerator) pendingReplacements(player *Player, work *fleetMaintenance) bool {
	if work == nil {
		return false
	}
	for _, planet := range t.game.Planets {
		if !planet.OwnedBy(player.Num) {
			continue
		}
		for _, item := range planet.ProductionQueue {
			if order := decodeMaintenance(item.Tags[replacementOrderTag]); order != nil && order.Group == work.Group && item.Quantity > 0 {
				return true
			}
		}
	}
	return false
}
func (t *turnGenerator) maintenanceNeeded(player *Player, fleet *Fleet) bool {
	if scoutRetirementDeferred(player, fleet) {
		return false
	}
	for _, token := range fleet.Tokens {
		if token.Quantity > 0 && (player.DesignMaintenance[token.DesignNum] == RetireDesign || player.DesignMaintenance[token.DesignNum] > 0 && (fleet.InDesignMaintenance() || player.Automation.RebuildFleets[fleet.Num] || player.Automation.FleetRoles[fleet.Num] == "scout")) {
			return true
		}
	}
	return t.pendingReplacements(player, readMaintenance(fleet))
}
func (t *turnGenerator) fleetDesignMaintenance() {
	for _, p := range t.automaticPlayers() {
		p.NormalizeDesignMaintenance(p.DesignMaintenance)
	}
	for _, fleet := range t.game.Fleets {
		if fleet.Delete || fleet.Starbase {
			continue
		}
		player := t.game.getPlayer(fleet.PlayerNum)
		if fleet.carriesInfrastructure() {
			continue
		}
		if fuelNetworkManaged(fleet) {
			continue
		}
		// A bulk upgrade can be queued during composition; finish gathering first.
		if player.FleetInActiveAssembly(fleet) && !fleet.InDesignMaintenance() {
			continue
		}
		if len(player.DesignMaintenance) == 0 && !fleet.InDesignMaintenance() {
			continue
		}
		if !fleet.InDesignMaintenance() && t.maintenanceNeeded(player, fleet) {
			delivery := logisticsWork(fleet)
			taskPending := false
			if job := readTerraformJob(fleet); job != nil && player.Automation.FleetRoles[fleet.Num] == "terraformer" {
				taskPending = terraformWork(player, t.game.getPlanet(job.PlanetNum))
			}
			if job := readMiningJob(fleet); job != nil && player.Automation.FleetRoles[fleet.Num] == "miner" {
				taskPending = t.game.Year < job.ArrivedYear+miningMinimumStay
			}
			if len(fleet.Waypoints) > 1 || delivery != nil || taskPending {
				fleet.SetTag("maintenance-pending", "Finish current navigation/delivery before rebuilding")
				fleet.RepeatOrders = false
				continue
			}
			delete(fleet.Tags, "maintenance-pending")
		}
		work := readMaintenance(fleet)
		if work != nil && work.RallyPlanetNum != 0 && !hasMaintenanceTargets(player, fleet) {
			t.rallyReplacementFleet(player, fleet, work)
			continue
		}
		if !t.maintenanceNeeded(player, fleet) {
			if work != nil {
				fleet.FleetOrders = work.OriginalOrders
				if len(fleet.Waypoints) > 0 {
					if fleet.Waypoints[0].Task != WaypointTaskNone {
						fleet.Waypoints = append([]Waypoint{{Position: fleet.Position}}, fleet.Waypoints...)
					} else {
						fleet.Waypoints[0] = Waypoint{Position: fleet.Position}
					}
				}
				delete(fleet.Tags, maintenanceTag)
				delete(fleet.Tags, replacementGroupTag)
				if work.OriginalRole != "" {
					if player.Automation.FleetRoles == nil {
						player.Automation.FleetRoles = map[int]string{}
					}
					player.Automation.FleetRoles[fleet.Num] = work.OriginalRole
				}
			}
			continue
		}
		if work == nil {
			data, _ := json.Marshal(fleet.FleetOrders)
			work = &fleetMaintenance{Group: fmt.Sprintf("%d:%d:%d", player.Num, fleet.Num, t.game.Year), BaseName: fleet.BaseName}
			_ = json.Unmarshal(data, &work.OriginalOrders)
			work.OriginalRole = player.Automation.FleetRoles[fleet.Num]
			// The role remains visible and resumes after maintenance. Controllers skip InDesignMaintenance fleets.
			delete(fleet.Tags, logisticsTag)
			t.releaseEscorts(player, fleet)
		}
		delete(player.Automation.RebuildFleets, fleet.Num)
		fleet.FleetOrders = FleetOrders{BattlePlanNum: fleet.BattlePlanNum, Purpose: fleet.Purpose, Waypoints: []Waypoint{{Position: fleet.Position}}}
		var closest *Planet
		best := math.Inf(1)
		var chosenRoute *expeditionRoute
		pending := t.pendingReplacements(player, work)
		if assigned := t.game.getPlanet(work.PlanetNum); work.Stage == "scrap" && (assigned == nil || !assigned.OwnedBy(player.Num) || !assigned.Spec.HasStarbase) {
			work.PlanetNum = 0
		}
		for _, planet := range t.game.Planets {
			if !planet.OwnedBy(player.Num) || !planet.Spec.HasStarbase {
				continue
			}
			if pending && work.PlanetNum != planet.Num {
				continue
			}
			if work.Stage == "scrap" && work.PlanetNum != 0 && work.PlanetNum != planet.Num {
				continue
			}
			fits := true
			for _, token := range fleet.Tokens {
				if target := player.GetDesign(player.DesignMaintenance[token.DesignNum]); target != nil && !planet.CanBuild(target.Spec.Mass) {
					fits = false
				}
			}
			if !fits {
				continue
			}
			route := t.planAutomaticMission(player, []*Fleet{fleet}, planet.Position, planet.MapObject.ToTarget(), automaticRoutePolicy{AllowThreat: true, EmergencyReturn: true, MaxTurns: 10000})
			if route == nil {
				continue
			}
			if float64(route.Turns) < best || float64(route.Turns) == best && closest != nil && planet.Num < closest.Num {
				closest, best, chosenRoute = planet, float64(route.Turns), route
			}
		}
		if closest != nil {
			if work.Stage == "" && hasShipsToReplace(player, fleet) && closest.Position.DistanceTo(fleet.Position) <= 100 {
				work.RallyPlanetNum = closest.Num
				t.scatterReplacementFleet(player, fleet, work)
				continue
			}
			work.PlanetNum = closest.Num
			work.Status = "Rebuilding at " + closest.Name
			applyExpeditionRoute(player, fleet, chosenRoute)
			if best > 0 {
				work.Status = "Returning to " + closest.Name
			}
		} else {
			if t.routeEmergencyReturn(player, fleet) {
				work.Status = "Returning toward fuel and shipyards while retaining the rebuild order"
			} else {
				work.Status = "Waiting for a reachable, suitable shipyard"
			}
			requestAutomaticRescue(player, fleet, true)
		}
		writeMaintenance(fleet, work)
		fleet.SetTag(replacementGroupTag, work.Group)
	}
}

// Scrapping pays out normally; replacements are full-price ordinary queue items.
func (t *turnGenerator) maintainShipsAtPlanet(player *Player, planet *Planet, available *Cost) {
	if !planet.Spec.HasStarbase || len(player.DesignMaintenance) == 0 {
		return
	}
	fleets := slices.Clone(t.game.Fleets)
	slices.SortFunc(fleets, func(a, b *Fleet) int { return a.Num - b.Num })
	for _, fleet := range fleets {
		if fleet.Delete || fleet.PlayerNum != player.Num || fleet.Position != planet.Position {
			continue
		}
		work := readMaintenance(fleet)
		if work == nil {
			continue
		}
		if work.Stage == "" && hasShipsToReplace(player, fleet) {
			continue
		}
		if work.PlanetNum != planet.Num {
			continue
		}
		work.PlanetNum, work.Status = planet.Num, "Waiting for replacement ships at "+planet.Name
		var replacements []ProductionQueueItem
		for index := 0; index < len(fleet.Tokens); {
			token := fleet.Tokens[index]
			targetNum := player.LatestReplacement(player.DesignMaintenance[token.DesignNum])
			if targetNum == 0 {
				index++
				continue
			}
			target := player.GetDesign(targetNum)
			if targetNum != RetireDesign && (target == nil || target.Hull != token.design.Hull || !planet.CanBuild(target.Spec.Mass)) {
				index++
				continue
			}
			planet.Cargo.Colonists += fleet.Cargo.Colonists
			*available = available.AddMineral(fleet.Cargo.ToMineral())
			fleet.Cargo = Cargo{}
			scrap := *fleet
			scrap.Spec.Cost = MultiplyCost(token.design.Spec.Cost, token.Quantity)
			recovered := scrap.getScrapAmount(&t.game.Rules, player, planet, false)
			*available = available.Add(recovered)
			report := t.maintenanceReport(player)
			report.scrapped[token.DesignNum] += token.Quantity
			report.recovered = report.recovered.Add(recovered)
			if target != nil {
				item := NewProductionQueueItemShip(token.Quantity, target).WithTag(replacementOrderTag, encodeMaintenance(work))
				item.design = target
				replacements = append(replacements, *item)
			}
			fleet.Tokens = slices.Delete(fleet.Tokens, index, index+1)
		}
		planet.ProductionQueue = append(replacements, planet.ProductionQueue...)
		if len(fleet.Tokens) == 0 {
			t.game.deleteFleet(fleet)
		} else {
			fleet.Spec = ComputeFleetSpec(&t.game.Rules, player, fleet)
			fleet.Fuel = min(fleet.Fuel, fleet.Spec.FuelCapacity)
			writeMaintenance(fleet, work)
		}
	}
}
func (t *turnGenerator) addReplacementShips(player *Player, planet *Planet, token ShipToken, work *fleetMaintenance) (*Fleet, error) {
	work.Stage = "rally"
	for _, fleet := range t.game.Fleets {
		if fleet.Delete || fleet.PlayerNum != player.Num || fleet.Position != planet.Position || fleet.GetTag(replacementGroupTag) != work.Group {
			continue
		}
		if existing := fleet.getTokenByDesign(token.DesignNum); existing != nil {
			existing.Quantity += token.Quantity
		} else {
			fleet.Tokens = append(fleet.Tokens, token)
		}
		fleet.Spec = ComputeFleetSpec(&t.game.Rules, player, fleet)
		fleet.Fuel = min(fleet.Spec.FuelCapacity, fleet.Fuel+token.design.Spec.FuelCapacity*token.Quantity)
		writeMaintenance(fleet, work)
		inheritReplacementRole(player, fleet, work)
		return fleet, nil
	}
	fleet, err := t.buildFleet(player, planet, token, Tags{}, nil)
	if err != nil {
		return nil, err
	}
	fleet.BaseName = work.BaseName
	fleet.Name = fmt.Sprintf("%s #%d", fleet.BaseName, fleet.Num)
	fleet.SetTag(replacementGroupTag, work.Group)
	writeMaintenance(fleet, work)
	inheritReplacementRole(player, fleet, work)
	return fleet, nil
}

// The role belongs to the replacement as soon as it is built. Controllers
// still wait for maintenance/regrouping to finish before issuing mission orders.
func inheritReplacementRole(player *Player, fleet *Fleet, work *fleetMaintenance) {
	delete(player.Automation.RebuildFleets, fleet.Num)
	if work.OriginalRole == "" {
		delete(player.Automation.FleetRoles, fleet.Num)
		return
	}
	if player.Automation.FleetRoles == nil {
		player.Automation.FleetRoles = map[int]string{}
	}
	player.Automation.FleetRoles[fleet.Num] = work.OriginalRole
}

// Keeping an already queued retired model must not block unrelated edits.
// It will be canceled/refunded (ships) or advanced to the successor (bases)
// during production. New or duplicated retired orders remain forbidden.
func validateRetiredQueueEdits(p *Player, existing, incoming []ProductionQueueItem) error {
	used := map[int]bool{}
	for _, item := range incoming {
		if (item.Type != QueueItemTypeShipToken && item.Type != QueueItemTypeStarbase) || p.DesignMaintenance[item.DesignNum] == 0 {
			continue
		}
		found := false
		for i, old := range existing {
			if !used[i] && old.Type == item.Type && old.DesignNum == item.DesignNum && item.Quantity <= old.Quantity && item.Allocated == old.Allocated {
				used[i] = true
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("cannot add production of a retired or replaced design")
		}
	}
	return nil
}
