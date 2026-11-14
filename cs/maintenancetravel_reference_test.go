package cs

import (
	"encoding/json"
	"fmt"
	"math"
)

func (t *turnGenerator) referenceFleetDesignMaintenance() {
	deferredSweepers := map[*Fleet]bool{}
	for _, p := range t.automaticPlayers() {
		p.NormalizeDesignMaintenance(p.DesignMaintenance)
		t.deferMinesweeperRebuilds(p, deferredSweepers)
	}
	for _, fleet := range t.game.Fleets {
		if fleet.Delete || fleet.Starbase {
			continue
		}
		player := t.game.getPlayer(fleet.PlayerNum)
		if deferredSweepers[fleet] {
			delete(fleet.Tags, "maintenance-pending")
			continue
		}
		if colonizerHasTask(player, fleet) {
			// Recover loaded legacy colonizers recalled by older maintenance rules.
			if work := readMaintenance(fleet); work != nil && work.Stage != "scrap" && work.Stage != "rally" && !t.pendingReplacements(player, work) {
				delete(fleet.Tags, maintenanceTag)
				delete(fleet.Tags, replacementGroupTag)
			}
			if t.maintenanceNeeded(player, fleet) {
				fleet.SetTag("maintenance-pending", "Colonizer finishes its current delivery or landing before maintenance")
			}
			continue
		}
		if fleet.InAutomaticSafety() {
			// Safety temporarily suspends refit without losing its group or mission.
			continue
		}
		if landingManaged(fleet) || fleet.carriesInfrastructure() {
			continue
		}
		if fuelNetworkManaged(fleet) {
			continue
		}
		// Finished automatic ships can work independently. Transports will
		// regroup with their landing army rather than wait for other shipyards.
		if work := readMaintenance(fleet); work != nil && work.Stage == "rally" && (work.OriginalRole == "patrol" || work.OriginalRole == "freighter" || work.OriginalRole == "minesweeper") && !hasMaintenanceTargets(player, fleet) {
			delete(fleet.Tags, maintenanceTag)
			delete(fleet.Tags, replacementGroupTag)
			delete(fleet.Tags, "maintenance-pending")
			if player.Automation.FleetRoles == nil {
				player.Automation.FleetRoles = map[int]string{}
			}
			player.Automation.FleetRoles[fleet.Num] = work.OriginalRole
			holdAutomaticFleet(player, fleet, "Rebuild completed; resuming automatic duties")
			continue
		}
		// A bulk upgrade can be queued during composition; finish gathering first.
		if player.FleetInActiveAssembly(fleet) && !fleet.InDesignMaintenance() {
			continue
		}
		if len(player.DesignMaintenance) == 0 && !fleet.InDesignMaintenance() {
			continue
		}
		if !fleet.InDesignMaintenance() && player.Automation.FleetRoles[fleet.Num] == "patrol" &&
			fleet.Spec.TotalShips > 1 && t.maintenanceNeeded(player, fleet) && (localPatrolReserve(fleet, t.game.Year) || patrolReserveNeeded(player, fleet)) {
			// Keep the queued rebuild for a later turn. Local battle readiness
			// must not disappear when an entire pack is due for replacement.
			delete(fleet.Tags, "maintenance-pending")
			continue
		}
		if !fleet.InDesignMaintenance() && t.maintenanceNeeded(player, fleet) {
			delivery := logisticsWork(fleet)
			taskPending := false
			if job := readTerraformJob(fleet); job != nil && player.Automation.FleetRoles[fleet.Num] == "terraformer" {
				taskPending = t.terraformWork(player, t.terraformTarget(player, fleet, job.PlanetNum))
			}
			if job := readMiningJob(fleet); job != nil && player.Automation.FleetRoles[fleet.Num] == "miner" {
				taskPending = t.game.Year < job.ArrivedYear+miningMinimumStay
			}
			if player.Automation.FleetRoles[fleet.Num] == "bomber" {
				if target := player.GetPlanetIntel(bomberGoal(fleet)); target != nil && target.Owned() && target.PlayerNum != player.Num && !player.IsFriend(target.PlayerNum) {
					taskPending = true
				}
			}
			if len(fleet.Waypoints) > 1 || delivery != nil || taskPending || player.Automation.FleetRoles[fleet.Num] == "freighter" && fleet.CargoMass() > 0 {
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
			if civilianFleet(player, fleet) && !t.civilianSiteSafe(player, fleet, planet.Position, 2) {
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
