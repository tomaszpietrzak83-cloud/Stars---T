package cs

import (
	"encoding/json"
	"slices"
)

const bomberTag = "bomber-orders"

type bomberOrders struct {
	Orders FleetOrders `json:"orders"`
	Status string      `json:"status"`
}

func (t *turnGenerator) planBomberEscorts() {
	for _, player := range t.game.Players {
		for _, fleet := range t.game.getFleets(player.Num) {
			if fleet.Delete || fleet.InDesignMaintenance() || fuelNetworkManaged(fleet) || fleet.InAutomaticSafety() || player.Automation.FleetRoles[fleet.Num] != "bomber" || fleet.GetTag(tankerAwaitTag) != "" {
				continue
			}
			var saved bomberOrders
			valid := json.Unmarshal([]byte(fleet.GetTag(bomberTag)), &saved) == nil
			if !valid {
				saved.Orders = fleet.FleetOrders
				saved.Orders.Waypoints = slices.Clone(fleet.Waypoints)
			}
			// New manual destinations replace the saved mission when they differ from the
			// controller's previous generated destination. Holding orders keep the mission.
			if valid && len(fleet.Waypoints) > 1 && len(saved.Orders.Waypoints) > 1 && fleet.Waypoints[len(fleet.Waypoints)-1].Position != saved.Orders.Waypoints[1].Position {
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
			for _, planet := range player.PlanetIntels {
				if planet != nil && planet.Owned() && !player.IsFriend(planet.PlayerNum) && planet.Position.DistanceTo(wp.Position) <= player.Automation.escortMarginAsFloat() {
					minimum = max(1, fleet.Spec.TotalShips)
					break
				}
			}
			job := &logisticsJob{Position: wp.Position, TargetNum: wp.TargetNum}
			t.prepareConvoyWithMinimum(player, fleet, job, wp.MapObjectTarget, minimum)
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
