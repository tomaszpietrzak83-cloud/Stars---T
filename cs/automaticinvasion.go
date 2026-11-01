package cs

import (
	"math"
	"sort"
)

func invasionColony(player *Player, planet *Planet) bool {
	return planet != nil && planet.Explored() && planet.Owned() && planet.PlayerNum != player.Num && !player.IsFriend(planet.PlayerNum) && !planet.Spec.HasStarbase && planet.Defenses == 0 && planet.Cargo.Colonists > 0 && prospectivePlanetQuality(player, planet) >= 50
}
func (t *turnGenerator) findAutomaticInvasion(player *Player, fleet *Fleet) *logisticsJob {
	if player.Race.Spec.LivesOnStarbases {
		return nil
	}
	workers, active := 0, 0
	claimed := map[int]bool{}
	reserved := map[int]int{}
	for _, f := range t.game.getFleets(player.Num) {
		if !f.Delete && player.Automation.FleetRoles[f.Num] == "freighter" {
			workers++
		}
		if job := logisticsWork(f); job != nil && (job.Kind == "invade" || job.Kind == "invade-load") {
			active++
			claimed[job.DeliveryNum] = true
			reserved[job.DeliveryNum] += f.Cargo.Colonists
			if job.Kind == "invade-load" {
				reserved[job.DeliveryNum] += job.Passengers
			}
		}
	}

	var chosen *logisticsJob
	best := math.Inf(1)
	for _, target := range player.PlanetIntels {
		if target == nil {
			continue
		}
		priority := player.Automation.PriorityPlanets[target.Num]
		// Bombers soften dense priority colonies before new transports depart.
		if priority && priorityNeedsBombing(target) {
			continue
		}
		if !invasionColony(player, target) || !priority && target.ReportAge > 2 || !priority && (claimed[target.Num] || active >= max(1, (workers+2)/3)) {
			continue
		}
		virtual := *fleet
		for _, source := range t.game.getPlanets(player.Num) {
			if fleet.Cargo.Colonists > 0 && source.Position != fleet.Position {
				continue
			}
			load := min(fleet.availableColonistSpace(), max(0, source.Cargo.Colonists-sourcePopulationReserve(player, source)))
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
			if priority {
				need = t.priorityLandingRequirement(player, target, route.Turns)
				if reserved[target.Num] >= need {
					continue
				}
			}
			if troops < need && !priority {
				continue
			}
			score := (1 + fleet.Position.DistanceTo(source.Position) + source.Position.DistanceTo(target.Position)) / math.Pow(float64(prospectivePlanetQuality(player, target))/100, 2)
			if priority {
				score /= 1e9
			}
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
	target := player.GetPlanetIntel(job.DeliveryNum)
	if player.Automation.PriorityPlanets[target.Num] && priorityNeedsBombing(target) {
		job.Status = "Waiting empty for bombardment or fresh reconnaissance of the priority target"
		holdAutomaticFleet(player, fleet, job.Status)
		saveLogisticsWork(fleet, job)
		return
	}
	load := min(fleet.availableColonistSpace(), max(0, source.Cargo.Colonists-sourcePopulationReserve(player, source)))
	if load < fleet.availableColonistSpace() {
		job.Status = "Waiting empty at source for a full invasion transport"
		holdAutomaticFleet(player, fleet, job.Status)
		saveLogisticsWork(fleet, job)
		return
	}
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

// A priority landing carries both the estimated combat losses and a garrison.
// Quantities use the same 100-person units as cargo and native ground combat.
func (t *turnGenerator) priorityLandingRequirement(p *Player, planet *Planet, years int) int {
	defender := t.game.getPlayer(planet.PlayerNum)
	defense := 1.0
	if defender != nil {
		defense = defender.Race.Spec.InvasionDefendBonus
	}
	losses := int(math.Ceil(float64(planet.Cargo.Colonists) * defense * math.Pow(1.25, float64(planet.ReportAge+years)) / max(0.1, p.Race.Spec.InvasionAttackBonus)))
	garrison := max(50, populationTarget(planet, p.Automation.destinationPopulationPercent()))
	return losses + garrison
}

// Combine simultaneous transports before checking invasion strength. Priority
// targets also permit smaller attrition landings; later full transports reinforce.
func (t *turnGenerator) resolveAutomaticInvasions() {
	for _, player := range t.game.Players {
		arrivals := map[int][]*Fleet{}
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
			arrivals[planet.Num] = append(arrivals[planet.Num], fleet)
		}
		keys := make([]int, 0, len(arrivals))
		for num := range arrivals {
			keys = append(keys, num)
		}
		sort.Ints(keys)
		for _, num := range keys {
			fleets := arrivals[num]
			planet := t.game.getPlanet(num)
			defender := t.game.getPlayer(planet.PlayerNum)
			if player.Race.Spec.LivesOnStarbases || defender.Race.Spec.LivesOnStarbases {
				continue
			}
			troops := 0
			for _, f := range fleets {
				troops += f.Cargo.Colonists
			}

			t.colonistDrops.addInvasion(invasion{planet: planet, defender: defender, attacker: player, attackers: troops * 100, fleets: fleets})
			for _, f := range fleets {
				f.Cargo.Colonists = 0
				delete(f.Tags, logisticsTag)
				t.releaseEscorts(player, f)
			}
		}
	}
}
