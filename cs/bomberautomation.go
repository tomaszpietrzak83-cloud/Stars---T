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
			if fleet.GetTag("maintenance-pending") != "" || fleet.Delete || fleet.InDesignMaintenance() || fuelNetworkManaged(fleet) || fleet.InAutomaticSafety() || player.Automation.FleetRoles[fleet.Num] != "bomber" || fleet.GetTag(tankerAwaitTag) != "" {
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
			if target == nil && bomberAtLandingReadyPriority(player, fleet) {
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
			}
			// New manual destinations replace the saved mission when they differ from the
			// controller's previous generated destination. Holding orders keep the mission.
			if valid && saved.Stage == "" && len(fleet.Waypoints) > 1 && len(saved.Orders.Waypoints) > 1 && fleet.Waypoints[len(fleet.Waypoints)-1].Position != saved.Orders.Waypoints[1].Position {
				saved.Orders = fleet.FleetOrders
				saved.Orders.Waypoints = slices.Clone(fleet.Waypoints)
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
			if !t.bomberUnescortedSafe(player, fleet, wp.Position) || planet != nil && planet.Spec.HasStarbase {
				minimum = max(1, fleet.Spec.TotalShips)
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
func priorityBombingTarget(p *Player, f *Fleet) *Planet {
	var best *Planet
	distance := math.Inf(1)
	for _, planet := range p.PlanetIntels {
		if planet == nil || !p.Automation.PriorityPlanets[planet.Num] || !planet.Explored() || !planet.Owned() || planet.PlayerNum == p.Num || p.IsFriend(planet.PlayerNum) || (planet.Cargo.Colonists <= 1000 && planet.Defenses == 0 && !planet.Spec.HasStarbase) {
			continue
		}
		d := f.Position.DistanceTo(planet.Position)
		if d < distance || d == distance && (best == nil || planet.Num < best.Num) {
			best, distance = planet, d
		}
	}
	return best
}

func bomberAtLandingReadyPriority(p *Player, f *Fleet) bool {
	for _, planet := range p.PlanetIntels {
		if planet != nil && p.Automation.PriorityPlanets[planet.Num] && planet.Position == f.Position && planet.Owned() && planet.PlayerNum != p.Num && !p.IsFriend(planet.PlayerNum) && planet.Cargo.Colonists <= 1000 && planet.Defenses == 0 && !planet.Spec.HasStarbase {
			return true
		}
	}
	return false
}
