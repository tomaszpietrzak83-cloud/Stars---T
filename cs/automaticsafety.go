package cs

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"
)

const safetyTag = "automatic-safety"

type automaticSafetyJob struct {
	Kind   string      `json:"kind"`
	Orders FleetOrders `json:"orders"`
	Status string      `json:"status"`
}

func (fleet *Fleet) InAutomaticSafety() bool { return fleet.GetTag(safetyTag) != "" }
func fleetDamagePercent(fleet *Fleet) int {
	worst := 0
	for _, token := range fleet.Tokens {
		if token.QuantityDamaged > 0 && token.design != nil && token.design.Spec.Armor > 0 {
			worst = max(worst, int(math.Ceil(100*token.Damage/float64(token.design.Spec.Armor))))
		}
	}
	return worst
}

// Use only fresh visible courses, never hidden enemy orders. Closest approach
// checks simultaneous movement during this turn instead of crossing lines alone.
func interceptionThreat(player *Player, fleet *Fleet) int {
	if player.Automation.FleetRoles[fleet.Num] == "freighter" {
		point := fleet.Position
		if len(fleet.Waypoints) > 1 {
			wp := fleet.Waypoints[1]
			point = wp.Position
			if d := fleet.Position.DistanceTo(point); wp.WarpSpeed != StargateWarpSpeed && d > fleet.StrategicSpeed(wp.WarpSpeed) {
				point = fleet.Position.ToFloat64().Add(point.Subtract(fleet.Position).Normalized().Scale(fleet.StrategicSpeed(wp.WarpSpeed))).ToInt(true)
			}
		}
		present, incoming := landingOrbitalReports(player, point)
		threat := 0
		for _, enemy := range append(present, incoming...) {
			threat = min(1_000_000_000, threat+patrolReportedPower(player, enemy))
		}
		return threat
	}
	ours := VectorFloat64{}
	if len(fleet.Waypoints) > 1 {
		wp := fleet.Waypoints[1]
		speed := min(fleet.StrategicSpeed(wp.WarpSpeed), fleet.Position.DistanceTo(wp.Position))
		ours = wp.Position.Subtract(fleet.Position).Normalized().Scale(speed)
	}
	threat := 0
	for _, enemy := range player.FleetIntels {
		if enemy == nil || enemy.Delete || player.IsFriend(enemy.PlayerNum) || enemy.ReportAge != 0 || !patrolReportedCombatant(player, enemy) || enemy.WarpSpeed < 1 || enemy.WarpSpeed > 10 {
			continue
		}
		speed := enemy.observedStrategicSpeed()
		ev := enemy.Heading.Scale(speed)
		dx, dy := float64(enemy.Position.X-fleet.Position.X), float64(enemy.Position.Y-fleet.Position.Y)
		vx, vy := ev.X-ours.X, ev.Y-ours.Y
		den := vx*vx + vy*vy
		if den == 0 {
			if math.Hypot(dx, dy) <= 3 {
				threat = min(1_000_000_000, threat+patrolReportedPower(player, enemy))
			}
			continue
		}
		at := -(dx*vx + dy*vy) / den
		if at < 0 || at > 1 {
			continue
		}
		if math.Hypot(dx+vx*at, dy+vy*at) <= 3 {
			threat = min(1_000_000_000, threat+patrolReportedPower(player, enemy))
		}
	}
	return threat
}
func (t *turnGenerator) planAutomaticSafety() {
	for _, player := range t.game.Players {
		for _, fleet := range t.game.getFleets(player.Num) {
			if !isAutomaticFleet(player, fleet) || fleet.Starbase || fleet.InDesignMaintenance() && !civilianFleet(player, fleet) {
				continue
			}
			var job automaticSafetyJob
			active := json.Unmarshal([]byte(fleet.GetTag(safetyTag)), &job) == nil
			damage := fleetDamagePercent(fleet)
			threat := interceptionThreat(player, fleet)
			if player.Automation.FleetRoles[fleet.Num] == "minesweeper" {
				threat = max(threat, bomberChaseThreat(player, fleet))
			}
			defenseReady, controlledBombardment := false, false
			protectedRefit := fleet.InDesignMaintenance() && civilianFleet(player, fleet) && player.Automation.FleetRoles[fleet.Num] != "freighter"
			if protectedRefit && len(t.civilianReports(player, fleet.Position, 1)) > 0 {
				controlledBombardment = true
				defenseReady = t.civilianSiteSafe(player, fleet, fleet.Position, 1)
				threat = max(threat, bomberImmediateMaximumThreat(player, fleet))
			}
			if !protectedRefit && player.Automation.FleetRoles[fleet.Num] == "bomber" && player.patrolEvaluator != nil && len(bomberWarningReports(player, fleet)) > 0 {
				controlledBombardment = true
				var remoteWarning bool
				defenseReady, remoteWarning = t.prepareBombardmentProtection(player, fleet)
				threat = max(threat, bomberImmediateMaximumThreat(player, fleet))
				if remoteWarning {
					for _, e := range bomberWarningReports(player, fleet) {
						threat = min(1_000_000_000, threat+patrolReportedPower(player, e))
					}
				}
			}
			if player.Automation.FleetRoles[fleet.Num] == "bomber" || player.Automation.FleetRoles[fleet.Num] == "tanker" {
				threat = max(threat, bomberChaseThreat(player, fleet))
				if threat > 0 && !controlledBombardment {
					t.assignBomberProtection(player, fleet, fleet.Position, true)
				}
			}
			defenders := 0
			defenders += collectionArrivalCover(player, fleet, t.game.getFleets(player.Num))
			for _, member := range convoyMembers(player, fleet, t.game.getFleets(player.Num)) {
				if member.Spec.HasWeapons {
					defenders += patrolPower(member)
				}
			}
			role := player.Automation.FleetRoles[fleet.Num]
			canStand := role != "scout" && colonizationMode(role) == "" && defenders > threat*3/2
			if role == "minesweeper" || fleetHasDedicatedMinesweeper(player, fleet) {
				canStand = false
			}
			if controlledBombardment {
				canStand = defenseReady
			}
			contactRecon := landingScout(player, fleet) || scoutReconActive(player, fleet) && !readScoutRecon(fleet).Watching || colonyReconActive(player, fleet)
			intercepted := !contactRecon && !player.Automation.DisableInterceptionEvasion && threat > 0 && !canStand
			// A safe refit already replaces damaged ships; only danger may preempt it.
			needsRepair := !fleet.InDesignMaintenance() && !contactRecon && !player.Automation.DisableAutomaticRepair && damage >= player.Automation.repairThreshold()
			if active && ((job.Kind == "repair" && (damage == 0 || player.Automation.DisableAutomaticRepair)) || (job.Kind == "evade" && !intercepted)) {
				delete(fleet.Tags, safetyTag)
				delete(fleet.Tags, repairTankerTag)
				delete(fleet.Tags, fuelStatusTag)
				orders := job.Orders
				orders.Waypoints = slices.Clone(orders.Waypoints)
				if len(orders.Waypoints) > 0 {
					orders.Waypoints[0] = Waypoint{Position: fleet.Position}
				}
				fleet.FleetOrders = orders
				fleet.ComputeFuelUsage(player)
				active = false
				if defenseReady {
					holdAutomaticFleet(player, fleet, "Local DD protection assembled or arriving this turn")
				}
			}
			if !active && !needsRepair && !intercepted {
				continue
			}
			if !active {
				job.Orders = fleet.FleetOrders
				job.Orders.Waypoints = slices.Clone(fleet.Waypoints)
				job.Kind = "evade"
			}
			if needsRepair {
				job.Kind = "repair"
			}
			// A tanker under repair cannot promise delivery to its former customers.
			if tankerJob := readTankerJob(fleet); tankerJob != nil {
				t.releaseTankerCustomers(player, fleet, tankerJob)
				delete(fleet.Tags, tankerJobTag)
			}
			delete(fleet.Tags, tankerAwaitTag)
			delete(fleet.Tags, fuelAwaitResumeTag)
			delete(fleet.Tags, fuelRequestTag)
			if role != "bomber" && role != "tanker" || !intercepted {
				t.releaseEscorts(player, fleet)
			}
			if fleet.GetTag(escortTag) != "" {
				delete(fleet.Tags, escortTag)
			}
			var chosen *Planet
			var route *expeditionRoute
			for _, planet := range expeditionStations(player, fleet) {
				// Retreat must be permitted out of the currently threatened corridor.
				if intercepted && planet.Position == fleet.Position {
					continue
				}
				if knownRouteThreat(player, planet.Position, planet.Position) > 0 {
					continue
				}
				candidate := planAutomaticRoute(&t.game.Rules, player, []*Fleet{fleet}, planet.Position, planet.MapObject.ToTarget(), automaticRoutePolicy{AllowThreat: true, Fast: intercepted && civilianFleet(player, fleet), HighestWarp: intercepted && civilianFleet(player, fleet)})
				if candidate == nil {
					continue
				}
				if intercepted && len(candidate.Waypoints) > 1 {
					probe := *fleet
					probe.Waypoints = candidate.Waypoints
					if civilianFleet(player, fleet) && player.Automation.FleetRoles[fleet.Num] != "freighter" && player.patrolEvaluator != nil {
						wp := candidate.Waypoints[1]
						point := wp.Position
						if distance := fleet.Position.DistanceTo(point); distance > fleet.StrategicSpeed(wp.WarpSpeed) {
							point = fleet.Position.ToFloat64().Add(point.Subtract(fleet.Position).Normalized().Scale(fleet.StrategicSpeed(wp.WarpSpeed))).ToInt(true)
						}
						closest, ending, _ := reportedEscapeSeparation(player, fleet, point)
						if closest <= 3 || ending <= 3 {
							continue
						}
					}
					if interceptionThreat(player, &probe) > 0 || knownRouteThreat(player, fleet.Position, fleet.Position) > 0 && !knownWithdrawalRoute(player, fleet.Position, candidate.Waypoints[1].Position) {
						continue
					}
				}
				if route == nil || candidate.Turns < route.Turns || candidate.Turns == route.Turns && planet.Num < chosen.Num {
					chosen, route = planet, candidate
				}
			}
			delete(fleet.Tags, repairTankerTag)
			if needsRepair && !intercepted {
				if donor, mobile := t.nearestTankerRepair(player, fleet, route, chosen); donor != nil {
					fleet.SetTag(repairTankerTag, strconv.Itoa(donor.Num))
					applyExpeditionRoute(player, fleet, mobile)
					t.protectEvadingBomber(player, fleet)
					holdAutomaticFleet(player, donor, "Mobile repair service reserved for incoming friendly ships")
					job.Status = repairTankerStatus(donor)
					raw, _ := json.Marshal(job)
					fleet.SetTag(safetyTag, string(raw))
					fleet.SetTag(fuelStatusTag, job.Status)
					continue
				}
			}
			if route == nil && intercepted {
				if escape := t.planOpenSpaceEscape(player, fleet); escape != nil {
					applyExpeditionRoute(player, fleet, escape)
					t.protectEvadingBomber(player, fleet)
					job.Status = "Evading interception in open space; return fuel protected"
					raw, _ := json.Marshal(job)
					fleet.SetTag(safetyTag, string(raw))
					fleet.SetTag(fuelStatusTag, job.Status)
					continue
				}
			}
			if route == nil {
				holdAutomaticFleet(player, fleet, "Retreat waiting for a reachable repair dock")
				requestAutomaticRescue(player, fleet, true)
				job.Status = "Retreat needs tanker assistance"
				if role == "bomber" && player.patrolEvaluator != nil {
					job.Status = "No confirmed funded escape corridor; DD protection/reconnaissance and fuel assistance required"
				}
			} else {
				applyExpeditionRoute(player, fleet, route)
				t.protectEvadingBomber(player, fleet)
				job.Status = fmt.Sprintf("%s: whole fleet returning to %s (%d%% worst ship damage)", job.Kind, chosen.Name, damage)
				if chosen.Position == fleet.Position {
					holdAutomaticFleet(player, fleet, job.Status)
				}
			}
			raw, _ := json.Marshal(job)
			fleet.SetTag(safetyTag, string(raw))
			fleet.SetTag(fuelStatusTag, job.Status)
		}
	}
}

