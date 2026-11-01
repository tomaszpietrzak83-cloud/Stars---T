package cs

import (
	"encoding/json"
	"strings"
)

// Manual control removes resumable automatic jobs, restores an illegally
// borrowed manual mission, and preserves cargo, notes and committed building work.
func (p *Player) TakeManualControl(f *Fleet) {
	var borrowed bomberBorrow
	if json.Unmarshal([]byte(f.GetTag(bomberBorrowTag)), &borrowed) == nil && borrowed.Role == "" && p.Automation.FleetRoles[f.Num] == "escort" && len(borrowed.Orders.Waypoints) > 0 {
		f.FleetOrders = borrowed.Orders
		f.Waypoints[0] = Waypoint{Position: f.Position}
	}
	delete(p.Automation.FleetRoles, f.Num)
	delete(p.Automation.RebuildFleets, f.Num)
	for key := range f.Tags {
		if strings.HasPrefix(key, "automatic-") || strings.HasPrefix(key, "scout-") || strings.HasPrefix(key, "patrol-") || strings.HasPrefix(key, "bomber-") || strings.HasPrefix(key, "bombing-") || strings.HasPrefix(key, "escort-") || strings.HasPrefix(key, "fuel-network-") || strings.HasPrefix(key, "logistics-") || key == tankerAwaitTag || key == frontierPatrolTag {
			delete(f.Tags, key)
		}
	}
}

func (t *turnGenerator) restoreBorrowedManualFleets() {
	for _, p := range t.automaticPlayers() {
		for _, f := range t.game.getFleets(p.Num) {
			var borrowed bomberBorrow
			if !f.Delete && !f.InDesignMaintenance() && (p.Automation.FleetRoles[f.Num] == "escort" || p.Automation.FleetRoles[f.Num] == "") && json.Unmarshal([]byte(f.GetTag(bomberBorrowTag)), &borrowed) == nil && borrowed.Role == "" {
				p.TakeManualControl(f)
			}
		}
	}
}
