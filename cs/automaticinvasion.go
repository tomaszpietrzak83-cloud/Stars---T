package cs

import "math"

func invasionColony(player *Player, planet *Planet) bool {
	return planet != nil && planet.Explored() && planet.Owned() && planet.PlayerNum != player.Num && !player.IsFriend(planet.PlayerNum) && !planet.Spec.HasStarbase && planet.Defenses == 0 && planet.Cargo.Colonists > 0 && prospectivePlanetQuality(player, planet) >= 50
}
func (t *turnGenerator) findAutomaticInvasion(player *Player, fleet *Fleet) *logisticsJob {
	if player.Race.Spec.LivesOnStarbases {
		return nil
	}
	workers, active := 0, 0
	claimed := map[int]bool{}
	for _, f := range t.game.getFleets(player.Num) {
		if !f.Delete && player.Automation.FleetRoles[f.Num] == "freighter" {
			workers++
		}
		if job := logisticsWork(f); job != nil && (job.Kind == "invade" || job.Kind == "invade-load") {
			active++
			claimed[job.DeliveryNum] = true
		}
	}
	if active >= max(1, (workers+2)/3) {
		return nil
	}
	var chosen *logisticsJob
	best := math.Inf(1)
	for _, target := range player.PlanetIntels {
		if !invasionColony(player, target) || target.ReportAge > 2 || claimed[target.Num] {
			continue
		}
		virtual := *fleet
		for _, source := range t.game.getPlanets(player.Num) {
			if fleet.Cargo.Colonists > 0 && source.Position != fleet.Position {
				continue
			}
			load := min(fleet.availableColonistSpace(), max(0, source.Cargo.Colonists-populationTarget(source, player.Automation.sourcePopulationPercent())))
			if fleet.Cargo.Colonists > 0 {
				load = 0
			}
			troops := fleet.Cargo.Colonists + load
			if troops <= 0 {
				continue
			}
			job := &logisticsJob{Kind: "invade-load", TargetNum: source.Num, DeliveryNum: target.Num, Position: source.Position, Passengers: load}
			if !t.passengerTripSafe(player, fleet, job) {
				continue
			}
			virtual.Position = source.Position
			virtual.Fuel = fleet.Spec.FuelCapacity
			virtual.Cargo.Colonists = troops
			route := planAutomaticRoute(&t.game.Rules, player, []*Fleet{&virtual}, target.Position, target.MapObject.ToTarget(), automaticRoutePolicy{MinWarp: automaticCruiseWarp(player, fleet), MaxTurns: 8})
			if route == nil {
				continue
			}
			need := int(math.Ceil(float64(target.Cargo.Colonists)*1.5*math.Pow(1.25, float64(target.ReportAge+route.Turns))/max(0.1, player.Race.Spec.InvasionAttackBonus))) + 5
			if troops < need {
				continue
			}
			score := (1 + fleet.Position.DistanceTo(source.Position) + source.Position.DistanceTo(target.Position)) / math.Pow(float64(prospectivePlanetQuality(player, target))/100, 2)
			if score < best {
				best, chosen = score, job
				if fleet.Cargo.Colonists > 0 {
					job.Kind = "invade"
					job.TargetNum = target.Num
					job.Position = target.Position
				}
			}
		}
	}
	return chosen
}

func (t *turnGenerator) loadAutomaticInvaders(player *Player, fleet *Fleet, job *logisticsJob) {
	source := t.game.getPlanet(job.TargetNum)
	if source == nil || !source.OwnedBy(player.Num) || !invasionColony(player, player.GetPlanetIntel(job.DeliveryNum)) || !t.passengerTripSafe(player, fleet, job) {
		delete(fleet.Tags, logisticsTag)
		return
	}
	load := min(job.Passengers, fleet.availableColonistSpace(), max(0, source.Cargo.Colonists-populationTarget(source, player.Automation.sourcePopulationPercent())))
	if load <= 0 {
		delete(fleet.Tags, logisticsTag)
		return
	}
	source.Cargo.Colonists -= load
	fleet.Cargo.Colonists += load
	job.Kind = "invade"
	job.TargetNum = job.DeliveryNum
	job.Position = player.GetPlanetIntel(job.DeliveryNum).Position
	job.Status = "Transporting colonists to reclaim a valuable colony"
	saveLogisticsWork(fleet, job)
}

// Queue normal ground combat in the same phase as manual colonist drops.
func (t *turnGenerator) resolveAutomaticInvasions() {
	for _, player := range t.game.Players {
		for _, fleet := range t.game.getFleets(player.Num) {
			job := logisticsWork(fleet)
			if job == nil || job.Kind != "invade" || fleet.Delete || fleet.InAutomaticSafety() || fleet.InDesignMaintenance() || player.Automation.FleetRoles[fleet.Num] != "freighter" || fleet.Position != job.Position || t.armedEnemyAt(player, fleet.Position) {
				continue
			}
			planet := t.game.getPlanet(job.TargetNum)
			if planet != nil && planet.OwnedBy(player.Num) {
				job.Kind = "settle"
				saveLogisticsWork(fleet, job)
				continue
			}
			if !invasionColony(player, planet) {
				delete(fleet.Tags, logisticsTag)
				t.releaseEscorts(player, fleet)
				continue
			}
			defender := t.game.getPlayer(planet.PlayerNum)
			if player.Race.Spec.LivesOnStarbases || float64(fleet.Cargo.Colonists)*player.Race.Spec.InvasionAttackBonus < float64(planet.Cargo.Colonists)*defender.Race.Spec.InvasionDefendBonus*1.5 {
				delete(fleet.Tags, logisticsTag)
				t.releaseEscorts(player, fleet)
				continue
			}
			t.colonistDrops.addInvasion(invasion{planet: planet, defender: defender, attacker: player, attackers: fleet.Cargo.Colonists * 100, fleets: []*Fleet{fleet}})
			fleet.Cargo.Colonists = 0
			delete(fleet.Tags, logisticsTag)
			t.releaseEscorts(player, fleet)
		}
	}
}
