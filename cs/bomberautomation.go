package cs

import (
	"encoding/json"
	"math"
	"slices"
)

const bomberTag = "bomber-orders"

type bomberOrders struct {
	Orders FleetOrders `json:"orders"`
	Status string      `json:"status"`
	Stage  string      `json:"stage,omitempty"`
}

func (t *turnGenerator) planBomberEscorts() {
	t.finishBombingPacks()
	for _, player := range t.game.Players {
		for _, fleet := range t.game.getFleets(player.Num) {
			if fleet.Delete || fleet.InDesignMaintenance() || fuelNetworkManaged(fleet) || fleet.InAutomaticSafety() || player.Automation.FleetRoles[fleet.Num] != "bomber" || fleet.GetTag(tankerAwaitTag) != "" {
				continue
			}
			if t.followBombingRally(player, fleet) {
				continue
			}
			var saved bomberOrders
			valid := json.Unmarshal([]byte(fleet.GetTag(bomberTag)), &saved) == nil
			if !valid {
				saved.Orders = fleet.FleetOrders
				saved.Orders.Waypoints = slices.Clone(fleet.Waypoints)
			}
			target := t.automaticBombingTarget(player, fleet)
			if fleet.GetTag("maintenance-pending") != "" {
				// Pending means finish the saved mission, not stop issuing its orders
				// forever. Do not start another mission after that target is finished.
				target = player.GetPlanetIntel(bomberGoal(fleet))
				if target == nil || !target.Owned() || target.PlayerNum == player.Num || player.IsFriend(target.PlayerNum) || !priorityNeedsBombing(target) && bombingDelegatedToLanding(&t.game.Rules, player, target, t.game.getFleets(player.Num)) {
					delete(fleet.Tags, bomberTag)
					t.releaseEscorts(player, fleet)
					holdAutomaticFleet(player, fleet, "Bombing mission completed; ready for pending rebuild")
					continue
				}
			}
			if target == nil && bomberAtLandingReadyPriority(&t.game.Rules, player, fleet, t.game.getFleets(player.Num)...) {
				var best *expeditionRoute
				for _, dock := range expeditionStations(player, fleet) {
					route := planAutomaticRoute(&t.game.Rules, player, []*Fleet{fleet}, dock.Position, dock.ToTarget(), automaticRoutePolicy{})
					if route != nil && (best == nil || route.Turns < best.Turns) {
						best = route
					}
				}
				if best != nil {
					applyExpeditionRoute(player, fleet, best)
					delete(fleet.Tags, bomberTag)
					t.releaseEscorts(player, fleet)
					continue
				}
			}
			if target != nil {
				saved.Orders.Waypoints = []Waypoint{{Position: fleet.Position}, {Position: target.Position, MapObjectTarget: target.ToTarget(), WarpSpeed: automaticMaxWarp(player, fleet)}}
				valid = false
				if target.Spec.HasStarbase && fleet.Position != target.Position {
					saved.Status = "Awaiting confirmed orbital base destruction before bombardment"
					raw, _ := json.Marshal(saved)
					fleet.SetTag(bomberTag, string(raw))
					t.releaseEscorts(player, fleet)
					holdAutomaticFleet(player, fleet, saved.Status)
					continue
				}
			}
			// New manual destinations replace the saved mission when they differ from the
			// controller's previous generated destination. Holding orders keep the mission.
			if valid && saved.Stage == "" && len(fleet.Waypoints) > 1 && len(saved.Orders.Waypoints) > 1 && fleet.Waypoints[len(fleet.Waypoints)-1].Position != saved.Orders.Waypoints[1].Position {
				saved.Orders = fleet.FleetOrders
				saved.Orders.Waypoints = slices.Clone(fleet.Waypoints)
			}
			if target != nil && fleet.Position == target.Position {
				// Arrival begins the bombardment, it does not complete the mission.
				// Keep its identity for pending refits and later landing handoff.
				saved.Status = "Bombarding the current target until the landing force is ready"
				raw, _ := json.Marshal(saved)
				fleet.SetTag(bomberTag, string(raw))
				holdAutomaticFleet(player, fleet, saved.Status)
				continue
			}
			for len(saved.Orders.Waypoints) > 1 && saved.Orders.Waypoints[1].Position == fleet.Position {
				saved.Orders.Waypoints = append(saved.Orders.Waypoints[:1], saved.Orders.Waypoints[2:]...)
			}
			if len(saved.Orders.Waypoints) < 2 {
				t.releaseEscorts(player, fleet)
				delete(fleet.Tags, bomberTag)
				continue
			}
			wp := saved.Orders.Waypoints[1]
			minimum := 0
			planet := player.GetPlanetIntel(wp.TargetNum)
			if !t.bomberUnescortedSafe(player, fleet, wp.Position) || planet != nil && packNeedsArmedOrbit(player, &bombingPack{Target: planet.Num}) {
				minimum = 1
			}

			job := &logisticsJob{Position: wp.Position, TargetNum: wp.TargetNum}
			t.prepareConvoyWithMinimum(player, fleet, job, wp.MapObjectTarget, minimum)
			if minimum > 0 && len(fleet.Waypoints) < 2 {
				t.assignBomberProtection(player, fleet, wp.Position, false)
				t.prepareConvoyWithMinimum(player, fleet, job, wp.MapObjectTarget, minimum)
			}
			saved.Stage = ""
			if len(fleet.Waypoints) < 2 && fleet.Position != wp.Position && fleet.Fuel < fleet.Spec.FuelCapacity {
				if t.bomberNearbyFuel(player, fleet) {
					saved.Stage = "refuel"
				} else {
					saved.Stage = "fuel-rendezvous"
				}
			}
			saved.Status = job.Status
			if len(fleet.Waypoints) > 1 {
				fleet.Waypoints[len(fleet.Waypoints)-1].Task = wp.Task
			}
			raw, _ := json.Marshal(saved)
			fleet.SetTag(bomberTag, string(raw))
		}
	}
}
func (a LogisticsAutomation) escortMarginAsFloat() float64 { return float64(a.escortMargin()) }

