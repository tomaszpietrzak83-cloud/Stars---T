package cs

import (
	"math"
	"sort"
)

func invasionColony(player *Player, planet *Planet) bool {
	return invasionCandidate(player, planet) && !planet.Spec.HasStarbase && !reportedGroundDefenses(planet) && planet.Cargo.Colonists > 0
}

func reportedGroundDefenses(planet *Planet) bool {
	return planet.Defenses > 0 || planet.Spec.DefenseCoverage > 0 || planet.Spec.DefenseCoverageSmart > 0
}

func invasionCandidate(p *Player, planet *Planet) bool {
	if planet == nil || !planet.Explored() || !planet.Owned() || planet.PlayerNum == p.Num || p.IsFriend(planet.PlayerNum) {
		return false
	}
	// An occupation can use immediate instaforming, not future construction.
	copyPlayer := *p
	copyPlayer.Automation.ServeTerraformingWorlds = false
	quality := prospectivePlanetQuality(&copyPlayer, planet)
	return quality >= 50 || p.Automation.PriorityPlanets[planet.Num] && quality > 0
}
func (t *turnGenerator) findAutomaticInvasion(player *Player, fleet *Fleet) *logisticsJob {
	return t.findInvasionTransfer(player, fleet, false)
}

func (t *turnGenerator) findInvasionTransfer(player *Player, fleet *Fleet, priorityOnly bool) *logisticsJob {
	if player.Race.Spec.LivesOnStarbases || !automaticPassengersAllowed(player, fleet) {
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
		if priorityOnly && !priority || target.ReportAge > 2 {
			continue
		}
		staging := false
		if priorityNeedsBombing(target) {
			forecast := t.forecastLandingAfterBombing(player, target)
			if forecast == nil {
				continue
			}
			target = forecast
			staging = true
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
			job := &logisticsJob{BombingStage: staging, Kind: "invade-load", TargetNum: source.Num, DeliveryNum: target.Num, Position: source.Position, Passengers: load}
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
	if !automaticPassengersAllowed(player, fleet) {
		delete(fleet.Tags, logisticsTag)
		return
	}
	source := t.game.getPlanet(job.TargetNum)
	if source == nil || !source.OwnedBy(player.Num) || !invasionColony(player, player.GetPlanetIntel(job.DeliveryNum)) || !t.passengerTripSafe(player, fleet, job) {
		delete(fleet.Tags, logisticsTag)
		return
	}
	target := player.GetPlanetIntel(job.DeliveryNum)
	if priorityNeedsBombing(target) && (!job.BombingStage || t.forecastLandingAfterBombing(player, target) == nil) {
		if job.BombingStage {
			delete(fleet.Tags, logisticsTag)
			t.releaseEscorts(player, fleet)
			return
		}
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
	t.invasionDestination(player, fleet, job, player.GetPlanetIntel(job.DeliveryNum))
	job.Status = "Transporting colonists to reclaim a valuable colony"
	saveLogisticsWork(fleet, job)
}

// A priority landing carries both the estimated combat losses and a garrison.
// Quantities use the same 100-person units as cargo and native ground combat.
func (t *turnGenerator) priorityLandingRequirement(p *Player, planet *Planet, years int) int {
	defense := reportedInvasionDefense(&t.game.Rules)
	losses := int(math.Ceil(float64(planet.Cargo.Colonists) * defense * math.Pow(1.25, float64(planet.ReportAge+years)) / max(0.1, p.Race.Spec.InvasionAttackBonus)))
	garrison := max(50, populationTarget(planet, p.Automation.destinationPopulationPercent()))
	return losses + garrison
}

// PRT is not public planet intelligence. Budget the strongest ground-defense
// modifier supported by the public rules, without consulting the hidden owner.
func reportedInvasionDefense(rules *Rules) float64 {
	bonus := 1.5
	for _, spec := range rules.PRTSpecs {
		bonus = max(bonus, spec.InvasionDefendBonus)
	}
	return bonus
}

// Combine simultaneous transports before checking invasion strength. Priority
// targets also permit smaller attrition landings; later full transports reinforce.
func (t *turnGenerator) resolveAutomaticInvasions() {
	for _, player := range t.game.Players {
		arrivals := map[int][]*Fleet{}
		for _, fleet := range t.game.getFleets(player.Num) {
			job := logisticsWork(fleet)
			if job == nil || job.Kind != "invade" || job.BombingStage || fleet.Delete || fleet.InAutomaticSafety() || fleet.InDesignMaintenance() || player.Automation.FleetRoles[fleet.Num] != "freighter" || fleet.Position != job.Position || t.armedEnemyAt(player, fleet.Position) {
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