// Release old evasion before mission assignment, rather than wasting another
// year at the dock after the threat has passed. Final safety runs again after
// planning and validates the new course and any newly assigned escorts.
func (t *turnGenerator) resumeCompletedEvades() {
	for _, p := range t.game.Players {
		for _, f := range t.game.getFleets(p.Num) {
			var job automaticSafetyJob
			if !isAutomaticFleet(p, f) || f.InDesignMaintenance() && p.Automation.FleetRoles[f.Num] != "freighter" || json.Unmarshal([]byte(f.GetTag(safetyTag)), &job) != nil || job.Kind != "evade" {
				continue
			}
			threat := interceptionThreat(p, f)
			if p.Automation.FleetRoles[f.Num] == "bomber" || p.Automation.FleetRoles[f.Num] == "tanker" {
				threat = max(threat, bomberChaseThreat(p, f))
			}
			if threat > 0 && !p.Automation.DisableInterceptionEvasion {
				continue
			}
			delete(f.Tags, safetyTag)
			delete(f.Tags, repairTankerTag)
			delete(f.Tags, fuelStatusTag)
			f.FleetOrders = job.Orders
			f.Waypoints = slices.Clone(job.Orders.Waypoints)
			if len(f.Waypoints) > 0 {
				f.Waypoints[0] = Waypoint{Position: f.Position}
			}
			f.ComputeFuelUsage(p)
		}
	}
}