// Only targets visible in this player's reports; do not bomb a landing-ready
// colony merely because it remains marked for colonization or reinforcement.
func priorityBombingTarget(rules *Rules, p *Player, f *Fleet, landings ...*Fleet) *Planet {
	var best *Planet
	distance := math.Inf(1)
	for _, planet := range p.PlanetIntels {
		if planet == nil || !p.Automation.PriorityPlanets[planet.Num] || !planet.Explored() || !planet.Owned() || planet.PlayerNum == p.Num || p.IsFriend(planet.PlayerNum) || !priorityNeedsBombing(planet) && bombingDelegatedToLanding(rules, p, planet, landings) {
			continue
		}
		d := f.Position.DistanceTo(planet.Position)
		if d < distance || d == distance && (best == nil || planet.Num < best.Num) {
			best, distance = planet, d
		}
	}
	return best
}

func bomberAtLandingReadyPriority(rules *Rules, p *Player, f *Fleet, landings ...*Fleet) bool {
	for _, planet := range p.PlanetIntels {
		if planet != nil && p.Automation.PriorityPlanets[planet.Num] && planet.Position == f.Position && planet.Owned() && planet.PlayerNum != p.Num && !p.IsFriend(planet.PlayerNum) && !priorityNeedsBombing(planet) && bombingDelegatedToLanding(rules, p, planet, landings) {
			return true
		}
	}
	return false
}

// Old population reports cannot certify that a priority colony is still a tiny
// landing target. Refresh it with bombers/recon instead of ignoring it forever.
func priorityNeedsBombing(planet *Planet) bool {
	if planet == nil {
		return false
	}
	if planet.Spec.HasStarbase || reportedGroundDefenses(planet) || planet.Cargo.Colonists > 1000 {
		return true
	}
	return planet.ReportAge > 2 && float64(planet.Cargo.Colonists)*math.Pow(1.25, float64(min(30, planet.ReportAge))) > 1000
}

func bombingDelegatedToLanding(rules *Rules, p *Player, planet *Planet, fleets []*Fleet) bool {
	if planet == nil || planet.ReportAge > 2 || !invasionColony(p, planet) {
		return false
	}
	troops, arrival := 0, 0
	for _, f := range fleets {
		if f == nil || f.Delete || p.Automation.FleetRoles[f.Num] != "freighter" || f.InAutomaticSafety() || f.InDesignMaintenance() {
			continue
		}
		job := logisticsWork(f)
		if job == nil || job.DeliveryNum != planet.Num || job.Kind != "invade" || job.BombingStage || f.Cargo.Colonists <= 0 || f.Position != planet.Position && (len(f.Waypoints) < 2 || f.Waypoints[1].WarpSpeed == 0) {
			continue
		}
		group := convoyMembers(p, f, fleets)
		route := planAutomaticRoute(rules, p, group, planet.Position, planet.ToTarget(), automaticRoutePolicy{AllowThreat: len(group) > 1 || f.Spec.HasWeapons, MaxTurns: 2, MinWarp: automaticCruiseWarp(p, f)})
		if route == nil || route.WaitForFuel {
			continue
		}
		troops += f.Cargo.Colonists
		arrival = max(arrival, route.Turns)
	}
	need := int(math.Ceil(float64(planet.Cargo.Colonists)*reportedInvasionDefense(rules)*math.Pow(1.25, float64(planet.ReportAge+arrival))/max(.1, p.Race.Spec.InvasionAttackBonus))) + 5
	return troops >= need
}
