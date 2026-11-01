package cs

import (
	"math"
	"sort"
)

func (t *turnGenerator) availableFuelNetworkDonors(player *Player, group []*Fleet) []*Fleet {
	donors := []*Fleet{}
	for _, donor := range t.game.getFleets(player.Num) {
		if donor.Delete || donor.Starbase || player.Automation.FleetRoles[donor.Num] != "tanker" || donor.InAutomaticSafety() || donor.InDesignMaintenance() || fuelNetworkReserved(donor) {
			continue
		}
		member := false
		for _, fleet := range group {
			member = member || fleet.Num == donor.Num
		}
		if member {
			continue
		}
		job := readTankerJob(donor)
		if job != nil && job.Stage != "patrol" {
			continue
		}
		donors = append(donors, donor)
	}
	return donors
}

func (t *turnGenerator) assistedColonyTarget(player *Player, fleet *Fleet, mode string, claimed map[int]int) (*Planet, *expeditionRoute) {
	if len(t.availableFuelNetworkDonors(player, []*Fleet{fleet})) == 0 {
		return nil, nil
	}
	type candidate struct {
		planet *Planet
		score  float64
	}
	candidates := []candidate{}
	maximum := automaticMaxWarp(player, fleet)
	if maximum == 0 {
		return nil, nil
	}
	for _, planet := range player.PlanetIntels {
		quality := colonyPlanetQuality(player, planet)
		if quality <= 0 || claimed[planet.Num] != 0 {
			continue
		}
		distance := fleet.Position.DistanceTo(planet.Position)
		turns := int(math.Ceil(distance / fleet.StrategicSpeed(maximum)))
		if turns > 50 {
			continue
		}
		score := colonyAutoScore(quality, turns)
		switch mode {
		case "nearest":
			score = -distance
		case "farthest":
			score = distance
		case "best":
			score = float64(quality)
		}
		candidates = append(candidates, candidate{planet, score})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].planet.Num < candidates[j].planet.Num
		}
		return candidates[i].score > candidates[j].score
	})
	var chosen *Planet
	var route *expeditionRoute
	best := -math.MaxFloat64
	for _, candidate := range candidates[:min(32, len(candidates))] {
		planet := candidate.planet
		planned := t.planAutomaticMission(player, []*Fleet{fleet}, planet.Position, planet.MapObject.ToTarget(), automaticRoutePolicy{OneWay: true, MaxTurns: 50})
		if planned == nil {
			continue
		}
		score := candidate.score
		if mode == "auto" {
			score = colonyAutoScore(colonyPlanetQuality(player, planet), planned.Turns)
		}
		if chosen == nil || score > best || score == best && planned.Turns < route.Turns {
			chosen, route, best = planet, planned, score
		}
	}
	return chosen, route
}

// Forecast mobile refuelling without spending or reserving fuel while comparing
// mission targets. Applying the chosen forecast requests an atomic reservation;
// the final departure guard prevents flight if another mission took the stock.
func (t *turnGenerator) planAutomaticMission(player *Player, group []*Fleet, position Vector, target MapObjectTarget, policy automaticRoutePolicy) *expeditionRoute {
	basic := planAutomaticRoute(&t.game.Rules, player, group, position, target, policy)
	if len(group) == 0 {
		return basic
	}
	maximum := 10
	for _, fleet := range group {
		maximum = min(maximum, automaticMaxWarp(player, fleet))
	}
	if maximum > 0 && basic != nil && basic.Turns <= max(1, int(math.Ceil(group[0].Position.DistanceTo(position)/group[0].StrategicSpeed(maximum)))) {
		return basic
	}
	if basic != nil && !fuelSupportForRole(player, group[0]) {
		return basic
	}
	donors := t.availableFuelNetworkDonors(player, group)
	if len(donors) == 0 {
		return basic
	}
	network := planFuelNetworkRoute(&t.game.Rules, player, group, donors, position, target, policy)
	if network == nil || basic != nil && network.Turns >= basic.Turns {
		return basic
	}
	request := &automaticFuelRequest{Destination: position, Target: target, Members: fleetNumbers(group), Urgent: basic == nil, OneWay: policy.OneWay, AllowThreat: policy.AllowThreat}
	if basic != nil {
		request.BaselineTurns = basic.Turns
	}
	return &expeditionRoute{Waypoints: []Waypoint{{Position: group[0].Position}, {Position: position, MapObjectTarget: target, WarpSpeed: max(1, maximum)}}, Turns: network.Turns, Fuel: network.Burned, ArrivalFuel: network.ArrivalFuel, NetworkRequest: request}
}
