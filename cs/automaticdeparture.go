package cs

import (
	"encoding/json"
	"slices"
	"strconv"
)

// Waypoint-zero loads/transfers run after mission planning. Recheck the actual
// fuel and cargo just before movement, and stop the whole convoy on a shortfall.
func (t *turnGenerator) enforceAutomaticDeparture() {
	for _, player := range t.automaticPlayers() {
		blocked := map[int]bool{}
		for _, fleet := range t.game.getFleets(player.Num) {
			if !isAutomaticFleet(player, fleet) || len(fleet.Waypoints) < 2 {
				continue
			}
			wp := fleet.Waypoints[1]
			if player.Automation.FleetRoles[fleet.Num] == "scout" && wp.TargetType == MapObjectTypeFleet && wp.TargetPlayerNum != player.Num && !player.IsFriend(wp.TargetPlayerNum) {
				job := readScoutRecon(fleet)
				report := player.GetFleetIntel(wp.TargetPlayerNum, wp.TargetNum)
				if job == nil || job.Watching || job.Enemy != wp.TargetPlayerNum || job.Target != wp.TargetNum || report == nil || report.Delete || report.ReportAge != 0 || !unknownFleetEquipment(player, report) {
					holdAutomaticFleet(player, fleet, "Obsolete contact order cancelled; equipment or target report changed")
					continue
				}
			}
			if player.Automation.FleetRoles[fleet.Num] == "patrol" {
				if !t.patrolDepartureConfirmed(player, fleet) {
					holdAutomaticFleet(player, fleet, "Departure cancelled: reported encounter exceeds the confirmed combat force")
					continue
				}
				job := readPatrolJob(fleet)
				if enemy := patrolEnemy(player, job); job.Leader != 0 && enemy != nil && patrolNeedsRecon(player, enemy) && !provisionalPatrolLeg(player, fleet) && !fleet.InAutomaticSafety() && !fleet.InDesignMaintenance() && !fundedEmergencyReturn(&t.game.Rules, player, fleet) && !knownWithdrawalRoute(player, fleet.Position, wp.Position) {
					holdAutomaticFleet(player, fleet, "Waiting for scout equipment reconnaissance")
					continue
				}
			}
			if wp.TargetType == MapObjectTypeFleet && wp.TargetPlayerNum != player.Num && !player.IsFriend(wp.TargetPlayerNum) && !fleet.Spec.HasWeapons && !scoutReconEligible(player, fleet) {
				delete(fleet.Tags, scoutReconTag)
				if !t.routeEmergencyReturn(player, fleet) {
					holdAutomaticFleet(player, fleet, "Unarmed cargo ships cannot intercept enemy fleets; returning or requesting fuel")
				}
				continue
			}
			allowThreat := fleet.GetTag(emergencyRiskTag) != "" || combatAutonomy(player, fleet) || scoutReconActive(player, fleet) || patrolCanAttack(player, fleet) || fleet.InAutomaticSafety() || fleet.GetTag(escortTag) != "" || len(convoyMembers(player, fleet, t.game.getFleets(player.Num))) > 1 || assemblyLeaderCanAdvance(player, fleet)
			allowThreat = allowThreat || collectionArrivalCover(player, fleet, t.game.getFleets(player.Num)) > 0
			if provisionalPatrolLeg(player, fleet) && !fleet.InAutomaticSafety() {
				allowThreat = false
			}
			if fleet.GetTag(patrolFrontTag) != "" && readPatrolJob(fleet).Leader == 0 && !fleet.InAutomaticSafety() && !fundedEmergencyReturn(&t.game.Rules, player, fleet) {
				allowThreat = false
			}
			if raw := fleet.GetTag(tankerAwaitTag); raw != "" {
				num, _ := strconv.Atoi(raw)
				if donor := t.game.getFleet(player.Num, num); donor != nil && !donor.Delete {
					if job := readTankerJob(donor); job != nil && slices.Contains(job.Members, fleet.Num) && job.Position == wp.Position {
						allowThreat = allowThreat || job.AllowThreat || job.Urgent
					}
				}
			}
			valid := true
			if wp.WarpSpeed == StargateWarpSpeed {
				valid = fleet.CanJump(player, fleet.Position.DistanceTo(wp.Position), player.GetPlanetIntel(fleet.OrbitingPlanetNum), player.GetPlanetIntel(wp.TargetNum))
			} else {
				valid = wp.WarpSpeed > 0 && wp.WarpSpeed <= automaticMaxWarp(player, fleet) && automaticLegSafe(&t.game.Rules, player, fleet.Position, wp.Position, wp.WarpSpeed, allowThreat, fleet.StrategicSpeed(wp.WarpSpeed))
				reserve := automaticReturnFuel(&t.game.Rules, player, fleet, wp.Position, expeditionStations(player, fleet), allowThreat, automaticReturnLimit(player, fleet, wp.MapObjectTarget))
				if networkReserve, managed := t.fuelNetworkDepartureReserve(player, fleet); managed {
					reserve = networkReserve
				}
				if fundedEmergencyReturn(&t.game.Rules, player, fleet) || fundedScoutWaypoints(&t.game.Rules, player, fleet) || fundedLogisticsWaypoints(&t.game.Rules, player, fleet) {
					reserve = 0
				}
				if job := readScoutRecon(fleet); scoutReconActive(player, fleet) && job != nil && !job.Watching {
					reserve = 0
				}
				if colonizationOneWay(player, fleet, wp) {
					reserve = 0
				}
				if raw := fleet.GetTag(tankerAwaitTag); raw != "" {
					num, _ := strconv.Atoi(raw)
					if donor := t.game.getFleet(player.Num, num); donor != nil && !donor.Delete {
						if job := readTankerJob(donor); job != nil && job.Position == wp.Position && slices.Contains(job.Members, fleet.Num) {
							reserve = 0
						}
					}
				}
				if job := readTankerJob(fleet); job != nil && job.TargetNum > 0 && job.Position == wp.Position {
					reserve += job.DeliveryFuel
				}
				enough := false
				if wp.WarpSpeed > 0 {
					_, _, _, enough = automaticLegFuel(player, fleet, wp.WarpSpeed, fleet.Position.DistanceTo(wp.Position), fleet.Fuel, reserve)
				}
				valid = valid && enough
			}
			if !valid {
				blocked[fleet.Num] = true
				if raw := fleet.GetTag(tankerAwaitTag); raw != "" {
					num, _ := strconv.Atoi(raw)
					blocked[num] = true
				}
				if raw := fleet.GetTag(escortTag); raw != "" {
					num, _ := strconv.Atoi(raw)
					blocked[num] = true
				}
				if job := readTankerJob(fleet); job != nil && job.TargetNum > 0 {
					for _, num := range job.Members {
						blocked[num] = true
					}
				}
			}
		}
		for _, fleet := range t.game.getFleets(player.Num) {
			var collection anticipatedSalvage
			if json.Unmarshal([]byte(fleet.GetTag(anticipatedSalvageTag)), &collection) == nil && blocked[collection.Hunter] {
				blocked[fleet.Num] = true
			}
			if raw := fleet.GetTag(escortTag); raw != "" {
				num, _ := strconv.Atoi(raw)
				if blocked[num] {
					blocked[fleet.Num] = true
				}
			}
		}
		for num := range blocked {
			if fleet := t.game.getFleet(player.Num, num); fleet != nil && !fleet.Delete {
				raw, _ := json.Marshal(fleet.FleetOrders)
				fleet.SetTag(fuelResumeTag, string(raw))
				holdAutomaticFleet(player, fleet, "Departure held: actual fuel, payload or route no longer matches the safe plan")
				requestAutomaticRescue(player, fleet, true)
				// An invalid fuel promise must not immobilize both parties indefinitely.
				if raw := fleet.GetTag(tankerAwaitTag); raw != "" {
					n, _ := strconv.Atoi(raw)
					if donor := t.game.getFleet(player.Num, n); donor != nil {
						if job := readTankerJob(donor); job != nil {
							t.releaseTankerCustomers(player, donor, job)
							delete(donor.Tags, tankerJobTag)
						}
					}
				}
				if job := readTankerJob(fleet); job != nil && job.TargetNum > 0 {
					t.releaseTankerCustomers(player, fleet, job)
					delete(fleet.Tags, tankerJobTag)
				}
				if !atOwnFuelDock(player, fleet) && fleet.GetTag(tankerAwaitTag) == "" {
					t.routeEmergencyReturn(player, fleet)
				}
			}
		}
	}
}
