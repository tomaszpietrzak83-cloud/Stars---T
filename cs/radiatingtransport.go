package cs

import (
	"slices"
	"strconv"
)

// Radiation restrictions apply to automatic freighters regardless of racial
// immunity. Separate escort engines never affect the passenger carrier's flag.
func automaticPassengersAllowed(p *Player, f *Fleet) bool {
	return p.Automation.FleetRoles[f.Num] != "freighter" || !f.Spec.Radiating
}

func (t *turnGenerator) prepareRadiatingTransports() {
	for _, p := range t.automaticPlayers() {
		for _, f := range t.game.getFleets(p.Num) {
			if f.Delete || automaticPassengersAllowed(p, f) {
				continue
			}
			previous := logisticsWork(f)
			passengerJob := previous != nil && (previous.Kind == "passengers" || previous.Kind == "settle" || previous.Kind == "invade" || previous.Kind == "invade-load")
			if passengerJob || f.Cargo.Colonists > 0 && (previous == nil || previous.Kind != "radiation-return") {
				if n, err := strconv.Atoi(f.GetTag(tankerAwaitTag)); err == nil {
					if donor := t.game.getFleet(p.Num, n); donor != nil {
						if job := readTankerJob(donor); job != nil && slices.Contains(job.Members, f.Num) {
							t.releaseTankerCustomers(p, donor, job)
							delete(donor.Tags, tankerJobTag)
						}
					}
				}
				delete(f.Tags, tankerAwaitTag)
				delete(f.Tags, fuelAwaitResumeTag)
				delete(f.Tags, fuelRequestTag)
				delete(f.Tags, fuelResumeTag)
				leader := f
				if n, err := strconv.Atoi(f.GetTag(fuelNetworkMemberTag)); err == nil {
					if other := t.game.getFleet(p.Num, n); other != nil {
						leader = other
					}
				}
				if plan := readFuelNetworkPlan(leader); plan != nil && (plan.Leader == f.Num || slices.Contains(plan.Members, f.Num)) {
					t.closeFuelNetwork(p, leader, plan, true)
				}
				delete(f.Tags, fuelNetworkMemberTag)
			}
			for i := range f.Waypoints {
				f.Waypoints[i].TransportTasks.Colonists = WaypointTransportTask{}
			}
			job := logisticsWork(f)
			if job != nil && (job.Kind == "passengers" || job.Kind == "settle" || job.Kind == "invade-load" || job.Kind == "invade") {
				delete(f.Tags, logisticsTag)
				t.releaseEscorts(p, f)
			}
			if f.Cargo.Colonists == 0 {
				if job != nil && job.Kind == "radiation-return" {
					delete(f.Tags, logisticsTag)
				}
				continue
			}
			for _, colony := range t.game.getPlanets(p.Num) {
				if colony.Position == f.Position {
					colony.Cargo.Colonists += f.Cargo.Colonists
					f.Cargo.Colonists = 0
					delete(f.Tags, logisticsTag)
					break
				}
			}
			if f.Cargo.Colonists == 0 || f.InDesignMaintenance() || f.InAutomaticSafety() {
				continue
			}
			var best *expeditionRoute
			var destination *Planet
			for _, colony := range t.game.getPlanets(p.Num) {
				route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, colony.Position, colony.ToTarget(), automaticRoutePolicy{Fast: true, MaxTurns: 20})
				if route != nil && !route.WaitForFuel && (best == nil || route.Turns < best.Turns) {
					best = route
					destination = colony
				}
			}
			if best != nil {
				applyExpeditionRoute(p, f, best)
				saveLogisticsWork(f, &logisticsJob{Kind: "radiation-return", TargetNum: destination.Num, DeliveryNum: destination.Num, Position: destination.Position, Status: "Evacuating existing passengers; radiating freighters accept only non-passenger work"})
			} else {
				t.routeEmergencyReturn(p, f)
			}
		}
	}
}
