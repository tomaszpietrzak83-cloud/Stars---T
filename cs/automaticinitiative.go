package cs

import (
	"fmt"
	"slices"
)

// Armed patrols and free escorts accept reported proximity risk. Actual
// interception evasion, engine limits and mine collisions remain separate checks.
func combatAutonomy(p *Player, f *Fleet) bool {
	role := p.Automation.FleetRoles[f.Num]
	return f.Spec.HasWeapons && (role == "patrol" || role == "escort")
}

func atOwnFuelDock(p *Player, f *Fleet) bool {
	for _, dock := range expeditionStations(p, f) {
		if dock.Position == f.Position {
			return true
		}
	}
	return false
}

// Final recovery pass: a failed mission is not an instruction to wait forever
// in empty space. Never invent fuel, or interrupt real mining/terraforming work.
func (t *turnGenerator) recoverIdleAutomaticFleets() {
	for _, p := range t.automaticPlayers() {
		for _, f := range t.game.getFleets(p.Num) {
			if !isAutomaticFleet(p, f) || f.Delete || f.Starbase || len(f.Waypoints) > 1 || atOwnFuelDock(p, f) || fuelNetworkManaged(f) || f.GetTag(tankerAwaitTag) != "" {
				continue
			}
			waiting := p.Automation.FleetRoles[f.Num] == "patrol" && f.GetTag(patrolMineSweepTag) != ""
			if job := readTankerJob(f); job != nil && job.TargetNum > 0 {
				for _, m := range t.tankerMembers(p, job) {
					waiting = waiting || m.Position != f.Position && len(m.Waypoints) > 1
				}
			}
			if job := readPatrolJob(f); job.Leader == f.Num && job.Leader != 0 {
				for _, m := range t.game.getFleets(p.Num) {
					waiting = waiting || m != f && readPatrolJob(m).Leader == f.Num && len(m.Waypoints) > 1
				}
			}
			if target := f.GetTag(escortTag); target != "" {
				for _, m := range t.game.getFleets(p.Num) {
					waiting = waiting || target == fmt.Sprint(m.Num) && m.Position == f.Position && !m.Delete
				}
			}
			if waiting {
				continue
			}
			role := p.Automation.FleetRoles[f.Num]
			if role != "scout" && role != "patrol" && role != "escort" && role != "freighter" && !f.InDesignMaintenance() && !f.InAutomaticSafety() {
				continue
			}
			if tag := f.GetTag(repairTankerTag); tag != "" && f.InAutomaticSafety() {
				repairing := false
				for _, donor := range t.game.getFleets(p.Num) {
					repairing = repairing || !donor.Delete && fmt.Sprint(donor.Num) == tag && donor.Position == f.Position
				}
				if repairing {
					continue
				}
			}
			legitimate := false
			for _, planet := range t.game.getPlanets(p.Num) {
				legitimate = legitimate || planet.Position == f.Position && (role == "freighter" || colonizationMode(role) != "")
			}
			if legitimate || role == "miner" && f.OrbitingPlanetNum != 0 || role == "terraformer" && f.OrbitingPlanetNum != 0 {
				continue
			}
			if len(f.Waypoints) > 0 && f.Waypoints[0].Task != WaypointTaskNone {
				continue
			}
			if t.routeEmergencyReturn(p, f) {
				f.SetTag(fuelStatusTag, "Returning toward an owned fuel dock; nearby patrols and escorts may refuel this ship en route")
			}
		}
	}
}

// Ships already together exchange genuinely spare fuel without dedicating a
// donor to an avoidable long rendezvous. A donor retains its own return reserve.
func (t *turnGenerator) shareOpportunisticFuel() {
	for _, p := range t.automaticPlayers() {
		fleets := slices.Clone(t.game.getFleets(p.Num))
		slices.SortFunc(fleets, func(a, b *Fleet) int { return a.Num - b.Num })
		for _, donor := range fleets {
			if !combatAutonomy(p, donor) || donor.Delete || fuelNetworkReserved(donor) || readTankerJob(donor) != nil || donor.GetTag(escortTag) != "" {
				continue
			}
			reserve := automaticReturnFuel(&t.game.Rules, p, donor, donor.Position, expeditionStations(p, donor), true)
			reserve = max(reserve, donor.Spec.FuelCapacity/4)
			if len(donor.Waypoints) > 1 {
				wp := donor.Waypoints[1]
				end := automaticReturnFuel(&t.game.Rules, p, donor, wp.Position, expeditionStations(p, donor), true)
				required, _, _, _ := automaticLegFuel(p, donor, wp.WarpSpeed, donor.Position.DistanceTo(wp.Position), donor.Spec.FuelCapacity, end)
				reserve = max(reserve, required)
			}
			for _, client := range fleets {
				if client == donor || client.Delete || client.Starbase || client.Position != donor.Position || !isAutomaticFleet(p, client) || client.Fuel*2 >= client.Spec.FuelCapacity || fuelNetworkReserved(client) {
					continue
				}
				amount := min(max(0, donor.Fuel-reserve), max(0, client.Spec.FuelCapacity/2-client.Fuel))
				if amount <= 0 {
					continue
				}
				donor.Fuel -= amount
				client.Fuel += amount
				client.SetTag(fuelStatusTag, "Received spare fuel from nearby "+donor.Name+"; continuing automatic orders")
			}
		}
	}
}

func (t *turnGenerator) withdrawEmptyFreighter(p *Player, f *Fleet, job *logisticsJob) bool {
	if f.CargoMass() != 0 || atOwnFuelDock(p, f) {
		return false
	}
	for _, planet := range t.game.getPlanets(p.Num) {
		if planet.Position == f.Position {
			return false
		}
	}
	if !t.routeEmergencyReturn(p, f) {
		return false
	}
	t.releaseEscorts(p, f)
	last := f.Waypoints[len(f.Waypoints)-1]
	*job = logisticsJob{Kind: "return", Position: last.Position, TargetNum: last.TargetNum, DeliveryNum: last.TargetNum, Status: "Empty transport returning to friendly fuel territory instead of waiting for an unavailable escort"}
	return true
}
